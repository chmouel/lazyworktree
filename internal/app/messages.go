package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/chmouel/lazyworktree/internal/app/screen"
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
		m.clearLoadingScreen()
	}
	if msg.err != nil {
		m.showInfo(fmt.Sprintf("Error loading worktrees: %v", msg.err), nil)
		return m, nil
	}

	// Preserve PR state across worktree reload to prevent race condition
	prStateMap := extractPRState(m.state.data.worktrees)
	m.state.data.worktrees = msg.worktrees
	restorePRState(m.state.data.worktrees, prStateMap)
	m.pruneStaleWorktreeNotes(m.state.data.worktrees)
	if !m.prDataLoaded && hasPRData(m.state.data.worktrees) {
		m.prDataLoaded = true
		m.updateTableColumns(m.state.ui.worktreeTable.Width())
	}

	// Populate LastSwitchedTS from access history
	for _, wt := range m.state.data.worktrees {
		if ts, ok := m.state.data.accessHistory[wt.Path]; ok {
			wt.LastSwitchedTS = ts
		}
	}
	m.resetDetailsCache()
	m.ensureRepoConfig()

	// If we have a pending selection (newly created worktree), record access first
	if m.pendingSelectWorktreePath != "" {
		m.recordAccess(m.pendingSelectWorktreePath)
		// Update the LastSwitchedTS for this worktree before sorting
		for _, wt := range m.state.data.worktrees {
			if wt.Path == m.pendingSelectWorktreePath {
				wt.LastSwitchedTS = m.state.data.accessHistory[wt.Path]
				break
			}
		}
	}

	// Apply pending PR metadata to the specific worktree path it was created for.
	if m.pendingPR != nil && m.pendingPRPath != "" {
		for _, wt := range m.state.data.worktrees {
			if wt.Path != m.pendingPRPath {
				continue
			}
			wt.PR = m.pendingPR
			wt.PRFetchStatus = models.PRFetchStatusLoaded
			if !m.prDataLoaded {
				m.prDataLoaded = true
				m.updateTableColumns(m.state.ui.worktreeTable.Width())
			}
			m.pendingPR = nil
			m.pendingPRPath = ""
			break
		}
	}

	// Now update table with the new timestamp
	m.updateTable()

	if m.pendingSelectWorktreePath != "" {
		// Find and select the worktree in the filtered list
		for i, wt := range m.state.data.filteredWts {
			if wt.Path == m.pendingSelectWorktreePath {
				m.state.ui.worktreeTable.SetCursor(i)
				m.state.data.selectedIndex = i
				break
			}
		}
		m.pendingSelectWorktreePath = ""
	}
	m.saveCache()
	if len(m.state.data.worktrees) == 0 {
		cwd, _ := os.Getwd()
		ws := screen.NewWelcomeScreen(cwd, m.getRepoWorktreeDir(), m.theme)
		ws.OnRefresh = func() tea.Cmd {
			return m.refreshWorktrees()
		}
		ws.OnQuit = func() tea.Cmd {
			m.quitting = true
			m.stopGitWatcher()
			return tea.Quit
		}
		m.state.ui.screenManager.Push(ws)
		return m, nil
	}
	// Clear welcome screen if worktrees were found
	if m.state.ui.screenManager.Type() == screen.TypeWelcome {
		m.state.ui.screenManager.Pop()
	}
	cmds := []tea.Cmd{}
	if cmd := m.updateDetailsView(); cmd != nil {
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

	// Filter out stale entries that no longer exist in git
	// If validPaths is nil, git service is unavailable - skip validation
	validPaths := m.getValidWorktreePaths()
	var validated []*models.WorktreeInfo
	if validPaths == nil {
		validated = msg.worktrees
	} else {
		validated = make([]*models.WorktreeInfo, 0, len(msg.worktrees))
		for _, wt := range msg.worktrees {
			if validPaths[normalizePath(wt.Path)] {
				validated = append(validated, wt)
			}
		}
		if len(validated) == 0 {
			return m, nil
		}
	}

	// Preserve PR state across worktree reload to prevent race condition
	prStateMap := extractPRState(m.state.data.worktrees)
	m.state.data.worktrees = validated
	restorePRState(m.state.data.worktrees, prStateMap)
	if !m.prDataLoaded && hasPRData(m.state.data.worktrees) {
		m.prDataLoaded = true
		m.updateTableColumns(m.state.ui.worktreeTable.Width())
	}
	// Populate LastSwitchedTS from access history
	for _, wt := range m.state.data.worktrees {
		if ts, ok := m.state.data.accessHistory[wt.Path]; ok {
			wt.LastSwitchedTS = ts
		}
	}
	m.updateTable()
	if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
		m.infoContent = m.buildInfoContent(m.state.data.filteredWts[m.state.data.selectedIndex])
	}
	m.statusContent = loadingRefreshWorktrees
	return m, nil
}

// handlePruneResult processes prune result message.
func (m *Model) handlePruneResult(msg pruneResultMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err == nil && msg.worktrees != nil {
		// Preserve PR state across worktree reload to prevent race condition
		prStateMap := extractPRState(m.state.data.worktrees)
		m.state.data.worktrees = msg.worktrees
		restorePRState(m.state.data.worktrees, prStateMap)
		m.pruneStaleWorktreeNotes(m.state.data.worktrees)
		if !m.prDataLoaded && hasPRData(m.state.data.worktrees) {
			m.prDataLoaded = true
			m.updateTableColumns(m.state.ui.worktreeTable.Width())
		}
		m.updateTable()
		m.saveCache()
	}
	var parts []string
	if msg.pruned > 0 {
		parts = append(parts, fmt.Sprintf("Pruned %d merged worktrees", msg.pruned))
	}
	if msg.orphansDeleted > 0 {
		parts = append(parts, fmt.Sprintf("deleted %d orphaned directories", msg.orphansDeleted))
	}
	if msg.failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", msg.failed))
	}
	if len(parts) == 0 {
		parts = append(parts, "Nothing to prune")
	}
	m.statusContent = strings.Join(parts, ", ")
	return m, nil
}

// handleAbsorbResult processes absorb merge result message.
func (m *Model) handleAbsorbResult(msg absorbMergeResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.showInfo(fmt.Sprintf("Absorb failed\n\n%s", msg.err.Error()), nil)
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
	case singlePRLoadedMsg:
		return m.handleSinglePRLoaded(msg)
	case ciStatusLoadedMsg:
		return m.handleCIStatusLoaded(msg)
	default:
		return m, nil
	}
}

// handlePRDataLoaded processes PR data loaded message.
func (m *Model) handlePRDataLoaded(msg prDataLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	m.clearLoadingScreen()
	if msg.err == nil {
		log.Printf("handlePRDataLoaded: prMap has %d entries, worktreePRs has %d entries, worktreeErrors has %d entries",
			len(msg.prMap), len(msg.worktreePRs), len(msg.worktreeErrors))

		for _, wt := range m.state.data.worktrees {
			// Clear previous status
			wt.PRFetchError = ""
			wt.PRFetchStatus = models.PRFetchStatusNoPR
			log.Printf("Processing worktree: Branch=%q Path=%q", wt.Branch, wt.Path)

			// First try matching by local branch name from the prMap
			if msg.prMap != nil {
				if pr, ok := msg.prMap[wt.Branch]; ok {
					wt.PR = pr
					wt.PRFetchStatus = models.PRFetchStatusLoaded
					log.Printf("  Assigned from prMap: PR#%d", pr.Number)
					continue
				} else {
					log.Printf("  Branch %q not found in prMap. Available keys:", wt.Branch)
					for key := range msg.prMap {
						log.Printf("    %q (match=%v, len=%d vs %d)",
							key, key == wt.Branch, len(key), len(wt.Branch))

						// Check for invisible characters
						if key != wt.Branch && strings.TrimSpace(key) == strings.TrimSpace(wt.Branch) {
							log.Printf("    whitespace difference detected")
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
			if wt.PR != nil {
				log.Printf("  Final: wt.PR = #%d, status = %s", wt.PR.Number, wt.PRFetchStatus)
			} else {
				log.Printf("  Final: wt.PR = nil, status = %s, error = %q", wt.PRFetchStatus, wt.PRFetchError)
			}
		}
		m.prDataLoaded = true
		// Update columns before rows to include the PR column
		m.updateTableColumns(m.state.ui.worktreeTable.Width())
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
		m.cache.ciCache.Set(msg.branch, msg.checks)
		// Refresh info content to show CI status
		if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
			wt := m.state.data.filteredWts[m.state.data.selectedIndex]
			if wt.Branch == msg.branch {
				m.infoContent = m.buildInfoContent(wt)
			}
		}
	}
	return m, nil
}

// handleSinglePRLoaded processes PR data loaded for a single worktree.
func (m *Model) handleSinglePRLoaded(msg singlePRLoadedMsg) (tea.Model, tea.Cmd) {
	// Find the worktree and update its PR
	for _, wt := range m.state.data.worktrees {
		if wt.Path == msg.worktreePath {
			switch {
			case msg.err != nil:
				wt.PRFetchError = msg.err.Error()
				wt.PRFetchStatus = models.PRFetchStatusError
				wt.PR = nil
			case msg.pr != nil:
				wt.PR = msg.pr
				wt.PRFetchStatus = models.PRFetchStatusLoaded
				wt.PRFetchError = ""
			default:
				wt.PR = nil
				wt.PRFetchStatus = models.PRFetchStatusNoPR
				wt.PRFetchError = ""
			}
			break
		}
	}

	// If PR data now exists, ensure prDataLoaded is set so table shows PR column
	if !m.prDataLoaded && hasPRData(m.state.data.worktrees) {
		m.prDataLoaded = true
		m.updateTableColumns(m.state.ui.worktreeTable.Width())
	}
	m.updateTable()

	// Refresh info content for currently selected worktree
	if m.state.data.selectedIndex >= 0 && m.state.data.selectedIndex < len(m.state.data.filteredWts) {
		m.infoContent = m.buildInfoContent(m.state.data.filteredWts[m.state.data.selectedIndex])
	}

	// Trigger CI fetch for the current worktree
	return m, m.maybeFetchCIStatus()
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

	// Build map of branches already attached to worktrees
	attachedBranches := make(map[string]string)
	for _, wt := range m.state.data.worktrees {
		if wt.Branch != "" {
			attachedBranches[wt.Branch] = filepath.Base(wt.Path)
		}
	}

	// Show PR selection screen
	prScr := screen.NewPRSelectionScreen(msg.prs, m.state.view.WindowWidth, m.state.view.WindowHeight, m.theme, m.config.IconsEnabled())
	prScr.AttachedBranches = attachedBranches
	prScr.OnSelect = func(pr *models.PRInfo) tea.Cmd {
		remoteBranch := strings.TrimSpace(pr.Branch)
		if remoteBranch == "" {
			m.showInfo(errPRBranchMissing, nil)
			return nil
		}

		template := strings.TrimSpace(m.config.PRBranchNameTemplate)
		if template == "" {
			template = "pr-{number}-{title}"
		}
		generatedTitle := ""
		if m.config.BranchNameScript != "" {
			prContent := fmt.Sprintf("%s\n\n%s", pr.Title, pr.Body)
			suggestedName := utils.GeneratePRWorktreeName(pr, template, "")
			aiTitle, scriptErr := runBranchNameScript(
				m.ctx,
				m.config.BranchNameScript,
				prContent,
				"pr",
				fmt.Sprintf("%d", pr.Number),
				template,
				suggestedName,
			)
			if scriptErr != nil {
				log.Printf("branch_name_script failed for PR #%d: %v", pr.Number, scriptErr)
			} else if aiTitle != "" {
				generatedTitle = aiTitle
			}
		}
		worktreeName := strings.TrimSpace(utils.GeneratePRWorktreeName(pr, template, generatedTitle))
		if worktreeName == "" {
			worktreeName = fmt.Sprintf("pr-%d", pr.Number)
		}

		requester := strings.TrimSpace(m.state.services.git.GetAuthenticatedUsername(m.ctx))
		isAuthor := requester != "" && strings.EqualFold(requester, strings.TrimSpace(pr.Author))
		useGeneratedBranch := requester != "" && !isAuthor

		localBranch := remoteBranch
		if useGeneratedBranch {
			localBranch = m.uniquePRGeneratedName(worktreeName)
			worktreeName = localBranch
		} else if wt := m.getWorktreeForBranch(localBranch); wt != nil {
			m.state.ui.screenManager.Clear()
			m.selectWorktreeByPath(wt.Path)
			m.showInfo(fmt.Sprintf("Branch %q is already checked out in worktree %q", localBranch, filepath.Base(wt.Path)), nil)
			return nil
		}

		targetPath := filepath.Join(m.getRepoWorktreeDir(), worktreeName)
		if m.worktreePathExists(targetPath) {
			m.showInfo(fmt.Sprintf("Path already exists: %s", targetPath), nil)
			return nil
		}

		if err := m.ensureWorktreeDir(m.getRepoWorktreeDir()); err != nil {
			return func() tea.Msg { return errMsg{err: err} }
		}

		// Create worktree from PR branch (can take time, so do it async with a loading pulse)
		m.loading = true
		m.statusContent = fmt.Sprintf("Creating worktree from PR/MR #%d...", pr.Number)
		m.state.ui.screenManager.Clear() // Clear all stacked screens before loading
		m.setLoadingScreen(m.statusContent)
		m.pendingSelectWorktreePath = targetPath
		return func() tea.Msg {
			ok := m.state.services.git.CreateWorktreeFromPR(m.ctx, pr.Number, remoteBranch, localBranch, targetPath)
			if !ok {
				return createFromPRResultMsg{
					prNumber:   pr.Number,
					branch:     localBranch,
					targetPath: targetPath,
					err:        fmt.Errorf("create worktree from PR/MR branch %q", remoteBranch),
				}
			}
			noteText, err := m.generateWorktreeNote("pr", pr.Number, pr.Title, pr.Body, pr.URL)
			if err != nil {
				m.debugf("worktree note script error for PR/MR #%d: %v", pr.Number, err)
			}
			return createFromPRResultMsg{
				prNumber:   pr.Number,
				branch:     localBranch,
				targetPath: targetPath,
				note:       noteText,
				pr:         pr,
			}
		}
	}
	prScr.OnCancel = func() tea.Cmd {
		return nil
	}
	m.state.ui.screenManager.Push(prScr)
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

	issueScr := screen.NewIssueSelectionScreen(msg.issues, m.state.view.WindowWidth, m.state.view.WindowHeight, m.theme, m.config.IconsEnabled())
	issueScr.OnSelect = func(issue *models.IssueInfo) tea.Cmd {
		defaultBase := m.state.services.git.GetMainBranch(m.ctx)
		return m.showBranchSelection(
			fmt.Sprintf("Select base branch for issue #%d", issue.Number),
			"Filter branches...",
			"No branches found.",
			defaultBase,
			func(baseBranch string) tea.Cmd {
				generatedTitle := ""
				scriptErr := ""

				if m.config.BranchNameScript != "" {
					issueContent := fmt.Sprintf("%s\n\n%s", issue.Title, issue.Body)
					template := m.config.IssueBranchNameTemplate
					if template == "" {
						template = "issue-{number}-{title}"
					}
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

				template := m.config.IssueBranchNameTemplate
				if template == "" {
					template = "issue-{number}-{title}"
				}

				defaultName := utils.GenerateIssueWorktreeName(issue, template, generatedTitle)

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

				inputScr := screen.NewInputScreen(
					fmt.Sprintf("Create worktree from issue #%d", issue.Number),
					"Worktree name",
					suggested,
					m.theme,
					m.config.IconsEnabled(),
				)

				inputScr.OnSubmit = func(value string, _ bool) tea.Cmd {
					newBranch := strings.TrimSpace(value)
					newBranch = sanitizeBranchNameFromTitle(newBranch, "")
					if newBranch == "" {
						inputScr.ErrorMsg = errBranchEmpty
						return nil
					}

					targetPath := filepath.Join(m.getRepoWorktreeDir(), newBranch)
					if errMsg := m.validateNewWorktreeTarget(newBranch, targetPath); errMsg != "" {
						inputScr.ErrorMsg = errMsg
						return nil
					}

					inputScr.ErrorMsg = ""
					if err := m.ensureWorktreeDir(m.getRepoWorktreeDir()); err != nil {
						return func() tea.Msg { return errMsg{err: err} }
					}

					// Create worktree from base branch (can take time, so do it async with a loading pulse)
					m.loading = true
					m.statusContent = fmt.Sprintf("Creating worktree from issue #%d...", issue.Number)
					m.state.ui.screenManager.Clear() // Clear all stacked screens before loading
					m.setLoadingScreen(m.statusContent)
					m.pendingSelectWorktreePath = targetPath
					return func() tea.Msg {
						ok := m.state.services.git.RunCommandChecked(
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
						noteText, err := m.generateWorktreeNote("issue", issue.Number, issue.Title, issue.Body, issue.URL)
						if err != nil {
							m.debugf("worktree note script error for issue #%d: %v", issue.Number, err)
						}
						return createFromIssueResultMsg{
							issueNumber: issue.Number,
							branch:      newBranch,
							targetPath:  targetPath,
							note:        noteText,
						}
					}
				}

				inputScr.OnCancel = func() tea.Cmd {
					return nil
				}

				m.state.ui.screenManager.Push(inputScr)
				return textinput.Blink
			},
		)
	}
	m.state.ui.screenManager.Push(issueScr)
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

// hasPRData checks if any worktree has PR data loaded.
func hasPRData(worktrees []*models.WorktreeInfo) bool {
	for _, wt := range worktrees {
		if wt.PR != nil {
			return true
		}
	}
	return false
}
