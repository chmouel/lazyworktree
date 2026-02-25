package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	appscreen "github.com/chmouel/lazyworktree/internal/app/screen"
	"github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/utils"
)

// ciDataSvc is the package-level CI data service instance.
var ciDataSvc = services.NewCIDataService()

// openCICheckSelection opens a selection screen for CI checks on the current worktree.
func (m *Model) openCICheckSelection() tea.Cmd {
	if m.config.DisablePR {
		m.showInfo("PR/MR display is disabled in configuration", nil)
		return nil
	}
	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		return nil
	}
	wt := m.state.data.filteredWts[m.state.data.selectedIndex]

	// Get CI checks from cache
	checks, _, ok := m.cache.ciCache.Get(wt.Branch)
	if !ok || len(checks) == 0 {
		m.showInfo("No CI checks available. Press 'p' to fetch PR data first.", nil)
		return nil
	}

	// Build selection items from CI checks
	items := make([]appscreen.SelectionItem, 0, len(checks))

	for i, check := range checks {
		symbol := getCIStatusIcon(check.Conclusion, false, m.config.IconsEnabled())
		iconStyle := m.ciIconStyle(check.Conclusion)
		label := fmt.Sprintf("%s %s", iconStyle.Render(symbol), check.Name)

		items = append(items, appscreen.SelectionItem{
			ID:    fmt.Sprintf("%d", i),
			Label: label,
		})
	}

	checks = sortCIChecks(checks)
	ciScreen := appscreen.NewListSelectionScreen(
		items,
		labelWithIcon(UIIconCICheck, "Select CI Check", m.config.IconsEnabled()),
		"Filter checks...",
		"No CI checks found.",
		m.state.view.WindowWidth,
		m.state.view.WindowHeight,
		"",
		m.theme,
	)
	ciScreen.FooterHint = "Enter open • Ctrl+v view logs • Ctrl+r restart"

	ciScreen.OnEnter = func(item appscreen.SelectionItem) tea.Cmd {
		var idx int
		if _, err := fmt.Sscanf(item.ID, "%d", &idx); err != nil || idx < 0 || idx >= len(checks) {
			return nil
		}
		return m.openURLInBrowser(checks[idx].Link)
	}

	ciScreen.OnCtrlV = func(item appscreen.SelectionItem) tea.Cmd {
		var idx int
		if _, err := fmt.Sscanf(item.ID, "%d", &idx); err != nil || idx < 0 || idx >= len(checks) {
			return nil
		}
		return m.showCICheckLog(checks[idx])
	}

	ciScreen.OnCtrlR = func(item appscreen.SelectionItem) tea.Cmd {
		var idx int
		if _, err := fmt.Sscanf(item.ID, "%d", &idx); err != nil || idx < 0 || idx >= len(checks) {
			return nil
		}
		return m.rerunCICheck(checks[idx])
	}

	ciScreen.OnCancel = func() tea.Cmd {
		return nil
	}

	m.state.ui.screenManager.Push(ciScreen)
	return textinput.Blink
}

// showCICheckLog opens the CI check log in a pager using gh run view.
// For external CI systems (non-GitHub Actions), it opens the check link in the browser.
func (m *Model) showCICheckLog(check *models.CICheck) tea.Cmd {
	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		return nil
	}
	wt := m.state.data.filteredWts[m.state.data.selectedIndex]

	// Extract run ID from the check link
	runID := extractRunIDFromLink(check.Link)
	if runID == "" {
		// Not a GitHub Actions URL - open in browser instead
		if check.Link == "" {
			m.showInfo("No link available for this check.", nil)
			return nil
		}
		return m.openURLInBrowser(check.Link)
	}

	// Build environment variables
	env := m.buildCommandEnv(wt.Branch, wt.Path)

	// Add CI-specific environment variables
	env["LW_CI_JOB_NAME"] = check.Name
	env["LW_CI_JOB_NAME_CLEAN"] = utils.SanitizeBranchName(check.Name, 0)
	env["LW_CI_RUN_ID"] = runID
	if !check.StartedAt.IsZero() {
		env["LW_CI_STARTED_AT"] = check.StartedAt.Format(time.RFC3339)
	}

	envVars := os.Environ()
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Use --log-failed for failed checks, --log for others
	logFlag := "--log"
	if check.Conclusion == iconFailure {
		logFlag = "--log-failed"
	}

	// Get CI-specific pager configuration
	pager, isInteractive := m.ciScriptPagerCommand()

	var cmdStr string
	if isInteractive {
		// Interactive pager - direct terminal control
		cmdStr = fmt.Sprintf("gh run view %s %s 2>&1 | %s", runID, logFlag, pager)
	} else {
		// Non-interactive pager - use pager environment settings
		pagerEnv := m.pagerEnv(pager)
		pagerCmd := pager
		if pagerEnv != "" {
			pagerCmd = fmt.Sprintf("%s %s", pagerEnv, pager)
		}
		cmdStr = fmt.Sprintf("set -o pipefail; gh run view %s %s 2>&1 | %s", runID, logFlag, pagerCmd)
	}

	// Create command
	// #nosec G204 -- command is constructed from controlled inputs
	c := m.commandRunner(m.ctx, "bash", "-c", cmdStr)
	c.Dir = wt.Path
	c.Env = envVars

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			// Ignore exit status 141 (SIGPIPE) which happens when the pager is closed early
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 141 {
				return refreshCompleteMsg{}
			}
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

// extractRunIDFromLink extracts the run ID from a GitHub Actions URL.
// Example URL: https://github.com/owner/repo/actions/runs/12345678/job/98765432
func extractRunIDFromLink(link string) string {
	return ciDataSvc.ExtractRunID(link)
}

// extractJobIDFromLink extracts the job ID from a GitHub Actions URL.
// Example URL: https://github.com/owner/repo/actions/runs/12345678/job/98765432 -> 98765432
func extractJobIDFromLink(link string) string {
	return ciDataSvc.ExtractJobID(link)
}

// extractRepoFromLink extracts the owner/repo from a GitHub Actions URL.
// Example URL: https://github.com/owner/repo/actions/runs/12345678 -> owner/repo
func extractRepoFromLink(link string) string {
	return ciDataSvc.ExtractRepo(link)
}

// rerunCICheck restarts a CI job and returns the run URL.
func (m *Model) rerunCICheck(check *models.CICheck) tea.Cmd {
	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		return nil
	}
	wt := m.state.data.filteredWts[m.state.data.selectedIndex]

	// Extract run ID and job ID from the check link
	runID := extractRunIDFromLink(check.Link)
	if runID == "" {
		m.showInfo("Cannot restart: not a GitHub Actions job.", nil)
		return nil
	}

	jobID := extractJobIDFromLink(check.Link)
	repo := extractRepoFromLink(check.Link)
	if repo == "" {
		m.showInfo("Cannot restart: unable to determine repository from link.", nil)
		return nil
	}

	// Show loading screen
	m.loading = true
	m.loadingOperation = "rerun"
	m.setLoadingScreen("Restarting CI job...")

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
		defer cancel()

		// Build the gh run rerun command
		args := []string{"run", "rerun", runID}
		if jobID != "" {
			args = append(args, "--job", jobID)
		}
		args = append(args, "-R", repo)

		// gh run rerun produces no stdout on success, so we can't check output.
		// RunGit with silent=false sends a notification on failure.
		m.state.services.git.RunGit(ctx, append([]string{"gh"}, args...), wt.Path, []int{0}, true, false)

		// Construct the run URL
		runURL := fmt.Sprintf("https://github.com/%s/actions/runs/%s", repo, runID)

		return ciRerunResultMsg{runURL: runURL}
	}
}

// getCIChecksForCurrentWorktree returns CI checks for the current worktree and whether they're visible.
func (m *Model) getCIChecksForCurrentWorktree() ([]*models.CICheck, bool) {
	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		return nil, false
	}
	wt := m.state.data.filteredWts[m.state.data.selectedIndex]
	checks, _, ok := m.cache.ciCache.Get(wt.Branch)
	if !ok || len(checks) == 0 {
		return nil, false
	}
	return sortCIChecks(checks), true
}

// sortCIChecks sorts CI checks so that GitHub Actions jobs appear first,
// followed by non-GitHub Actions checks (e.g., Tekton, external status checks).
func sortCIChecks(checks []*models.CICheck) []*models.CICheck {
	return ciDataSvc.Sort(checks)
}

func (m *Model) fetchCIStatus(prNumber int, branch string) tea.Cmd {
	return func() tea.Msg {
		checks, err := m.state.services.git.FetchCIStatus(m.ctx, prNumber, branch)
		return ciStatusLoadedMsg{
			branch: branch,
			checks: checks,
			err:    err,
		}
	}
}

// fetchCIStatusByCommit fetches CI status for a commit SHA (non-PR branches on GitHub).
func (m *Model) fetchCIStatusByCommit(worktreePath, branch string) tea.Cmd {
	return func() tea.Msg {
		commitSHA := m.state.services.git.GetHeadSHA(m.ctx, worktreePath)
		if commitSHA == "" {
			return ciStatusLoadedMsg{branch: branch, checks: nil, err: nil}
		}
		checks, err := m.state.services.git.FetchCIStatusByCommit(m.ctx, commitSHA, worktreePath)
		return ciStatusLoadedMsg{branch: branch, checks: checks, err: err}
	}
}

// maybeFetchCIStatus triggers CI fetch for current worktree if it has a PR or commit and cache is stale.
func (m *Model) maybeFetchCIStatus() tea.Cmd {
	if m.config.DisablePR {
		return nil
	}
	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		return nil
	}
	wt := m.state.data.filteredWts[m.state.data.selectedIndex]

	// Check cache - skip if fresh (within ciCacheTTL)
	if m.cache.ciCache.IsFresh(wt.Branch, ciCacheTTL) {
		return nil
	}

	// If we have an OPEN PR, use PR-based CI fetch
	// For merged/closed PRs, gh pr checks returns empty, so use commit-based fetch
	if wt.PR != nil && wt.PR.State == prStateOpen {
		return m.fetchCIStatus(wt.PR.Number, wt.Branch)
	}

	// For non-PR branches on GitHub, use commit-based CI fetch
	if m.state.services.git.IsGitHub(m.ctx) {
		return m.fetchCIStatusByCommit(wt.Path, wt.Branch)
	}

	return nil
}

// shouldRefreshCI returns true if we should periodically refresh CI status.
// For open PRs, always refresh (new jobs can start anytime via push/re-run).
// For closed PRs or non-PR branches, only refresh if jobs are still pending.
func (m *Model) shouldRefreshCI() bool {
	if m.config.DisablePR {
		return false
	}
	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		return false
	}
	wt := m.state.data.filteredWts[m.state.data.selectedIndex]

	// Always refresh for open PRs (new jobs can start anytime)
	if wt.PR != nil && wt.PR.State == prStateOpen {
		return true
	}

	// For closed PRs or non-PR branches, only refresh if jobs are pending
	checks, _, ok := m.cache.ciCache.Get(wt.Branch)
	if !ok {
		return true // No cache yet, should fetch
	}
	for _, check := range checks {
		if check.Conclusion == "pending" || check.Conclusion == "" {
			return true
		}
	}
	return false
}

// ciScriptPagerCommand returns the pager command for CI logs.
// Returns (pager, isInteractive) - ci_script_pager is implicitly interactive.
func (m *Model) ciScriptPagerCommand() (string, bool) {
	if m.config != nil {
		if ciPager := strings.TrimSpace(m.config.CIScriptPager); ciPager != "" {
			return ciPager, true
		}
	}
	return m.pagerCommand(), false
}

// getCIStatusIcon returns the appropriate icon for CI status.
// Draft PRs show "D" instead of CI status per user preference.
func getCIStatusIcon(ciStatus string, isDraft, showIcons bool) string {
	if isDraft {
		return "D"
	}
	if showIcons {
		if icon := ciIconForConclusion(ciStatus); icon != "" {
			return icon
		}
	}
	switch ciStatus {
	case "success":
		return "S"
	case "failure":
		return "F"
	case "skipped":
		return "-"
	case "cancelled":
		return "C"
	case "pending":
		return "P"
	default:
		return "?"
	}
}
