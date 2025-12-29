import asyncio
import json
import os
import subprocess
import webbrowser
import shutil
import re
import yaml
from pathlib import Path
from datetime import datetime
from typing import List, Optional, Iterable

from textual import on, work, events
from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.css.query import NoMatches
from textual.command import DiscoveryHit, Hit, Provider
from textual.containers import Container, Horizontal, Vertical
from textual.timer import Timer
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

from .config import AppConfig, normalize_command_list
from .models import WorktreeInfo, LAST_SELECTED_FILENAME, CACHE_FILENAME
from .git_service import GitService
from .security import TrustManager, TrustStatus
from .screens import (
    ConfirmScreen,
    InputScreen,
    HelpScreen,
    CommitScreen,
    FocusableRichLog,
    TrustScreen,
    WelcomeScreen,
)


class GitWtStatusCommands(Provider):
    """Command provider for Git Worktree Status actions."""

    COMMANDS = [
        ("Jump to worktree", "jump", "Jump to selected worktree"),
        ("Create worktree", "create", "Create a new worktree"),
        ("Rename worktree", "rename", "Rename selected worktree"),
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
                yield Hit(
                    match,
                    matcher.highlight(name),
                    self._make_callback(action),
                    help=help_text,
                )


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
        Binding("p", "fetch_prs", "PR Info"),
        Binding("c", "create", "Create", show=False),
        Binding("m", "rename", "Rename", show=False),
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
    _pr_data_loaded: bool = False
    repo_name: str = ""

    def __init__(
        self,
        initial_filter: str = "",
        git_service: Optional[GitService] = None,
        config: Optional[AppConfig] = None,
    ):
        super().__init__()
        self._initial_filter = initial_filter
        self._repo_key: Optional[str] = None
        self._cache: dict = {}
        self._divergence_cache: dict = {}
        self._notified_errors: set[str] = set()
        self._git = git_service or GitService(self.notify, self._notify_once)
        self._config = config or AppConfig()
        self.sort_by_active = self._config.sort_by_active
        self._auto_fetch_prs_done = False
        self._trust_manager = TrustManager()
        self._debounce_timer: Optional[Timer] = None

    def log_debug(self, message: str) -> None:
        if not self._config.debug_log:
            return
        try:
            timestamp = datetime.now().isoformat()
            with open(self._config.debug_log, "a", encoding="utf-8") as f:
                f.write(f"[{timestamp}] {message}\n")
        except Exception as e:
            # Fallback to notify if logging fails, but don't recurse
            self._notify_once("debug_log_fail", f"Failed to write to debug log: {e}")

    def _notify_once(self, key: str, message: str, severity: str = "error") -> None:
        if key in self._notified_errors:
            return
        self._notified_errors.add(key)
        self.notify(message, severity=severity)

    @property
    def worktree_dir(self) -> str:
        # Should be guaranteed by main.py, but fallback just in case
        return self._config.worktree_dir or os.path.expanduser(
            "~/.local/share/worktrees"
        )

    async def _get_repo_commands(
        self, repo_root: str, command_type: str
    ) -> Optional[List[str]]:
        """
        Retrieves commands from repo-local .wt file with TOFU (Trust On First Use).
        Returns:
            - List[str]: The commands to run (empty if none or blocked).
            - None: If the operation should be CANCELLED.
        """
        trust_mode = self._config.trust_mode
        if trust_mode == "never":
            return []

        config_path = os.path.join(repo_root, ".wt")
        if not os.path.exists(config_path):
            return []

        try:
            with open(config_path, "r", encoding="utf-8") as f:
                data = yaml.safe_load(f) or {}
            commands = normalize_command_list(data.get(command_type))
        except Exception as e:
            self.notify(f"Error loading .wt config: {e}", severity="error")
            return []

        if not commands:
            return []

        if trust_mode == "always":
            return commands

        path_obj = Path(config_path)
        trust_status = self._trust_manager.check_trust(path_obj)

        if trust_status == TrustStatus.TRUSTED:
            return commands

        # If untrusted, ask user
        loop = asyncio.get_running_loop()
        future = loop.create_future()

        def on_dismiss(result: str):
            future.set_result(result)

        self.push_screen(TrustScreen(config_path, commands), on_dismiss)

        # This will pause execution of this coroutine until user clicks a button
        result = await future

        if result == "trust":
            self._trust_manager.trust_file(path_obj)
            return commands
        elif result == "block":
            return []
        else:  # cancel / escape
            return None

    def compose(self) -> ComposeResult:
        yield Header()
        with Container(id="filter-container"):
            yield Input(placeholder="Filter worktrees...", id="filter-input")
        with Horizontal(id="main-content"):
            yield DataTable(id="worktree-table", cursor_type="row")
            with Vertical(id="right-pane"):
                yield FocusableRichLog(
                    id="status-pane", wrap=True, markup=True, auto_scroll=False
                )
                yield DataTable(id="log-pane", cursor_type="row")
        yield Footer()

    def on_mount(self) -> None:
        table = self.query_one("#worktree-table", DataTable)
        table.border_title = "[bold cyan][1][/] [bold white]Worktrees[/]"
        table.add_columns("Worktree", "Status", "±", "PR", "Last Active")
        table.focus()
        self._set_focused_pane(table)
        self.query_one(
            "#status-pane", RichLog
        ).border_title = "[bold cyan][2][/] [bold white]Info/Diff[/]"
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
        return [
            self.query_one("#worktree-table", DataTable),
            self.query_one("#status-pane", RichLog),
            self.query_one("#log-pane", DataTable),
        ]

    def _set_focused_pane(self, widget) -> None:
        for pane in self._focus_widgets():
            pane.remove_class("focused")
        widget.add_class("focused")
        self._apply_focus_layout(
            getattr(widget, "id", "") in {"status-pane", "log-pane"}
        )

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
        if table.row_count == 0 or table.cursor_row is None:
            return None
        row_key = table.coordinate_to_cell_key((table.cursor_row, 0)).row_key
        return str(row_key.value)

    def _try_query_one(self, selector, expect_type):
        try:
            return self.query_one(selector, expect_type)
        except NoMatches:
            return None

    def on_focus(self, event) -> None:
        if isinstance(event.widget, (DataTable, RichLog)):
            self._set_focused_pane(event.widget)

    @on(events.Click, "#status-pane")
    def on_status_click(self) -> None:
        pane = self.query_one("#status-pane", RichLog)
        pane.focus()
        self._set_focused_pane(pane)

    @on(events.Click, "#log-pane")
    def on_log_click(self) -> None:
        pane = self.query_one("#log-pane", DataTable)
        pane.focus()
        self._set_focused_pane(pane)

    @on(events.Click, "#worktree-table")
    def on_table_click(self) -> None:
        table = self.query_one("#worktree-table", DataTable)
        table.focus()
        self._set_focused_pane(table)

    def _resolve_repo_name(self) -> str:
        repo_name = ""
        try:
            repo_name = subprocess.check_output(
                [
                    "gh",
                    "repo",
                    "view",
                    "--json",
                    "nameWithOwner",
                    "-q",
                    ".nameWithOwner",
                ],
                text=True,
                stderr=subprocess.DEVNULL,
            ).strip()
        except (FileNotFoundError, subprocess.CalledProcessError):
            repo_name = ""
        except Exception as exc:
            self._notify_once(
                "repo_name_gh", f"Failed to resolve repo name via gh: {exc}"
            )
            repo_name = ""

        if not repo_name:
            try:
                out = subprocess.check_output(
                    ["glab", "repo", "view", "-F", "json"],
                    text=True,
                    stderr=subprocess.DEVNULL,
                ).strip()
                if out:
                    data = json.loads(out)
                    repo_name = data.get("path_with_namespace", "")
            except (
                FileNotFoundError,
                subprocess.CalledProcessError,
                json.JSONDecodeError,
            ):
                repo_name = ""
            except Exception as exc:
                self._notify_once(
                    "repo_name_glab", f"Failed to resolve repo name via glab: {exc}"
                )
                repo_name = ""

        if not repo_name:
            try:
                remote_url = subprocess.check_output(
                    ["git", "remote", "get-url", "origin"],
                    text=True,
                    stderr=subprocess.DEVNULL,
                ).strip()
                match = re.search(r"[:/]([^/]+/[^/]+)(\.git)?$", remote_url)
                if match:
                    repo_name = match.group(1)
            except (FileNotFoundError, subprocess.CalledProcessError):
                pass
            except Exception as exc:
                self._notify_once(
                    "repo_name_remote",
                    f"Failed to resolve repo name from origin: {exc}",
                )
        if not repo_name:
            try:
                toplevel = subprocess.check_output(
                    ["git", "rev-parse", "--show-toplevel"],
                    text=True,
                    stderr=subprocess.DEVNULL,
                ).strip()
                repo_name = os.path.basename(toplevel)
            except (FileNotFoundError, subprocess.CalledProcessError):
                repo_name = "unknown"
            except Exception as exc:
                self._notify_once(
                    "repo_name_toplevel", f"Failed to resolve repo name from git: {exc}"
                )
                repo_name = "unknown"
        return repo_name or "unknown"

    def _get_repo_key(self) -> str:
        if self._repo_key:
            return self._repo_key
        self._repo_key = self._resolve_repo_name()
        return self._repo_key

    def _last_selected_file(self) -> str:
        repo_key = self._get_repo_key()
        repo_root = os.path.expanduser(f"{self.worktree_dir}/{repo_key}")
        return os.path.join(repo_root, LAST_SELECTED_FILENAME)

    def _cache_file(self) -> str:
        repo_key = self._get_repo_key()
        repo_root = os.path.expanduser(f"{self.worktree_dir}/{repo_key}")
        return os.path.join(repo_root, CACHE_FILENAME)

    def _load_cache(self) -> dict:
        try:
            cache_path = self._cache_file()
            if os.path.exists(cache_path):
                with open(cache_path, "r", encoding="utf-8") as f:
                    return json.load(f)
        except json.JSONDecodeError as exc:
            self._notify_once("cache_decode", f"Invalid cache file format: {exc}")
        except OSError as exc:
            self._notify_once("cache_read", f"Failed to read cache file: {exc}")
        return {}

    def _save_cache(self, data: dict) -> None:
        try:
            cache_path = self._cache_file()
            os.makedirs(os.path.dirname(cache_path), exist_ok=True)
            with open(cache_path, "w", encoding="utf-8") as f:
                json.dump(data, f)
        except OSError as exc:
            self._notify_once("cache_write", f"Failed to write cache file: {exc}")

    def _write_last_selected(self, path: str) -> None:
        if not path:
            return
        last_selected = self._last_selected_file()
        try:
            os.makedirs(os.path.dirname(last_selected), exist_ok=True)
            with open(last_selected, "w", encoding="utf-8") as handle:
                handle.write(f"{path}\n")
        except OSError as exc:
            self._notify_once(
                "last_selected_write", f"Failed to save last selected worktree: {exc}"
            )

    def _select_worktree(self, path: str) -> None:
        if path:
            self._write_last_selected(path)
            self.repo_name = self._get_repo_key()
        self.exit(result=path)

    async def run_git(
        self,
        args: List[str],
        cwd: Optional[str] = None,
        ok_returncodes: Iterable[int] = (0,),
        strip: bool = True,
    ) -> str:
        return await self._git.run_git(
            args, cwd=cwd, ok_returncodes=ok_returncodes, strip=strip
        )

    async def _run_command_checked(
        self, args: List[str], cwd: Optional[str], error_prefix: str
    ) -> bool:
        return await self._git.run_command_checked(args, cwd, error_prefix)

    async def get_main_branch(self) -> str:
        return await self._git.get_main_branch()

    async def get_worktrees(self) -> List[WorktreeInfo]:
        return await self._git.get_worktrees()

    async def fetch_pr_data(self) -> bool:
        pr_map = await self._git.fetch_pr_map()
        if pr_map is None:
            return False
        for wt in self.worktrees:
            if wt.branch in pr_map:
                wt.pr = pr_map[wt.branch]
        self._pr_data_loaded = True
        return True

    @work(exclusive=True)
    async def refresh_data(self) -> None:
        header = self.query_one(Header)
        header.loading = True
        self._pr_data_loaded = False

        # Try to load from cache first for immediate feedback
        self._cache = self._load_cache()
        cached_wts = []
        if self._cache and "worktrees" in self._cache:
            for w in self._cache["worktrees"]:
                try:
                    # Reconstruct WorktreeInfo from cache
                    wt = WorktreeInfo(
                        path=w["path"],
                        branch=w["branch"],
                        is_main=w.get("is_main", False),
                        dirty=w.get("dirty", False),
                        ahead=w.get("ahead", 0),
                        behind=w.get("behind", 0),
                        last_active=w.get("last_active", ""),
                        last_active_ts=w.get("last_active_ts", 0),
                        untracked=w.get("untracked", 0),
                        modified=w.get("modified", 0),
                        staged=w.get("staged", 0),
                        divergence=w.get("divergence", ""),
                    )
                    cached_wts.append(wt)
                except Exception:
                    continue

            if cached_wts:
                self.worktrees = cached_wts
                self.update_table()

        # Fetch fresh data
        self.worktrees = await self.get_worktrees()

        if not self.worktrees and not self._cache.get("worktrees"):

            def show_welcome():
                self.push_screen(
                    WelcomeScreen(os.getcwd(), self.worktree_dir),
                    self._welcome_callback,
                )

            show_welcome()

        cache_data = {
            "worktrees": [
                {
                    "path": wt.path,
                    "branch": wt.branch,
                    "is_main": wt.is_main,
                    "dirty": wt.dirty,
                    "ahead": wt.ahead,
                    "behind": wt.behind,
                    "last_active": wt.last_active,
                    "last_active_ts": wt.last_active_ts,
                    "untracked": wt.untracked,
                    "modified": wt.modified,
                    "staged": wt.staged,
                    "divergence": wt.divergence,
                }
                for wt in self.worktrees
            ]
        }
        self._save_cache(cache_data)

        fetch_success: Optional[bool] = None
        if self._config.auto_fetch_prs and not self._auto_fetch_prs_done:
            self._auto_fetch_prs_done = True
            self.notify("Fetching PR data from GitHub...")
            fetch_success = await self.fetch_pr_data()

        self.update_table()
        header.loading = False
        self.update_details_view()

        if fetch_success is not None:
            if fetch_success:
                self.notify("PR data fetched successfully!")
            else:
                self.notify("Failed to fetch PR data", severity="error")

    def _welcome_callback(self, retry: bool):
        if retry:
            self.refresh_data()

    def update_table(self):
        table = self.query_one("#worktree-table", DataTable)
        current_row_key = None
        if table.row_count > 0 and table.cursor_row is not None:
            try:
                current_row_key = table.coordinate_to_cell_key(
                    (table.cursor_row, 0)
                ).row_key
            except Exception:
                pass
        table.clear()
        query = self.filter_query.strip().lower()
        query_has_path_sep = "/" in query
        if not query:
            filtered_wts = list(self.worktrees)
        else:
            filtered_wts = []
            for wt in self.worktrees:
                name = os.path.basename(wt.path) if not wt.is_main else "main"
                haystacks = [name.lower(), wt.branch.lower()]
                if query_has_path_sep:
                    haystacks.append(wt.path.lower())
                if any(query in h for h in haystacks):
                    filtered_wts.append(wt)
        if self.sort_by_active:
            filtered_wts.sort(key=lambda x: x.last_active_ts, reverse=True)
        else:
            filtered_wts.sort(key=lambda x: x.path)
        for wt in filtered_wts:
            name = os.path.basename(wt.path) if not wt.is_main else "main"
            status_str = "[yellow]✎[/]" if wt.dirty else "[green]✔[/]"
            ab_str = f"[cyan]↑{wt.ahead}[/] " if wt.ahead else ""
            ab_str += f"[red]↓{wt.behind}[/] " if wt.behind else ""
            if not ab_str:
                ab_str = "0"
            pr_str = "-"
            if wt.pr:
                color = (
                    "green"
                    if wt.pr.state == "OPEN"
                    else "magenta"
                    if wt.pr.state == "MERGED"
                    else "red"
                )
                pr_str = f"[white]#{wt.pr.number}[/] [{color}]{wt.pr.state[:1]}[/]"
            table.add_row(
                f"[magenta bold]{name}[/]" if wt.is_main else name,
                status_str,
                ab_str,
                pr_str,
                wt.last_active,
                key=wt.path,
            )
        if current_row_key:
            try:
                index = table.get_row_index(current_row_key)
                table.move_cursor(row=index)
            except Exception:
                if table.row_count > 0:
                    table.move_cursor(row=0)
        elif table.row_count > 0:
            table.move_cursor(row=0)

    def on_data_table_row_selected(self, event: DataTable.RowSelected) -> None:
        table = self._try_query_one("#worktree-table", DataTable)
        log_table = self._try_query_one("#log-pane", DataTable)
        if table is None or log_table is None:
            return
        data_table = getattr(event, "data_table", None) or getattr(
            event, "control", None
        )
        if data_table is log_table:
            self.open_commit_view()
            return
        if data_table is not None and data_table is not table:
            return
        path = str(event.row_key.value)
        self._select_worktree(path)

    def on_data_table_row_highlighted(self, event: DataTable.RowHighlighted) -> None:
        table = self._try_query_one("#worktree-table", DataTable)
        if table is None:
            return
        data_table = getattr(event, "data_table", None) or getattr(
            event, "control", None
        )
        if data_table is not None and data_table is not table:
            return

        if self._debounce_timer:
            self._debounce_timer.stop()
        self._debounce_timer = self.set_timer(0.2, self.update_details_view)

    def action_open_pr(self) -> None:
        table = self.query_one("#worktree-table", DataTable)
        if table.cursor_row is not None:
            try:
                row_key = table.coordinate_to_cell_key((table.cursor_row, 0)).row_key
                path = str(row_key.value)
                wt = next((w for w in self.worktrees if w.path == path), None)
                if wt and wt.pr:
                    webbrowser.open(wt.pr.url)
            except Exception:
                pass

    @work(exclusive=True)
    async def update_details_view(self) -> None:
        await asyncio.sleep(0.1)
        table = self.query_one("#worktree-table", DataTable)
        if table.row_count == 0:
            self.query_one("#status-pane", RichLog).clear()
            self.query_one("#log-pane", DataTable).clear()
            return
        try:
            row_index = table.cursor_row
            row_key = table.coordinate_to_cell_key((row_index, 0)).row_key
            path = str(row_key.value)
        except Exception:
            return
        wt = next((w for w in self.worktrees if w.path == path), None)
        if not wt:
            return
        self.bind("o", "open_pr", description="Open PR", show=bool(wt.pr))
        self.query_one(Footer).refresh()
        status_task = self.run_git(["git", "status", "--short"], cwd=path)
        log_task = self.run_git(
            ["git", "log", "-20", "--pretty=format:%h%x09%s"], cwd=path
        )

        async def get_div():
            cache_key = f"{path}:{wt.branch}"
            if cache_key in self._divergence_cache:
                return self._divergence_cache[cache_key]
            if wt.divergence:
                return wt.divergence
            main_branch = await self.get_main_branch()
            if wt.is_main:
                return ""
            res = await self.run_git(
                ["git", "rev-list", "--left-right", "--count", f"{main_branch}...HEAD"],
                cwd=path,
            )
            if res:
                try:
                    m_behind, m_ahead = res.split()
                    divergence = f"Main: ↑{m_ahead} ↓{m_behind}"
                    self._divergence_cache[cache_key] = divergence
                    return divergence
                except Exception:
                    pass
            return ""

        status_raw, log_raw, divergence = await asyncio.gather(
            status_task, log_task, get_div()
        )
        if divergence:
            wt.divergence = divergence
        grid = Table.grid(padding=(0, 2))
        grid.add_column(style="bold blue", justify="right", no_wrap=True)
        grid.add_column()
        grid.add_row("Path:", f"[blue]{path}[/]")
        grid.add_row("Branch:", f"[yellow]{wt.branch}[/]")
        if wt.divergence:
            grid.add_row(
                "Divergence:",
                wt.divergence.replace("↑", "[cyan]↑[/]").replace("↓", "[red]↓[/]"),
            )
        if wt.pr:
            state_color = (
                "green"
                if wt.pr.state == "OPEN"
                else "magenta"
                if wt.pr.state == "MERGED"
                else "red"
            )
            grid.add_row(
                "PR:",
                f"[white]#{wt.pr.number}[/] {wt.pr.title} [[{state_color}]{wt.pr.state}[/]]",
            )
            grid.add_row("", f"[underline blue]{wt.pr.url}[/]")
        if not status_raw:
            status_renderable = Text("✔ Clean working tree", style="dim green")
        else:
            status_table = Table.grid(padding=(0, 1))
            status_table.add_column(no_wrap=True)
            status_table.add_column()
            for line in status_raw.splitlines():
                code = line[:2]
                rest = line[3:] if len(line) > 3 else ""
                display_code = code.strip() or code
                if display_code == "??":
                    display_code = "U"
                style = (
                    "yellow"
                    if "M" in code
                    else "green"
                    if "A" in code or "?" in code
                    else "red"
                    if "D" in code
                    else "cyan"
                    if "R" in code
                    else None
                )
                status_table.add_row(Text(display_code, style=style), Text(rest))
            status_renderable = status_table
        status_panel = Panel(status_renderable, title="[bold blue]Status[/]")
        diff_text = ""
        use_delta = False
        if status_raw:
            diff_text, use_delta = await self._build_diff_text(path)
        if diff_text:
            diff_panel = self._make_diff_panel("Diff", diff_text, use_delta)
            layout = Group(Panel(grid, title="[bold blue]Info[/]"), diff_panel)
        else:
            layout = Group(Panel(grid, title="[bold blue]Info[/]"), status_panel)
        status_log = self.query_one("#status-pane", RichLog)
        status_log.clear()
        status_log.write(layout)
        log_table = self.query_one("#log-pane", DataTable)
        current_row_key = None
        if log_table.row_count > 0 and log_table.cursor_row is not None:
            try:
                current_row_key = log_table.coordinate_to_cell_key(
                    (log_table.cursor_row, 0)
                ).row_key
            except Exception:
                current_row_key = None
        log_table.clear()
        if getattr(log_table, "column_count", 0) == 0:
            log_table.add_columns("SHA", "Message")
            log_table.show_header = False
        if not log_raw:
            log_table.add_row("-", "No commits", key="NO_COMMITS")
        else:
            for line in log_raw.splitlines():
                sha, msg = (line.split("\t", 1) + [""])[:2]
                if sha:
                    log_table.add_row(Text(sha, style="yellow"), msg, key=sha)
        if current_row_key:
            try:
                index = log_table.get_row_index(current_row_key)
                log_table.move_cursor(row=index)
            except Exception:
                if log_table.row_count > 0:
                    log_table.move_cursor(row=0)
        elif log_table.row_count > 0:
            log_table.move_cursor(row=0)

    def action_cursor_down(self) -> None:
        focused = self.focused
        if isinstance(focused, RichLog):
            focused.scroll_down(animate=False)
            return
        if isinstance(focused, DataTable):
            focused.action_cursor_down()
            return
        self.query_one("#worktree-table", DataTable).action_cursor_down()

    def action_cursor_up(self) -> None:
        focused = self.focused
        if isinstance(focused, RichLog):
            focused.scroll_up(animate=False)
            return
        if isinstance(focused, DataTable):
            focused.action_cursor_up()
            return
        self.query_one("#worktree-table", DataTable).action_cursor_up()

    def action_scroll_details_down(self) -> None:
        focused = self.focused
        if isinstance(focused, RichLog):
            focused.scroll_page_down(animate=False)
            return
        if isinstance(focused, DataTable) and getattr(focused, "id", "") == "log-pane":
            action = getattr(focused, "action_page_down", None)
            if callable(action):
                action()
            else:
                focused.action_cursor_down()
            return
        self.query_one("#status-pane", RichLog).scroll_page_down(animate=False)

    def action_scroll_details_up(self) -> None:
        focused = self.focused
        if isinstance(focused, RichLog):
            focused.scroll_page_up(animate=False)
            return
        if isinstance(focused, DataTable) and getattr(focused, "id", "") == "log-pane":
            action = getattr(focused, "action_page_up", None)
            if callable(action):
                action()
            else:
                focused.action_cursor_up()
            return
        self.query_one("#status-pane", RichLog).scroll_page_up(animate=False)

    def action_cycle_focus(self) -> None:
        panes = self._focus_widgets()
        focused = self.focused
        try:
            index = panes.index(focused)
        except ValueError:
            index = 0
        next_pane = panes[(index + 1) % len(panes)]
        next_pane.focus()
        self._set_focused_pane(next_pane)

    def action_fetch(self) -> None:
        self.fetch_remotes_async()

    def action_fetch_prs(self) -> None:
        if self._pr_data_loaded:
            self.notify(
                "PR data already loaded. Use 'r' to refresh.", severity="information"
            )
            return
        self.fetch_pr_data_async()

    @work(exclusive=True)
    async def fetch_pr_data_async(self) -> None:
        self.notify("Fetching PR data from GitHub...")
        self.query_one(Header).loading = True
        success = await self.fetch_pr_data()
        self.update_table()
        self.query_one(Header).loading = False
        self.update_details_view()
        if success:
            self.notify("PR data fetched successfully!")
        else:
            self.notify("Failed to fetch PR data", severity="error")

    @work(exclusive=True)
    async def fetch_remotes_async(self) -> None:
        self.notify("Fetching all remotes...")
        self.query_one(Header).loading = True
        await self.run_git(["git", "fetch", "--all", "--quiet"], strip=False)
        self.query_one(Header).loading = False
        self.refresh_data()

    async def _get_main_worktree_path(self) -> str:
        return await self._git.get_main_worktree_path()

    async def _link_topsymlinks(self, main_path: str, target_path: str) -> None:
        try:
            process = await asyncio.create_subprocess_exec(
                "git",
                "ls-files",
                "--others",
                "--ignored",
                "--exclude-standard",
                cwd=main_path,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
            )
            stdout, _ = await process.communicate()
            out = stdout.decode(errors="replace")
            for line in out.splitlines():
                line = line.strip()
                if (
                    not line
                    or "/" in line
                    or line == ".DS_Store"
                    or ".mypy_cache" in line
                ):
                    continue
                src = os.path.join(main_path, line)
                dst = os.path.join(target_path, line)
                if os.path.exists(src) and not os.path.exists(dst):
                    try:
                        os.symlink(src, dst)
                    except OSError:
                        pass
            for editordir in [".cursor", ".claude", ".idea", ".vscode"]:
                src = os.path.join(main_path, editordir)
                dst = os.path.join(target_path, editordir)
                if os.path.isdir(src) and not os.path.exists(dst):
                    try:
                        os.symlink(src, dst)
                    except OSError:
                        pass
            os.makedirs(os.path.join(target_path, "tmp"), exist_ok=True)
            if os.path.exists(os.path.join(target_path, ".envrc")) and shutil.which(
                "direnv"
            ):
                process = await asyncio.create_subprocess_exec(
                    "direnv", "allow", ".", cwd=target_path
                )
                await process.communicate()
        except Exception as e:
            self.notify(f"Error in link_topsymlinks: {e}", severity="error")

    async def _run_wt_commands(self, commands: List[str], cwd: str, env: dict) -> None:
        for cmd in commands:
            if cmd == "link_topsymlinks":
                main_path = env.get("MAIN_WORKTREE_PATH")
                if main_path:
                    await self._link_topsymlinks(main_path, cwd)
            else:
                expanded_cmd = os.path.expandvars(cmd)
                try:
                    process = await asyncio.create_subprocess_shell(
                        expanded_cmd,
                        cwd=cwd,
                        env=env,
                        stdout=asyncio.subprocess.PIPE,
                        stderr=asyncio.subprocess.PIPE,
                    )
                    await process.communicate()
                except Exception as e:
                    self.notify(f"Error running command '{cmd}': {e}", severity="error")

    def action_sort(self) -> None:
        self.sort_by_active = not self.sort_by_active
        sort_name = "Last Active" if self.sort_by_active else "Path"
        self.notify(f"Sorting by {sort_name}")
        self.update_table()

    def action_filter(self) -> None:
        container = self.query_one("#filter-container")
        container.styles.display = "block"
        self.query_one("#filter-input").focus()

    @on(Input.Changed, "#filter-input")
    def on_filter_changed(self, event: Input.Changed) -> None:
        self.filter_query = event.value
        self.update_table()

    @on(Input.Submitted, "#filter-input")
    def on_filter_submitted(self) -> None:
        self.query_one("#filter-container").styles.display = "none"
        self.query_one("#worktree-table", DataTable).focus()

    def action_help(self) -> None:
        self.push_screen(HelpScreen())

    def action_create(self) -> None:
        async def on_submit(name: Optional[str]):
            if not name:
                return
            name = name.strip()
            if not name:
                return
            self.notify(f"Creating worktree {name}...")
            repo_key = self._get_repo_key()
            new_path_root = os.path.expanduser(f"{self.worktree_dir}/{repo_key}")
            new_path = os.path.join(new_path_root, name)
            os.makedirs(new_path_root, exist_ok=True)
            try:
                main_path = await self._get_main_worktree_path()

                # Check security/trust before doing heavy lifting if possible,
                # but 'git worktree add' creates the dir.
                # Actually, hooks run AFTER. So we can run the command first?
                # No, if the user cancels trust, we might want to abort the whole thing
                # or just not run the hooks.
                # The user intent "Cancel" usually means "Stop", but if we already created the WT...
                # Ideally, check trust first.

                repo_cmds = await self._get_repo_commands(main_path, "init_commands")
                if repo_cmds is None:
                    self.notify("Operation cancelled")
                    return

                self.log_debug(f"Creating worktree {name} at {new_path}")
                process = await asyncio.create_subprocess_exec(
                    "git",
                    "worktree",
                    "add",
                    new_path,
                    name,
                    stdout=asyncio.subprocess.PIPE,
                    stderr=asyncio.subprocess.PIPE,
                )
                stdout, stderr = await process.communicate()

                if process.returncode != 0:
                    err_msg = stderr.decode(errors="replace").strip()
                    # If the branch doesn't exist (invalid reference), try creating it as a new branch with -b
                    if "invalid reference" in err_msg:
                        self.log_debug(
                            f"Branch {name} not found, trying to create new branch..."
                        )
                        process = await asyncio.create_subprocess_exec(
                            "git",
                            "worktree",
                            "add",
                            "-b",
                            name,
                            new_path,
                            stdout=asyncio.subprocess.PIPE,
                            stderr=asyncio.subprocess.PIPE,
                        )
                        stdout, stderr = await process.communicate()
                        if process.returncode != 0:
                            err_msg = stderr.decode(errors="replace").strip()

                    if process.returncode != 0:
                        self.log_debug(
                            f"Failed to create worktree {name}. Exit code: {process.returncode}\nStderr: {err_msg}"
                        )
                        self.notify(
                            f"Failed to create worktree {name}: {err_msg}",
                            severity="error",
                        )
                        return

                self.log_debug(f"Worktree {name} created successfully")

                init_commands = list(self._config.init_commands)
                init_commands.extend(repo_cmds)

                if init_commands:
                    env = os.environ.copy()
                    env["WORKTREE_BRANCH"] = name
                    env["MAIN_WORKTREE_PATH"] = main_path
                    env["WORKTREE_PATH"] = new_path
                    env["WORKTREE_NAME"] = name
                    await self._run_wt_commands(init_commands, new_path, env)
                self.notify(f"Created worktree {name}")
                self.refresh_data()
            except Exception as e:
                self.notify(f"Error: {e}", severity="error")

        self.push_screen(
            InputScreen("Enter new branch/worktree name:"),
            lambda name: self.run_worker(on_submit(name)),
        )

    async def _apply_delta(self, diff_text: str) -> tuple[str, bool]:
        use_delta = shutil.which("delta") is not None
        if not use_delta:
            return diff_text, False
        try:
            proc = await asyncio.create_subprocess_exec(
                "delta",
                "--no-gitconfig",
                "--paging=never",
                stdin=asyncio.subprocess.PIPE,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
            )
            stdout, stderr = await proc.communicate(diff_text.encode())
            if proc.returncode == 0:
                return stdout.decode(errors="replace"), True
        except Exception:
            pass
        return diff_text, False

    async def _build_diff_text(self, path: str) -> tuple[str, bool]:
        staged_task = self.run_git(
            ["git", "diff", "--cached", "--patch", "--no-color"], cwd=path, strip=False
        )
        unstaged_task = self.run_git(
            ["git", "diff", "--patch", "--no-color"], cwd=path, strip=False
        )
        untracked_task = self.run_git(
            ["git", "ls-files", "--others", "--exclude-standard"], cwd=path
        )
        staged, unstaged, untracked = await asyncio.gather(
            staged_task, unstaged_task, untracked_task
        )
        untracked_patches: List[str] = []
        untracked_files = [f for f in untracked.splitlines() if f]
        max_untracked_diffs = self._config.max_untracked_diffs
        if max_untracked_diffs <= 0:
            if untracked_files:
                untracked_patches.append("# Note: Untracked diffs disabled\n")
            untracked_files = []
        elif len(untracked_files) > max_untracked_diffs:
            untracked_files = untracked_files[:max_untracked_diffs]
            untracked_patches.append(
                f"# Note: Showing first {max_untracked_diffs} untracked files (total: {len(untracked.splitlines())})\n"
            )
        if untracked_files:
            untracked_tasks = [
                self.run_git(
                    [
                        "git",
                        "diff",
                        "--no-index",
                        "--no-color",
                        "--",
                        "/dev/null",
                        file,
                    ],
                    cwd=path,
                    ok_returncodes=(0, 1),
                    strip=False,
                )
                for file in untracked_files
            ]
            untracked_results = await asyncio.gather(*untracked_tasks)
            untracked_patches.extend([p for p in untracked_results if p])
        parts: List[str] = []
        if staged.strip():
            parts.append("# Staged\n" + staged.strip("\n"))
        if unstaged.strip():
            parts.append("# Unstaged\n" + unstaged.strip("\n"))
        if untracked_patches:
            parts.append(
                "# Untracked\n" + "\n\n".join(p.strip("\n") for p in untracked_patches)
            )
        diff_text = "\n\n".join(parts).strip("\n")
        if not diff_text:
            return "", False
        max_chars = self._config.max_diff_chars
        if max_chars > 0 and len(diff_text) > max_chars:
            diff_text = diff_text[:max_chars] + "\n\n# [truncated]"
        return await self._apply_delta(diff_text)

    def _make_diff_panel(self, title: str, diff_text: str, use_delta: bool) -> Panel:
        renderable = (
            Text.from_ansi(diff_text)
            if use_delta
            else Syntax(diff_text, "diff", word_wrap=True)
        )
        return Panel(renderable, title=f"[bold blue]{title}[/]", expand=True)

    async def _get_commit_info(self, path: str, sha: str) -> Optional[dict]:
        fmt = "%H%n%an <%ae>%n%ad%n%s%n%b"
        info_raw = await self.run_git(
            ["git", "show", "-s", f"--format={fmt}", sha], cwd=path, strip=False
        )
        if not info_raw.strip():
            return None
        lines = info_raw.splitlines()
        if len(lines) < 4:
            return None
        return {
            "sha": lines[0].strip(),
            "author": lines[1].strip(),
            "date": lines[2].strip(),
            "subject": lines[3].strip(),
            "body": "\n".join(lines[4:]).strip(),
        }

    async def _build_commit_view(
        self, path: str, sha: str
    ) -> tuple[Optional[dict], str, bool]:
        info = await self._get_commit_info(path, sha)
        diff_raw = await self.run_git(
            ["git", "show", "--patch", "--no-color", "--pretty=format:", sha],
            cwd=path,
            strip=False,
        )
        diff_text = diff_raw.strip("\n")
        if not diff_text:
            return info, "", False
        max_chars = self._config.max_diff_chars
        if max_chars > 0 and len(diff_text) > max_chars:
            diff_text = diff_text[:max_chars] + "\n\n# [truncated]"
        diff_text, use_delta = await self._apply_delta(diff_text)
        return info, diff_text, use_delta

    def action_diff(self) -> None:
        self.open_diff_view()

    @work(exclusive=True)
    async def open_diff_view(self) -> None:
        table = self.query_one("#worktree-table", DataTable)
        if table.cursor_row is None:
            self.notify("No worktree selected", severity="warning")
            return
        row_key = table.coordinate_to_cell_key((table.cursor_row, 0)).row_key
        path = str(row_key.value)
        diff_text, use_delta = await self._build_diff_text(path)
        if not diff_text:
            self.notify("No changes in this worktree", severity="information")
            return
        title = f"Diff: {os.path.basename(path) or path}"
        renderable = self._make_diff_panel(title, diff_text, use_delta)
        status_log = self.query_one("#status-pane", RichLog)
        status_log.clear()
        status_log.write(renderable)
        status_log.scroll_home(animate=False)
        status_log.focus()
        self._set_focused_pane(status_log)

    def action_rename(self) -> None:
        table = self.query_one("#worktree-table", DataTable)
        if table.row_count == 0:
            return
        try:
            row_key = table.coordinate_to_cell_key((table.cursor_row, 0)).row_key
            path = str(row_key.value)
        except Exception:
            return
        wt = next((w for w in self.worktrees if w.path == path), None)
        if not wt:
            return
        if wt.is_main:
            self.notify("Cannot rename main worktree", severity="error")
            return

        async def do_rename(new_name: Optional[str]):
            if not new_name:
                return
            new_name = new_name.strip()
            if not new_name or new_name == wt.branch:
                return

            self.notify(f"Renaming {wt.branch} to {new_name}...")
            repo_key = self._get_repo_key()
            new_path_root = os.path.expanduser(f"{self.worktree_dir}/{repo_key}")
            new_path = os.path.join(new_path_root, new_name)

            if os.path.exists(new_path):
                self.notify(f"Destination {new_path} already exists", severity="error")
                return

            try:
                success = await self._git.rename_worktree(
                    path, new_path, wt.branch, new_name
                )
                if success:
                    self.notify(f"Renamed worktree to {new_name}")
                    self.refresh_data()
            except Exception as e:
                self.notify(f"Failed to rename: {e}", severity="error")

        self.push_screen(
            InputScreen(
                f"Enter new name for '{wt.branch}':",
                value=wt.branch,
                placeholder=wt.branch,
            ),
            lambda name: self.run_worker(do_rename(name)),
        )

    def action_delete(self) -> None:
        table = self.query_one("#worktree-table", DataTable)
        if table.row_count == 0:
            return
        try:
            row_key = table.coordinate_to_cell_key((table.cursor_row, 0)).row_key
            path = str(row_key.value)
        except Exception:
            return
        wt = next((w for w in self.worktrees if w.path == path), None)
        if not wt:
            return
        if wt.is_main:
            self.notify("Cannot delete main worktree", severity="error")
            return

        async def do_delete(confirm: Optional[bool]):
            if not confirm:
                return
            self.notify(f"Deleting {wt.branch}...")
            try:
                main_path = await self._get_main_worktree_path()

                repo_cmds = await self._get_repo_commands(
                    main_path, "terminate_commands"
                )
                if repo_cmds is None:
                    self.notify("Operation cancelled")
                    return

                terminate_commands = list(self._config.terminate_commands)
                terminate_commands.extend(repo_cmds)

                if terminate_commands:
                    env = os.environ.copy()
                    env["WORKTREE_BRANCH"] = wt.branch
                    env["MAIN_WORKTREE_PATH"] = main_path
                    env["WORKTREE_PATH"] = path
                    env["WORKTREE_NAME"] = os.path.basename(path)
                    await self._run_wt_commands(terminate_commands, main_path, env)
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
            except Exception as e:
                self.notify(f"Failed to delete: {e}", severity="error")

        self.push_screen(
            ConfirmScreen(
                f"Are you sure you want to delete worktree?\n\nPath: {path}\nBranch: {wt.branch}"
            ),
            lambda confirm: self.run_worker(do_delete(confirm)),
        )

    def action_absorb(self) -> None:
        table = self.query_one("#worktree-table", DataTable)
        if table.row_count == 0:
            return
        try:
            row_key = table.coordinate_to_cell_key((table.cursor_row, 0)).row_key
            path = str(row_key.value)
        except Exception:
            return
        wt = next((w for w in self.worktrees if w.path == path), None)
        if not wt:
            return
        if wt.is_main:
            self.notify("Cannot absorb main worktree", severity="error")
            return

        async def do_absorb(confirm: Optional[bool]):
            if not confirm:
                return
            self.notify(f"Absorbing {wt.branch}...")
            try:
                main_path = await self._get_main_worktree_path()

                repo_cmds = await self._get_repo_commands(
                    main_path, "terminate_commands"
                )
                if repo_cmds is None:
                    self.notify("Operation cancelled")
                    return

                terminate_commands = list(self._config.terminate_commands)
                terminate_commands.extend(repo_cmds)

                if terminate_commands:
                    env = os.environ.copy()
                    env["WORKTREE_BRANCH"] = wt.branch
                    env["MAIN_WORKTREE_PATH"] = main_path
                    env["WORKTREE_PATH"] = path
                    env["WORKTREE_NAME"] = os.path.basename(path)
                    await self._run_wt_commands(terminate_commands, main_path, env)
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
            except Exception as e:
                self.notify(f"Failed to absorb: {e}", severity="error")

        self.push_screen(
            ConfirmScreen(
                f"Absorb worktree to main branch?\n\nPath: {path}\nBranch: {wt.branch}\n\nThis will merge changes to main and delete the worktree."
            ),
            lambda confirm: self.run_worker(do_absorb(confirm)),
        )

    def action_lazygit(self) -> None:
        table = self.query_one("#worktree-table", DataTable)
        if table.cursor_row is None:
            self.notify("No worktree selected", severity="warning")
            return
        row_key = table.coordinate_to_cell_key((table.cursor_row, 0)).row_key
        path = str(row_key.value)
        if shutil.which("lazygit") is None:
            self.notify(
                "`lazygit` not found in PATH (required for `g`)", severity="error"
            )
            return
        try:
            suspend_process = getattr(self, "suspend_process", None) or getattr(
                self.app, "suspend_process", None
            )
            if callable(suspend_process):
                suspend_process(subprocess.run, ["lazygit"], cwd=path)
            else:
                with self.suspend():
                    subprocess.run(["lazygit"], cwd=path, check=False)
            self.refresh_data()
        except Exception as e:
            self.notify(f"Failed to run lazygit: {e}", severity="error")

    def action_jump(self) -> None:
        focused = self.focused
        if isinstance(focused, DataTable) and getattr(focused, "id", "") == "log-pane":
            self.open_commit_view()
            return
        table = self.query_one("#worktree-table", DataTable)
        if table.row_count > 0:
            row_key = table.coordinate_to_cell_key((table.cursor_row, 0)).row_key
            path = str(row_key.value)
            self._select_worktree(path)

    @work(exclusive=True)
    async def open_commit_view(self) -> None:
        log_table = self.query_one("#log-pane", DataTable)
        if log_table.cursor_row is None or log_table.row_count == 0:
            self.notify("No commit selected", severity="warning")
            return
        try:
            row_key = log_table.coordinate_to_cell_key(
                (log_table.cursor_row, 0)
            ).row_key
            sha = str(row_key.value)
        except Exception:
            self.notify("No commit selected", severity="warning")
            return
        if not sha or sha == "NO_COMMITS":
            self.notify("No commit selected", severity="warning")
            return
        path = self._selected_worktree_path()
        if not path:
            self.notify("No worktree selected", severity="warning")
            return
        info, diff_text, use_delta = await self._build_commit_view(path, sha)
        if not info and not diff_text:
            self.notify("No commit content found", severity="information")
            return
        header_grid = Table.grid(padding=(0, 1))
        header_grid.add_column(style="bold blue", no_wrap=True)
        header_grid.add_column()
        if info:
            header_grid.add_row("Commit:", f"[yellow]{info['sha']}[/]")
            header_grid.add_row("Author:", info["author"])
            header_grid.add_row("Date:", info["date"])
            header_grid.add_row("Subject:", f"[white]{info['subject']}[/]")
            if info["body"]:
                header_grid.add_row("Message:", info["body"])
        header_panel = Panel(header_grid, title="[bold blue]Commit[/]")
        diff_panel = self._make_diff_panel("Diff", diff_text or "No diff", use_delta)
        self.push_screen(CommitScreen(header_panel, diff_panel))
