// Package git wraps git commands and helpers used by lazyworktree.
package git

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/chmouel/lazyworktree/internal/commands"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

const (
	gitHostGitLab  = "gitlab"
	gitHostGithub  = "github"
	gitHostUnknown = "unknown"

	// CI conclusion constants
	ciSuccess   = "success"
	ciFailure   = "failure"
	ciPending   = "pending"
	ciSkipped   = "skipped"
	ciCancelled = "cancelled"

	// PR state constants
	prStateOpen = "OPEN"
)

// NotifyFn receives ongoing notifications.
type NotifyFn func(message string, severity string)

// NotifyOnceFn reports deduplicated notification messages.
type NotifyOnceFn func(key string, message string, severity string)

// Service orchestrates git and helper commands for the UI.
type Service struct {
	notify      NotifyFn
	notifyOnce  NotifyOnceFn
	semaphore   chan struct{}
	mainBranch  string
	gitHost     string
	notifiedSet map[string]bool
	useDelta    bool
	deltaArgs   []string
	debugLogger *log.Logger
}

// NewService constructs a Service and sets up concurrency limits.
func NewService(notify NotifyFn, notifyOnce NotifyOnceFn) *Service {
	limit := runtime.NumCPU() * 2
	if limit < 4 {
		limit = 4
	}
	if limit > 32 {
		limit = 32
	}

	semaphore := make(chan struct{}, limit) // Limit concurrent operations
	for i := 0; i < limit; i++ {
		semaphore <- struct{}{}
	}

	s := &Service{
		notify:      notify,
		notifyOnce:  notifyOnce,
		semaphore:   semaphore,
		notifiedSet: make(map[string]bool),
	}

	// Detect delta availability
	s.detectDelta()

	return s
}

// SetDebugLogger attaches a logger for debug output.
func (s *Service) SetDebugLogger(logger *log.Logger) {
	s.debugLogger = logger
}

// SetDeltaArgs sets additional delta arguments used when formatting diffs.
func (s *Service) SetDeltaArgs(args []string) {
	if len(args) == 0 {
		s.deltaArgs = nil
		return
	}
	s.deltaArgs = append([]string{}, args...)
}

func (s *Service) debugf(format string, args ...interface{}) {
	if s.debugLogger == nil {
		return
	}
	s.debugLogger.Printf(format, args...)
}

func prepareAllowedCommand(ctx context.Context, args []string) (*exec.Cmd, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no command provided")
	}

	switch args[0] {
	case "git":
		// #nosec G204 -- arguments for git command come from internal logic and are not shell interpolated
		return exec.CommandContext(ctx, "git", args[1:]...), nil
	case "glab":
		// #nosec G204 -- arguments for glab command are controlled by the application workflow
		return exec.CommandContext(ctx, "glab", args[1:]...), nil
	case "gh":
		// #nosec G204 -- arguments for gh command are supplied by vetted code paths
		return exec.CommandContext(ctx, "gh", args[1:]...), nil
	default:
		return nil, fmt.Errorf("unsupported command %q", args[0])
	}
}

func (s *Service) detectDelta() {
	cmd := exec.Command("delta", "--version")
	if err := cmd.Run(); err == nil {
		s.useDelta = true
	}
}

// ApplyDelta pipes diff output through delta if available
// ApplyDelta pipes diff output through delta when available.
func (s *Service) ApplyDelta(ctx context.Context, diff string) string {
	if !s.useDelta || diff == "" {
		return diff
	}

	args := []string{"--no-gitconfig", "--paging=never"}
	if len(s.deltaArgs) > 0 {
		args = append(args, s.deltaArgs...)
	}
	cmd := exec.CommandContext(ctx, "delta", args...)
	cmd.Stdin = strings.NewReader(diff)
	output, err := cmd.Output()
	if err != nil {
		return diff
	}

	return string(output)
}

// UseDelta reports whether delta integration is enabled.
func (s *Service) UseDelta() bool {
	return s.useDelta
}

// ExecuteCommands runs provided shell commands sequentially inside the given working directory.
func (s *Service) ExecuteCommands(ctx context.Context, cmdList []string, cwd string, env map[string]string) error {
	for _, cmdStr := range cmdList {
		if strings.TrimSpace(cmdStr) == "" {
			continue
		}

		s.debugf("exec: %s (cwd=%s)", cmdStr, cwd)
		if cmdStr == "link_topsymlinks" {
			mainPath := env["MAIN_WORKTREE_PATH"]
			wtPath := env["WORKTREE_PATH"]
			statusFunc := func(ctx context.Context, path string) string {
				return s.RunGit(ctx, []string{"git", "status", "--porcelain", "--ignored"}, path, []int{0}, true, false)
			}
			if err := commands.LinkTopSymlinks(ctx, mainPath, wtPath, statusFunc); err != nil {
				return err
			}
			continue
		}
		// #nosec G204 -- commands are defined in the local config and executed through bash intentionally
		command := exec.CommandContext(ctx, "bash", "-lc", cmdStr)
		if cwd != "" {
			command.Dir = cwd
		}
		command.Env = append(os.Environ(), formatEnv(env)...)
		out, err := command.CombinedOutput()
		if err != nil {
			detail := strings.TrimSpace(string(out))
			if detail != "" {
				return fmt.Errorf("%s: %s", cmdStr, detail)
			}
			return fmt.Errorf("%s: %w", cmdStr, err)
		}
	}
	return nil
}

func formatEnv(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	formatted := make([]string, 0, len(env))
	for k, v := range env {
		formatted = append(formatted, fmt.Sprintf("%s=%s", k, v))
	}
	return formatted
}

func (s *Service) acquireSemaphore() {
	<-s.semaphore
}

func (s *Service) releaseSemaphore() {
	s.semaphore <- struct{}{}
}

// RunGit executes a git command and optionally trims its output.
func (s *Service) RunGit(ctx context.Context, args []string, cwd string, okReturncodes []int, strip bool, silent bool) string {
	command := strings.Join(args, " ")
	if command == "" {
		command = "<empty>"
	}
	s.debugf("run: %s (cwd=%s)", command, cwd)

	cmd, err := prepareAllowedCommand(ctx, args)
	if err != nil {
		key := fmt.Sprintf("unsupported_cmd:%s", command)
		s.notifyOnce(key, fmt.Sprintf("Unsupported command: %s", command), "error")
		s.debugf("error: %s (unsupported command)", command)
		return ""
	}
	if cwd != "" {
		cmd.Dir = cwd
	}

	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			returnCode := exitError.ExitCode()
			allowed := false
			for _, code := range okReturncodes {
				if returnCode == code {
					allowed = true
					break
				}
			}
			if !allowed {
				if silent {
					s.debugf("error: %s (exit %d, silenced)", command, returnCode)
					return ""
				}
				stderr := string(exitError.Stderr)
				suffix := ""
				if stderr != "" {
					suffix = ": " + strings.TrimSpace(stderr)
				} else {
					suffix = fmt.Sprintf(" (exit %d)", returnCode)
				}
				key := fmt.Sprintf("git_fail:%s:%s", cwd, command)
				s.notifyOnce(key, fmt.Sprintf("Command failed: %s%s", command, suffix), "error")
				s.debugf("error: %s%s", command, suffix)
				return ""
			}
		} else {
			if !silent {
				command := args[0]
				if len(args) > 0 {
					command = args[0]
				}
				key := fmt.Sprintf("cmd_missing:%s", command)
				s.notifyOnce(key, fmt.Sprintf("Command not found: %s", command), "error")
				s.debugf("error: command not found: %s", command)
			}
			return ""
		}
	}

	out := string(output)
	if strip {
		out = strings.TrimSpace(out)
	}
	s.debugf("ok: %s", command)
	return out
}

// RunCommandChecked runs the provided git command and reports failures via notify callbacks.
func (s *Service) RunCommandChecked(ctx context.Context, args []string, cwd string, errorPrefix string) bool {
	command := strings.Join(args, " ")
	if command == "" {
		command = "<empty>"
	}
	s.debugf("run: %s (cwd=%s)", command, cwd)

	cmd, err := prepareAllowedCommand(ctx, args)
	if err != nil {
		message := fmt.Sprintf("%s: %v", errorPrefix, err)
		if errorPrefix == "" {
			message = fmt.Sprintf("command error: %v", err)
		}
		s.notify(message, "error")
		s.debugf("error: %s", message)
		return false
	}
	if cwd != "" {
		cmd.Dir = cwd
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail != "" {
			s.notify(fmt.Sprintf("%s: %s", errorPrefix, detail), "error")
			s.debugf("error: %s: %s", errorPrefix, detail)
		} else {
			s.notify(fmt.Sprintf("%s: %v", errorPrefix, err), "error")
			s.debugf("error: %s: %v", errorPrefix, err)
		}
		return false
	}

	s.debugf("ok: %s", command)
	return true
}

// GetMainBranch returns the main branch name for the current repository.
func (s *Service) GetMainBranch(ctx context.Context) string {
	if s.mainBranch != "" {
		return s.mainBranch
	}

	out := s.RunGit(ctx, []string{"git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD"}, "", []int{0}, true, false)
	if out != "" {
		parts := strings.Split(out, "/")
		if len(parts) > 0 {
			s.mainBranch = parts[len(parts)-1]
		}
	}
	if s.mainBranch == "" {
		s.mainBranch = "main"
	}
	return s.mainBranch
}

// GetWorktrees parses git worktree metadata and returns the list of worktrees.
func (s *Service) GetWorktrees(ctx context.Context) ([]*models.WorktreeInfo, error) {
	rawWts := s.RunGit(ctx, []string{"git", "worktree", "list", "--porcelain"}, "", []int{0}, true, false)
	if rawWts == "" {
		return []*models.WorktreeInfo{}, nil
	}

	type wtData struct {
		path   string
		branch string
		isMain bool
	}

	var wts []wtData
	var currentWt *wtData

	lines := strings.Split(rawWts, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			if currentWt != nil {
				wts = append(wts, *currentWt)
			}
			path := strings.TrimPrefix(line, "worktree ")
			currentWt = &wtData{path: path}
		} else if strings.HasPrefix(line, "branch ") {
			if currentWt != nil {
				branch := strings.TrimPrefix(line, "branch ")
				branch = strings.TrimPrefix(branch, "refs/heads/")
				currentWt.branch = branch
			}
		}
	}
	if currentWt != nil {
		wts = append(wts, *currentWt)
	}

	// Mark first as main
	for i := range wts {
		wts[i].isMain = (i == 0)
	}

	branchRaw := s.RunGit(ctx, []string{
		"git", "for-each-ref",
		"--format=%(refname:short)|%(committerdate:relative)|%(committerdate:unix)",
		"refs/heads",
	}, "", []int{0}, true, false)

	branchInfo := make(map[string]struct {
		lastActive   string
		lastActiveTS int64
	})

	for _, line := range strings.Split(branchRaw, "\n") {
		if strings.Contains(line, "|") {
			parts := strings.Split(line, "|")
			if len(parts) == 3 {
				branch := parts[0]
				lastActive := parts[1]
				lastActiveTS, _ := strconv.ParseInt(parts[2], 10, 64)
				branchInfo[branch] = struct {
					lastActive   string
					lastActiveTS int64
				}{lastActive: lastActive, lastActiveTS: lastActiveTS}
			}
		}
	}

	// Get worktree info concurrently
	type result struct {
		wt  *models.WorktreeInfo
		err error
	}

	results := make(chan result, len(wts))
	var wg sync.WaitGroup

	for _, wt := range wts {
		wg.Add(1)
		go func(wtData wtData) {
			defer wg.Done()
			s.acquireSemaphore()
			defer s.releaseSemaphore()

			path := wtData.path
			branch := wtData.branch
			if branch == "" {
				branch = "(detached)"
			}

			statusRaw := s.RunGit(ctx, []string{"git", "status", "--porcelain=v2", "--branch"}, path, []int{0}, true, false)

			ahead := 0
			behind := 0
			hasUpstream := false
			untracked := 0
			modified := 0
			staged := 0

			for _, line := range strings.Split(statusRaw, "\n") {
				switch {
				case strings.HasPrefix(line, "# branch.upstream "):
					hasUpstream = true
				case strings.HasPrefix(line, "# branch.ab "):
					// branch.ab only appears when upstream is set per Git porcelain v2 spec
					hasUpstream = true
					parts := strings.Fields(line)
					if len(parts) >= 4 {
						aheadStr := strings.TrimPrefix(parts[2], "+")
						behindStr := strings.TrimPrefix(parts[3], "-")
						ahead, _ = strconv.Atoi(aheadStr)
						behind, _ = strconv.Atoi(behindStr)
					}
				case strings.HasPrefix(line, "?"):
					untracked++
				case strings.HasPrefix(line, "1 "), strings.HasPrefix(line, "2 "):
					parts := strings.Fields(line)
					if len(parts) > 1 {
						xy := parts[1]
						if len(xy) >= 2 {
							if xy[0] != '.' {
								staged++
							}
							if xy[1] != '.' {
								modified++
							}
						}
					}
				}
			}

			info, exists := branchInfo[branch]
			lastActive := ""
			lastActiveTS := int64(0)
			if exists {
				lastActive = info.lastActive
				lastActiveTS = info.lastActiveTS
			}

			wt := &models.WorktreeInfo{
				Path:         path,
				Branch:       branch,
				IsMain:       wtData.isMain,
				Dirty:        (untracked + modified + staged) > 0,
				Ahead:        ahead,
				Behind:       behind,
				HasUpstream:  hasUpstream,
				LastActive:   lastActive,
				LastActiveTS: lastActiveTS,
				Untracked:    untracked,
				Modified:     modified,
				Staged:       staged,
			}

			results <- result{wt: wt, err: nil}
		}(wt)
	}

	wg.Wait()
	close(results)

	worktrees := make([]*models.WorktreeInfo, 0, len(wts))
	for r := range results {
		if r.err == nil {
			worktrees = append(worktrees, r.wt)
		}
	}

	return worktrees, nil
}

// detectHost detects the git host (github, gitlab, or unknown)
func (s *Service) detectHost(ctx context.Context) string {
	if s.gitHost != "" {
		return s.gitHost
	}

	remoteURL := s.RunGit(ctx, []string{"git", "remote", "get-url", "origin"}, "", []int{0}, true, true)
	if remoteURL != "" {
		re := regexp.MustCompile(`(?:git@|https?://|ssh://|git://)(?:[^@]+@)?([^/:]+)`)
		matches := re.FindStringSubmatch(remoteURL)
		if len(matches) > 1 {
			hostname := strings.ToLower(matches[1])
			if strings.Contains(hostname, gitHostGitLab) {
				s.gitHost = gitHostGitLab
				return gitHostGitLab
			}
			if strings.Contains(hostname, gitHostGithub) {
				s.gitHost = gitHostGithub
				return gitHostGithub
			}
		}
	}

	s.gitHost = gitHostUnknown
	return gitHostUnknown
}

func (s *Service) fetchGitLabPRs(ctx context.Context) (map[string]*models.PRInfo, error) {
	prRaw := s.RunGit(ctx, []string{"glab", "api", "merge_requests?state=all&per_page=100"}, "", []int{0}, false, false)
	if prRaw == "" {
		return nil, nil
	}

	var prs []map[string]interface{}
	if err := json.Unmarshal([]byte(prRaw), &prs); err != nil {
		key := "pr_json_decode_glab"
		s.notifyOnce(key, fmt.Sprintf("Failed to parse GLAB PR data: %v", err), "error")
		return nil, err
	}

	prMap := make(map[string]*models.PRInfo)
	for _, p := range prs {
		state, _ := p["state"].(string)
		state = strings.ToUpper(state)
		if state == "OPENED" {
			state = prStateOpen
		}

		iid, _ := p["iid"].(float64)
		title, _ := p["title"].(string)
		webURL, _ := p["web_url"].(string)
		sourceBranch, _ := p["source_branch"].(string)

		if sourceBranch != "" {
			prMap[sourceBranch] = &models.PRInfo{
				Number: int(iid),
				State:  state,
				Title:  title,
				URL:    webURL,
				Branch: sourceBranch,
			}
		}
	}

	return prMap, nil
}

// FetchPRMap fetches PR/MR information from GitHub or GitLab
// FetchPRMap gathers PR information via supported host APIs.
func (s *Service) FetchPRMap(ctx context.Context) (map[string]*models.PRInfo, error) {
	host := s.detectHost(ctx)
	if host == gitHostGitLab {
		return s.fetchGitLabPRs(ctx)
	}

	// Default to GitHub
	prRaw := s.RunGit(ctx, []string{
		"gh", "pr", "list",
		"--state", "all",
		"--json", "headRefName,state,number,title,url",
		"--limit", "100",
	}, "", []int{0}, false, host == gitHostUnknown)

	if prRaw == "" {
		return nil, nil
	}

	var prs []map[string]interface{}
	if err := json.Unmarshal([]byte(prRaw), &prs); err != nil {
		key := "pr_json_decode"
		s.notifyOnce(key, fmt.Sprintf("Failed to parse PR data: %v", err), "error")
		return nil, err
	}

	prMap := make(map[string]*models.PRInfo)
	for _, p := range prs {
		headRefName, _ := p["headRefName"].(string)
		state, _ := p["state"].(string)
		number, _ := p["number"].(float64)
		title, _ := p["title"].(string)
		url, _ := p["url"].(string)

		if headRefName != "" {
			prMap[headRefName] = &models.PRInfo{
				Number: int(number),
				State:  state,
				Title:  title,
				URL:    url,
				Branch: headRefName,
			}
		}
	}

	return prMap, nil
}

// FetchAllOpenPRs fetches all open PRs/MRs and returns them as a slice.
func (s *Service) FetchAllOpenPRs(ctx context.Context) ([]*models.PRInfo, error) {
	host := s.detectHost(ctx)
	if host == gitHostGitLab {
		return s.fetchGitLabOpenPRs(ctx)
	}

	// Default to GitHub
	prRaw := s.RunGit(ctx, []string{
		"gh", "pr", "list",
		"--state", "open",
		"--json", "headRefName,state,number,title,url",
		"--limit", "100",
	}, "", []int{0}, false, host == gitHostUnknown)

	if prRaw == "" {
		return []*models.PRInfo{}, nil
	}

	var prs []map[string]interface{}
	if err := json.Unmarshal([]byte(prRaw), &prs); err != nil {
		key := "pr_json_decode"
		s.notifyOnce(key, fmt.Sprintf("Failed to parse PR data: %v", err), "error")
		return nil, err
	}

	result := make([]*models.PRInfo, 0, len(prs))
	for _, p := range prs {
		state, _ := p["state"].(string)
		if !strings.EqualFold(state, prStateOpen) {
			continue
		}
		number, _ := p["number"].(float64)
		title, _ := p["title"].(string)
		url, _ := p["url"].(string)
		headRefName, _ := p["headRefName"].(string)

		result = append(result, &models.PRInfo{
			Number: int(number),
			State:  prStateOpen,
			Title:  title,
			URL:    url,
			Branch: headRefName,
		})
	}

	return result, nil
}

func (s *Service) fetchGitLabOpenPRs(ctx context.Context) ([]*models.PRInfo, error) {
	prRaw := s.RunGit(ctx, []string{"glab", "api", "merge_requests?state=opened&per_page=100"}, "", []int{0}, false, false)
	if prRaw == "" {
		return []*models.PRInfo{}, nil
	}

	var prs []map[string]interface{}
	if err := json.Unmarshal([]byte(prRaw), &prs); err != nil {
		key := "pr_json_decode_glab"
		s.notifyOnce(key, fmt.Sprintf("Failed to parse GLAB PR data: %v", err), "error")
		return nil, err
	}

	result := make([]*models.PRInfo, 0, len(prs))
	for _, p := range prs {
		state, _ := p["state"].(string)
		state = strings.ToUpper(state)
		if state == "OPENED" {
			state = prStateOpen
		}
		if state != prStateOpen {
			continue
		}

		iid, _ := p["iid"].(float64)
		title, _ := p["title"].(string)
		webURL, _ := p["web_url"].(string)
		sourceBranch, _ := p["source_branch"].(string)

		result = append(result, &models.PRInfo{
			Number: int(iid),
			State:  state,
			Title:  title,
			URL:    webURL,
			Branch: sourceBranch,
		})
	}

	return result, nil
}

// FetchCIStatus fetches CI check statuses for a PR from GitHub or GitLab.
func (s *Service) FetchCIStatus(ctx context.Context, prNumber int, branch string) ([]*models.CICheck, error) {
	host := s.detectHost(ctx)
	switch host {
	case gitHostGithub:
		return s.fetchGitHubCI(ctx, prNumber)
	case gitHostGitLab:
		return s.fetchGitLabCI(ctx, branch)
	default:
		return nil, nil
	}
}

func (s *Service) fetchGitHubCI(ctx context.Context, prNumber int) ([]*models.CICheck, error) {
	// Use gh pr checks to get CI status
	out := s.RunGit(ctx, []string{
		"gh", "pr", "checks", fmt.Sprintf("%d", prNumber),
		"--json", "name,state,bucket",
	}, "", []int{0, 1, 8}, true, true) // exit code 8 = checks pending

	if out == "" {
		return nil, nil
	}

	var checks []struct {
		Name   string `json:"name"`
		State  string `json:"state"`
		Bucket string `json:"bucket"` // pass, fail, pending, skipping, cancel
	}

	if err := json.Unmarshal([]byte(out), &checks); err != nil {
		return nil, err
	}

	result := make([]*models.CICheck, 0, len(checks))
	for _, c := range checks {
		// Map bucket to our conclusion format
		conclusion := s.githubBucketToConclusion(c.Bucket)
		result = append(result, &models.CICheck{
			Name:       c.Name,
			Status:     strings.ToLower(c.State),
			Conclusion: conclusion,
		})
	}
	return result, nil
}

func (s *Service) githubBucketToConclusion(bucket string) string {
	switch strings.ToLower(bucket) {
	case "pass":
		return ciSuccess
	case "fail":
		return ciFailure
	case "skipping":
		return ciSkipped
	case "cancel":
		return ciCancelled
	case "pending":
		return ciPending
	default:
		return bucket
	}
}

func (s *Service) fetchGitLabCI(ctx context.Context, branch string) ([]*models.CICheck, error) {
	// Use glab ci status to get pipeline jobs
	out := s.RunGit(ctx, []string{
		"glab", "ci", "status", "--branch", branch, "--output", "json",
	}, "", []int{0, 1}, true, true)

	if out == "" {
		return nil, nil
	}

	var pipeline struct {
		Jobs []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"jobs"`
	}

	if err := json.Unmarshal([]byte(out), &pipeline); err != nil {
		// Try parsing as array of jobs directly
		var jobs []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		}
		if err2 := json.Unmarshal([]byte(out), &jobs); err2 != nil {
			return nil, err
		}
		result := make([]*models.CICheck, 0, len(jobs))
		for _, j := range jobs {
			result = append(result, &models.CICheck{
				Name:       j.Name,
				Status:     strings.ToLower(j.Status),
				Conclusion: s.gitlabStatusToConclusion(j.Status),
			})
		}
		return result, nil
	}

	result := make([]*models.CICheck, 0, len(pipeline.Jobs))
	for _, j := range pipeline.Jobs {
		result = append(result, &models.CICheck{
			Name:       j.Name,
			Status:     strings.ToLower(j.Status),
			Conclusion: s.gitlabStatusToConclusion(j.Status),
		})
	}
	return result, nil
}

func (s *Service) gitlabStatusToConclusion(status string) string {
	switch strings.ToLower(status) {
	case "success", "passed":
		return ciSuccess
	case "failed":
		return ciFailure
	case "canceled", "cancelled":
		return ciCancelled
	case "skipped":
		return ciSkipped
	case "running", "pending", "created", "waiting_for_resource", "preparing":
		return ciPending
	default:
		return status
	}
}

// GetMainWorktreePath returns the path of the main worktree.
func (s *Service) GetMainWorktreePath(ctx context.Context) string {
	rawWts := s.RunGit(ctx, []string{"git", "worktree", "list", "--porcelain"}, "", []int{0}, true, false)
	for _, line := range strings.Split(rawWts, "\n") {
		if strings.HasPrefix(line, "worktree ") {
			return strings.TrimPrefix(line, "worktree ")
		}
	}
	cwd, _ := os.Getwd()
	return cwd
}

// RenameWorktree attempts to move and rename branches for a worktree migration.
func (s *Service) RenameWorktree(ctx context.Context, oldPath, newPath, oldBranch, newBranch string) bool {
	// 1. Move the worktree directory
	if !s.RunCommandChecked(ctx, []string{"git", "worktree", "move", oldPath, newPath}, "", fmt.Sprintf("Failed to move worktree from %s to %s", oldPath, newPath)) {
		return false
	}

	// 2. Rename the branch
	if !s.RunCommandChecked(ctx, []string{"git", "branch", "-m", oldBranch, newBranch}, newPath, fmt.Sprintf("Failed to rename branch from %s to %s", oldBranch, newBranch)) {
		return false
	}

	return true
}

// CreateWorktreeFromPR creates a worktree from a PR's remote branch.
// It uses gh/glab CLI tools to checkout the PR with proper tracking, then creates a worktree.
func (s *Service) CreateWorktreeFromPR(ctx context.Context, prNumber int, remoteBranch, localBranch, targetPath string) bool {
	host := s.detectHost(ctx)

	// Save current branch to restore it later
	currentBranch := s.RunGit(ctx, []string{"git", "branch", "--show-current"}, "", []int{0}, true, true)
	if currentBranch == "" {
		// If we're in detached HEAD, get the commit
		currentBranch = s.RunGit(ctx, []string{"git", "rev-parse", "HEAD"}, "", []int{0}, true, true)
	}

	// Use gh/glab to checkout the PR with proper configuration
	switch host {
	case gitHostGithub:
		// Use gh pr checkout which sets up all the proper tracking
		if !s.RunCommandChecked(ctx, []string{"gh", "pr", "checkout", fmt.Sprintf("%d", prNumber), "-b", localBranch}, "", fmt.Sprintf("Failed to checkout PR #%d", prNumber)) {
			return false
		}
	case gitHostGitLab:
		// Use glab mr checkout which sets up all the proper tracking
		if !s.RunCommandChecked(ctx, []string{"glab", "mr", "checkout", fmt.Sprintf("%d", prNumber), "-b", localBranch}, "", fmt.Sprintf("Failed to checkout MR #%d", prNumber)) {
			return false
		}
	default:
		// Unknown host, fall back to manual fetch
		if !s.RunCommandChecked(ctx, []string{"git", "fetch", "origin", remoteBranch}, "", fmt.Sprintf("Failed to fetch remote branch %s", remoteBranch)) {
			return false
		}
		remoteRef := fmt.Sprintf("origin/%s", remoteBranch)
		if !s.RunCommandChecked(ctx, []string{"git", "worktree", "add", "-b", localBranch, targetPath, remoteRef}, "", fmt.Sprintf("Failed to create worktree from PR branch %s", remoteBranch)) {
			return false
		}
		return true
	}

	// Restore the original branch in the main worktree
	if currentBranch != "" {
		s.RunGit(ctx, []string{"git", "checkout", currentBranch}, "", []int{0, 1}, true, true)
	}

	// Now create a new worktree from the branch that gh/glab just created
	if !s.RunCommandChecked(ctx, []string{"git", "worktree", "add", targetPath, localBranch}, "", fmt.Sprintf("Failed to create worktree from branch %s", localBranch)) {
		return false
	}

	return true
}

// ResolveRepoName resolves the repository name using various methods
// ResolveRepoName returns the repository identifier for caching purposes.
func (s *Service) ResolveRepoName(ctx context.Context) string {
	// Try gh repo view
	out := s.RunGit(ctx, []string{"gh", "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner"}, "", []int{0}, true, true)
	if out != "" {
		return out
	}

	// Try glab repo view
	out = s.RunGit(ctx, []string{"glab", "repo", "view", "-F", "json"}, "", []int{0}, false, true)
	if out != "" {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(out), &data); err == nil {
			if path, ok := data["path_with_namespace"].(string); ok {
				return path
			}
		}
	}

	// Try git remote get-url origin
	out = s.RunGit(ctx, []string{"git", "remote", "get-url", "origin"}, "", []int{0}, true, true)
	if out != "" {
		re := regexp.MustCompile(`[:/]([^/]+/[^/]+)(?:\.git)?$`)
		matches := re.FindStringSubmatch(out)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	// Try git rev-parse --show-toplevel
	out = s.RunGit(ctx, []string{"git", "rev-parse", "--show-toplevel"}, "", []int{0}, true, true)
	if out != "" {
		return filepath.Base(out)
	}

	return "unknown"
}

// BuildThreePartDiff builds a comprehensive diff with staged, unstaged, and untracked changes
// BuildThreePartDiff assembles a diff showing staged, modified, and untracked sections.
func (s *Service) BuildThreePartDiff(ctx context.Context, path string, cfg *config.AppConfig) string {
	var parts []string
	totalChars := 0

	// Part 1: Staged changes
	stagedDiff := s.RunGit(ctx, []string{"git", "diff", "--cached", "--patch", "--no-color"}, path, []int{0}, false, false)
	if stagedDiff != "" {
		header := "=== Staged Changes ===\n"
		parts = append(parts, header+stagedDiff)
		totalChars += len(header) + len(stagedDiff)
	}

	// Part 2: Unstaged changes
	if totalChars < cfg.MaxDiffChars {
		unstagedDiff := s.RunGit(ctx, []string{"git", "diff", "--patch", "--no-color"}, path, []int{0}, false, false)
		if unstagedDiff != "" {
			header := "=== Unstaged Changes ===\n"
			parts = append(parts, header+unstagedDiff)
			totalChars += len(header) + len(unstagedDiff)
		}
	}

	// Part 3: Untracked files (limited by config)
	if totalChars < cfg.MaxDiffChars && cfg.MaxUntrackedDiffs > 0 {
		untrackedFiles := s.getUntrackedFiles(ctx, path)
		untrackedCount := len(untrackedFiles)
		displayCount := untrackedCount
		if displayCount > cfg.MaxUntrackedDiffs {
			displayCount = cfg.MaxUntrackedDiffs
		}

		for i := 0; i < displayCount && totalChars < cfg.MaxDiffChars; i++ {
			file := untrackedFiles[i]
			diff := s.RunGit(ctx, []string{"git", "diff", "--no-index", "/dev/null", file}, path, []int{0, 1}, false, false)
			if diff != "" {
				header := fmt.Sprintf("=== Untracked: %s ===\n", file)
				parts = append(parts, header+diff)
				totalChars += len(header) + len(diff)
			}
		}

		if untrackedCount > displayCount {
			notice := fmt.Sprintf("\n[...showing %d of %d untracked files]", displayCount, untrackedCount)
			parts = append(parts, notice)
		}
	}

	result := strings.Join(parts, "\n\n")

	if len(result) > cfg.MaxDiffChars {
		result = result[:cfg.MaxDiffChars]
		result += fmt.Sprintf("\n\n[...truncated at %d chars]", cfg.MaxDiffChars)
	}

	return result
}

func (s *Service) getUntrackedFiles(ctx context.Context, path string) []string {
	statusRaw := s.RunGit(ctx, []string{"git", "status", "--porcelain"}, path, []int{0}, false, false)
	var untracked []string
	for _, line := range strings.Split(statusRaw, "\n") {
		if strings.HasPrefix(line, "?? ") {
			file := strings.TrimPrefix(line, "?? ")
			untracked = append(untracked, file)
		}
	}
	return untracked
}
