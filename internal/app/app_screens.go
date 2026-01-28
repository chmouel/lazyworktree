package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/lazyworktree/internal/app/commands"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/theme"
)

func screenName(screen screenType) string {
	switch screen {
	case screenNone:
		return "none"
	case screenConfirm:
		return "confirm"
	case screenInfo:
		return "info"
	case screenInput:
		return "input"
	case screenTrust:
		return "trust"
	case screenCommitFiles:
		return "commit-files"
	default:
		return "unknown"
	}
}

func (m *Model) showInfo(message string, action tea.Cmd) {
	m.infoScreen = NewInfoScreen(message, m.theme)
	m.infoAction = action
	m.currentScreen = screenInfo
}

func (m *Model) handleScreenKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// New path: delegate to screen manager for migrated screens
	if m.screenManager.IsActive() {
		current := m.screenManager.Current()
		scr, cmd := current.Update(msg)
		if scr == nil {
			// Only pop if the current screen hasn't already changed.
			if m.screenManager.Current() == current {
				m.screenManager.Pop()
			}
		} else {
			m.screenManager.Set(scr)
		}
		return m, cmd
	}

	m.debugf("screen key: %s screen=%s", msg.String(), screenName(m.currentScreen))
	switch m.currentScreen {
	// PRSelection, IssueSelection, and CommandPalette now handled by screen manager
	case screenCommitFiles:
		if m.commitFilesScreen == nil {
			m.currentScreen = screenNone
			return m, nil
		}
		keyStr := msg.String()

		// If filter or search is active, delegate to screen first
		if m.commitFilesScreen.showingFilter || m.commitFilesScreen.showingSearch {
			cs, cmd := m.commitFilesScreen.Update(msg)
			if updated, ok := cs.(*CommitFilesScreen); ok {
				m.commitFilesScreen = updated
			}
			return m, cmd
		}

		switch keyStr {
		case keyQ, keyCtrlC:
			m.commitFilesScreen = nil
			m.currentScreen = screenNone
			return m, nil
		case keyEsc, keyEscRaw:
			m.commitFilesScreen = nil
			m.currentScreen = screenNone
			return m, nil
		case "f":
			// Start filter mode
			m.commitFilesScreen.showingFilter = true
			m.commitFilesScreen.showingSearch = false
			m.commitFilesScreen.filterInput.Placeholder = placeholderFilterFiles
			m.commitFilesScreen.filterInput.SetValue(m.commitFilesScreen.filterQuery)
			m.commitFilesScreen.filterInput.Focus()
			return m, textinput.Blink
		case "/":
			// Start search mode
			m.commitFilesScreen.showingSearch = true
			m.commitFilesScreen.showingFilter = false
			m.commitFilesScreen.filterInput.Placeholder = searchFiles
			m.commitFilesScreen.filterInput.SetValue("")
			m.commitFilesScreen.searchQuery = ""
			m.commitFilesScreen.filterInput.Focus()
			return m, textinput.Blink
		case "d":
			// Show full commit diff via pager
			sha := m.commitFilesScreen.commitSHA
			wtPath := m.commitFilesScreen.worktreePath
			m.commitFilesScreen = nil
			m.currentScreen = screenNone
			// Find the worktree to pass to showCommitDiff
			var wt *models.WorktreeInfo
			for _, w := range m.filteredWts {
				if w.Path == wtPath {
					wt = w
					break
				}
			}
			if wt != nil {
				return m, m.showCommitDiff(sha, wt)
			}
			return m, nil
		case keyEnter:
			node := m.commitFilesScreen.GetSelectedNode()
			if node == nil {
				return m, nil
			}
			if node.IsDir() {
				m.commitFilesScreen.ToggleCollapse(node.Path)
				return m, nil
			}
			// Show diff for this specific file
			sha := m.commitFilesScreen.commitSHA
			wtPath := m.commitFilesScreen.worktreePath
			return m, m.showCommitFileDiff(sha, node.File.Filename, wtPath)
		}
		// Delegate navigation to screen
		cs, cmd := m.commitFilesScreen.Update(msg)
		if updated, ok := cs.(*CommitFilesScreen); ok {
			m.commitFilesScreen = updated
		}
		return m, cmd
	case screenConfirm:
		if m.confirmScreen != nil {
			_, cmd := m.confirmScreen.Update(msg)
			// Check if the confirm screen sent a result
			select {
			case confirmed := <-m.confirmScreen.result:
				if confirmed {
					// Perform confirmed action (delete, prune, etc.)
					var actionCmd tea.Cmd
					if m.confirmAction != nil {
						actionCmd = m.confirmAction()
					}
					m.confirmScreen = nil
					m.confirmAction = nil
					m.confirmCancel = nil
					if m.currentScreen == screenConfirm {
						m.currentScreen = screenNone
					}
					if actionCmd != nil {
						return m, actionCmd
					}
					return m, nil
				} else {
					// Action cancelled
					var cancelCmd tea.Cmd
					if m.confirmCancel != nil {
						cancelCmd = m.confirmCancel()
					}
					m.confirmScreen = nil
					m.confirmAction = nil
					m.confirmCancel = nil
					m.currentScreen = screenNone
					if cancelCmd != nil {
						return m, cancelCmd
					}
					return m, nil
				}
			default:
				return m, cmd
			}
		}
	case screenInfo:
		if m.infoScreen != nil {
			_, cmd := m.infoScreen.Update(msg)
			select {
			case <-m.infoScreen.result:
				action := m.infoAction
				m.infoScreen = nil
				m.infoAction = nil
				m.currentScreen = screenNone
				if action != nil {
					return m, action
				}
				return m, nil
			default:
				return m, cmd
			}
		}
	}
	return m, nil
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
		}
	}

	// Create screen with callbacks
	paletteScreen := appscreen.NewCommandPaletteScreen(
		items,
		m.view.WindowWidth,
		m.view.WindowHeight,
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
	m.screenManager.Push(paletteScreen)
	return textinput.Blink
}

func (m *Model) registerPaletteActions(registry *commands.Registry) {
	commands.RegisterWorktreeActions(registry, commands.WorktreeHandlers{
		Create:            m.showCreateWorktree,
		Delete:            m.showDeleteWorktree,
		Rename:            m.showRenameWorktree,
		Absorb:            m.showAbsorbWorktree,
		Prune:             m.showPruneMerged,
		CreateFromCurrent: m.showCreateFromCurrent,
		CreateFromBranch: func() tea.Cmd {
			defaultBase := m.git.GetMainBranch(m.ctx)
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
			defaultBase := m.git.GetMainBranch(m.ctx)
			return m.showCommitSelection(defaultBase)
		},
		CreateFromPR:    m.showCreateFromPR,
		CreateFromIssue: m.showCreateFromIssue,
		CreateFreeform: func() tea.Cmd {
			defaultBase := m.git.GetMainBranch(m.ctx)
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
			return m.git != nil && m.git.IsGitHub(m.ctx)
		},
		OpenPR:      m.openPR,
		OpenLazyGit: m.openLazyGit,
		RunCommand:  m.showRunCommand,
	})

	commands.RegisterStatusPaneActions(registry, commands.StatusHandlers{
		StageFile: func() tea.Cmd {
			if len(m.statusTreeFlat) > 0 && m.statusTreeIndex >= 0 && m.statusTreeIndex < len(m.statusTreeFlat) {
				node := m.statusTreeFlat[m.statusTreeIndex]
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
			if len(m.statusTreeFlat) > 0 && m.statusTreeIndex >= 0 && m.statusTreeIndex < len(m.statusTreeFlat) {
				node := m.statusTreeFlat[m.statusTreeIndex]
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
			if m.view.ZoomedPane >= 0 {
				m.view.ZoomedPane = -1
			} else {
				m.view.ZoomedPane = m.view.FocusedPane
			}
			return nil
		},
		Filter: func() tea.Cmd {
			target := filterTargetWorktrees
			switch m.view.FocusedPane {
			case 1:
				target = filterTargetStatus
			case 2:
				target = filterTargetLog
			}
			return m.startFilter(target)
		},
		Search: func() tea.Cmd {
			target := searchTargetWorktrees
			switch m.view.FocusedPane {
			case 1:
				target = searchTargetStatus
			case 2:
				target = searchTargetLog
			}
			return m.startSearch(target)
		},
		FocusWorktree: func() tea.Cmd {
			m.view.ZoomedPane = -1
			m.view.FocusedPane = 0
			m.worktreeTable.Focus()
			return nil
		},
		FocusStatus: func() tea.Cmd {
			m.view.ZoomedPane = -1
			m.view.FocusedPane = 1
			m.rebuildStatusContentWithHighlight()
			return nil
		},
		FocusLog: func() tea.Cmd {
			m.view.ZoomedPane = -1
			m.view.FocusedPane = 2
			m.logTable.Focus()
			return nil
		},
		SortCycle: func() tea.Cmd {
			m.sortMode = (m.sortMode + 1) % 3
			m.updateTable()
			return nil
		},
	})

	commands.RegisterSettingsActions(registry, commands.SettingsHandlers{
		Theme: m.showThemeSelection,
		Help: func() tea.Cmd {
			helpScreen := appscreen.NewHelpScreen(m.view.WindowWidth, m.view.WindowHeight, m.config.CustomCommands, m.theme, m.config.IconsEnabled())
			m.screenManager.Push(helpScreen)
			return nil
		},
	})
}

func (m *Model) fetchPRDataWithState() tea.Cmd {
	m.ciCache = make(map[string]*ciCacheEntry)
	m.prDataLoaded = false
	m.updateTable()
	m.updateTableColumns(m.worktreeTable.Width())
	m.loading = true
	m.statusContent = "Fetching PR data..."
	m.loadingScreen = NewLoadingScreen("Fetching PR data...", m.theme, m.config.IconsEnabled())
	m.currentScreen = screenLoading
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
		m.view.WindowWidth,
		m.view.WindowHeight,
		m.originalTheme,
		m.theme,
	)

	listScreen.OnCursorChange = func(item appscreen.SelectionItem) {
		m.UpdateTheme(item.ID)
	}

	listScreen.OnSelect = func(item appscreen.SelectionItem) tea.Cmd {
		m.confirmScreen = NewConfirmScreen(fmt.Sprintf("Save theme '%s' to config file?", item.ID), m.theme)
		m.confirmAction = func() tea.Cmd {
			m.config.Theme = item.ID
			if err := config.SaveConfig(m.config); err != nil {
				m.debugf("failed to save config: %v", err)
			}
			m.originalTheme = ""
			return nil
		}
		m.currentScreen = screenConfirm
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

	m.screenManager.Push(listScreen)
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

	m.worktreeTable.SetStyles(s)
	m.logTable.SetStyles(s)

	// Update spinner style
	m.spinner.Style = lipgloss.NewStyle().Foreground(thm.Accent)

	// Update filter input styles
	m.filterInput.PromptStyle = lipgloss.NewStyle().Foreground(thm.Accent)
	m.filterInput.TextStyle = lipgloss.NewStyle().Foreground(thm.TextFg)

	// Update other screens if they exist
	if m.confirmScreen != nil {
		m.confirmScreen.thm = thm
	}
	if m.infoScreen != nil {
		m.infoScreen.thm = thm
	}
	if m.loadingScreen != nil {
		m.loadingScreen.thm = thm
	}
	if m.screenManager.IsActive() && m.screenManager.Type() == appscreen.TypeInput {
		if inputScreen, ok := m.screenManager.Current().(*appscreen.InputScreen); ok {
			inputScreen.Thm = thm
		}
	}
	if m.screenManager.IsActive() && m.screenManager.Type() == appscreen.TypePRSelect {
		if prScreen, ok := m.screenManager.Current().(*appscreen.PRSelectionScreen); ok {
			prScreen.Thm = thm
		}
	}
	if m.screenManager.IsActive() && m.screenManager.Type() == appscreen.TypeIssueSelect {
		if issueScreen, ok := m.screenManager.Current().(*appscreen.IssueSelectionScreen); ok {
			issueScreen.Thm = thm
		}
	}
	if m.screenManager.IsActive() && m.screenManager.Type() == appscreen.TypeChecklist {
		if checkScreen, ok := m.screenManager.Current().(*appscreen.ChecklistScreen); ok {
			checkScreen.Thm = thm
		}
	}
	if m.screenManager.IsActive() && m.screenManager.Type() == appscreen.TypeHelp {
		if helpScreen, ok := m.screenManager.Current().(*appscreen.HelpScreen); ok {
			helpScreen.Thm = thm
		}
	}
	if m.commitFilesScreen != nil {
		m.commitFilesScreen.thm = thm
	}

	// Re-render info content with new theme
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.filteredWts) {
		m.infoContent = m.buildInfoContent(m.filteredWts[m.selectedIndex])
	}
}

func (m *Model) showRunCommand() tea.Cmd {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
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

	m.screenManager.Push(inputScr)
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
	if m.view.FocusedPane != 2 {
		return nil
	}

	// Validate: commit must be selected
	if len(m.logEntries) == 0 {
		return nil
	}

	cursor := m.logTable.Cursor()
	if cursor < 0 || cursor >= len(m.logEntries) {
		return nil
	}

	// Get source worktree and commit
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	sourceWorktree := m.filteredWts[m.selectedIndex]
	selectedCommit := m.logEntries[cursor]

	// Build worktree selection items (exclude source worktree)
	items := make([]selectionItem, 0, len(m.worktrees)-1)
	for _, wt := range m.worktrees {
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
		m.view.WindowWidth,
		m.view.WindowHeight,
		"",
		m.theme,
	)

	listScreen.OnSelect = func(item appscreen.SelectionItem) tea.Cmd {
		var targetWorktree *models.WorktreeInfo
		for _, wt := range m.worktrees {
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

	m.screenManager.Push(listScreen)
	return textinput.Blink
}

func (m *Model) showCommitFilesScreen(commitSHA, worktreePath string) tea.Cmd {
	return func() tea.Msg {
		files, err := m.git.GetCommitFiles(m.ctx, commitSHA, worktreePath)
		if err != nil {
			return errMsg{err: err}
		}
		// Fetch commit metadata
		metaRaw := m.git.RunGit(
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
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.filteredWts) {
		return nil
	}
	if len(m.logEntries) == 0 {
		return nil
	}

	cursor := m.logTable.Cursor()
	if cursor < 0 || cursor >= len(m.logEntries) {
		return nil
	}
	entry := m.logEntries[cursor]
	wt := m.filteredWts[m.selectedIndex]

	return m.showCommitFilesScreen(entry.sha, wt.Path)
}

func (m *Model) persistCurrentSelection() {
	idx := m.selectedIndex
	if idx < 0 || idx >= len(m.filteredWts) {
		idx = m.worktreeTable.Cursor()
	}
	if idx < 0 || idx >= len(m.filteredWts) {
		return
	}
	m.persistLastSelected(m.filteredWts[idx].Path)
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
		}
		switch {
		case cmd.Tmux != nil:
			tmuxItems = append(tmuxItems, item)
		case cmd.Zellij != nil:
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
			})
		}
	}

	// Build result with sections
	var items []commands.PaletteItem
	if len(regularItems) > 0 {
		items = append(items, commands.PaletteItem{Label: "Custom Commands", IsSection: true})
		items = append(items, regularItems...)
	}

	// Multiplexer section for custom tmux/zellij commands
	if hasTmux || hasZellij {
		items = append(items, commands.PaletteItem{Label: "Multiplexer", IsSection: true})
		if hasTmux {
			items = append(items, tmuxItems...)
		}
		if hasZellij {
			items = append(items, zellijItems...)
		}
	}

	// Active Tmux Sessions section (appears after Multiplexer)
	if len(activeTmuxSessions) > 0 {
		items = append(items, commands.PaletteItem{Label: "Active Tmux Sessions", IsSection: true})
		items = append(items, activeTmuxSessions...)
	}

	// Active Zellij Sessions section (appears after Active Tmux Sessions)
	if len(activeZellijSessions) > 0 {
		items = append(items, commands.PaletteItem{Label: "Active Zellij Sessions", IsSection: true})
		items = append(items, activeZellijSessions...)
	}

	return items
}
