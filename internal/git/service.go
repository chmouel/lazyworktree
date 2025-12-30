package git

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

// NotifyFn is a function type for notifications
type NotifyFn func(message string, severity string)

// NotifyOnceFn is a function type for one-time notifications
type NotifyOnceFn func(key string, message string, severity string)

// Service handles Git operations
type Service struct {
	notify      NotifyFn
	notifyOnce  NotifyOnceFn
	semaphore   chan struct{}
	mainBranch  string
	gitHost     string
	notifiedSet map[string]bool
	useDelta    bool
}

// NewService creates a new GitService instance
func NewService(notify NotifyFn, notifyOnce NotifyOnceFn) *Service {
	semaphore := make(chan struct{}, 24) // Limit to 24 concurrent operations
	for i := 0; i < 24; i++ {
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

// detectDelta checks if delta is available
func (s *Service) detectDelta() {
	cmd := exec.Command("delta", "--version")
	if err := cmd.Run(); err == nil {
		s.useDelta = true
	}
}

// ApplyDelta pipes diff output through delta if available
func (s *Service) ApplyDelta(diff string) string {
	if !s.useDelta || diff == "" {
		return diff
	}

	cmd := exec.Command("delta", "--no-gitconfig", "--paging=never")
	cmd.Stdin = strings.NewReader(diff)
	output, err := cmd.Output()
	if err != nil {
		// Silently fall back to plain diff
		return diff
	}

	return string(output)
}

// UseDelta returns whether delta is available
func (s *Service) UseDelta() bool {
	return s.useDelta
}

// ExecuteCommands runs shell commands with optional environment and cwd
func (s *Service) ExecuteCommands(ctx context.Context, commands []string, cwd string, env map[string]string) error {
	for _, cmdStr := range commands {
		if strings.TrimSpace(cmdStr) == "" {
			continue
		}
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

// acquireSemaphore acquires a semaphore token
func (s *Service) acquireSemaphore() {
	<-s.semaphore
}

// releaseSemaphore releases a semaphore token
func (s *Service) releaseSemaphore() {
	s.semaphore <- struct{}{}
}

// RunGit executes a git command and returns the output
func (s *Service) RunGit(ctx context.Context, args []string, cwd string, okReturncodes []int, strip bool, silent bool) string {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
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
					return ""
				}
				stderr := string(exitError.Stderr)
				command := strings.Join(args, " ")
				suffix := ""
				if stderr != "" {
					suffix = ": " + strings.TrimSpace(stderr)
				} else {
					suffix = fmt.Sprintf(" (exit %d)", returnCode)
				}
				key := fmt.Sprintf("git_fail:%s:%s", cwd, command)
				s.notifyOnce(key, fmt.Sprintf("Command failed: %s%s", command, suffix), "error")
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
			}
			return ""
		}
	}

	out := string(output)
	if strip {
		out = strings.TrimSpace(out)
	}
	return out
}

// RunCommandChecked executes a command and returns true if successful
func (s *Service) RunCommandChecked(ctx context.Context, args []string, cwd string, errorPrefix string) bool {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	if cwd != "" {
		cmd.Dir = cwd
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail != "" {
			s.notify(fmt.Sprintf("%s: %s", errorPrefix, detail), "error")
		} else {
			s.notify(fmt.Sprintf("%s: %v", errorPrefix, err), "error")
		}
		return false
	}

	return true
}

// GetMainBranch returns the main branch name
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

// GetWorktrees returns a list of all worktrees
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

	// Get branch info
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
			untracked := 0
			modified := 0
			staged := 0

			for _, line := range strings.Split(statusRaw, "\n") {
				if strings.HasPrefix(line, "# branch.ab ") {
					parts := strings.Fields(line)
					if len(parts) >= 4 {
						aheadStr := strings.TrimPrefix(parts[2], "+")
						behindStr := strings.TrimPrefix(parts[3], "-")
						ahead, _ = strconv.Atoi(aheadStr)
						behind, _ = strconv.Atoi(behindStr)
					}
				} else if strings.HasPrefix(line, "?") {
					untracked++
				} else if strings.HasPrefix(line, "1 ") || strings.HasPrefix(line, "2 ") {
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
			if strings.Contains(hostname, "gitlab") {
				s.gitHost = "gitlab"
				return "gitlab"
			}
			if strings.Contains(hostname, "github") {
				s.gitHost = "github"
				return "github"
			}
		}
	}

	s.gitHost = "unknown"
	return "unknown"
}

// fetchGitLabPRs fetches PR information from GitLab
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
			state = "OPEN"
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
			}
		}
	}

	return prMap, nil
}

// FetchPRMap fetches PR/MR information from GitHub or GitLab
func (s *Service) FetchPRMap(ctx context.Context) (map[string]*models.PRInfo, error) {
	host := s.detectHost(ctx)
	if host == "gitlab" {
		return s.fetchGitLabPRs(ctx)
	}

	// Default to GitHub
	prRaw := s.RunGit(ctx, []string{
		"gh", "pr", "list",
		"--state", "all",
		"--json", "headRefName,state,number,title,url",
		"--limit", "100",
	}, "", []int{0}, false, host == "unknown")

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
			}
		}
	}

	return prMap, nil
}

// GetMainWorktreePath returns the path to the main worktree
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

// RenameWorktree renames a worktree and its branch
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

// ResolveRepoName resolves the repository name using various methods
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

		// Add truncation notice if we limited untracked files
		if untrackedCount > displayCount {
			notice := fmt.Sprintf("\n[...showing %d of %d untracked files]", displayCount, untrackedCount)
			parts = append(parts, notice)
		}
	}

	result := strings.Join(parts, "\n\n")

	// Truncate if exceeds max chars
	if len(result) > cfg.MaxDiffChars {
		result = result[:cfg.MaxDiffChars]
		result += fmt.Sprintf("\n\n[...truncated at %d chars]", cfg.MaxDiffChars)
	}

	return result
}

// getUntrackedFiles returns a list of untracked files in the worktree
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
