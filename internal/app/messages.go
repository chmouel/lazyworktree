package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/models"
)

// handleWorktreeMessages processes worktree-related messages.
func (m *Model) handleWorktreeMessages(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case worktreesLoadedMsg:
		return m.handleWorktreesLoaded(msg)
	case cachedWorktreesMsg:
		return m.handleCachedWorktrees(msg)
	case pruneResultMsg:
		return m.handlePruneResult(msg)
	case absorbMergeResultMsg:
		return m.handleAbsorbResult(msg)
	default:
		return m, nil
	}
}

// handleWorktreesLoaded processes worktrees loaded message.
func (m *Model) handleWorktreesLoaded(msg worktreesLoadedMsg) (tea.Model, tea.Cmd) {
	m.worktreesLoaded = true
	m.loading = false
	if m.currentScreen == screenLoading {
		m.currentScreen = screenNone
		m.loadingScreen = nil
	}
	if msg.err != nil {
		m.showInfo(fmt.Sprintf("Error loading worktrees: %v", msg.err), nil)
		return m, nil
	}
	m.worktrees = msg.worktrees
	// Populate LastSwitchedTS from access history
	for _, wt := range m.worktrees {
		if ts, ok := m.accessHistory[wt.Path]; ok {
			wt.LastSwitchedTS = ts
		}
	}
	m.detailsCache = make(map[string]*detailsCacheEntry)
	m.ensureRepoConfig()
	m.updateTable()
	if m.pendingSelectWorktreePath != "" {
		for i, wt := range m.filteredWts {
			if wt.Path == m.pendingSelectWorktreePath {
				m.worktreeTable.SetCursor(i)
				m.selectedIndex = i
				break
			}
		}
		m.pendingSelectWorktreePath = ""
	}
	m.saveCache()
	if len(m.worktrees) == 0 {
		cwd, _ := os.Getwd()
		m.welcomeScreen = NewWelcomeScreen(cwd, m.getRepoWorktreeDir(), m.theme)
		m.currentScreen = screenWelcome
		return m, nil
	}
	if m.currentScreen == screenWelcome {
		m.currentScreen = screenNone
		m.welcomeScreen = nil
	}
	if m.config.AutoFetchPRs && !m.prDataLoaded {
		m.loading = true
		m.loadingScreen = NewLoadingScreen("Fetching PR data...", m.theme)
		m.currentScreen = screenLoading
		return m, m.fetchPRData()
	}
	return m, m.updateDetailsView()
}

// handleCachedWorktrees processes cached worktrees message.
func (m *Model) handleCachedWorktrees(msg cachedWorktreesMsg) (tea.Model, tea.Cmd) {
	if m.worktreesLoaded || len(msg.worktrees) == 0 {
		return m, nil
	}
	m.worktrees = msg.worktrees
	// Populate LastSwitchedTS from access history
	for _, wt := range m.worktrees {
		if ts, ok := m.accessHistory[wt.Path]; ok {
			wt.LastSwitchedTS = ts
		}
	}
	m.updateTable()
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.filteredWts) {
		m.infoContent = m.buildInfoContent(m.filteredWts[m.selectedIndex])
	}
	m.statusContent = loadingRefreshWorktrees
	return m, nil
}

// handlePruneResult processes prune result message.
func (m *Model) handlePruneResult(msg pruneResultMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err == nil && msg.worktrees != nil {
		m.worktrees = msg.worktrees
		m.updateTable()
		m.saveCache()
	}
	summary := fmt.Sprintf("Pruned %d merged worktrees", msg.pruned)
	if msg.failed > 0 {
		summary = fmt.Sprintf("%s (%d failed)", summary, msg.failed)
	}
	m.statusContent = summary
	return m, nil
}

// handleAbsorbResult processes absorb merge result message.
func (m *Model) handleAbsorbResult(msg absorbMergeResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.infoScreen = NewInfoScreen(fmt.Sprintf("Absorb failed\n\n%s", msg.err.Error()), m.theme)
		m.currentScreen = screenInfo
		return m, nil
	}
	cmd := m.deleteWorktreeCmd(&models.WorktreeInfo{Path: msg.path, Branch: msg.branch})
	if cmd != nil {
		return m, cmd()
	}
	return m, nil
}

// handlePRMessages processes PR and CI-related messages.
func (m *Model) handlePRMessages(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case prDataLoadedMsg:
		return m.handlePRDataLoaded(msg)
	case ciStatusLoadedMsg:
		return m.handleCIStatusLoaded(msg)
	default:
		return m, nil
	}
}

// handlePRDataLoaded processes PR data loaded message.
func (m *Model) handlePRDataLoaded(msg prDataLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if m.currentScreen == screenLoading {
		m.currentScreen = screenNone
		m.loadingScreen = nil
	}
	if msg.err == nil {
		for _, wt := range m.worktrees {
			// First try matching by local branch name from the prMap
			if msg.prMap != nil {
				if pr, ok := msg.prMap[wt.Branch]; ok {
					wt.PR = pr
					continue
				}
			}
			// Then check if we have a direct worktree PR lookup
			// This handles fork PRs where local branch differs from remote
			if msg.worktreePRs != nil {
				if pr, ok := msg.worktreePRs[wt.Path]; ok {
					wt.PR = pr
				}
			}
		}
		m.prDataLoaded = true
		// Update columns before rows to include the PR column
		m.updateTableColumns(m.worktreeTable.Width())
		m.updateTable()
		return m, m.updateDetailsView()
	}
	return m, nil
}

// handleCIStatusLoaded processes CI status loaded message.
func (m *Model) handleCIStatusLoaded(msg ciStatusLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err == nil && msg.checks != nil {
		m.ciCache[msg.branch] = &ciCacheEntry{
			checks:    msg.checks,
			fetchedAt: time.Now(),
		}
		// Refresh info content to show CI status
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.filteredWts) {
			wt := m.filteredWts[m.selectedIndex]
			if wt.Branch == msg.branch {
				m.infoContent = m.buildInfoContent(wt)
			}
		}
	}
	return m, nil
}

// handleOpenPRsLoaded handles the result of fetching open PRs.
func (m *Model) handleOpenPRsLoaded(msg openPRsLoadedMsg) tea.Cmd {
	if msg.err != nil {
		m.showInfo(fmt.Sprintf("Failed to fetch PRs: %v", msg.err), nil)
		return nil
	}

	if len(msg.prs) == 0 {
		m.showInfo("No open PRs/MRs found.", nil)
		return nil
	}

	// Show PR selection screen
	m.prSelectionScreen = NewPRSelectionScreen(msg.prs, m.windowWidth, m.windowHeight, m.theme, m.config.ShowIcons)
	m.prSelectionSubmit = func(pr *models.PRInfo) tea.Cmd {
		// Generate branch name
		defaultName := ""
		scriptErr := ""

		// If branch_name_script is configured, run it with PR title and body
		if m.config.BranchNameScript != "" {
			prContent := fmt.Sprintf("%s\n\n%s", pr.Title, pr.Body)
			if generatedName, err := runBranchNameScript(m.ctx, m.config.BranchNameScript, prContent); err != nil {
				scriptErr = fmt.Sprintf("Branch name script error: %v", err)
			} else if generatedName != "" {
				defaultName = generatedName
			}
		}

		// If no script or script returned empty, use default generation
		if defaultName == "" {
			template := m.config.PRBranchNameTemplate
			if template == "" {
				template = "pr-{number}-{title}"
			}
			defaultName = generatePRWorktreeName(pr, template)
		}

		// Suggest branch name (check for duplicates)
		suggested := strings.TrimSpace(defaultName)
		if suggested != "" {
			suggested = m.suggestBranchName(suggested)
		}

		if scriptErr != "" {
			m.showInfo(scriptErr, func() tea.Msg {
				m.inputScreen = NewInputScreen(
					fmt.Sprintf("Create worktree from PR #%d (branch: %s)", pr.Number, pr.Branch),
					"Worktree name",
					suggested,
					m.theme,
				)
				m.inputSubmit = func(value string, checked bool) (tea.Cmd, bool) {
					newBranch := strings.TrimSpace(value)
					if newBranch == "" {
						m.inputScreen.errorMsg = errBranchEmpty
						return nil, false
					}

					// Prevent duplicates
					for _, wt := range m.worktrees {
						if wt.Branch == newBranch {
							m.inputScreen.errorMsg = fmt.Sprintf("Branch %q already exists.", newBranch)
							return nil, false
						}
					}

					targetPath := filepath.Join(m.getRepoWorktreeDir(), newBranch)
					if _, err := os.Stat(targetPath); err == nil {
						m.inputScreen.errorMsg = fmt.Sprintf("Path already exists: %s", targetPath)
						return nil, false
					}

					// Validate that PR has a branch
					if pr.Branch == "" {
						m.inputScreen.errorMsg = errPRBranchMissing
						return nil, false
					}

					m.inputScreen.errorMsg = ""
					if err := os.MkdirAll(m.getRepoWorktreeDir(), defaultDirPerms); err != nil {
						return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to create worktree directory: %w", err)} }, true
					}

					// Create worktree from PR branch (can take time, so do it async with a loading pulse)
					m.loading = true
					m.statusContent = fmt.Sprintf("Creating worktree from PR/MR #%d...", pr.Number)
					m.loadingScreen = NewLoadingScreen(m.statusContent, m.theme)
					m.currentScreen = screenLoading
					m.pendingSelectWorktreePath = targetPath
					return func() tea.Msg {
						ok := m.git.CreateWorktreeFromPR(m.ctx, pr.Number, pr.Branch, newBranch, targetPath)
						if !ok {
							return createFromPRResultMsg{
								prNumber:   pr.Number,
								branch:     newBranch,
								targetPath: targetPath,
								err:        fmt.Errorf("create worktree from PR/MR branch %q", pr.Branch),
							}
						}
						return createFromPRResultMsg{
							prNumber:   pr.Number,
							branch:     newBranch,
							targetPath: targetPath,
							err:        nil,
						}
					}, true
				}
				m.currentScreen = screenInput
				return nil
			})
			return nil
		}

		// Show input screen with generated name
		m.inputScreen = NewInputScreen(
			fmt.Sprintf("Create worktree from PR #%d (branch: %s)", pr.Number, pr.Branch),
			"Worktree name",
			suggested,
			m.theme,
		)
		m.inputSubmit = func(value string, checked bool) (tea.Cmd, bool) {
			newBranch := strings.TrimSpace(value)
			if newBranch == "" {
				m.inputScreen.errorMsg = errBranchEmpty
				return nil, false
			}

			// Prevent duplicates
			for _, wt := range m.worktrees {
				if wt.Branch == newBranch {
					m.inputScreen.errorMsg = fmt.Sprintf("Branch %q already exists.", newBranch)
					return nil, false
				}
			}

			targetPath := filepath.Join(m.getRepoWorktreeDir(), newBranch)
			if _, err := os.Stat(targetPath); err == nil {
				m.inputScreen.errorMsg = fmt.Sprintf("Path already exists: %s", targetPath)
				return nil, false
			}

			// Validate that PR has a branch
			if pr.Branch == "" {
				m.inputScreen.errorMsg = errPRBranchMissing
				return nil, false
			}

			m.inputScreen.errorMsg = ""
			if err := os.MkdirAll(m.getRepoWorktreeDir(), defaultDirPerms); err != nil {
				return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to create worktree directory: %w", err)} }, true
			}

			// Create worktree from PR branch (can take time, so do it async with a loading pulse)
			m.loading = true
			m.statusContent = fmt.Sprintf("Creating worktree from PR/MR #%d...", pr.Number)
			m.loadingScreen = NewLoadingScreen(m.statusContent, m.theme)
			m.currentScreen = screenLoading
			m.pendingSelectWorktreePath = targetPath
			return func() tea.Msg {
				ok := m.git.CreateWorktreeFromPR(m.ctx, pr.Number, pr.Branch, pr.Branch, targetPath)
				if !ok {
					return createFromPRResultMsg{
						prNumber:   pr.Number,
						branch:     pr.Branch,
						targetPath: targetPath,
						err:        fmt.Errorf("create worktree from PR/MR branch %q", pr.Branch),
					}
				}
				return createFromPRResultMsg{prNumber: pr.Number, branch: pr.Branch, targetPath: targetPath}
			}, true
		}
		m.currentScreen = screenInput
		return textinput.Blink
	}
	m.currentScreen = screenPRSelect
	return textinput.Blink
}

// handleOpenIssuesLoaded handles the result of fetching open issues.
func (m *Model) handleOpenIssuesLoaded(msg openIssuesLoadedMsg) tea.Cmd {
	if msg.err != nil {
		m.showInfo(fmt.Sprintf("Failed to fetch issues: %v", msg.err), nil)
		return nil
	}

	if len(msg.issues) == 0 {
		m.showInfo("No open issues found.", nil)
		return nil
	}

	// Show issue selection screen
	m.issueSelectionScreen = NewIssueSelectionScreen(msg.issues, m.windowWidth, m.windowHeight, m.theme, m.config.ShowIcons)
	m.issueSelectionSubmit = func(issue *models.IssueInfo) tea.Cmd {
		// Show base branch selection
		defaultBase := m.git.GetMainBranch(m.ctx)
		return m.showBranchSelection(
			fmt.Sprintf("Select base branch for issue #%d", issue.Number),
			"Filter branches...",
			"No branches found.",
			defaultBase,
			func(baseBranch string) tea.Cmd {
				// Generate branch name
				defaultName := ""
				scriptErr := ""

				// If branch_name_script is configured, run it with issue title and body
				if m.config.BranchNameScript != "" {
					issueContent := fmt.Sprintf("%s\n\n%s", issue.Title, issue.Body)
					if generatedName, err := runBranchNameScript(m.ctx, m.config.BranchNameScript, issueContent); err != nil {
						scriptErr = fmt.Sprintf("Branch name script error: %v", err)
					} else if generatedName != "" {
						defaultName = generatedName
					}
				}

				// If no script or script returned empty, use default generation
				if defaultName == "" {
					template := m.config.IssueBranchNameTemplate
					if template == "" {
						template = "issue-{number}-{title}"
					}
					defaultName = generateIssueWorktreeName(issue, template)
				}

				// Suggest branch name (check for duplicates)
				suggested := strings.TrimSpace(defaultName)
				if suggested != "" {
					suggested = m.suggestBranchName(suggested)
				}

				if scriptErr != "" {
					m.showInfo(scriptErr, func() tea.Msg {
						cmd := m.showBranchNameInput(baseBranch, suggested)
						if cmd != nil {
							return cmd()
						}
						return nil
					})
					return nil
				}

				// Show input screen with generated name
				m.inputScreen = NewInputScreen(
					fmt.Sprintf("Create worktree from issue #%d", issue.Number),
					"Worktree name",
					suggested,
					m.theme,
				)
				m.inputSubmit = func(value string, checked bool) (tea.Cmd, bool) {
					newBranch := strings.TrimSpace(value)
					if newBranch == "" {
						m.inputScreen.errorMsg = errBranchEmpty
						return nil, false
					}

					// Prevent duplicates
					for _, wt := range m.worktrees {
						if wt.Branch == newBranch {
							m.inputScreen.errorMsg = fmt.Sprintf("Branch %q already exists.", newBranch)
							return nil, false
						}
					}

					targetPath := filepath.Join(m.getRepoWorktreeDir(), newBranch)
					if _, err := os.Stat(targetPath); err == nil {
						m.inputScreen.errorMsg = fmt.Sprintf("Path already exists: %s", targetPath)
						return nil, false
					}

					m.inputScreen.errorMsg = ""
					if err := os.MkdirAll(m.getRepoWorktreeDir(), defaultDirPerms); err != nil {
						return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to create worktree directory: %w", err)} }, true
					}

					// Create worktree from base branch (can take time, so do it async with a loading pulse)
					m.loading = true
					m.statusContent = fmt.Sprintf("Creating worktree from issue #%d...", issue.Number)
					m.loadingScreen = NewLoadingScreen(m.statusContent, m.theme)
					m.currentScreen = screenLoading
					m.pendingSelectWorktreePath = targetPath
					return func() tea.Msg {
						ok := m.git.RunCommandChecked(
							m.ctx,
							[]string{"git", "worktree", "add", "-b", newBranch, targetPath, baseBranch},
							"",
							fmt.Sprintf("Failed to create worktree %s from %s", newBranch, baseBranch),
						)
						if !ok {
							return createFromIssueResultMsg{
								issueNumber: issue.Number,
								branch:      newBranch,
								targetPath:  targetPath,
								err:         fmt.Errorf("create worktree from issue #%d", issue.Number),
							}
						}
						return createFromIssueResultMsg{issueNumber: issue.Number, branch: newBranch, targetPath: targetPath}
					}, true
				}
				m.currentScreen = screenInput
				return textinput.Blink
			},
		)
	}
	m.currentScreen = screenIssueSelect
	return textinput.Blink
}

// handleCreateFromChangesReady handles the result of checking for changes.
func (m *Model) handleCreateFromChangesReady(msg createFromChangesReadyMsg) tea.Cmd {
	wt := msg.worktree
	currentBranch := msg.currentBranch

	// Generate default name based on current branch
	defaultName := fmt.Sprintf("%s-changes", currentBranch)

	// If branch_name_script is configured, run it to generate a suggested name
	scriptErr := ""
	if m.config.BranchNameScript != "" && msg.diff != "" {
		if generatedName, err := runBranchNameScript(m.ctx, m.config.BranchNameScript, msg.diff); err != nil {
			// Log error but continue with default name
			scriptErr = fmt.Sprintf("Branch name script error: %v", err)
		} else if generatedName != "" {
			defaultName = generatedName
		}
	}

	if scriptErr != "" {
		m.showInfo(scriptErr, func() tea.Msg {
			cmd := m.showCreateFromChangesInput(wt, currentBranch, defaultName)
			if cmd != nil {
				return cmd()
			}
			return nil
		})
		return nil
	}

	return m.showCreateFromChangesInput(wt, currentBranch, defaultName)
}

// handleCherryPickResult handles the result of a cherry-pick operation.
func (m *Model) handleCherryPickResult(msg cherryPickResultMsg) tea.Cmd {
	if msg.err != nil {
		errorMessage := fmt.Sprintf("Cherry-pick failed\n\nCommit: %s\nTarget: %s (%s)\n\nError: %v",
			msg.commitSHA,
			filepath.Base(msg.targetWorktree.Path),
			msg.targetWorktree.Branch,
			msg.err)
		m.showInfo(errorMessage, nil)
		return nil
	}

	successMessage := fmt.Sprintf("Cherry-pick successful\n\nCommit: %s\nApplied to: %s (%s)",
		msg.commitSHA,
		filepath.Base(msg.targetWorktree.Path),
		msg.targetWorktree.Branch)
	m.showInfo(successMessage, m.refreshWorktrees())
	return nil
}
