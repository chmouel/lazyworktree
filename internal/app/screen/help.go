package screen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/theme"
)

// HelpScreen renders searchable documentation for the app controls.
type HelpScreen struct {
	Viewport    viewport.Model
	Width       int
	Height      int
	FullText    []string
	SearchInput textinput.Model
	Searching   bool
	SearchQuery string
	Thm         *theme.Theme
	ShowIcons   bool
}

const (
	helpDefaultWidth  = 80
	helpDefaultHeight = 30
	helpMinWidth      = 60
	helpMinHeight     = 20
	helpMarginX       = 4
	helpMarginY       = 2
)

func helpDimensions(maxWidth, maxHeight int) (int, int) {
	width := helpDefaultWidth
	height := helpDefaultHeight

	if maxWidth > 0 {
		width = maxInt(helpMinWidth, maxWidth-helpMarginX)
	}
	if maxHeight > 0 {
		height = maxInt(helpMinHeight, maxHeight-helpMarginY)
	}

	return width, height
}

// NewHelpScreen initializes help content with the available screen size.
func NewHelpScreen(maxWidth, maxHeight int, customCommands map[string]*config.CustomCommand, thm *theme.Theme, showIcons bool) *HelpScreen {
	helpTextTemplate := `{{HELP_TITLE}}LazyWorktree Help Guide

**{{HELP_NAV}}Navigation**
- j / {{ARROW_DOWN}}: Move cursor down in lists and menus
- k / {{ARROW_UP}}: Move cursor up in lists and menus
- 1 / 2 / 3: Switch to pane (or toggle zoom if already focused)
- h / l: Left / Right pane
- [ / ]: Previous / Next pane
- Tab: Cycle to next pane
- L: Toggle layout (default / top)
- Enter: Jump to selected worktree (exit and cd)
- q: Quit application

**{{HELP_STATUS_PANE}}Status Pane (when focused)**
- j / k: Navigate files and directories, or CI checks (when visible)
- Enter: Toggle directory collapse, show file diff, or open selected CI check URL
- PR number in the info panel is clickable in terminals that support OSC-8 hyperlinks
- Ctrl+v: View selected CI check logs in pager (when CI check is selected)
- e: Open selected file in editor
- d: Show full diff (all files) in pager
- s: Stage/unstage selected file or directory
- D: Delete selected file or directory (with confirmation)
- c: Commit staged changes
- C: Stage all changes and commit
- Ctrl+{{ARROW_LEFT}} / {{ARROW_RIGHT}}: Jump to previous / next folder
- /: Search file or directory names
- Ctrl+D / Space: Half page down
- Ctrl+U: Half page up
- PageUp / PageDown: Half page up/down

**{{HELP_CI_CHECKS}}CI Checks Navigation (in Status Pane)**
When CI checks are displayed in the info panel:
- j / k: Navigate through CI checks (wraps to/from file tree at boundaries)
- Enter: Open selected CI check URL in browser
- Ctrl+v: View selected CI check logs in pager

**{{HELP_LOG}}Log Pane**
- j / k: Move between commits
- Ctrl+J: Next commit and open file tree
- Enter: Open commit file tree (browse changed files)
- d: Show full commit diff in pager
- C: Cherry-pick commit to another worktree
- /: Search commit titles

**{{HELP_COMMIT_TREE}}Commit File Tree (viewing files in a commit)**
- j / k: Navigate files and directories
- Enter: Toggle directory or show file diff
- d: Show full commit diff in pager
- f: Filter files by name
- /: Search files (incremental)
- n / N: Next / previous search match
- Ctrl+D / Space: Half page down
- Ctrl+U: Half page up
- g / G: Jump to top / bottom
- q / Esc: Return to commit log

**{{HELP_WORKTREE_ACTIONS}}Worktree Actions**
- c: Create new worktree (branch, commit, PR/MR, issue, or custom)
- Create from current: supply an explicit name or leave empty for auto-generation
- Create from PR/MR: worktree name is generated from the PR template/script; branch name uses the PR branch when you are the author, otherwise uses the generated name
- Create from PR/MR or issue can auto-add a note when worktree_note_script is configured
- Existing local branch: choose to checkout the branch or create a new one based on it
- Tab / Shift+Tab: Move focus to the "Include current file changes" checkbox
- Space: Toggle "Include current file changes"
- i: Open selected worktree notes (viewer if present, editor if empty)
- Note viewer: j/k scroll, Ctrl+D/Ctrl+U half-page, g/G top/bottom, e edit, E edit in external editor, q/Esc close
- Note editor: Ctrl+S saves, Enter adds a new line, Esc cancels
- T: Open Taskboard (grouped by worktree from markdown checkbox notes)
- Taskboard: a adds a new task, Enter/Space toggles selected checkbox task, f filters tasks, q/Esc closes
- Worktrees with non-empty notes show a note marker beside the name
- worktree_notes_path can store notes in one shared JSON file with repo-relative keys for easier synchronisation
- In the Info pane, notes render Markdown for headings, bold text, inline code, lists, quotes, links, and fenced code blocks
- Uppercase note tags such as TODO, FIXME, or WARNING: are highlighted with icons outside fenced code blocks; lowercase tags are left unchanged
- m: Rename selected worktree
- D: Delete selected worktree
- A: Absorb worktree into main (merge or rebase based on configuration, then delete)
- X: Prune merged worktrees (auto-refreshes PR data, then checks PR/branch merge status)
- !: Run arbitrary command in selected worktree

**{{HELP_BRANCH_NAMING}}Branch Naming**
Special characters in branch names are automatically converted to hyphens for compatibility with Git and terminal multiplexers. Examples:
- "feature.new" {{ARROW_RIGHT}} "feature-new"
- "bug fix here" {{ARROW_RIGHT}} "bug-fix-here"
- "path/to/branch" {{ARROW_RIGHT}} "path-to-branch"
Supported: Letters (a-z, A-Z), numbers (0-9), and hyphens (-). See help for full details.

**{{HELP_VIEWING_TOOLS}}Viewing & Tools**
- d: Show diff in pager (worktree or commit)
- o: Open PR/MR in browser (or root repo in editor if main branch with merged/closed/no PR)
- g: Open LazyGit (or go to top in diff pane)
- =: Toggle zoom for focused pane
- : / Ctrl+P: Command Palette
- ?: Show this help

**{{HELP_REPO_OPS}}Repository Operations**
- r: Refresh worktree list (also refreshes PR/MR/CI for current worktree on GitHub/GitLab)
- R: Fetch all remotes
- S: Synchronise with upstream (git pull, then git push, current branch only, requires a clean worktree, honours merge_method)
- P: Push to upstream branch (current branch only, requires a clean worktree, prompts to set upstream when missing)
- v: View CI checks (opens selection screen)
- Enter: Open selected CI job in browser (within CI check selection screen)
- Ctrl+v: View selected CI check logs in pager (within CI check selection screen, or in status pane when CI check is selected)
- Ctrl+r: Restart selected CI job (GitHub Actions only, within CI check selection screen)
- s: Cycle sort (Path / Last Active / Last Switched)

**{{HELP_BACKGROUND_REFRESH}}Background Refresh**
- Configured via auto_refresh and refresh_interval in the configuration file

**{{HELP_TIPS}}Tips & Shortcuts**
{{HELP_TIP_LINES}}

**{{HELP_FILTERING_SEARCH}}Filtering & Search**
- f: Filter focused pane
- Selection menus: press f to show the filter, Esc returns to the list
- /: Search focused pane (incremental)
- Alt+N / Alt+P: Move selection and fill filter input
- {{ARROW_UP}} / {{ARROW_DOWN}}: Move selection (filter active, no fill)
- Ctrl+J / Ctrl+K: Same as above
- Home / End: Jump to first / last item

Search Mode:
- Type: Jump to first matching item
- n / N: Next / previous match
- Enter: Close search
- Esc: Clear search

**{{HELP_STATUS_INDICATORS}}Status Indicators**
- -: Clean and synchronised with remote
- {{STATUS_DIRTY}}: Uncommitted changes (dirty)
- {{STATUS_AHEAD}}N: Ahead of remote by N commits
- {{STATUS_BEHIND}}N: Behind remote by N commits
- {{STATUS_DIRTY}} {{STATUS_AHEAD}}N: Dirty and ahead by N commits

**{{HELP_HELP_NAVIGATION}}Help Navigation**
- /: Search help (Enter to apply, Esc to clear)
- q / Esc: Close help
- j / k: Scroll up / down
- Ctrl+D / Ctrl+U: Scroll half page down / up

- Ctrl+D / Ctrl+U: Scroll half page down / up `

	helpTipLines := strings.Join(HelpTips(), "\n")

	replacer := strings.NewReplacer(
		"{{HELP_TITLE}}", iconPrefix(UIIconHelpTitle, showIcons),
		"{{HELP_NAV}}", iconPrefix(UIIconNavigation, showIcons),
		"{{HELP_STATUS_PANE}}", iconPrefix(UIIconStatusPane, showIcons),
		"{{HELP_CI_CHECKS}}", iconPrefix(UIIconCICheck, showIcons),
		"{{HELP_LOG}}", iconPrefix(UIIconLogPane, showIcons),
		"{{HELP_COMMIT_TREE}}", iconPrefix(UIIconCommitTree, showIcons),
		"{{HELP_WORKTREE_ACTIONS}}", iconPrefix(UIIconWorktreeActions, showIcons),
		"{{HELP_BRANCH_NAMING}}", iconPrefix(UIIconBranchNaming, showIcons),
		"{{HELP_VIEWING_TOOLS}}", iconPrefix(UIIconViewingTools, showIcons),
		"{{HELP_REPO_OPS}}", iconPrefix(UIIconRepoOps, showIcons),
		"{{HELP_BACKGROUND_REFRESH}}", iconPrefix(UIIconBackgroundRefresh, showIcons),
		"{{HELP_TIPS}}", iconPrefix(UIIconTip, showIcons),
		"{{HELP_TIP_LINES}}", helpTipLines,
		"{{HELP_FILTERING_SEARCH}}", iconPrefix(UIIconFilterSearch, showIcons),
		"{{HELP_STATUS_INDICATORS}}", iconPrefix(UIIconStatusIndicators, showIcons),
		"{{HELP_HELP_NAVIGATION}}", iconPrefix(UIIconHelpNavigation, showIcons),
		"{{HELP_SHELL_COMPLETION}}", iconPrefix(UIIconShellCompletion, showIcons),
		"{{HELP_CONFIGURATION}}", iconPrefix(UIIconConfiguration, showIcons),
		"{{HELP_ICON_CONFIGURATION}}", iconPrefix(UIIconIconConfiguration, showIcons),
		"{{STATUS_CLEAN}}", statusIndicator(true, showIcons),
		"{{STATUS_DIRTY}}", statusIndicator(false, showIcons),
		"{{STATUS_AHEAD}}", aheadIndicator(showIcons),
		"{{STATUS_BEHIND}}", behindIndicator(showIcons),
		"{{ARROW_UP}}", arrowUp(showIcons),
		"{{ARROW_DOWN}}", arrowDown(showIcons),
		"{{ARROW_LEFT}}", arrowLeft(showIcons),
		"{{ARROW_RIGHT}}", arrowRight(showIcons),
	)

	helpText := replacer.Replace(helpTextTemplate)

	// Append custom commands section if any exist with show_help=true
	if len(customCommands) > 0 {
		var customKeys []string
		for key, cmd := range customCommands {
			if cmd != nil && cmd.ShowHelp {
				customKeys = append(customKeys, fmt.Sprintf("- %s: %s", key, cmd.Description))
			}
		}

		if len(customKeys) > 0 {
			sort.Strings(customKeys)
			customTitle := labelWithIcon(UIIconConfiguration, "Custom Commands", showIcons)
			helpText += "\n\n**" + customTitle + "**\n" + strings.Join(customKeys, "\n")
		}
	}

	width, height := helpDimensions(maxWidth, maxHeight)

	vp := viewport.New(width, maxInt(5, height-3))
	fullLines := strings.Split(helpText, "\n")

	ti := textinput.New()
	ti.Placeholder = "Search help (/ to start, Enter to apply, Esc to clear)"
	ti.CharLimit = 64
	ti.Prompt = "/ "
	ti.SetValue("")
	ti.Blur()
	ti.Width = maxInt(20, width-6)

	hs := &HelpScreen{
		Viewport:    vp,
		Width:       width,
		Height:      height,
		FullText:    fullLines,
		SearchInput: ti,
		Thm:         thm,
		ShowIcons:   showIcons,
	}

	hs.refreshContent()
	return hs
}

// Type returns TypeHelp to identify this screen.
func (s *HelpScreen) Type() Type {
	return TypeHelp
}

// Update handles scrolling and search input for the help screen.
func (s *HelpScreen) Update(msg tea.KeyMsg) (Screen, tea.Cmd) {
	var cmd tea.Cmd
	key := msg.String()

	switch key {
	case "/":
		if !s.Searching {
			s.Searching = true
			s.SearchInput.Focus()
			return s, textinput.Blink
		}
	case "enter":
		if s.Searching {
			s.SearchQuery = strings.TrimSpace(s.SearchInput.Value())
			s.Searching = false
			s.SearchInput.Blur()
			s.refreshContent()
			return s, nil
		}
	case "esc", "ctrl+c":
		// If searching, clear search; otherwise close help
		if s.Searching || s.SearchQuery != "" {
			s.Searching = false
			s.SearchInput.SetValue("")
			s.SearchQuery = ""
			s.SearchInput.Blur()
			s.refreshContent()
			return s, nil
		}
		// Close help screen
		return nil, nil
	case "q":
		// Always close on 'q'
		return nil, nil
	}

	if s.Searching {
		s.SearchInput, cmd = s.SearchInput.Update(msg)
		newQuery := strings.TrimSpace(s.SearchInput.Value())
		if newQuery != s.SearchQuery {
			s.SearchQuery = newQuery
			s.refreshContent()
		}
		return s, cmd
	}

	// Handle viewport scrolling
	switch key {
	case "ctrl+d", " ":
		s.Viewport.HalfPageDown()
		return s, nil
	case "ctrl+u":
		s.Viewport.HalfPageUp()
		return s, nil
	case "j", "down":
		s.Viewport.ScrollDown(1)
		return s, nil
	case "k", "up":
		s.Viewport.ScrollUp(1)
		return s, nil
	}

	s.Viewport, cmd = s.Viewport.Update(msg)
	return s, cmd
}

// refreshContent updates the viewport with styled and filtered content.
func (s *HelpScreen) refreshContent() {
	content := s.renderContent()
	s.Viewport.SetContent(content)
	s.Viewport.GotoTop()
}

// SetSize updates the help screen dimensions (useful on terminal resize).
func (s *HelpScreen) SetSize(maxWidth, maxHeight int) {
	width, height := helpDimensions(maxWidth, maxHeight)
	s.Width = width
	s.Height = height

	// Update viewport size
	// height - 4 for borders/header/footer
	s.Viewport.Width = s.Width - 2
	s.Viewport.Height = maxInt(5, s.Height-4)
}

// renderContent applies styling and search filtering to help text.
func (s *HelpScreen) renderContent() string {
	lines := s.FullText

	// Apply styling to help content
	styledLines := []string{}
	titleStyle := lipgloss.NewStyle().Foreground(s.Thm.Accent).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(s.Thm.SuccessFg).Bold(true)

	for _, line := range lines {
		// Style section headers (lines that start with ** and end with **)
		if strings.HasPrefix(line, "**") && strings.HasSuffix(line, "**") {
			header := strings.TrimPrefix(strings.TrimSuffix(line, "**"), "**")
			prefix := disclosureIndicator(false, s.ShowIcons)
			styledLines = append(styledLines, titleStyle.Render(prefix+" "+header))
			continue
		}

		// Style key bindings (lines starting with "- " and containing ": ")
		if strings.HasPrefix(line, "- ") {
			// Split on ": " (colon + space) to handle keys that contain ":"
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) == 2 {
				keys := strings.TrimPrefix(parts[0], "- ")
				description := parts[1]
				styledLine := "  " + keyStyle.Render(keys) + ": " + description
				styledLines = append(styledLines, styledLine)
				continue
			}
		}

		styledLines = append(styledLines, line)
	}

	// Handle search filtering
	if strings.TrimSpace(s.SearchQuery) != "" {
		query := strings.ToLower(strings.TrimSpace(s.SearchQuery))
		highlightStyle := lipgloss.NewStyle().Foreground(s.Thm.AccentFg).Background(s.Thm.Accent).Bold(true)
		filteredLines := []string{}
		for _, line := range styledLines {
			lower := strings.ToLower(line)
			if strings.Contains(lower, query) {
				filteredLines = append(filteredLines, highlightMatches(line, lower, query, highlightStyle))
			}
		}

		if len(filteredLines) == 0 {
			return fmt.Sprintf("No help entries match %q", s.SearchQuery)
		}
		return strings.Join(filteredLines, "\n")
	}

	return strings.Join(styledLines, "\n")
}

// highlightMatches highlights all occurrences of the query in the line.
func highlightMatches(line, lowerLine, lowerQuery string, style lipgloss.Style) string {
	if lowerQuery == "" {
		return line
	}

	var b strings.Builder
	searchFrom := 0
	qLen := len(lowerQuery)

	for {
		idx := strings.Index(lowerLine[searchFrom:], lowerQuery)
		if idx < 0 {
			b.WriteString(line[searchFrom:])
			break
		}
		start := searchFrom + idx
		end := start + qLen
		b.WriteString(line[searchFrom:start])
		b.WriteString(style.Render(line[start:end]))
		searchFrom = end
	}

	return b.String()
}

// View renders the help content and search input inside the viewport.
func (s *HelpScreen) View() string {
	content := s.renderContent()

	// Keep viewport sized to available area (minus header/search lines)
	vHeight := maxInt(5, s.Height-4) // -4 for borders/header/footer
	s.Viewport.Width = s.Width - 2   // -2 for borders
	s.Viewport.Height = vHeight
	s.Viewport.SetContent(content)

	// Enhanced help modal with rounded border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.Thm.Accent).
		Width(s.Width).
		Padding(0)

	titleStyle := lipgloss.NewStyle().
		Foreground(s.Thm.Accent).
		Bold(true).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(s.Thm.BorderDim).
		Width(s.Width-2).
		Padding(0, 1).
		Render("❓ Help")

	// Search bar styling
	searchView := ""
	if s.Searching || s.SearchQuery != "" {
		searchView = lipgloss.NewStyle().
			Width(s.Width-2).
			Padding(0, 1).
			Render(s.SearchInput.View())

		// Add separator after search
		searchView += "\n" + lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(s.Thm.BorderDim).
			Width(s.Width-2).
			Render("")
	}

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg).
		Align(lipgloss.Left).
		Width(s.Width - 2).
		PaddingTop(1)
	footer := footerStyle.Render("j/k: scroll • Ctrl+d/u: page • /: search • esc: close")

	// Viewport styling
	vpStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.Width - 2)

	body := vpStyle.Render(s.Viewport.View())

	contentBlock := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle,
		searchView,
		body,
		footer,
	)

	return boxStyle.Render(contentBlock)
}

// Helper functions for min/max
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
