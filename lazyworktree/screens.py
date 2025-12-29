from textual import on
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Container, VerticalScroll
from textual.screen import ModalScreen
from textual.widgets import (
    Button,
    Input,
    Label,
    Static,
    Markdown,
    RichLog,
)
from rich.panel import Panel
from rich.text import Text
from rich.syntax import Syntax


class ConfirmScreen(ModalScreen[bool]):
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
            text = Text.from_ansi(self._diff_text)
            renderable = Panel(text, title=f"[bold blue]{self._title}[/]", expand=True)
        else:
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


class TrustScreen(ModalScreen[str]):
    CSS = """
    TrustScreen {
        align: center middle;
    }
    #dialog {
        grid-size: 2;
        grid-gutter: 1 2;
        grid-rows: auto 1fr auto;
        padding: 0 1;
        width: 70;
        height: 80%;
        border: thick $background 80%;
        background: $surface;
    }
    #question {
        column-span: 2;
        content-align: center middle;
        padding: 1;
    }
    #commands-view {
        column-span: 2;
        border: solid $secondary;
        height: 1fr;
        margin: 1;
    }
    #buttons {
        column-span: 2;
        layout: horizontal;
        align: center middle;
        height: auto;
    }
    Button {
        margin: 0 1;
    }
    """

    def __init__(self, file_path: str, commands: list[str]):
        super().__init__()
        self.file_path = file_path
        self.commands_text = "\n".join(commands)

    def compose(self) -> ComposeResult:
        with Container(id="dialog"):
            yield Label(
                f"The repository config '{self.file_path}' defines the following commands.\n"
                "This file has changed or hasn't been trusted yet.\n"
                "Do you trust these commands to run?",
                id="question",
            )
            with VerticalScroll(id="commands-view"):
                yield Static(
                    Syntax(self.commands_text, "bash", theme="monokai", word_wrap=True)
                )
            with Container(id="buttons"):
                yield Button("Trust & Run", variant="success", id="trust")
                yield Button("Block (Skip)", variant="warning", id="block")
                yield Button("Cancel Operation", variant="error", id="cancel")

    @on(Button.Pressed, "#trust")
    def action_trust(self):
        self.dismiss("trust")

    @on(Button.Pressed, "#block")
    def action_block(self):
        self.dismiss("block")

    @on(Button.Pressed, "#cancel")
    def action_cancel(self):
        self.dismiss("cancel")


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


class WelcomeScreen(ModalScreen):
    CSS = """
    WelcomeScreen {
        align: center middle;
    }
    #dialog {
        grid-size: 1;
        grid-gutter: 1;
        grid-rows: auto auto auto 1fr;
        padding: 1 2;
        width: 70;
        height: 60%;
        border: thick $primary;
        background: $surface;
    }
    #title {
        content-align: center middle;
        text-style: bold;
        color: $accent;
        height: 3;
    }
    #message {
        content-align: center middle;
        height: auto;
        margin: 1 0;
    }
    #buttons {
        layout: horizontal;
        align: center middle;
        height: auto;
        margin-top: 2;
    }
    Button {
        margin: 0 1;
    }
    """

    def __init__(self, current_dir: str, worktree_dir: str):
        super().__init__()
        self.current_dir = current_dir
        self.worktree_dir = worktree_dir

    def compose(self) -> ComposeResult:
        with Container(id="dialog"):
            yield Label("Welcome to LazyWorktree", id="title")
            yield Label(
                f"No worktrees found.\n\n"
                f"Current Directory: [yellow]{self.current_dir}[/]\n"
                f"Worktree Root: [blue]{self.worktree_dir}[/]\n\n"
                "Please ensure you are in a git repository or the configured worktree root.\n"
                "You may need to initialize a repository or configure 'worktree_dir' in config.",
                id="message",
            )
            with Container(id="buttons"):
                yield Button("Quit", variant="error", id="quit")
                yield Button("Retry", variant="primary", id="retry")

    @on(Button.Pressed, "#quit")
    def action_quit(self):
        self.app.exit()

    @on(Button.Pressed, "#retry")
    def action_retry(self):
        self.dismiss(True)
