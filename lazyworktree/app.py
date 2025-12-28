import asyncio
import json
import os
import subprocess
import sys
import webbrowser
import shutil
import re
import yaml
from typing import List, Optional, Iterable

from textual import on, work, events
from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.css.query import NoMatches
from textual.command import DiscoveryHit, Hit, Provider
from textual.containers import Container, Horizontal, Vertical
from textual.widgets import (
    DataTable,
    Footer,
    Header,
    Input,
    RichLog,
)
from rich.panel import Panel
from rich.text import Text
from rich.table import Table
from rich.console import Group
from rich.syntax import Syntax

from .models import (
    PRInfo,
    WorktreeInfo,
    WORKTREE_DIR,
    LAST_SELECTED_FILENAME,
    CACHE_FILENAME,
)
from .screens import (
    ConfirmScreen,
    InputScreen,
    HelpScreen,
    DiffScreen,
    CommitScreen,
    FocusableRichLog,
)

class GitWtStatusCommands(Provider):
    """Command provider for Git Worktree Status actions."""
    COMMANDS = [
        ("Jump to worktree", "jump", "Jump to selected worktree"),
        ("Create worktree", "create", "Create a new worktree"),
        ("Delete worktree", "delete", "Delete selected worktree"),
        ("Absorb worktree", "absorb", "Merge worktree to main and delete it"),
        ("View diff", "diff", "View full diff of changes"),
        ("Fetch remotes", "fetch", "Fetch all remotes"),
        ("Fetch PR status", "fetch_prs", "Fetch PR information from GitHub"),
        ("Refresh list", "refresh", "Refresh worktree list"),
        ("Sort worktrees", "sort", "Toggle sort by Name/Last Active"),
        ("Filter worktrees", "filter", "Filter worktrees by name/branch"),
        ("Open LazyGit", "lazygit", "Open LazyGit for selected worktree"),
        ("Open PR", "open_pr", "Open PR in browser"),
        ("Show help", "help", "Show help screen"),
    ]
    def _make_callback(self, action_name: str):
        def callback():
            action = getattr(self.app, f"action_{action_name}", None)
            if action is None:
                return
            result = action()
            if asyncio.iscoroutine(result):
                asyncio.create_task(result)
        return callback
    async def discover(self):
        for name, action, help_text in self.COMMANDS:
            yield DiscoveryHit(name, self._make_callback(action), help=help_text)
    async def search(self, query: str):
        matcher = self.matcher(query)
        for name, action, help_text in self.COMMANDS:
            match = matcher.match(name)
            if match > 0:
                yield Hit(match, matcher.highlight(name), self._make_callback(action), help=help_text)

class GitWtStatus(App):
    TITLE = "Git Worktree Status"
    COMMANDS = {GitWtStatusCommands}
    CSS = """
    #main-content { height: 1fr; }
    #right-pane { width: 2fr; height: 100%; }
    #worktree-table { width: 3fr; height: 100%; border: solid $secondary; }
    #status-pane { width: 1fr; background: $surface-darken-1; padding: 0 1; border: solid $secondary; }
    #log-pane { width: 1fr; background: $surface-darken-1; padding: 0 1; border: solid $secondary; }
    #status-pane { height: 2fr; }
    #log-pane { height: 1fr; }
    #worktree-table.compact { width: 1fr; }
    #right-pane.expanded { width: 3fr; }
    .focused { border: solid $primary; }
    #filter-container { height: 3; dock: top; display: none; }
    .dirty { color: yellow; }
    .clean { color: green; }
    .ahead { color: cyan; }
    .behind { color: red; }
    """
    BINDINGS = [
        Binding("q", "quit", "Quit"),
        Binding("ctrl+q", "quit", "Quit", show=False),
        Binding("ctrl+c", "quit", "Quit", show=False),
        Binding("1", "focus_worktree", "Worktrees", show=False),
        Binding("2", "focus_status", "Info/Diff", show=False),
        Binding("3", "focus_log", "Log", show=False),
        Binding("j", "cursor_down", "Down", show=False),
        Binding("k", "cursor_up", "Up", show=False),
        Binding("J", "scroll_details_down", "Scroll Down", show=False),
        Binding("K", "scroll_details_up", "Scroll Up", show=False),
        Binding("ctrl+d", "scroll_details_down", "Scroll Down", show=False),
        Binding("ctrl+u", "scroll_details_up", "Scroll Up", show=False),
        Binding("ctrl+n", "cursor_down", "Down", show=False),
        Binding("ctrl+p", "cursor_up", "Up", show=False),
        Binding("up", "cursor_up", "Up", show=False),
        Binding("down", "cursor_down", "Down", show=False),
        Binding("o", "open_pr", description="Open PR", show=False),
        Binding("g", "lazygit", description="LazyGit", priority=True),
        Binding("r", "refresh", "Refresh"),
        Binding("f", "fetch", "Fetch", show=False),
        Binding("p", "fetch_prs", "Fetch PRs", show=False),
        Binding("c", "create", "Create", show=False),
        Binding("d", "diff", "Diff"),
        Binding("D", "delete", "Delete"),
        Binding("s", "sort", "Sort", show=False),
        Binding("/", "filter", "Filter"),
        Binding("?", "help", "Help"),
        Binding("enter", "jump", "Jump"),
        Binding("ctrl+slash", "command_palette", "Commands Palette"),
        Binding("tab", "cycle_focus", "Next Pane", show=False, priority=True),
    ]

    worktrees: List[WorktreeInfo] = []
    sort_by_active: bool = True
    filter_query: str = ""
    _main_branch: Optional[str] = None
    _pr_data_loaded: bool = False
    repo_name: str = ""

    def __init__(self, initial_filter: str = ""):
        super().__init__()
        self._initial_filter = initial_filter
        self._semaphore = asyncio.Semaphore(24)
        self._repo_key: Optional[str] = None
        self._cache: dict = {}
        self._divergence_cache: dict = {}
        self._notified_errors: set[str] = set()

    def _notify_once(self, key: str, message: str, severity: str = "error") -> None:
        if key in self._notified_errors:
            return
        self._notified_errors.add(key)
        self.notify(message, severity=severity)

    def compose(self) -> ComposeResult:
        yield Header()
        with Container(id="filter-container"):
            yield Input(placeholder="Filter worktrees...", id="filter-input")
        with Horizontal(id="main-content"):
            yield DataTable(id="worktree-table", cursor_type="row")
            with Vertical(id="right-pane"):
                yield FocusableRichLog(id="status-pane", wrap=True, markup=True, auto_scroll=False)
                yield DataTable(id="log-pane", cursor_type="row")
        yield Footer()

    def on_mount(self) -> None:
        table = self.query_one("#worktree-table", DataTable)
        table.border_title = "[bold cyan][1][/] [bold white]Worktrees[/]"
        table.add_columns("Worktree", "Status", "±", "PR", "Last Active")
        table.focus()
        self._set_focused_pane(table)
        self.query_one("#status-pane", RichLog).border_title = "[bold cyan][2][/] [bold white]Info/Diff[/]"
        log_table = self.query_one("#log-pane", DataTable)
        log_table.border_title = "[bold cyan][3][/] [bold white]Log[/]"
        log_table.add_columns("SHA", "Message")
        log_table.show_header = False
        if self._initial_filter:
            self.filter_query = self._initial_filter
            self.query_one("#filter-container").styles.display = "block"
            self.query_one("#filter-input", Input).value = self._initial_filter
        self.refresh_data()

    def action_focus_worktree(self) -> None:
        pane = self.query_one("#worktree-table", DataTable)
        pane.focus()
        self._set_focused_pane(pane)

    def action_focus_status(self) -> None:
        pane = self.query_one("#status-pane", RichLog)
        pane.focus()
        self._set_focused_pane(pane)

    def action_focus_log(self) -> None:
        pane = self.query_one("#log-pane", DataTable)
        pane.focus()
        self._set_focused_pane(pane)

    def _focus_widgets(self):
        return [self.query_one("#worktree-table", DataTable), self.query_one("#status-pane", RichLog), self.query_one("#log-pane", DataTable)]

    def _set_focused_pane(self, widget) -> None:
        for pane in self._focus_widgets(): pane.remove_class("focused")
        widget.add_class("focused")
        self._apply_focus_layout(getattr(widget, "id", "") in {"status-pane", "log-pane"})

    def _apply_focus_layout(self, right_focused: bool) -> None:
        table = self.query_one("#worktree-table", DataTable)
        right_pane = self.query_one("#right-pane", Vertical)
        if right_focused:
            table.add_class("compact")
            right_pane.add_class("expanded")
        else:
            table.remove_class("compact")
            right_pane.remove_class("expanded")

    def _selected_worktree_path(self) -> Optional[str]:
        table = self.query_one("#worktree-table", DataTable)
        if table.row_count == 0 or table.cursor_row is None: return None
        row_key = table.coordinate_to_cell_key((table.cursor_row, 0)).row_key
        return str(row_key.value)

    def _try_query_one(self, selector, expect_type):
        try:
            return self.query_one(selector, expect_type)
        except NoMatches:
            return None

    def on_focus(self, event) -> None:
        if isinstance(event.widget, (DataTable, RichLog)): self._set_focused_pane(event.widget)

    @on(events.Click, "#status-pane")
    def on_status_click(self) -> None:
        pane = self.query_one("#status-pane", RichLog)
        pane.focus(); self._set_focused_pane(pane)

    @on(events.Click, "#log-pane")
    def on_log_click(self) -> None:
        pane = self.query_one("#log-pane", DataTable)
        pane.focus(); self._set_focused_pane(pane)

    @on(events.Click, "#worktree-table")
    def on_table_click(self) -> None:
        table = self.query_one("#worktree-table", DataTable)
        table.focus(); self._set_focused_pane(table)

    def _resolve_repo_name(self) -> str:
        repo_name = ""
        try:
            repo_name = subprocess.check_output(["gh", "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner"], text=True, stderr=subprocess.DEVNULL).strip()
        except (FileNotFoundError, subprocess.CalledProcessError):
            repo_name = ""
        except Exception as exc:
            self._notify_once("repo_name_gh", f"Failed to resolve repo name via gh: {exc}")
            repo_name = ""
        if not repo_name:
            try:
                remote_url = subprocess.check_output(["git", "remote", "get-url", "origin"], text=True, stderr=subprocess.DEVNULL).strip()
                match = re.search(r"[:/]([^/]+/[^/]+)(\.git)?$", remote_url)
                if match: repo_name = match.group(1)
            except (FileNotFoundError, subprocess.CalledProcessError):
                pass
            except Exception as exc:
                self._notify_once("repo_name_remote", f"Failed to resolve repo name from origin: {exc}")
        if not repo_name:
            try:
                toplevel = subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True, stderr=subprocess.DEVNULL).strip()
                repo_name = os.path.basename(toplevel)
            except (FileNotFoundError, subprocess.CalledProcessError):
                repo_name = "unknown"
            except Exception as exc:
                self._notify_once("repo_name_toplevel", f"Failed to resolve repo name from git: {exc}")
                repo_name = "unknown"
        return repo_name or "unknown"

    def _get_repo_key(self) -> str:
        if self._repo_key: return self._repo_key
        self._repo_key = self._resolve_repo_name()
        return self._repo_key

    def _last_selected_file(self) -> str:
        repo_key = self._get_repo_key()
        repo_root = os.path.expanduser(f"{WORKTREE_DIR}/{repo_key}")
        return os.path.join(repo_root, LAST_SELECTED_FILENAME)

    def _cache_file(self) -> str:
        repo_key = self._get_repo_key()
        repo_root = os.path.expanduser(f"{WORKTREE_DIR}/{repo_key}")
        return os.path.join(repo_root, CACHE_FILENAME)

    def _load_cache(self) -> dict:
        try:
            cache_path = self._cache_file()
            if os.path.exists(cache_path):
                with open(cache_path, "r", encoding="utf-8") as f: return json.load(f)
        except json.JSONDecodeError as exc:
            self._notify_once("cache_decode", f"Invalid cache file format: {exc}")
        except OSError as exc:
            self._notify_once("cache_read", f"Failed to read cache file: {exc}")
        return {}

    def _save_cache(self, data: dict) -> None:
        try:
            cache_path = self._cache_file()
            os.makedirs(os.path.dirname(cache_path), exist_ok=True)
            with open(cache_path, "w", encoding="utf-8") as f: json.dump(data, f)
        except OSError as exc:
            self._notify_once("cache_write", f"Failed to write cache file: {exc}")

    def _write_last_selected(self, path: str) -> None:
        if not path: return
        last_selected = self._last_selected_file()
        try:
            os.makedirs(os.path.dirname(last_selected), exist_ok=True)
            with open(last_selected, "w", encoding="utf-8") as handle: handle.write(f"{path}\n")
        except OSError as exc:
            self._notify_once("last_selected_write", f"Failed to save last selected worktree: {exc}")

    def _select_worktree(self, path: str) -> None:
        if path:
            self._write_last_selected(path)
            self.repo_name = self._get_repo_key()
        self.exit(result=path)

    async def run_git(self, args: List[str], cwd: Optional[str] = None, ok_returncodes: Iterable[int] = (0,), strip: bool = True) -> str:
        try:
            proc = await asyncio.create_subprocess_exec(*args, cwd=cwd, stdout=asyncio.subprocess.PIPE, stderr=asyncio.subprocess.PIPE)
            stdout, stderr = await proc.communicate()
            if proc.returncode not in set(ok_returncodes):
                detail = stderr.decode(errors="replace").strip() or stdout.decode(errors="replace").strip()
                command = " ".join(args)
                suffix = f": {detail}" if detail else f" (exit {proc.returncode})"
                self._notify_once(f"git_fail:{cwd}:{command}", f"Command failed: {command}{suffix}")
                return ""
            out = stdout.decode(errors="replace")
            return out.strip() if strip else out
        except FileNotFoundError as exc:
            command = args[0] if args else "command"
            self._notify_once(f"cmd_missing:{command}", f"Command not found: {command}")
            return ""
        except Exception as exc:
            command = " ".join(args)
            self._notify_once(f"cmd_error:{cwd}:{command}", f"Failed to run command: {command}: {exc}")
            return ""

    async def _run_command_checked(self, args: List[str], cwd: Optional[str], error_prefix: str) -> bool:
        try:
            proc = await asyncio.create_subprocess_exec(*args, cwd=cwd, stdout=asyncio.subprocess.PIPE, stderr=asyncio.subprocess.PIPE)
            stdout, stderr = await proc.communicate()
        except Exception as exc:
            self.notify(f"{error_prefix}: {exc}", severity="error")
            return False
        if proc.returncode == 0:
            return True
        detail = stderr.decode(errors="replace").strip() or stdout.decode(errors="replace").strip()
        if detail:
            self.notify(f"{error_prefix}: {detail}", severity="error")
        else:
            self.notify(error_prefix, severity="error")
        return False

    async def get_main_branch(self) -> str:
        if self._main_branch: return self._main_branch
        try:
            out = await self.run_git(["git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD"])
            if out: self._main_branch = out.split("/")[-1]
            else: self._main_branch = "main"
        except Exception: self._main_branch = "main"
        return self._main_branch

    async def get_worktrees(self) -> List[WorktreeInfo]:
        try: raw_wts = await self.run_git(["git", "worktree", "list", "--porcelain"])
        except Exception: return []
        wts = []
        current_wt = {}
        for line in raw_wts.splitlines():
            if line.startswith("worktree "):
                if current_wt: wts.append(current_wt)
                current_wt = {"path": line.split(" ", 1)[1]}
            elif line.startswith("branch "):
                current_wt["branch"] = line.split(" ", 1)[1].replace("refs/heads/", "")
        if current_wt: wts.append(current_wt)
        for i, wt_data in enumerate(wts): wt_data["is_main"] = i == 0
        branch_raw = await self.run_git(["git", "for-each-ref", "--format=%(refname:short)|%(committerdate:relative)|%(committerdate:unix)", "refs/heads"])
        branch_info = {}
        for line in branch_raw.splitlines():
            if "|" in line:
                parts = line.split("|")
                if len(parts) == 3: branch_info[parts[0]] = (parts[1], int(parts[2]))
        async def get_wt_info(wt_data):
            async with self._semaphore:
                path = wt_data["path"]
                branch = wt_data.get("branch", "(detached)")
                status_raw = await self.run_git(["git", "status", "--porcelain=v2", "--branch"], cwd=path)
                ahead = 0; behind = 0; untracked = 0; modified = 0; staged = 0
                for line in status_raw.splitlines():
                    if line.startswith("# branch.ab "):
                        parts = line.split()
                        if len(parts) >= 4: ahead = int(parts[2].replace("+", "")); behind = int(parts[3].replace("-", ""))
                    elif line.startswith("?"): untracked += 1
                    elif line.startswith("1 ") or line.startswith("2 "):
                        if len(line.split()) > 1:
                            xy = line.split()[1]
                            if len(xy) >= 2:
                                if xy[0] != ".": staged += 1
                                if xy[1] != ".": modified += 1
                last_active, last_active_ts = branch_info.get(branch, ("", 0))
                return WorktreeInfo(path=path, branch=branch, is_main=wt_data["is_main"], dirty=(untracked + modified + staged > 0), ahead=ahead, behind=behind, last_active=last_active, last_active_ts=last_active_ts, untracked=untracked, modified=modified, staged=staged)
        return await asyncio.gather(*(get_wt_info(wt) for wt in wts))

    async def fetch_pr_data(self) -> None:
        pr_raw = await self.run_git(["gh", "pr", "list", "--state", "all", "--json", "headRefName,state,number,title,url", "--limit", "100"])
        pr_map = {}
        if pr_raw:
            try:
                prs = json.loads(pr_raw)
                for p in prs: pr_map[p["headRefName"]] = PRInfo(p["number"], p["state"], p["title"], p["url"])
            except json.JSONDecodeError as exc:
                self._notify_once("pr_json_decode", f"Failed to parse PR data: {exc}")
        for wt in self.worktrees:
            if wt.branch in pr_map: wt.pr = pr_map[wt.branch]
        self._pr_data_loaded = True

    @work(exclusive=True)
    async def refresh_data(self) -> None:
        self.query_one(Header).loading = True
        self._pr_data_loaded = False
        self._cache = self._load_cache()
        self.worktrees = await self.get_worktrees()
        cache_data = {"worktrees": [{"path": wt.path, "branch": wt.branch, "last_active_ts": wt.last_active_ts} for wt in self.worktrees]}
        self._save_cache(cache_data)
        self.update_table()
        self.query_one(Header).loading = False
        self.update_details_view()

    def update_table(self):
        table = self.query_one("#worktree-table", DataTable)
        current_row_key = None
        if table.row_count > 0 and table.cursor_row is not None:
            try: current_row_key = table.coordinate_to_cell_key((table.cursor_row, 0)).row_key
            except: pass
        table.clear()
        query = self.filter_query.strip().lower()
        query_has_path_sep = "/" in query
        if not query: filtered_wts = list(self.worktrees)
        else:
            filtered_wts = []
            for wt in self.worktrees:
                name = os.path.basename(wt.path) if not wt.is_main else "main"
                haystacks = [name.lower(), wt.branch.lower()]
                if query_has_path_sep: haystacks.append(wt.path.lower())
                if any(query in h for h in haystacks): filtered_wts.append(wt)
        if self.sort_by_active: filtered_wts.sort(key=lambda x: x.last_active_ts, reverse=True)
        else: filtered_wts.sort(key=lambda x: x.path)
        for wt in filtered_wts:
            name = os.path.basename(wt.path) if not wt.is_main else "main"
            status_str = "[yellow]✎[/]" if wt.dirty else "[green]✔[/]"
            ab_str = f"[cyan]↑{wt.ahead}[/] " if wt.ahead else ""
            ab_str += f"[red]↓{wt.behind}[/] " if wt.behind else ""
            if not ab_str: ab_str = "0"
            pr_str = "-"
            if wt.pr:
                color = "green" if wt.pr.state == "OPEN" else "magenta" if wt.pr.state == "MERGED" else "red"
                pr_str = f"[white]#{wt.pr.number}[/] [{color}]{wt.pr.state[:1]}[/]"
            table.add_row(f"[magenta bold]{name}[/]" if wt.is_main else name, status_str, ab_str, pr_str, wt.last_active, key=wt.path)
        if current_row_key:
            try:
                index = table.get_row_index(current_row_key)
                table.move_cursor(row=index)
            except: pass

    def on_data_table_row_selected(self, event: DataTable.RowSelected) -> None:
        table = self._try_query_one("#worktree-table", DataTable)
        log_table = self._try_query_one("#log-pane", DataTable)
        if table is None or log_table is None:
            return
        data_table = getattr(event, "data_table", None) or getattr(event, "control", None)
        if data_table is log_table: self.open_commit_view(); return
        if data_table is not None and data_table is not table: return
        path = str(event.row_key.value)
        self._select_worktree(path)

    def on_data_table_row_highlighted(self, event: DataTable.RowHighlighted) -> None:
        table = self._try_query_one("#worktree-table", DataTable)
        if table is None:
            return
        data_table = getattr(event, "data_table", None) or getattr(event, "control", None)
        if data_table is not None and data_table is not table: return
        self.update_details_view()

    def action_open_pr(self) -> None:
        table = self.query_one("#worktree-table", DataTable)
        if table.cursor_row is not None:
            try:
                row_key = table.coordinate_to_cell_key((table.cursor_row, 0)).row_key
                path = str(row_key.value)
                wt = next((w for w in self.worktrees if w.path == path), None)
                if wt and wt.pr: webbrowser.open(wt.pr.url)
            except Exception: pass

    @work(exclusive=True)
    async def update_details_view(self) -> None:
        await asyncio.sleep(0.1)
        table = self.query_one("#worktree-table", DataTable)
        if table.row_count == 0:
            self.query_one("#status-pane", RichLog).clear()
            self.query_one("#log-pane", DataTable).clear(); return
        try:
            row_index = table.cursor_row
            row_key = table.coordinate_to_cell_key((row_index, 0)).row_key
            path = str(row_key.value)
        except Exception: return
        wt = next((w for w in self.worktrees if w.path == path), None)
        if not wt: return
        self.bind("o", "open_pr", description="Open PR", show=bool(wt.pr))
        self.query_one(Footer).refresh()
        status_task = self.run_git(["git", "status", "--short"], cwd=path)
        log_task = self.run_git(["git", "log", "-20", "--pretty=format:%h%x09%s"], cwd=path)
        async def get_div():
            cache_key = f"{path}:{wt.branch}"
            if cache_key in self._divergence_cache: return self._divergence_cache[cache_key]
            if wt.divergence: return wt.divergence
            main_branch = await self.get_main_branch()
            if wt.is_main: return ""
            res = await self.run_git(["git", "rev-list", "--left-right", "--count", f"{main_branch}...HEAD"], cwd=path)
            if res:
                try:
                    m_behind, m_ahead = res.split()
                    divergence = f"Main: ↑{m_ahead} ↓{m_behind}"
                    self._divergence_cache[cache_key] = divergence; return divergence
                except Exception: pass
            return ""
        status_raw, log_raw, divergence = await asyncio.gather(status_task, log_task, get_div())
        if divergence: wt.divergence = divergence
        grid = Table.grid(padding=(0, 2))
        grid.add_column(style="bold blue", justify="right", no_wrap=True)
        grid.add_column()
        grid.add_row("Path:", f"[blue]{path}[/]")
        grid.add_row("Branch:", f"[yellow]{wt.branch}[/]")
        if wt.divergence: grid.add_row("Divergence:", wt.divergence.replace("↑", "[cyan]↑[/]").replace("↓", "[red]↓[/]"))
        if wt.pr:
            state_color = "green" if wt.pr.state == "OPEN" else "magenta" if wt.pr.state == "MERGED" else "red"
            grid.add_row("PR:", f"[white]#{wt.pr.number}[/] {wt.pr.title} [[{state_color}]{wt.pr.state}[/]]")
            grid.add_row("", f"[underline blue]{wt.pr.url}[/]")
        if not status_raw: status_renderable = Text("✔ Clean working tree", style="dim green")
        else:
            status_table = Table.grid(padding=(0, 1))
            status_table.add_column(no_wrap=True); status_table.add_column()
            for line in status_raw.splitlines():
                code = line[:2]; rest = line[3:] if len(line) > 3 else ""; display_code = code.strip() or code
                if display_code == "??": display_code = "U"
                style = "yellow" if "M" in code else "green" if "A" in code or "?" in code else "red" if "D" in code else "cyan" if "R" in code else None
                status_table.add_row(Text(display_code, style=style), Text(rest))
            status_renderable = status_table
        status_panel = Panel(status_renderable, title="[bold blue]Status[/]")
        diff_text = ""; use_delta = False
        if status_raw: diff_text, use_delta = await self._build_diff_text(path)
        if diff_text:
            diff_panel = self._make_diff_panel("Diff", diff_text, use_delta)
            layout = Group(Panel(grid, title="[bold blue]Info[/]"), diff_panel)
        else: layout = Group(Panel(grid, title="[bold blue]Info[/]"), status_panel)
        status_log = self.query_one("#status-pane", RichLog)
        status_log.clear(); status_log.write(layout)
        log_table = self.query_one("#log-pane", DataTable)
        current_row_key = None
        if log_table.row_count > 0 and log_table.cursor_row is not None:
            try: current_row_key = log_table.coordinate_to_cell_key((log_table.cursor_row, 0)).row_key
            except Exception: current_row_key = None
        log_table.clear()
        if getattr(log_table, "column_count", 0) == 0:
            log_table.add_columns("SHA", "Message"); log_table.show_header = False
        if not log_raw: log_table.add_row("-", "No commits", key="NO_COMMITS")
        else:
            for line in log_raw.splitlines():
                sha, msg = (line.split("\t", 1) + [""])[:2]
                if sha: log_table.add_row(Text(sha, style="yellow"), msg, key=sha)
        if current_row_key:
            try: index = log_table.get_row_index(current_row_key); log_table.move_cursor(row=index)
            except Exception:
                if log_table.row_count > 0: log_table.move_cursor(row=0)
        elif log_table.row_count > 0: log_table.move_cursor(row=0)

    def action_cursor_down(self) -> None:
        focused = self.focused
        if isinstance(focused, RichLog): focused.scroll_down(animate=False); return
        if isinstance(focused, DataTable): focused.action_cursor_down(); return
        self.query_one("#worktree-table", DataTable).action_cursor_down()

    def action_cursor_up(self) -> None:
        focused = self.focused
        if isinstance(focused, RichLog): focused.scroll_up(animate=False); return
        if isinstance(focused, DataTable): focused.action_cursor_up(); return
        self.query_one("#worktree-table", DataTable).action_cursor_up()

    def action_scroll_details_down(self) -> None:
        focused = self.focused
        if isinstance(focused, RichLog): focused.scroll_page_down(animate=False); return
        if isinstance(focused, DataTable) and getattr(focused, "id", "") == "log-pane":
            action = getattr(focused, "action_page_down", None)
            if callable(action): action()
            else: focused.action_cursor_down()
            return
        self.query_one("#status-pane", RichLog).scroll_page_down(animate=False)

    def action_scroll_details_up(self) -> None:
        focused = self.focused
        if isinstance(focused, RichLog): focused.scroll_page_up(animate=False); return
        if isinstance(focused, DataTable) and getattr(focused, "id", "") == "log-pane":
            action = getattr(focused, "action_page_up", None)
            if callable(action): action()
            else: focused.action_cursor_up()
            return
        self.query_one("#status-pane", RichLog).scroll_page_up(animate=False)

    def action_cycle_focus(self) -> None:
        panes = self._focus_widgets()
        focused = self.focused
        try: index = panes.index(focused)
        except ValueError: index = 0
        next_pane = panes[(index + 1) % len(panes)]
        next_pane.focus(); self._set_focused_pane(next_pane)

    def action_fetch(self) -> None:
        self.notify("Fetching all remotes...")
        subprocess.run(["git", "fetch", "--all", "--quiet"])
        self.refresh_data()

    def action_fetch_prs(self) -> None:
        if self._pr_data_loaded: self.notify("PR data already loaded. Use 'r' to refresh.", severity="information"); return
        self.fetch_pr_data_async()

    @work(exclusive=True)
    async def fetch_pr_data_async(self) -> None:
        self.notify("Fetching PR data from GitHub...")
        self.query_one(Header).loading = True
        await self.fetch_pr_data()
        self.update_table()
        self.query_one(Header).loading = False
        self.update_details_view()
        self.notify("PR data fetched successfully!")

    async def _get_main_worktree_path(self) -> str:
        try:
            raw_wts = await self.run_git(["git", "worktree", "list", "--porcelain"])
            for line in raw_wts.splitlines():
                if line.startswith("worktree "): return line.split(" ", 1)[1]
        except Exception: pass
        return os.getcwd()

    async def _link_topsymlinks(self, main_path: str, target_path: str) -> None:
        try:
            process = await asyncio.create_subprocess_exec("git", "ls-files", "--others", "--ignored", "--exclude-standard", cwd=main_path, stdout=asyncio.subprocess.PIPE, stderr=asyncio.subprocess.PIPE)
            stdout, _ = await process.communicate()
            out = stdout.decode(errors="replace")
            for line in out.splitlines():
                line = line.strip()
                if not line or "/" in line or line == ".DS_Store" or ".mypy_cache" in line: continue
                src = os.path.join(main_path, line); dst = os.path.join(target_path, line)
                if os.path.exists(src) and not os.path.exists(dst):
                    try: os.symlink(src, dst)
                    except OSError: pass
            for editordir in [".cursor", ".claude", ".idea", ".vscode"]:
                src = os.path.join(main_path, editordir); dst = os.path.join(target_path, editordir)
                if os.path.isdir(src) and not os.path.exists(dst):
                    try: os.symlink(src, dst)
                    except OSError: pass
            os.makedirs(os.path.join(target_path, "tmp"), exist_ok=True)
            if os.path.exists(os.path.join(target_path, ".envrc")) and shutil.which("direnv"):
                process = await asyncio.create_subprocess_exec("direnv", "allow", ".", cwd=target_path)
                await process.communicate()
        except Exception as e: self.notify(f"Error in link_topsymlinks: {e}", severity="error")

    async def _run_wt_commands(self, commands: List[str], cwd: str, env: dict) -> None:
        for cmd in commands:
            if cmd == "link_topsymlinks":
                main_path = env.get("MAIN_WORKTREE_PATH")
                if main_path: await self._link_topsymlinks(main_path, cwd)
            else:
                expanded_cmd = os.path.expandvars(cmd)
                try:
                    process = await asyncio.create_subprocess_shell(expanded_cmd, cwd=cwd, env=env, stdout=asyncio.subprocess.PIPE, stderr=asyncio.subprocess.PIPE)
                    await process.communicate()
                except Exception as e: self.notify(f"Error running command '{cmd}': {e}", severity="error")

    def action_sort(self) -> None:
        self.sort_by_active = not self.sort_by_active
        sort_name = "Last Active" if self.sort_by_active else "Path"
        self.notify(f"Sorting by {sort_name}"); self.update_table()

    def action_filter(self) -> None:
        container = self.query_one("#filter-container")
        container.styles.display = "block"; self.query_one("#filter-input").focus()

    @on(Input.Changed, "#filter-input")
    def on_filter_changed(self, event: Input.Changed) -> None:
        self.filter_query = event.value; self.update_table()

    @on(Input.Submitted, "#filter-input")
    def on_filter_submitted(self) -> None:
        self.query_one("#filter-container").styles.display = "none"; self.query_one("#worktree-table", DataTable).focus()

    def action_help(self) -> None: self.push_screen(HelpScreen())

    def action_create(self) -> None:
        async def on_submit(name: Optional[str]):
            if not name: return
            name = name.strip()
            if not name: return
            self.notify(f"Creating worktree {name}...")
            repo_key = self._get_repo_key()
            new_path_root = os.path.expanduser(f"{WORKTREE_DIR}/{repo_key}")
            new_path = os.path.join(new_path_root, name); os.makedirs(new_path_root, exist_ok=True)
            try:
                main_path = await self._get_main_worktree_path()
                process = await asyncio.create_subprocess_exec("git", "worktree", "add", new_path, name)
                await process.communicate()
                if process.returncode != 0: self.notify(f"Failed to create worktree {name}", severity="error"); return
                config_path = os.path.join(main_path, ".wt")
                if os.path.exists(config_path):
                    try:
                        with open(config_path, "r") as f: config = yaml.safe_load(f)
                        init_commands = config.get("init_commands", [])
                        env = os.environ.copy()
                        env["WORKTREE_BRANCH"] = name; env["MAIN_WORKTREE_PATH"] = main_path; env["WORKTREE_PATH"] = new_path; env["WORKTREE_NAME"] = name
                        await self._run_wt_commands(init_commands, new_path, env)
                    except Exception as config_err: self.notify(f"Error loading .wt config: {config_err}", severity="error")
                self.notify(f"Created worktree {name}"); self.refresh_data()
            except Exception as e: self.notify(f"Error: {e}", severity="error")
        self.push_screen(InputScreen("Enter new branch/worktree name:"), lambda name: self.run_worker(on_submit(name)))

    async def _apply_delta(self, diff_text: str) -> tuple[str, bool]:
        use_delta = shutil.which("delta") is not None
        if not use_delta: return diff_text, False
        try:
            proc = await asyncio.create_subprocess_exec("delta", "--no-gitconfig", "--paging=never", stdin=asyncio.subprocess.PIPE, stdout=asyncio.subprocess.PIPE, stderr=asyncio.subprocess.PIPE)
            stdout, stderr = await proc.communicate(diff_text.encode())
            if proc.returncode == 0: return stdout.decode(errors="replace"), True
        except Exception: pass
        return diff_text, False

    async def _build_diff_text(self, path: str) -> tuple[str, bool]:
        staged_task = self.run_git(["git", "diff", "--cached", "--patch", "--no-color"], cwd=path, strip=False)
        unstaged_task = self.run_git(["git", "diff", "--patch", "--no-color"], cwd=path, strip=False)
        untracked_task = self.run_git(["git", "ls-files", "--others", "--exclude-standard"], cwd=path)
        staged, unstaged, untracked = await asyncio.gather(staged_task, unstaged_task, untracked_task)
        untracked_patches: List[str] = []
        untracked_files = [f for f in untracked.splitlines() if f]
        max_untracked_diffs = 10
        if len(untracked_files) > max_untracked_diffs:
            untracked_files = untracked_files[:max_untracked_diffs]
            untracked_patches.append(f"# Note: Showing first {max_untracked_diffs} untracked files (total: {len(untracked.splitlines())})\n")
        if untracked_files:
            untracked_tasks = [self.run_git(["git", "diff", "--no-index", "--no-color", "--", "/dev/null", file], cwd=path, ok_returncodes=(0, 1), strip=False) for file in untracked_files]
            untracked_results = await asyncio.gather(*untracked_tasks)
            untracked_patches.extend([p for p in untracked_results if p])
        parts: List[str] = []
        if staged.strip(): parts.append("# Staged\n" + staged.strip("\n"))
        if unstaged.strip(): parts.append("# Unstaged\n" + unstaged.strip("\n"))
        if untracked_patches: parts.append("# Untracked\n" + "\n\n".join(p.strip("\n") for p in untracked_patches))
        diff_text = "\n\n".join(parts).strip("\n")
        if not diff_text: return "", False
        max_chars = 200_000
        if len(diff_text) > max_chars: diff_text = diff_text[:max_chars] + "\n\n# [truncated]"
        return await self._apply_delta(diff_text)

    def _make_diff_panel(self, title: str, diff_text: str, use_delta: bool) -> Panel:
        renderable = Text.from_ansi(diff_text) if use_delta else Syntax(diff_text, "diff", word_wrap=True)
        return Panel(renderable, title=f"[bold blue]{title}[/]", expand=True)

    async def _get_commit_info(self, path: str, sha: str) -> Optional[dict]:
        fmt = "%H%n%an <%ae>%n%ad%n%s%n%b"
        info_raw = await self.run_git(["git", "show", "-s", f"--format={fmt}", sha], cwd=path, strip=False)
        if not info_raw.strip(): return None
        lines = info_raw.splitlines()
        if len(lines) < 4: return None
        return {"sha": lines[0].strip(), "author": lines[1].strip(), "date": lines[2].strip(), "subject": lines[3].strip(), "body": "\n".join(lines[4:]).strip()}

    async def _build_commit_view(self, path: str, sha: str) -> tuple[Optional[dict], str, bool]:
        info = await self._get_commit_info(path, sha)
        diff_raw = await self.run_git(["git", "show", "--patch", "--no-color", "--pretty=format:", sha], cwd=path, strip=False)
        diff_text = diff_raw.strip("\n")
        if not diff_text: return info, "", False
        max_chars = 200_000
        if len(diff_text) > max_chars: diff_text = diff_text[:max_chars] + "\n\n# [truncated]"
        diff_text, use_delta = await self._apply_delta(diff_text)
        return info, diff_text, use_delta

    def action_diff(self) -> None: self.open_diff_view()

    @work(exclusive=True)
    async def open_diff_view(self) -> None:
        table = self.query_one("#worktree-table", DataTable)
        if table.cursor_row is None: self.notify("No worktree selected", severity="warning"); return
        row_key = table.coordinate_to_cell_key((table.cursor_row, 0)).row_key
        path = str(row_key.value); diff_text, use_delta = await self._build_diff_text(path)
        if not diff_text: self.notify("No changes in this worktree", severity="information"); return
        title = f"Diff: {os.path.basename(path) or path}"
        renderable = self._make_diff_panel(title, diff_text, use_delta)
        status_log = self.query_one("#status-pane", RichLog)
        status_log.clear(); status_log.write(renderable); status_log.scroll_home(animate=False); status_log.focus(); self._set_focused_pane(status_log)

    def action_delete(self) -> None:
        table = self.query_one("#worktree-table", DataTable)
        if table.row_count == 0: return
        try:
            row_key = table.coordinate_to_cell_key((table.cursor_row, 0)).row_key
            path = str(row_key.value)
        except: return
        wt = next((w for w in self.worktrees if w.path == path), None)
        if not wt: return
        if wt.is_main: self.notify("Cannot delete main worktree", severity="error"); return
        async def do_delete(confirm: Optional[bool]):
            if not confirm: return
            self.notify(f"Deleting {wt.branch}...")
            try:
                main_path = await self._get_main_worktree_path()
                config_path = os.path.join(main_path, ".wt")
                if os.path.exists(config_path):
                    try:
                        with open(config_path, "r") as f: config = yaml.safe_load(f)
                        terminate_commands = config.get("terminate_commands", [])
                        env = os.environ.copy()
                        env["WORKTREE_BRANCH"] = wt.branch; env["MAIN_WORKTREE_PATH"] = main_path; env["WORKTREE_PATH"] = path; env["WORKTREE_NAME"] = os.path.basename(path)
                        await self._run_wt_commands(terminate_commands, main_path, env)
                    except Exception as config_err: self.notify(f"Error loading .wt config: {config_err}", severity="error")
                removed = await self._run_command_checked(
                    ["git", "worktree", "remove", "--force", path],
                    cwd=None,
                    error_prefix=f"Failed to remove worktree {path}",
                )
                if not removed:
                    return
                deleted = await self._run_command_checked(
                    ["git", "branch", "-D", wt.branch],
                    cwd=None,
                    error_prefix=f"Failed to delete branch {wt.branch}",
                )
                if not deleted:
                    return
                self.notify("Worktree deleted")
                self.refresh_data()
            except Exception as e: self.notify(f"Failed to delete: {e}", severity="error")
        self.push_screen(ConfirmScreen(f"Are you sure you want to delete worktree?\n\nPath: {path}\nBranch: {wt.branch}"), lambda confirm: self.run_worker(do_delete(confirm)))

    def action_absorb(self) -> None:
        table = self.query_one("#worktree-table", DataTable)
        if table.row_count == 0: return
        try:
            row_key = table.coordinate_to_cell_key((table.cursor_row, 0)).row_key
            path = str(row_key.value)
        except: return
        wt = next((w for w in self.worktrees if w.path == path), None)
        if not wt: return
        if wt.is_main: self.notify("Cannot absorb main worktree", severity="error"); return
        async def do_absorb(confirm: Optional[bool]):
            if not confirm: return
            self.notify(f"Absorbing {wt.branch}...")
            try:
                main_path = await self._get_main_worktree_path()
                config_path = os.path.join(main_path, ".wt")
                if os.path.exists(config_path):
                    try:
                        with open(config_path, "r") as f: config = yaml.safe_load(f)
                        terminate_commands = config.get("terminate_commands", [])
                        env = os.environ.copy()
                        env["WORKTREE_BRANCH"] = wt.branch; env["MAIN_WORKTREE_PATH"] = main_path; env["WORKTREE_PATH"] = path; env["WORKTREE_NAME"] = os.path.basename(path)
                        await self._run_wt_commands(terminate_commands, main_path, env)
                    except Exception as config_err: self.notify(f"Error loading .wt config: {config_err}", severity="error")
                main_branch = await self.get_main_branch()
                checked_out = await self._run_command_checked(
                    ["git", "checkout", main_branch],
                    cwd=path,
                    error_prefix=f"Failed to checkout {main_branch}",
                )
                if not checked_out:
                    return
                merged = await self._run_command_checked(
                    ["git", "merge", "--no-edit", wt.branch],
                    cwd=path,
                    error_prefix=f"Failed to merge {wt.branch} into {main_branch}",
                )
                if not merged:
                    return
                removed = await self._run_command_checked(
                    ["git", "worktree", "remove", "--force", path],
                    cwd=None,
                    error_prefix=f"Failed to remove worktree {path}",
                )
                if not removed:
                    return
                deleted = await self._run_command_checked(
                    ["git", "branch", "-D", wt.branch],
                    cwd=None,
                    error_prefix=f"Failed to delete branch {wt.branch}",
                )
                if not deleted:
                    return
                self.notify("Worktree absorbed successfully")
                self.refresh_data()
            except Exception as e: self.notify(f"Failed to absorb: {e}", severity="error")
        self.push_screen(ConfirmScreen(f"Absorb worktree to main branch?\n\nPath: {path}\nBranch: {wt.branch}\n\nThis will merge changes to main and delete the worktree."), lambda confirm: self.run_worker(do_absorb(confirm)))

    def action_lazygit(self) -> None:
        table = self.query_one("#worktree-table", DataTable)
        if table.cursor_row is None: self.notify("No worktree selected", severity="warning"); return
        row_key = table.coordinate_to_cell_key((table.cursor_row, 0)).row_key
        path = str(row_key.value)
        if shutil.which("lazygit") is None: self.notify("`lazygit` not found in PATH (required for `g`)", severity="error"); return
        try:
            suspend_process = getattr(self, "suspend_process", None) or getattr(self.app, "suspend_process", None)
            if callable(suspend_process): suspend_process(subprocess.run, ["lazygit"], cwd=path)
            else:
                with self.suspend(): subprocess.run(["lazygit"], cwd=path, check=False)
            self.refresh_data()
        except Exception as e: self.notify(f"Failed to run lazygit: {e}", severity="error")

    def action_jump(self) -> None:
        focused = self.focused
        if isinstance(focused, DataTable) and getattr(focused, "id", "") == "log-pane": self.open_commit_view(); return
        table = self.query_one("#worktree-table", DataTable)
        if table.row_count > 0:
            row_key = table.coordinate_to_cell_key((table.cursor_row, 0)).row_key
            path = str(row_key.value); self._select_worktree(path)

    @work(exclusive=True)
    async def open_commit_view(self) -> None:
        log_table = self.query_one("#log-pane", DataTable)
        if log_table.cursor_row is None or log_table.row_count == 0: self.notify("No commit selected", severity="warning"); return
        try:
            row_key = log_table.coordinate_to_cell_key((log_table.cursor_row, 0)).row_key
            sha = str(row_key.value)
        except Exception: self.notify("No commit selected", severity="warning"); return
        if not sha or sha == "NO_COMMITS": self.notify("No commit selected", severity="warning"); return
        path = self._selected_worktree_path()
        if not path: self.notify("No worktree selected", severity="warning"); return
        info, diff_text, use_delta = await self._build_commit_view(path, sha)
        if not info and not diff_text: self.notify("No commit content found", severity="information"); return
        header_grid = Table.grid(padding=(0, 1))
        header_grid.add_column(style="bold blue", no_wrap=True); header_grid.add_column()
        if info:
            header_grid.add_row("Commit:", f"[yellow]{info['sha']}[/]"); header_grid.add_row("Author:", info["author"]); header_grid.add_row("Date:", info["date"]); header_grid.add_row("Subject:", f"[white]{info['subject']}[/]")
            if info["body"]: header_grid.add_row("Message:", info["body"])
        header_panel = Panel(header_grid, title="[bold blue]Commit[/]")
        diff_panel = self._make_diff_panel("Diff", diff_text or "No diff", use_delta)
        self.push_screen(CommitScreen(header_panel, diff_panel))
