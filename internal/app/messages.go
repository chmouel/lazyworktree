package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	log "github.com/chmouel/lazyworktree/internal/log"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/utils"
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
	// Don't clear loading screen if we're in the middle of push/sync operations
	if m.loadingOperation != "push" && m.loadingOperation != "sync" {
		m.loading = false
		if m.currentScreen == screenLoading {
			m.currentScreen = screenNone
			m.loadingScreen = nil
		}
	}
	if msg.err != nil {
		m.showInfo(fmt.Sprintf("Error loading worktrees: %v", msg.err), nil)
		return m, nil
	}

	// Preserve PR state across worktree reload to prevent race condition
	prStateMap := extractPRState(m.worktrees)
	m.worktrees = msg.worktrees
	restorePRState(m.worktrees, prStateMap)

	// Populate LastSwitchedTS from access history
	for _, wt := range m.worktrees {
		if ts, ok := m.accessHistory[wt.Path]; ok {
			wt.LastSwitchedTS = ts
		}
	}
	m.detailsCache = make(map[string]*detailsCacheEntry)
	m.ensureRepoConfig()

	// If we have a pending selection (newly created worktree), record access first
	if m.pendingSelectWorktreePath != "" {
		m.recordAccess(m.pendingSelectWorktreePath)
		// Update the LastSwitchedTS for this worktree before sorting
		for _, wt := range m.worktrees {
			if wt.Path == m.pendingSelectWorktreePath {
				wt.LastSwitchedTS = m.accessHistory[wt.Path]
				break
			}
		}
	}

	// Now update table with the new timestamp
	m.updateTable()

	if m.pendingSelectWorktreePath != "" {
		// Find and select the worktree in the filtered list
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
	cmds := []tea.Cmd{}
	if m.config.AutoFetchPRs && !m.prDataLoaded {
		m.loading = true
		m.loadingScreen = NewLoadingScreen("Fetching PR data...", m.theme)
		m.currentScreen = screenLoading
		cmds = append(cmds, m.fetchPRData())
	} else if cmd := m.updateDetailsView(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.startAutoRefresh(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.startGitWatcher(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

// handleCachedWorktrees processes cached worktrees message.
func (m *Model) handleCachedWorktrees(msg cachedWorktreesMsg) (tea.Model, tea.Cmd) {
	if m.worktreesLoaded || len(msg.worktrees) == 0 {
		return m, nil
	}
	// Preserve PR state across worktree reload to prevent race condition
	prStateMap := extractPRState(m.worktrees)
	m.worktrees = msg.worktrees
	restorePRState(m.worktrees, prStateMap)
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
		// Preserve PR state across worktree reload to prevent race condition
		prStateMap := extractPRState(m.worktrees)
		m.worktrees = msg.worktrees
		restorePRState(m.worktrees, prStateMap)
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
		log.Printf("handlePRDataLoaded: prMap has %d entries, worktreePRs has %d entries, worktreeErrors has %d entries",
			len(msg.prMap), len(msg.worktreePRs), len(msg.worktreeErrors))

		for _, wt := range m.worktrees {
			// Clear previous status
			wt.PRFetchError = ""
			wt.PRFetchStatus = models.PRFetchStatusNoPR

			// DEBUG: Log assignment attempts
			log.Printf("Processing worktree: Branch=%q Path=%q", wt.Branch, wt.Path)

			// First try matching by local branch name from the prMap
			if msg.prMap != nil {
				if pr, ok := msg.prMap[wt.Branch]; ok {
					wt.PR = pr
					wt.PRFetchStatus = models.PRFetchStatusLoaded
					log.Printf("  Assigned from prMap: PR#%d", pr.Number)
					continue
				} else {
					// DEBUG: Show why match failed
					log.Printf("  Branch %q not found in prMap. Available keys:", wt.Branch)
					for key := range msg.prMap {
						log.Printf("    %q (match=%v, len=%d vs %d)",
							key, key == wt.Branch, len(key), len(wt.Branch))

						// Check for invisible characters
						if key != wt.Branch && strings.TrimSpace(key) == strings.TrimSpace(wt.Branch) {
							log.Printf("    ^ WHITESPACE DIFFERENCE DETECTED")
						}
					}
				}
			}
			// Then check if we have a direct worktree PR lookup
			// This handles fork PRs where local branch differs from remote
			if msg.worktreePRs != nil {
				if pr, ok := msg.worktreePRs[wt.Path]; ok {
					wt.PR = pr
					wt.PRFetchStatus = models.PRFetchStatusLoaded
					log.Printf("  Assigned from worktreePRs: PR#%d", pr.Number)
					continue
				}
			}

			// Check if there was an error for this worktree
			if msg.worktreeErrors != nil {
				if errMsg, hasErr := msg.worktreeErrors[wt.Path]; hasErr {
					wt.PRFetchError = errMsg
					wt.PRFetchStatus = models.PRFetchStatusError
					log.Printf("  Error: %s", errMsg)
				} else {
					log.Printf("  No PR found in either map, no error")
				}
			}

			// DEBUG: Final status
			if wt.PR != nil {
				log.Printf("  Final: wt.PR = #%d, status = %s", wt.PR.Number, wt.PRFetchStatus)
			} else {
				log.Printf("  Final: wt.PR = nil, status = %s, error = %q", wt.PRFetchStatus, wt.PRFetchError)
			}
		}
		m.prDataLoaded = true
		// Update columns before rows to include the PR column
		m.updateTableColumns(m.worktreeTable.Width())
		m.updateTable()

		// If we were triggered from showPruneMerged, run the merged check now
		if m.checkMergedAfterPRRefresh {
			m.checkMergedAfterPRRefresh = false
			return m, m.performMergedWorktreeCheck()
		}

		return m, m.updateDetailsView()
	}
	// Even if PR fetch failed, run merged check if requested (will fall back to git-based detection)
	if m.checkMergedAfterPRRefresh {
		m.checkMergedAfterPRRefresh = false
		return m, m.performMergedWorktreeCheck()
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
		// Get AI-generated title (if configured)
		generatedTitle := ""
		scriptErr := ""

		if m.config.BranchNameScript != "" {
			prContent := fmt.Sprintf("%s\n\n%s", pr.Title, pr.Body)
			template := m.config.PRBranchNameTemplate
			if template == "" {
				template = "pr-{number}-{title}"
			}
			// Pass empty string for generatedTitle since we're getting it now
			suggestedName := utils.GeneratePRWorktreeName(pr, template, "")

			if aiTitle, err := runBranchNameScript(
				m.ctx,
				m.config.BranchNameScript,
				prContent,
				"pr",
				fmt.Sprintf("%d", pr.Number),
				template,
				suggestedName,
			); err != nil {
				scriptErr = fmt.Sprintf("Branch name script error: %v", err)
			} else if aiTitle != "" {
				generatedTitle = aiTitle
			}
		}

		// Apply template with both original and generated titles
		template := m.config.PRBranchNameTemplate
		if template == "" {
			template = "pr-{number}-{title}"
		}

		defaultName := utils.GeneratePRWorktreeName(pr, template, generatedTitle)

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
					newBranch = sanitizeBranchNameFromTitle(newBranch, "")
					if newBranch == "" {
						m.inputScreen.errorMsg = errBranchEmpty
						return nil, false
					}

					targetPath := filepath.Join(m.getRepoWorktreeDir(), newBranch)
					if errMsg := m.validateNewWorktreeTarget(newBranch, targetPath); errMsg != "" {
						m.inputScreen.errorMsg = errMsg
						return nil, false
					}

					// Validate that PR has a branch
					if pr.Branch == "" {
						m.inputScreen.errorMsg = errPRBranchMissing
						return nil, false
					}

					m.inputScreen.errorMsg = ""
					if err := m.ensureWorktreeDir(m.getRepoWorktreeDir()); err != nil {
						return func() tea.Msg { return errMsg{err: err} }, true
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
			newBranch = sanitizeBranchNameFromTitle(newBranch, "")
			if newBranch == "" {
				m.inputScreen.errorMsg = errBranchEmpty
				return nil, false
			}

			targetPath := filepath.Join(m.getRepoWorktreeDir(), newBranch)
			if errMsg := m.validateNewWorktreeTarget(newBranch, targetPath); errMsg != "" {
				m.inputScreen.errorMsg = errMsg
				return nil, false
			}

			// Validate that PR has a branch
			if pr.Branch == "" {
				m.inputScreen.errorMsg = errPRBranchMissing
				return nil, false
			}

			m.inputScreen.errorMsg = ""
			if err := m.ensureWorktreeDir(m.getRepoWorktreeDir()); err != nil {
				return func() tea.Msg { return errMsg{err: err} }, true
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
				// Get AI-generated title (if configured)
				generatedTitle := ""
				scriptErr := ""

				if m.config.BranchNameScript != "" {
					issueContent := fmt.Sprintf("%s\n\n%s", issue.Title, issue.Body)
					template := m.config.IssueBranchNameTemplate
					if template == "" {
						template = "issue-{number}-{title}"
					}
					// Pass empty string for generatedTitle since we're getting it now
					suggestedName := utils.GenerateIssueWorktreeName(issue, template, "")

					if aiTitle, err := runBranchNameScript(
						m.ctx,
						m.config.BranchNameScript,
						issueContent,
						"issue",
						fmt.Sprintf("%d", issue.Number),
						template,
						suggestedName,
					); err != nil {
						scriptErr = fmt.Sprintf("Branch name script error: %v", err)
					} else if aiTitle != "" {
						generatedTitle = aiTitle
					}
				}

				// Apply template with both original and generated titles
				template := m.config.IssueBranchNameTemplate
				if template == "" {
					template = "issue-{number}-{title}"
				}

				defaultName := utils.GenerateIssueWorktreeName(issue, template, generatedTitle)

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
					newBranch = sanitizeBranchNameFromTitle(newBranch, "")
					if newBranch == "" {
						m.inputScreen.errorMsg = errBranchEmpty
						return nil, false
					}

					targetPath := filepath.Join(m.getRepoWorktreeDir(), newBranch)
					if errMsg := m.validateNewWorktreeTarget(newBranch, targetPath); errMsg != "" {
						m.inputScreen.errorMsg = errMsg
						return nil, false
					}

					m.inputScreen.errorMsg = ""
					if err := m.ensureWorktreeDir(m.getRepoWorktreeDir()); err != nil {
						return func() tea.Msg { return errMsg{err: err} }, true
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
		if generatedName, err := runBranchNameScript(
			m.ctx,
			m.config.BranchNameScript,
			msg.diff,
			"diff",
			"",
			"",
			"",
		); err != nil {
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

// prState holds all PR-related state for a worktree.
type prState struct {
	PR            *models.PRInfo
	PRFetchError  string
	PRFetchStatus string
}

// extractPRState creates a map of PR state indexed by worktree path.
// This preserves all PR-related information before worktree slice is replaced.
func extractPRState(worktrees []*models.WorktreeInfo) map[string]*prState {
	stateMap := make(map[string]*prState)
	for _, wt := range worktrees {
		if wt.PR != nil || wt.PRFetchError != "" || wt.PRFetchStatus != "" {
			stateMap[wt.Path] = &prState{
				PR:            wt.PR,
				PRFetchError:  wt.PRFetchError,
				PRFetchStatus: wt.PRFetchStatus,
			}
		}
	}
	return stateMap
}

// restorePRState applies previously extracted PR state to worktrees.
// This ensures all PR-related information persists across worktree reloads.
func restorePRState(worktrees []*models.WorktreeInfo, stateMap map[string]*prState) {
	for _, wt := range worktrees {
		if state, ok := stateMap[wt.Path]; ok {
			wt.PR = state.PR
			wt.PRFetchError = state.PRFetchError
			wt.PRFetchStatus = state.PRFetchStatus
		}
	}
}
