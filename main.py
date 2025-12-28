#!/usr/bin/env -S uv --quiet run --script
# /// script
# requires-python = ">=3.12"
# dependencies = [
#     "textual",
#     "rich",
#     "PyYAML",
# ]
# ///
#
import asyncio
import json
import os
import subprocess
import sys
import webbrowser
import shutil
import re
import yaml
from dataclasses import dataclass
from typing import List, Optional, Iterable


from textual import on, work, events
from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.command import DiscoveryHit, Hit, Provider
from textual.containers import Container, Horizontal, Vertical, VerticalScroll
from textual.screen import ModalScreen
from textual.widgets import (
    Button,
    DataTable,
    Footer,
    Header,
    Input,
    Label,
    Static,
    Markdown,
    RichLog,
)
from rich.panel import Panel
from rich.text import Text
from rich.table import Table
from rich.console import Group
from rich.syntax import Syntax

WORKTREE_DIR = "~/.local/share/worktrees"
LAST_SELECTED_FILENAME = ".last-selected"
CACHE_FILENAME = ".worktree-cache.json"


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
        """Create a callback for the given action."""

        def callback():
            action = getattr(self.app, f"action_{action_name}", None)
            if action is None:
                return
            result = action()
            if asyncio.iscoroutine(result):
                asyncio.create_task(result)

        return callback

    async def discover(self):
        """Show all commands when palette is opened with no query."""
        for name, action, help_text in self.COMMANDS:
            yield DiscoveryHit(
                name,
                self._make_callback(action),
                help=help_text,
            )

    async def search(self, query: str):
        """Search for commands matching the query."""
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


@dataclass
class PRInfo:
    number: int
    state: str
    title: str
    url: str


@dataclass
class WorktreeInfo:
    path: str
    branch: str
    is_main: bool
    dirty: bool
    ahead: int = 0
    behind: int = 0
    last_active: str = ""
    last_active_ts: int = 0
    pr: Optional[PRInfo] = None
    untracked: int = 0
    modified: int = 0
    staged: int = 0
    divergence: str = ""


class ConfirmScreen(ModalScreen[bool]):
    """A screen for confirmation dialogs."""

    CSS = """
    ConfirmScreen {
        align: center middle;
    }
    #dialog {
        grid-size: 2;
        grid-gutter: 1 2;
        grid-rows: 1fr 3;
        padding: 0 1;
        width: 60;
        height: 11;
        border: thick $background 80%;
        background: $surface;
    }
    #question {
        column-span: 2;
        height: 1fr;
        content-align: center middle;
    }
    Button {
        width: 100%;
    }
    """

    def __init__(self, message: str):
        super().__init__()
        self.message = message

    def compose(self) -> ComposeResult:
        with Container(id="dialog"):
            yield Label(self.message, id="question")
            yield Button("Cancel", variant="primary", id="cancel")
            yield Button("Confirm", variant="error", id="confirm")

    @on(Button.Pressed, "#cancel")
    def cancel(self):
        self.dismiss(False)

    @on(Button.Pressed, "#confirm")
    def confirm(self):
        self.dismiss(True)


class InputScreen(ModalScreen[str]):
    """A screen for text input."""

    CSS = """
    InputScreen {
        align: center middle;
    }
    #dialog {
        width: 60;
        height: auto;
        border: thick $background 80%;
        background: $surface;
        padding: 1 2;
    }
    Label {
        margin-bottom: 1;
    }
    """

    def __init__(self, prompt: str, placeholder: str = ""):
        super().__init__()
        self.prompt = prompt
        self.placeholder = placeholder

    def compose(self) -> ComposeResult:
        with Container(id="dialog"):
            yield Label(self.prompt)
            yield Input(placeholder=self.placeholder)

    @on(Input.Submitted)
    def submit(self, event: Input.Submitted):
        self.dismiss(event.value)

    def on_key(self, event):
        if event.key == "escape":
            self.dismiss(None)


class HelpScreen(ModalScreen):
    """Screen to show help."""

    CSS = """
    HelpScreen {
        align: center middle;
    }
    #help-container {
        width: 80;
        height: 80%;
        border: thick $primary;
        background: $surface;
        padding: 1 2;
    }
    """

    BINDINGS = [("escape", "dismiss", "Close")]

    def compose(self) -> ComposeResult:
        help_text = """
# Git Worktree Status Help

**Navigation**
- `j` / `Down`: Move cursor down
- `k` / `Up`: Move cursor up
- `1`: Focus Worktree pane
- `2`: Focus Info/Diff pane
- `3`: Focus Log pane
- `Enter`: Jump to selected worktree (exit and cd)
- `Tab`: Cycle focus (table → status → log)
- `j` / `k` in Recent Log: Move between commits
- `Enter` in Recent Log: Open commit details and diff
- `Ctrl+/`: Open command palette

**Actions**
- `c`: Create new worktree
- `d`: Refresh diff in the status pane (auto-shown when dirty; uses delta if available)
- `D`: Delete selected worktree
- `f`: Fetch all remotes
- `p`: Fetch PR status from GitHub
- `r`: Refresh list
- `s`: Sort (toggle Name/Last Active)
- `/`: Filter worktrees
- `g`: Open LazyGit
- `?`: Show this help

**Status Indicators**
- `✔ Clean`: No local changes
- `✎ Dirty`: Uncommitted changes
- `↑N`: Ahead of remote by N commits
- `↓N`: Behind remote by N commits

**Performance Note**
PR data is not fetched by default for speed.
Press `p` to fetch PR information from GitHub.

**Command Palette**
Press `Ctrl+/` to open the command palette and search for any action.
        """
        with Container(id="help-container"):
            yield Markdown(help_text)
            yield Button("Close", variant="primary", id="close")

    @on(Button.Pressed, "#close")
    def action_dismiss(self):
        self.dismiss()


class DiffScreen(ModalScreen[None]):
    CSS = """
    DiffScreen {
        align: center middle;
    }
    #dialog {
        width: 95%;
        height: 95%;
        border: thick $background 80%;
        background: $surface;
        layout: vertical;
    }
    #content {
        height: 1fr;
        width: 1fr;
        padding: 0 1;
    }
    #diff-content {
        width: 100%;
    }
    """

    BINDINGS = [
        Binding("q", "close", "Close"),
        Binding("esc", "close", show=False),
        Binding("j", "scroll_down", "Down", show=False),
        Binding("k", "scroll_up", "Up", show=False),
        Binding("down", "scroll_down", "Down", show=False),
        Binding("up", "scroll_up", "Up", show=False),
        Binding("ctrl+d", "page_down", "Page Down", show=False),
        Binding("ctrl+u", "page_up", "Page Up", show=False),
        Binding("space", "page_down", "Page Down", show=False),
        Binding("g", "scroll_top", "Top", show=False, priority=True),
        Binding("G", "scroll_bottom", "Bottom", show=False, priority=True),
    ]

    def __init__(self, title: str, diff_text: str, use_delta: bool = False):
        super().__init__()
        self._title = title
        self._diff_text = diff_text
        self._use_delta = use_delta

    def compose(self) -> ComposeResult:
        with Container(id="dialog"):
            with VerticalScroll(id="content"):
                yield Static(id="diff-content")

    def on_mount(self) -> None:
        if self._use_delta:
            # Delta output contains ANSI codes, use Text to preserve colors
            from rich.text import Text

            text = Text.from_ansi(self._diff_text)
            renderable = Panel(
                text,
                title=f"[bold blue]{self._title}[/]",
                expand=True,
            )
        else:
            # Use syntax highlighting for plain diff
            renderable = Panel(
                Syntax(self._diff_text, "diff", word_wrap=True),
                title=f"[bold blue]{self._title}[/]",
                expand=True,
            )
        self.query_one("#diff-content", Static).update(renderable)
        self.query_one("#content", VerticalScroll).focus()

    def action_close(self) -> None:
        self.dismiss(None)

    def action_scroll_down(self) -> None:
        self.query_one("#content", VerticalScroll).scroll_down(animate=False)

    def action_scroll_up(self) -> None:
        self.query_one("#content", VerticalScroll).scroll_up(animate=False)

    def action_page_down(self) -> None:
        self.query_one("#content", VerticalScroll).scroll_page_down(animate=False)

    def action_page_up(self) -> None:
        self.query_one("#content", VerticalScroll).scroll_page_up(animate=False)

    def action_scroll_top(self) -> None:
        self.query_one("#content", VerticalScroll).scroll_home(animate=False)

    def action_scroll_bottom(self) -> None:
        self.query_one("#content", VerticalScroll).scroll_end(animate=False)


class CommitScreen(ModalScreen[None]):
    CSS = """
    CommitScreen {
        align: center middle;
    }
    #dialog {
        width: 95%;
        height: 95%;
        border: thick $background 80%;
        background: $surface;
        layout: vertical;
    }
    #header {
        height: auto;
        padding: 0 1;
    }
    #diff {
        height: 1fr;
        width: 1fr;
        padding: 0 1;
    }
    #diff-content {
        width: 100%;
    }
    """

    BINDINGS = [
        Binding("q", "close", "Close"),
        Binding("esc", "close", show=False),
        Binding("j", "scroll_down", "Down", show=False),
        Binding("k", "scroll_up", "Up", show=False),
        Binding("down", "scroll_down", "Down", show=False),
        Binding("up", "scroll_up", "Up", show=False),
        Binding("ctrl+d", "page_down", "Page Down", show=False),
        Binding("ctrl+u", "page_up", "Page Up", show=False),
        Binding("space", "page_down", "Page Down", show=False),
        Binding("g", "scroll_top", "Top", show=False),
        Binding("G", "scroll_bottom", "Bottom", show=False),
    ]

    def __init__(self, header_panel, diff_renderable):
        super().__init__()
        self._header_panel = header_panel
        self._diff_renderable = diff_renderable
        self._header_collapsed = False

    def compose(self) -> ComposeResult:
        with Container(id="dialog"):
            yield Static(id="header")
            with CommitDiffScroll(id="diff"):
                yield Static(id="diff-content")

    def on_mount(self) -> None:
        self.query_one("#header", Static).update(self._header_panel)
        self.query_one("#diff-content", Static).update(self._diff_renderable)
        self.query_one("#diff", VerticalScroll).focus()
        self._set_header_collapsed(False)

    def _set_header_collapsed(self, collapsed: bool) -> None:
        if collapsed == self._header_collapsed:
            return
        header = self.query_one("#header", Static)
        header.styles.display = "none" if collapsed else "block"
        self._header_collapsed = collapsed

    def action_close(self) -> None:
        self.dismiss(None)

    def action_scroll_down(self) -> None:
        self.query_one("#diff", VerticalScroll).scroll_down(animate=False)

    def action_scroll_up(self) -> None:
        self.query_one("#diff", VerticalScroll).scroll_up(animate=False)

    def action_page_down(self) -> None:
        self.query_one("#diff", VerticalScroll).scroll_page_down(animate=False)

    def action_page_up(self) -> None:
        self.query_one("#diff", VerticalScroll).scroll_page_up(animate=False)

    def action_scroll_top(self) -> None:
        self.query_one("#diff", VerticalScroll).scroll_home(animate=False)

    def action_scroll_bottom(self) -> None:
        self.query_one("#diff", VerticalScroll).scroll_end(animate=False)


class FocusableRichLog(RichLog):
    can_focus = True


class CommitDiffScroll(VerticalScroll):
    can_focus = True

    def on_key(self, event) -> None:
        key = event.key
        if key in {"j", "down"}:
            self.scroll_down(animate=False)
            self._sync_header()
            event.stop()
        elif key in {"k", "up"}:
            self.scroll_up(animate=False)
            self._sync_header()
            event.stop()
        elif key == "ctrl+d":
            self.scroll_page_down(animate=False)
            self._sync_header()
            event.stop()
        elif key == "ctrl+u":
            self.scroll_page_up(animate=False)
            self._sync_header()
            event.stop()
        elif key == "space":
            self.scroll_page_down(animate=False)
            self._sync_header()
            event.stop()
        elif key == "g":
            self.scroll_home(animate=False)
            self._sync_header()
            event.stop()
        elif key == "G":
            self.scroll_end(animate=False)
            self._sync_header()
            event.stop()

    def _sync_header(self) -> None:
        screen = getattr(self, "screen", None)
        if screen and hasattr(screen, "_set_header_collapsed"):
            screen._set_header_collapsed(self.scroll_y > 0)


class GitWtStatus(App):
    TITLE = "Git Worktree Status"
    COMMANDS = {GitWtStatusCommands}
    CSS = """
    #main-content {
        height: 1fr;
    }

    #right-pane {
        width: 2fr;
        height: 100%;
    }

    #worktree-table {
        width: 3fr;
        height: 100%;
        border: solid $secondary;
    }

    #status-pane {
        width: 1fr;
        background: $surface-darken-1;
        padding: 0 1;
        border: solid $secondary;
    }

    #log-pane {
        width: 1fr;
        background: $surface-darken-1;
        padding: 0 1;
        border: solid $secondary;
    }

    #status-pane { height: 2fr; }
    #log-pane { height: 1fr; }

    #worktree-table.compact {
        width: 1fr;
    }

    #right-pane.expanded {
        width: 3fr;
    }

    .focused {
        border: solid $primary;
    }

    #filter-container {
        height: 3;
        dock: top;
        display: none;
    }

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
        self._semaphore = asyncio.Semaphore(24)  # Increased for better concurrency
        self._repo_key: Optional[str] = None
        self._cache: dict = {}
        self._divergence_cache: dict = {}  # Cache divergence calculations

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
        except Exception:
            repo_name = ""

        if not repo_name:
            try:
                remote_url = subprocess.check_output(
                    ["git", "remote", "get-url", "origin"],
                    text=True,
                    stderr=subprocess.DEVNULL,
                ).strip()
                match = re.search(r"[:/]([^/]+/[^/]+)(\\.git)?$", remote_url)
                if match:
                    repo_name = match.group(1)
            except Exception:
                pass

        if not repo_name:
            try:
                toplevel = subprocess.check_output(
                    ["git", "rev-parse", "--show-toplevel"],
                    text=True,
                    stderr=subprocess.DEVNULL,
                ).strip()
                repo_name = os.path.basename(toplevel)
            except Exception:
                repo_name = "unknown"

        return repo_name or "unknown"

    def _get_repo_key(self) -> str:
        if self._repo_key:
            return self._repo_key
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
        """Load cached worktree data for faster startup."""
        try:
            cache_path = self._cache_file()
            if os.path.exists(cache_path):
                with open(cache_path, "r", encoding="utf-8") as f:
                    return json.load(f)
        except Exception:
            pass
        return {}

    def _save_cache(self, data: dict) -> None:
        """Save worktree data to cache."""
        try:
            cache_path = self._cache_file()
            os.makedirs(os.path.dirname(cache_path), exist_ok=True)
            with open(cache_path, "w", encoding="utf-8") as f:
                json.dump(data, f)
        except Exception:
            pass

    def _write_last_selected(self, path: str) -> None:
        if not path:
            return
        last_selected = self._last_selected_file()
        try:
            os.makedirs(os.path.dirname(last_selected), exist_ok=True)
            with open(last_selected, "w", encoding="utf-8") as handle:
                handle.write(f"{path}\n")
        except Exception:
            pass

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
        try:
            proc = await asyncio.create_subprocess_exec(
                *args,
                cwd=cwd,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
            )
            stdout, stderr = await proc.communicate()
            if proc.returncode not in set(ok_returncodes):
                # Log error if needed, but for now just return empty or raise
                # print(f"Error running {args}: {stderr.decode()}")
                return ""
            out = stdout.decode(errors="replace")
            return out.strip() if strip else out
        except Exception:
            return ""

    async def get_main_branch(self) -> str:
        if self._main_branch:
            return self._main_branch
        try:
            out = await self.run_git(
                ["git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD"]
            )
            if out:
                self._main_branch = out.split("/")[-1]
            else:
                self._main_branch = "main"
        except Exception:
            self._main_branch = "main"
        return self._main_branch

    async def get_worktrees(self) -> List[WorktreeInfo]:
        try:
            raw_wts = await self.run_git(["git", "worktree", "list", "--porcelain"])
        except Exception:
            return []

        wts = []
        current_wt = {}
        for line in raw_wts.splitlines():
            if line.startswith("worktree "):
                if current_wt:
                    wts.append(current_wt)
                current_wt = {"path": line.split(" ", 1)[1]}
            elif line.startswith("branch "):
                current_wt["branch"] = line.split(" ", 1)[1].replace("refs/heads/", "")
        if current_wt:
            wts.append(current_wt)

        for i, wt_data in enumerate(wts):
            wt_data["is_main"] = i == 0

        # Fetch branch info only (no PR data by default for speed)
        branch_raw = await self.run_git(
            [
                "git",
                "for-each-ref",
                "--format=%(refname:short)|%(committerdate:relative)|%(committerdate:unix)",
                "refs/heads",
            ]
        )

        pr_map = {}

        branch_info = {}
        for line in branch_raw.splitlines():
            if "|" in line:
                parts = line.split("|")
                if len(parts) == 3:
                    branch_info[parts[0]] = (parts[1], int(parts[2]))

        async def get_wt_info(wt_data):
            async with self._semaphore:
                path = wt_data["path"]
                branch = wt_data.get("branch", "(detached)")

                status_raw = await self.run_git(
                    ["git", "status", "--porcelain=v2", "--branch"], cwd=path
                )

                ahead = 0
                behind = 0
                untracked = 0
                modified = 0
                staged = 0

                for line in status_raw.splitlines():
                    if line.startswith("# branch.ab "):
                        parts = line.split()
                        if len(parts) >= 4:
                            ahead = int(parts[2].replace("+", ""))
                            behind = int(parts[3].replace("-", ""))
                    elif line.startswith("?"):
                        untracked += 1
                    elif line.startswith("1 ") or line.startswith("2 "):
                        if len(line.split()) > 1:
                            xy = line.split()[1]
                            if len(xy) >= 2:
                                if xy[0] != ".":
                                    staged += 1
                                if xy[1] != ".":
                                    modified += 1

                # Use cached branch info if available (prioritize for-each-ref data)
                last_active, last_active_ts = branch_info.get(branch, ("", 0))
                # Skip redundant git log call if we have branch info

                return WorktreeInfo(
                    path=path,
                    branch=branch,
                    is_main=wt_data["is_main"],
                    dirty=(untracked + modified + staged > 0),
                    ahead=ahead,
                    behind=behind,
                    last_active=last_active,
                    last_active_ts=last_active_ts,
                    pr=pr_map.get(branch),
                    untracked=untracked,
                    modified=modified,
                    staged=staged,
                    divergence="",  # Calculated lazily in details view
                )

        return await asyncio.gather(*(get_wt_info(wt) for wt in wts))

    async def fetch_pr_data(self) -> None:
        """Fetch PR data from GitHub and update worktrees."""
        pr_raw = await self.run_git(
            [
                "gh",
                "pr",
                "list",
                "--state",
                "all",
                "--json",
                "headRefName,state,number,title,url",
                "--limit",
                "100",
            ]
        )

        pr_map = {}
        if pr_raw:
            try:
                prs = json.loads(pr_raw)
                for p in prs:
                    pr_map[p["headRefName"]] = PRInfo(
                        p["number"], p["state"], p["title"], p["url"]
                    )
            except Exception:
                pass

        # Update existing worktrees with PR info
        for wt in self.worktrees:
            if wt.branch in pr_map:
                wt.pr = pr_map[wt.branch]

        self._pr_data_loaded = True

    @work(exclusive=True)
    async def refresh_data(self) -> None:
        self.query_one(Header).loading = True
        self._pr_data_loaded = False  # Reset PR data flag on refresh

        # Load cache for faster startup
        self._cache = self._load_cache()

        self.worktrees = await self.get_worktrees()

        # Save cache after fetching
        cache_data = {
            "worktrees": [
                {
                    "path": wt.path,
                    "branch": wt.branch,
                    "last_active_ts": wt.last_active_ts,
                }
                for wt in self.worktrees
            ]
        }
        self._save_cache(cache_data)

        self.update_table()
        self.query_one(Header).loading = False
        self.update_details_view()

    def update_table(self):
        table = self.query_one("#worktree-table", DataTable)

        # Save current selection
        current_row_key = None
        if table.row_count > 0 and table.cursor_row is not None:
            try:
                current_row_key = table.coordinate_to_cell_key(
                    (table.cursor_row, 0)
                ).row_key
            except:
                pass

        table.clear()

        # Filter
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

        # Sort
        if self.sort_by_active:
            filtered_wts.sort(key=lambda x: x.last_active_ts, reverse=True)
        else:
            filtered_wts.sort(key=lambda x: x.path)

        for wt in filtered_wts:
            name = os.path.basename(wt.path) if not wt.is_main else "main"

            status_parts = []
            if wt.dirty:
                status_parts.append("[yellow]✎[/]")
            else:
                status_parts.append("[green]✔[/]")

            status_str = " ".join(status_parts)

            ab_str = ""
            if wt.ahead:
                ab_str += f"[cyan]↑{wt.ahead}[/] "
            if wt.behind:
                ab_str += f"[red]↓{wt.behind}[/] "
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

        # Restore selection if possible
        if current_row_key:
            try:
                index = table.get_row_index(current_row_key)
                table.move_cursor(row=index)
            except:
                pass

    def on_data_table_row_selected(self, event: DataTable.RowSelected) -> None:
        table = self.query_one("#worktree-table", DataTable)
        log_table = self.query_one("#log-pane", DataTable)
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
        table = self.query_one("#worktree-table", DataTable)
        data_table = getattr(event, "data_table", None) or getattr(
            event, "control", None
        )
        if data_table is not None and data_table is not table:
            return
        self.update_details_view()

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
        # Debounce slightly? Textual handles exclusive workers by cancelling previous ones
        # so this acts as a debounce if the user scrolls fast.
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

        # Fetch details in parallel
        status_task = self.run_git(["git", "status", "--short"], cwd=path)
        log_task = self.run_git(
            ["git", "log", "-20", "--pretty=format:%h%x09%s"], cwd=path
        )

        async def get_div():
            # Check permanent cache first
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
                    # Cache permanently
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

        # Build UI
        grid = Table.grid(padding=(0, 2))
        grid.add_column(style="bold blue", justify="right", no_wrap=True)
        grid.add_column()

        grid.add_row("Path:", f"[blue]{path}[/]")
        grid.add_row("Branch:", f"[yellow]{wt.branch}[/]")
        if wt.divergence:
            div = wt.divergence.replace("↑", "[cyan]↑[/]").replace("↓", "[red]↓[/]")
            grid.add_row("Divergence:", div)

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

        # Status Panel
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
            layout = Group(
                Panel(grid, title="[bold blue]Info[/]"),
                diff_panel,
            )
        else:
            layout = Group(
                Panel(grid, title="[bold blue]Info[/]"),
                status_panel,
            )

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
        self.notify("Fetching all remotes...")
        subprocess.run(["git", "fetch", "--all", "--quiet"])
        self.refresh_data()

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
        await self.fetch_pr_data()
        self.update_table()
        self.query_one(Header).loading = False
        self.update_details_view()
        self.notify("PR data fetched successfully!")

    async def _get_main_worktree_path(self) -> str:
        """Find the main worktree path."""
        try:
            raw_wts = await self.run_git(["git", "worktree", "list", "--porcelain"])
            for line in raw_wts.splitlines():
                if line.startswith("worktree "):
                    return line.split(" ", 1)[1]
        except Exception:
            pass
        return os.getcwd()

    async def _link_topsymlinks(self, main_path: str, target_path: str) -> None:
        """Symlink untracked/ignored files and editor configs from main worktree."""
        try:
            # Get ignored/untracked files from main worktree (root only)
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
                # Skip subdirectories and specific files
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

            # Symlink editor configs
            for editordir in [".cursor", ".claude", ".idea", ".vscode"]:
                src = os.path.join(main_path, editordir)
                dst = os.path.join(target_path, editordir)
                if os.path.isdir(src) and not os.path.exists(dst):
                    try:
                        os.symlink(src, dst)
                    except OSError:
                        pass

            # Ensure tmp directory exists
            os.makedirs(os.path.join(target_path, "tmp"), exist_ok=True)

            # Direnv support
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
        """Run initialization or termination commands from .wt config."""
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
            new_path_root = os.path.expanduser(f"{WORKTREE_DIR}/{repo_key}")
            new_path = os.path.join(new_path_root, name)
            os.makedirs(new_path_root, exist_ok=True)

            try:
                main_path = await self._get_main_worktree_path()
                # Run git worktree add
                process = await asyncio.create_subprocess_exec(
                    "git", "worktree", "add", new_path, name
                )
                await process.communicate()

                if process.returncode != 0:
                    self.notify(f"Failed to create worktree {name}", severity="error")
                    return

                # Load .wt config
                config_path = os.path.join(main_path, ".wt")
                if os.path.exists(config_path):
                    try:
                        with open(config_path, "r") as f:
                            config = yaml.safe_load(f)
                        init_commands = config.get("init_commands", [])
                        env = os.environ.copy()
                        env["WORKTREE_BRANCH"] = name
                        env["MAIN_WORKTREE_PATH"] = main_path
                        env["WORKTREE_PATH"] = new_path
                        env["WORKTREE_NAME"] = name
                        await self._run_wt_commands(init_commands, new_path, env)
                    except Exception as config_err:
                        self.notify(
                            f"Error loading .wt config: {config_err}", severity="error"
                        )

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
        # Fetch staged and unstaged diffs in parallel
        staged_task = self.run_git(
            ["git", "diff", "--cached", "--patch", "--no-color"],
            cwd=path,
            strip=False,
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

        # Limit untracked file diffs for performance (max 10 files)
        max_untracked_diffs = 10
        if len(untracked_files) > max_untracked_diffs:
            # Show first 10 and add a note
            untracked_files = untracked_files[:max_untracked_diffs]
            untracked_patches.append(
                f"# Note: Showing first {max_untracked_diffs} untracked files (total: {len(untracked.splitlines())})\n"
            )

        # Batch untracked file diffs in parallel
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

        max_chars = 200_000
        if len(diff_text) > max_chars:
            diff_text = diff_text[:max_chars] + "\n\n# [truncated]"

        return await self._apply_delta(diff_text)

    def _make_diff_panel(self, title: str, diff_text: str, use_delta: bool) -> Panel:
        if use_delta:
            renderable = Text.from_ansi(diff_text)
        else:
            renderable = Syntax(diff_text, "diff", word_wrap=True)
        return Panel(renderable, title=f"[bold blue]{title}[/]", expand=True)

    async def _get_commit_info(self, path: str, sha: str) -> Optional[dict]:
        fmt = "%H%n%an <%ae>%n%ad%n%s%n%b"
        info_raw = await self.run_git(
            ["git", "show", "-s", f"--format={fmt}", sha],
            cwd=path,
            strip=False,
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
        max_chars = 200_000
        if len(diff_text) > max_chars:
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

    def action_delete(self) -> None:
        table = self.query_one("#worktree-table", DataTable)
        if table.row_count == 0:
            return

        try:
            row_key = table.coordinate_to_cell_key((table.cursor_row, 0)).row_key
            path = str(row_key.value)
        except:
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
                config_path = os.path.join(main_path, ".wt")
                if os.path.exists(config_path):
                    try:
                        with open(config_path, "r") as f:
                            config = yaml.safe_load(f)
                        terminate_commands = config.get("terminate_commands", [])
                        env = os.environ.copy()
                        env["WORKTREE_BRANCH"] = wt.branch
                        env["MAIN_WORKTREE_PATH"] = main_path
                        env["WORKTREE_PATH"] = path
                        env["WORKTREE_NAME"] = os.path.basename(path)
                        await self._run_wt_commands(terminate_commands, main_path, env)
                    except Exception as config_err:
                        self.notify(
                            f"Error loading .wt config: {config_err}", severity="error"
                        )

                process = await asyncio.create_subprocess_exec(
                    "git", "worktree", "remove", "--force", path
                )
                await process.communicate()
                process = await asyncio.create_subprocess_exec(
                    "git", "branch", "-D", wt.branch
                )
                await process.communicate()
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
        except:
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
                config_path = os.path.join(main_path, ".wt")
                if os.path.exists(config_path):
                    try:
                        with open(config_path, "r") as f:
                            config = yaml.safe_load(f)
                        terminate_commands = config.get("terminate_commands", [])
                        env = os.environ.copy()
                        env["WORKTREE_BRANCH"] = wt.branch
                        env["MAIN_WORKTREE_PATH"] = main_path
                        env["WORKTREE_PATH"] = path
                        env["WORKTREE_NAME"] = os.path.basename(path)
                        await self._run_wt_commands(terminate_commands, main_path, env)
                    except Exception as config_err:
                        self.notify(
                            f"Error loading .wt config: {config_err}", severity="error"
                        )

                main_branch = await self.get_main_branch()
                process = await asyncio.create_subprocess_exec(
                    "git", "checkout", main_branch, cwd=path
                )
                await process.communicate()
                process = await asyncio.create_subprocess_exec(
                    "git", "merge", "--no-edit", wt.branch, cwd=path
                )
                await process.communicate()
                process = await asyncio.create_subprocess_exec(
                    "git", "worktree", "remove", "--force", path
                )
                await process.communicate()
                process = await asyncio.create_subprocess_exec(
                    "git", "branch", "-D", wt.branch
                )
                await process.communicate()
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
            # Textual v6+ uses suspend() context manager; older versions had suspend_process().
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


def main():
    initial_filter = " ".join(sys.argv[1:]).strip()
    app = GitWtStatus(initial_filter=initial_filter)
    run_result = app.run()
    if run_result is None:
        sys.exit(0)
    result = run_result
    if result:
        print(result)


if __name__ == "__main__":
    main()
# vim: ft=python
