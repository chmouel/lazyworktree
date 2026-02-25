package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/chmouel/lazyworktree/internal/app/commands"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/app/state"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/theme"
)

func (m *Model) showInfo(message string, action tea.Cmd) {
	infoScreen := appscreen.NewInfoScreen(message, m.theme)
	infoScreen.OnClose = func() tea.Cmd { return action }
	m.state.ui.screenManager.Push(infoScreen)
}

func (m *Model) handleScreenKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if !m.state.ui.screenManager.IsActive() {
		return m, nil
	}
	current := m.state.ui.screenManager.Current()
	scr, cmd := current.Update(msg)
	if scr == nil {
		// Only pop if the current screen hasn't already changed.
		if m.state.ui.screenManager.Current() == current {
			m.state.ui.screenManager.Pop()
		}
	} else {
		// Only update if the current screen hasn't already changed (e.g. a
		// callback pushed a new screen via screenManager.Push).
		if m.state.ui.screenManager.Current() == current {
			m.state.ui.screenManager.Set(scr)
		}
	}
	return m, cmd
}

func (m *Model) showCommandPalette() tea.Cmd {
	m.debugf("open palette")
	customItems := m.customPaletteItems()
	registry := commands.NewRegistry()
	m.registerPaletteActions(registry)

	m.debugf("palette MRU: enabled=%v, history_len=%d", m.config.PaletteMRU, len(m.paletteHistory))
	paletteItems := commands.BuildPaletteItems(commands.PaletteOptions{
		MRUEnabled:  m.config.PaletteMRU,
		MRULimit:    m.config.PaletteMRULimit,
		History:     m.paletteHistory,
		Actions:     registry.Actions(),
		CustomItems: customItems,
	})

	mruCount := 0
	for _, item := range paletteItems {
		if item.IsMRU {
			mruCount++
		}
	}
	m.debugf("palette MRU: built %d items", mruCount)

	// Convert commands.PaletteItem to appscreen.PaletteItem
	items := make([]appscreen.PaletteItem, len(paletteItems))
	for i, item := range paletteItems {
		items[i] = appscreen.PaletteItem{
			ID:          item.ID,
			Label:       item.Label,
			Description: item.Description,
			IsSection:   item.IsSection,
			IsMRU:       item.IsMRU,
			Shortcut:    item.Shortcut,
			Icon:        item.Icon,
		}
	}

	// Create screen with callbacks
	paletteScreen := appscreen.NewCommandPaletteScreen(
		items,
		m.state.view.WindowWidth,
		m.state.view.WindowHeight,
		m.theme,
	)

	// Set OnSelect callback (preserve all existing logic)
	paletteScreen.OnSelect = func(action string) tea.Cmd {
		m.debugf("palette action: %s", action)

		// IMPORTANT: Track usage for MRU
		m.addToPaletteHistory(action)

		// Handle tmux active session attachment
		if after, ok := strings.CutPrefix(action, "tmux-attach:"); ok {
			sessionName := after
			insideTmux := os.Getenv("TMUX") != ""
			fullSessionName := m.config.SessionPrefix + sessionName
			return m.attachTmuxSessionCmd(fullSessionName, insideTmux)
		}

		// Handle zellij active session attachment
		if after, ok := strings.CutPrefix(action, "zellij-attach:"); ok {
			sessionName := after
			fullSessionName := m.config.SessionPrefix + sessionName
			return m.attachZellijSessionCmd(fullSessionName)
		}

		// Handle custom commands
		if _, ok := m.config.CustomCommands[action]; ok {
			return m.executeCustomCommand(action)
		}

		// Handle registry actions
		return registry.Execute(action)
	}

	paletteScreen.OnCancel = func() tea.Cmd {
		return nil
	}

	// Push to screen manager
	m.state.ui.screenManager.Push(paletteScreen)
	return textinput.Blink
}

func (m *Model) registerPaletteActions(registry *commands.Registry) {
	commands.RegisterWorktreeActions(registry, commands.WorktreeHandlers{
		Create:            m.showCreateWorktree,
		Delete:            m.showDeleteWorktree,
		Rename:            m.showRenameWorktree,
		Annotate:          m.showAnnotateWorktree,
		Absorb:            m.showAbsorbWorktree,
		Prune:             m.showPruneMerged,
		CreateFromCurrent: m.showCreateFromCurrent,
		CreateFromBranch: func() tea.Cmd {
			defaultBase := m.state.services.git.GetMainBranch(m.ctx)
			return m.showBranchSelection(
				"Select base branch",
				"Filter branches...",
				"No branches found.",
				defaultBase,
				func(branch string) tea.Cmd {
					suggestedName := stripRemotePrefix(branch)
					return m.showBranchNameInput(branch, suggestedName)
				},
			)
		},
		CreateFromCommit: func() tea.Cmd {
			defaultBase := m.state.services.git.GetMainBranch(m.ctx)
			return m.showCommitSelection(defaultBase)
		},
		CreateFromPR:    m.showCreateFromPR,
		CreateFromIssue: m.showCreateFromIssue,
		CreateFreeform: func() tea.Cmd {
			defaultBase := m.state.services.git.GetMainBranch(m.ctx)
			return m.showFreeformBaseInput(defaultBase)
		},
	})

	commands.RegisterGitOperations(registry, commands.GitHandlers{
		ShowDiff:    m.showDiff,
		Refresh:     m.refreshWorktrees,
		Fetch:       m.fetchRemotes,
		Push:        m.pushToUpstream,
		Sync:        m.syncWithUpstream,
		FetchPRData: m.fetchPRDataWithState,
		ViewCIChecks: func() tea.Cmd {
			return m.openCICheckSelection()
		},
		CIChecksAvailable: func() bool {
			return m.state.services.git != nil && m.state.services.git.IsGitHub(m.ctx)
		},
		OpenPR:      m.openPR,
		OpenLazyGit: m.openLazyGit,
		RunCommand:  m.showRunCommand,
	})

	commands.RegisterStatusPaneActions(registry, commands.StatusHandlers{
		StageFile: func() tea.Cmd {
			if len(m.state.services.statusTree.TreeFlat) > 0 && m.state.services.statusTree.Index >= 0 && m.state.services.statusTree.Index < len(m.state.services.statusTree.TreeFlat) {
				node := m.state.services.statusTree.TreeFlat[m.state.services.statusTree.Index]
				if node.IsDir() {
					return m.stageDirectory(node)
				}
				return m.stageCurrentFile(*node.File)
			}
			return nil
		},
		CommitStaged: m.commitStagedChanges,
		CommitAll:    m.commitAllChanges,
		EditFile: func() tea.Cmd {
			if len(m.state.services.statusTree.TreeFlat) > 0 && m.state.services.statusTree.Index >= 0 && m.state.services.statusTree.Index < len(m.state.services.statusTree.TreeFlat) {
				node := m.state.services.statusTree.TreeFlat[m.state.services.statusTree.Index]
				if !node.IsDir() {
					return m.openStatusFileInEditor(*node.File)
				}
			}
			return nil
		},
		DeleteFile: m.showDeleteFile,
	})

	commands.RegisterLogPaneActions(registry, commands.LogHandlers{
		CherryPick: m.showCherryPick,
		CommitView: m.openCommitView,
	})

	commands.RegisterNavigationActions(registry, commands.NavigationHandlers{
		ToggleZoom: func() tea.Cmd {
			if m.state.view.ZoomedPane >= 0 {
				m.state.view.ZoomedPane = -1
			} else {
				m.state.view.ZoomedPane = m.state.view.FocusedPane
			}
			return nil
		},
		ToggleLayout: func() tea.Cmd {
			if m.state.view.Layout == state.LayoutDefault {
				m.state.view.Layout = state.LayoutTop
			} else {
				m.state.view.Layout = state.LayoutDefault
			}
			m.state.view.ZoomedPane = -1
			return nil
		},
		Filter: func() tea.Cmd {
			target := filterTargetWorktrees
			switch m.state.view.FocusedPane {
			case 1:
				target = filterTargetStatus
			case 2:
				target = filterTargetLog
			}
			return m.startFilter(target)
		},
		Search: func() tea.Cmd {
			target := searchTargetWorktrees
			switch m.state.view.FocusedPane {
			case 1:
				target = searchTargetStatus
			case 2:
				target = searchTargetLog
			}
			return m.startSearch(target)
		},
		FocusWorktree: func() tea.Cmd {
			m.state.view.ZoomedPane = -1
			m.state.view.FocusedPane = 0
			m.state.ui.worktreeTable.Focus()
			return nil
		},
		FocusStatus: func() tea.Cmd {
			m.state.view.ZoomedPane = -1
			m.state.view.FocusedPane = 1
			m.rebuildStatusContentWithHighlight()
			return nil
		},
		FocusLog: func() tea.Cmd {
			m.state.view.ZoomedPane = -1
			m.state.view.FocusedPane = 2
			m.state.ui.logTable.Focus()
			return nil
		},
		SortCycle: func() tea.Cmd {
			m.sortMode = (m.sortMode + 1) % 3
			m.updateTable()
			return nil
		},
	})

	commands.RegisterClipboardActions(registry, commands.ClipboardHandlers{
		CopyPath:   m.yankContextual,
		CopyBranch: m.yankBranch,
		CopyPRURL:  m.yankPRURL,
	})

	commands.RegisterSettingsActions(registry, commands.SettingsHandlers{
		Theme: m.showThemeSelection,
		Taskboard: func() tea.Cmd {
			return m.showTaskboard()
		},
		Help: func() tea.Cmd {
			helpScreen := appscreen.NewHelpScreen(m.state.view.WindowWidth, m.state.view.WindowHeight, m.config.CustomCommands, m.theme, m.config.IconsEnabled())
			m.state.ui.screenManager.Push(helpScreen)
			return nil
		},
	})
}

func (m *Model) fetchPRDataWithState() tea.Cmd {
	if m.config.DisablePR {
		m.showInfo("PR/MR display is disabled in configuration", nil)
		return nil
	}
	m.cache.ciCache.Clear()
	m.prDataLoaded = false
	m.updateTable()
	m.updateTableColumns(m.state.ui.worktreeTable.Width())
	m.loading = true
	m.statusContent = "Fetching PR data..."
	m.setLoadingScreen("Fetching PR data...")
	return m.fetchPRData()
}

func (m *Model) showThemeSelection() tea.Cmd {
	m.originalTheme = m.config.Theme
	themes := theme.AvailableThemesWithCustoms(config.CustomThemesToThemeDataMap(m.config.CustomThemes))
	sort.Strings(themes)
	items := make([]appscreen.SelectionItem, 0, len(themes))
	for _, t := range themes {
		items = append(items, appscreen.SelectionItem{ID: t, Label: t})
	}

	listScreen := appscreen.NewListSelectionScreen(
		items,
		labelWithIcon(UIIconThemeSelect, "Select Theme", m.config.IconsEnabled()),
		"Filter themes...",
		"",
		m.state.view.WindowWidth,
		m.state.view.WindowHeight,
		m.originalTheme,
		m.theme,
	)

	listScreen.OnCursorChange = func(item appscreen.SelectionItem) {
		m.UpdateTheme(item.ID)
	}

	listScreen.OnSelect = func(item appscreen.SelectionItem) tea.Cmd {
		confirmScreen := appscreen.NewConfirmScreen(fmt.Sprintf("Save theme '%s' to config file?", item.ID), m.theme)
		confirmScreen.OnConfirm = func() tea.Cmd {
			m.config.Theme = item.ID
			if err := config.SaveConfig(m.config); err != nil {
				m.debugf("failed to save config: %v", err)
			}
			m.originalTheme = ""
			return nil
		}
		confirmScreen.OnCancel = func() tea.Cmd {
			// Keep the selected theme for this session; only skip persistence.
			m.originalTheme = ""

			// Close confirm + underlying theme picker to return to the main UI.
			if m.state.ui.screenManager.IsActive() && m.state.ui.screenManager.Type() == appscreen.TypeConfirm {
				m.state.ui.screenManager.Pop()
			}
			if m.state.ui.screenManager.IsActive() && m.state.ui.screenManager.Type() == appscreen.TypeListSelect {
				m.state.ui.screenManager.Pop()
			}
			return nil
		}
		m.state.ui.screenManager.Push(confirmScreen)
		return nil
	}

	listScreen.OnCancel = func() tea.Cmd {
		// Restore original theme on cancel
		if m.originalTheme != "" {
			m.UpdateTheme(m.originalTheme)
			m.originalTheme = ""
		}
		return nil
	}

	m.state.ui.screenManager.Push(listScreen)
	return textinput.Blink
}

// UpdateTheme refreshes UI styles for the selected theme.
func (m *Model) UpdateTheme(themeName string) {
	thm := theme.GetThemeWithCustoms(themeName, config.CustomThemesToThemeDataMap(m.config.CustomThemes))
	m.theme = thm

	// Update table styles
	s := table.DefaultStyles()
	s.Selected = s.Selected.Foreground(thm.AccentFg).Background(thm.Accent)
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(thm.BorderDim).
		BorderBottom(true).
		Bold(true).
		Foreground(thm.Cyan).
		Background(thm.AccentDim)
	s.Selected = s.Selected.Bold(true) // Arrow indicator shows selection, no background

	m.state.ui.worktreeTable.SetStyles(s)
	m.state.ui.logTable.SetStyles(s)

	// Update spinner style
	m.state.ui.spinner.Style = lipgloss.NewStyle().Foreground(thm.Accent)

	// Update filter input styles
	filterStyles := m.state.ui.filterInput.Styles()
	filterStyles.Focused.Prompt = lipgloss.NewStyle().Foreground(thm.Accent)
	filterStyles.Blurred.Prompt = lipgloss.NewStyle().Foreground(thm.Accent)
	filterStyles.Focused.Text = lipgloss.NewStyle().Foreground(thm.TextFg)
	filterStyles.Blurred.Text = lipgloss.NewStyle().Foreground(thm.TextFg)
	m.state.ui.filterInput.SetStyles(filterStyles)

	// Update other screens if they exist
	if loadingScreen := m.loadingScreen(); loadingScreen != nil {
		loadingScreen.SetTheme(thm)
	}
	if m.state.ui.screenManager.IsActive() {
		switch scr := m.state.ui.screenManager.Current().(type) {
		case *appscreen.InputScreen:
			scr.Thm = thm
		case *appscreen.PRSelectionScreen:
			scr.Thm = thm
		case *appscreen.IssueSelectionScreen:
			scr.Thm = thm
		case *appscreen.ChecklistScreen:
			scr.Thm = thm
		case *appscreen.HelpScreen:
			scr.Thm = thm
		case *appscreen.CommitFilesScreen:
			scr.Thm = thm
		case *appscreen.LoadingScreen:
			scr.SetTheme(thm)
		}
	}

	// Re-render info content with new theme
	if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
		m.infoContent = m.buildInfoContent(m.state.data.filteredWts[m.state.data.selectedIndex])
	}
}

func (m *Model) showRunCommand() tea.Cmd {
	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		return nil
	}

	inputScr := appscreen.NewInputScreen(
		"Run command in worktree",
		"e.g., make test, npm install, etc.",
		"",
		m.theme,
		m.config.IconsEnabled(),
	)
	// Enable bash-style history navigation with up/down arrows
	// Always set history, even if empty - it will populate as commands are added
	inputScr.SetHistory(m.commandHistory)

	inputScr.OnSubmit = func(value string, _ bool) tea.Cmd {
		cmdStr := strings.TrimSpace(value)
		if cmdStr == "" {
			return nil // Close without running
		}
		// Add command to history
		m.addToCommandHistory(cmdStr)
		return m.executeArbitraryCommand(cmdStr)
	}

	inputScr.OnCancel = func() tea.Cmd {
		return nil
	}

	m.state.ui.screenManager.Push(inputScr)
	return textinput.Blink
}

func (m *Model) customFooterHints() []string {
	keys := m.customCommandKeys()
	if len(keys) == 0 {
		return nil
	}

	hints := make([]string, 0, len(keys))
	for _, key := range keys {
		cmd := m.config.CustomCommands[key]
		if cmd == nil || !cmd.ShowHelp {
			continue
		}
		label := strings.TrimSpace(cmd.Description)
		if label == "" {
			label = strings.TrimSpace(cmd.Command)
		}
		if label == "" {
			label = customCommandPlaceholder
		}
		hints = append(hints, m.renderKeyHint(key, label))
	}
	return hints
}

func (m *Model) showCherryPick() tea.Cmd {
	// Validate: log pane must be focused
	if m.state.view.FocusedPane != 2 {
		return nil
	}

	// Validate: commit must be selected
	if len(m.state.data.logEntries) == 0 {
		return nil
	}

	cursor := m.state.ui.logTable.Cursor()
	if cursor < 0 || cursor >= len(m.state.data.logEntries) {
		return nil
	}

	// Get source worktree and commit
	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		return nil
	}
	sourceWorktree := m.state.data.filteredWts[m.state.data.selectedIndex]
	selectedCommit := m.state.data.logEntries[cursor]

	// Build worktree selection items (exclude source worktree)
	items := make([]selectionItem, 0, len(m.state.data.worktrees)-1)
	for _, wt := range m.state.data.worktrees {
		if wt.Path == sourceWorktree.Path {
			continue // Skip source worktree
		}

		name := filepath.Base(wt.Path)
		if wt.IsMain {
			name = "main"
		}

		desc := wt.Branch
		if wt.Dirty {
			desc += " (has changes)"
		}

		items = append(items, selectionItem{
			id:          wt.Path,
			label:       name,
			description: desc,
		})
	}

	if len(items) == 0 {
		m.showInfo("No other worktrees available for cherry-pick.", nil)
		return nil
	}

	screenItems := make([]appscreen.SelectionItem, len(items))
	for i, item := range items {
		screenItems[i] = appscreen.SelectionItem{
			ID:          item.id,
			Label:       item.label,
			Description: item.description,
		}
	}

	title := fmt.Sprintf("Cherry-pick %s to worktree", selectedCommit.sha)
	listScreen := appscreen.NewListSelectionScreen(
		screenItems,
		title,
		filterWorktreesPlaceholder,
		"No worktrees found.",
		m.state.view.WindowWidth,
		m.state.view.WindowHeight,
		"",
		m.theme,
	)

	listScreen.OnSelect = func(item appscreen.SelectionItem) tea.Cmd {
		var targetWorktree *models.WorktreeInfo
		for _, wt := range m.state.data.worktrees {
			if wt.Path == item.ID {
				targetWorktree = wt
				break
			}
		}

		if targetWorktree == nil {
			return func() tea.Msg {
				return errMsg{err: fmt.Errorf("target worktree not found")}
			}
		}

		return m.executeCherryPick(selectedCommit.sha, targetWorktree)
	}

	listScreen.OnCancel = func() tea.Cmd {
		return nil
	}

	m.state.ui.screenManager.Push(listScreen)
	return textinput.Blink
}

func (m *Model) showCommitFilesScreen(commitSHA, worktreePath string) tea.Cmd {
	return func() tea.Msg {
		files, err := m.state.services.git.GetCommitFiles(m.ctx, commitSHA, worktreePath)
		if err != nil {
			return errMsg{err: err}
		}
		// Fetch commit metadata
		metaRaw := m.state.services.git.RunGit(
			m.ctx,
			[]string{
				"git", "log", "-1",
				"--pretty=format:%H%x1f%an%x1f%ae%x1f%ad%x1f%s%x1f%b",
				commitSHA,
			},
			worktreePath,
			[]int{0},
			true,
			false,
		)
		meta := parseCommitMeta(metaRaw)
		// Ensure SHA is set even if parsing fails
		if meta.sha == "" {
			meta.sha = commitSHA
		}
		return commitFilesLoadedMsg{
			sha:          commitSHA,
			worktreePath: worktreePath,
			files:        files,
			meta:         meta,
		}
	}
}

func (m *Model) openCommitView() tea.Cmd {
	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		return nil
	}
	if len(m.state.data.logEntries) == 0 {
		return nil
	}

	cursor := m.state.ui.logTable.Cursor()
	if cursor < 0 || cursor >= len(m.state.data.logEntries) {
		return nil
	}
	entry := m.state.data.logEntries[cursor]
	wt := m.state.data.filteredWts[m.state.data.selectedIndex]

	return m.showCommitFilesScreen(entry.sha, wt.Path)
}

func (m *Model) persistCurrentSelection() {
	idx := m.state.data.selectedIndex
	if idx < 0 || idx >= len(m.state.data.filteredWts) {
		idx = m.state.ui.worktreeTable.Cursor()
	}
	if idx < 0 || idx >= len(m.state.data.filteredWts) {
		return
	}
	m.persistLastSelected(m.state.data.filteredWts[idx].Path)
}

func (m *Model) persistLastSelected(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	m.debugf("persist last-selected: %s", path)
	repoKey := m.getRepoKey()
	lastSelectedPath := filepath.Join(m.getWorktreeDir(), repoKey, models.LastSelectedFilename)
	if err := os.MkdirAll(filepath.Dir(lastSelectedPath), defaultDirPerms); err != nil {
		return
	}
	_ = os.WriteFile(lastSelectedPath, []byte(path+"\n"), defaultFilePerms)
	m.recordAccess(path)
}

func (m *Model) customPaletteItems() []commands.PaletteItem {
	keys := m.customCommandKeys()
	if len(keys) == 0 {
		return nil
	}

	// Separate commands into categories
	var regularItems, tmuxItems, zellijItems []commands.PaletteItem
	for _, key := range keys {
		cmd := m.config.CustomCommands[key]
		if cmd == nil {
			continue
		}
		label := m.customCommandLabel(cmd, key)
		description := customCommandPlaceholder
		switch {
		case cmd.Command != "":
			description = cmd.Command
		case cmd.Zellij != nil:
			description = zellijSessionLabel
		case cmd.Tmux != nil:
			description = tmuxSessionLabel
		}
		item := commands.PaletteItem{
			ID:          key,
			Label:       label,
			Description: description,
			Icon:        commands.IconCustom,
		}
		switch {
		case cmd.Tmux != nil:
			item.Icon = commands.IconMultiplex
			tmuxItems = append(tmuxItems, item)
		case cmd.Zellij != nil:
			item.Icon = commands.IconMultiplex
			zellijItems = append(zellijItems, item)
		default:
			regularItems = append(regularItems, item)
		}
	}

	// Check if tmux/zellij are available
	_, tmuxErr := exec.LookPath("tmux")
	_, zellijErr := exec.LookPath("zellij")
	hasTmux := len(tmuxItems) > 0 && tmuxErr == nil
	hasZellij := len(zellijItems) > 0 && zellijErr == nil

	// Get active tmux sessions
	var activeTmuxSessions []commands.PaletteItem
	if tmuxErr == nil {
		sessions := m.getTmuxActiveSessions()
		for _, sessionName := range sessions {
			activeTmuxSessions = append(activeTmuxSessions, commands.PaletteItem{
				ID:          "tmux-attach:" + sessionName,
				Label:       sessionName,
				Description: "active tmux session",
				Icon:        commands.IconMultiplex,
			})
		}
	}

	// Get active zellij sessions
	var activeZellijSessions []commands.PaletteItem
	if zellijErr == nil {
		sessions := m.getZellijActiveSessions()
		for _, sessionName := range sessions {
			activeZellijSessions = append(activeZellijSessions, commands.PaletteItem{
				ID:          "zellij-attach:" + sessionName,
				Label:       sessionName,
				Description: "active zellij session",
				Icon:        commands.IconMultiplex,
			})
		}
	}

	// Build result with sections
	var items []commands.PaletteItem
	if len(regularItems) > 0 {
		items = append(items, commands.PaletteItem{Label: "Custom Commands", IsSection: true, Icon: commands.IconCustom})
		items = append(items, regularItems...)
	}

	// Multiplexer section for custom tmux/zellij commands
	if hasTmux || hasZellij {
		items = append(items, commands.PaletteItem{Label: "Multiplexer", IsSection: true, Icon: commands.IconMultiplex})
		if hasTmux {
			items = append(items, tmuxItems...)
		}
		if hasZellij {
			items = append(items, zellijItems...)
		}
	}

	// Active Tmux Sessions section (appears after Multiplexer)
	if len(activeTmuxSessions) > 0 {
		items = append(items, commands.PaletteItem{Label: "Active Tmux Sessions", IsSection: true, Icon: commands.IconMultiplex})
		items = append(items, activeTmuxSessions...)
	}

	// Active Zellij Sessions section (appears after Active Tmux Sessions)
	if len(activeZellijSessions) > 0 {
		items = append(items, commands.PaletteItem{Label: "Active Zellij Sessions", IsSection: true, Icon: commands.IconMultiplex})
		items = append(items, activeZellijSessions...)
	}

	return items
}
