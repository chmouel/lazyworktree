// Package git wraps git commands and helpers used by lazyworktree.
package git

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chmouel/lazyworktree/internal/commands"
	"github.com/chmouel/lazyworktree/internal/config"
	log "github.com/chmouel/lazyworktree/internal/log"
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
	// Issue state constants
	issueStateOpened = "OPENED"
)

// LookupPath is used to find executables in PATH. It's exposed as a package variable
// so tests can mock it and avoid depending on system binaries being installed.
var LookupPath = exec.LookPath

// NotifyFn receives ongoing notifications.
type NotifyFn func(message string, severity string)

// NotifyOnceFn reports deduplicated notification messages.
type NotifyOnceFn func(key string, message string, severity string)

// Service orchestrates git and helper commands for the UI.
type Service struct {
	notify        NotifyFn
	notifyOnce    NotifyOnceFn
	semaphore     chan struct{}
	mainBranch    string
	gitHost       string
	notifiedSet   map[string]bool
	useGitPager   bool
	gitPagerArgs  []string
	gitPager      string
	commandRunner func(ctx context.Context, name string, args ...string) *exec.Cmd
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

	// Initialize counting semaphore: channel starts full with 'limit' tokens.
	// acquireSemaphore() takes a token (blocks if none available), releaseSemaphore() returns it.
	// This limits concurrent git operations to 'limit' goroutines.
	semaphore := make(chan struct{}, limit)
	for i := 0; i < limit; i++ {
		semaphore <- struct{}{}
	}

	s := &Service{
		notify:        notify,
		notifyOnce:    notifyOnce,
		semaphore:     semaphore,
		notifiedSet:   make(map[string]bool),
		commandRunner: exec.CommandContext,
	}

	// Detect diff pager availability
	s.detectGitPager()

	return s
}

// SetCommandRunner allows overriding the command runner (primarily for testing).
func (s *Service) SetCommandRunner(runner func(ctx context.Context, name string, args ...string) *exec.Cmd) {
	s.commandRunner = runner
}

func (s *Service) prepareAllowedCommand(ctx context.Context, args []string) (*exec.Cmd, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no command provided")
	}

	switch args[0] {
	case "git", "glab", "gh":
		return s.commandRunner(ctx, args[0], args[1:]...), nil
	default:
		return nil, fmt.Errorf("unsupported command %q", args[0])
	}
}

// SetGitPagerArgs sets additional arguments used when formatting diffs.
func (s *Service) SetGitPagerArgs(args []string) {
	if len(args) == 0 {
		s.gitPagerArgs = nil
		return
	}
	s.gitPagerArgs = append([]string{}, args...)
}

// SetGitPager sets the diff formatter/pager command and enables/disables based on empty string.
func (s *Service) SetGitPager(pager string) {
	s.gitPager = strings.TrimSpace(pager)
	s.detectGitPager()
}

func (s *Service) isGitPagerAvailable() bool {
	if s.gitPager == "" {
		return false
	}
	_, err := LookupPath(s.gitPager)
	return err == nil
}

func (s *Service) debugf(format string, args ...any) {
	log.Printf(format, args...)
}

func (s *Service) detectGitPager() {
	s.useGitPager = s.isGitPagerAvailable()
}

// ApplyGitPager pipes diff output through the configured git pager when available.
func (s *Service) ApplyGitPager(ctx context.Context, diff string) string {
	if !s.useGitPager || diff == "" {
		return diff
	}

	args := []string{}
	if s.gitPager == "delta" {
		args = append(args, "--no-gitconfig", "--paging=never")
	}
	if len(s.gitPagerArgs) > 0 {
		args = append(args, s.gitPagerArgs...)
	}
	// #nosec G204 -- git_pager comes from local config and is controlled by the user
	cmd := exec.CommandContext(ctx, s.gitPager, args...)
	cmd.Stdin = strings.NewReader(diff)
	output, err := cmd.Output()
	if err != nil {
		return diff
	}

	return string(output)
}

// UseGitPager reports whether diff pager integration is enabled.
func (s *Service) UseGitPager() bool {
	return s.useGitPager
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
func (s *Service) RunGit(ctx context.Context, args []string, cwd string, okReturncodes []int, strip, silent bool) string {
	command := strings.Join(args, " ")
	if command == "" {
		command = "<empty>"
	}
	s.debugf("run: %s (cwd=%s)", command, cwd)

	cmd, err := s.prepareAllowedCommand(ctx, args)
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
			allowed := slices.Contains(okReturncodes, returnCode)
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
				command := "<unknown>"
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
func (s *Service) RunCommandChecked(ctx context.Context, args []string, cwd, errorPrefix string) bool {
	command := strings.Join(args, " ")
	if command == "" {
		command = "<empty>"
	}
	s.debugf("run: %s (cwd=%s)", command, cwd)

	cmd, err := s.prepareAllowedCommand(ctx, args)
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

// RunGitWithCombinedOutput executes a git command with environment variables and returns its combined output and error.
func (s *Service) RunGitWithCombinedOutput(ctx context.Context, args []string, cwd string, env map[string]string) ([]byte, error) {
	command := strings.Join(args, " ")
	s.debugf("run: %s (cwd=%s)", command, cwd)

	cmd, err := s.prepareAllowedCommand(ctx, args)
	if err != nil {
		return nil, err
	}
	if cwd != "" {
		cmd.Dir = cwd
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), formatEnv(env)...)
	}

	return cmd.CombinedOutput()
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

// GetCurrentBranch returns the current branch name from the current working directory.
// Returns an error if not in a git repository or if HEAD is detached.
func (s *Service) GetCurrentBranch(ctx context.Context) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	// Get current branch using git rev-parse --abbrev-ref HEAD
	branchName := s.RunGit(
		ctx,
		[]string{"git", "rev-parse", "--abbrev-ref", "HEAD"},
		cwd,
		[]int{0},
		true,
		false,
	)

	branchName = strings.TrimSpace(branchName)

	if branchName == "" || branchName == "HEAD" {
		return "", fmt.Errorf("not currently on a branch (detached HEAD)")
	}

	return branchName, nil
}

// GetHeadSHA returns the HEAD commit SHA for a worktree path.
func (s *Service) GetHeadSHA(ctx context.Context, worktreePath string) string {
	return s.RunGit(ctx, []string{"git", "rev-parse", "HEAD"}, worktreePath, []int{0}, true, true)
}

// GetMergedBranches returns local branches that have been merged into the specified base branch.
func (s *Service) GetMergedBranches(ctx context.Context, baseBranch string) []string {
	output := s.RunGit(ctx, []string{"git", "branch", "--merged", baseBranch}, "", []int{0}, true, false)
	if output == "" {
		return nil
	}

	var merged []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		// Skip current branch marker (starts with "* ")
		line = strings.TrimPrefix(line, "* ")
		// Skip worktree branch marker (starts with "+ ")
		line = strings.TrimPrefix(line, "+ ")
		line = strings.TrimSpace(line)
		if line == "" || line == baseBranch {
			continue
		}
		merged = append(merged, line)
	}
	return merged
}

// GetWorktrees parses git worktree metadata and returns the list of worktrees.
// This method concurrently fetches status information for each worktree to improve performance.
// The first worktree in the list is marked as the main worktree.
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

	lines := strings.SplitSeq(rawWts, "\n")
	for line := range lines {
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

	for line := range strings.SplitSeq(branchRaw, "\n") {
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
			upstreamBranch := ""
			untracked := 0
			modified := 0
			staged := 0

			for _, line := range strings.Split(statusRaw, "\n") {
				switch {
				case strings.HasPrefix(line, "# branch.upstream "):
					hasUpstream = true
					upstreamBranch = strings.TrimPrefix(line, "# branch.upstream ")
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

			// Calculate unpushed commits for branches without upstream
			unpushed := 0
			if !hasUpstream {
				unpushedRaw := s.RunGit(ctx, []string{"git", "rev-list", "-100", "HEAD", "--not", "--remotes"}, path, []int{0}, true, true)
				for _, line := range strings.Split(unpushedRaw, "\n") {
					if strings.TrimSpace(line) != "" {
						unpushed++
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
				Path:           path,
				Branch:         branch,
				IsMain:         wtData.isMain,
				Dirty:          (untracked + modified + staged) > 0,
				Ahead:          ahead,
				Behind:         behind,
				Unpushed:       unpushed,
				HasUpstream:    hasUpstream,
				UpstreamBranch: upstreamBranch,
				LastActive:     lastActive,
				LastActiveTS:   lastActiveTS,
				Untracked:      untracked,
				Modified:       modified,
				Staged:         staged,
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

// DetectHost detects the git host (github, gitlab, or unknown)
func (s *Service) DetectHost(ctx context.Context) string {
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

// IsGitHubOrGitLab returns true if the repository is connected to GitHub or GitLab.
func (s *Service) IsGitHubOrGitLab(ctx context.Context) bool {
	host := s.DetectHost(ctx)
	return host == gitHostGithub || host == gitHostGitLab
}

// IsGitHub returns true if the repository is connected to GitHub.
func (s *Service) IsGitHub(ctx context.Context) bool {
	return s.DetectHost(ctx) == gitHostGithub
}

func (s *Service) fetchGitLabPRs(ctx context.Context) (map[string]*models.PRInfo, error) {
	prRaw := s.RunGit(ctx, []string{"glab", "api", "merge_requests?state=all&per_page=100"}, "", []int{0}, false, false)
	if prRaw == "" {
		return make(map[string]*models.PRInfo), nil
	}

	var prs []map[string]any
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
		description, _ := p["description"].(string)
		webURL, _ := p["web_url"].(string)
		sourceBranch, _ := p["source_branch"].(string)

		author := ""
		authorName := ""
		authorIsBot := false
		if authorObj, ok := p["author"].(map[string]any); ok {
			if username, ok := authorObj["username"].(string); ok {
				author = username
			}
			if name, ok := authorObj["name"].(string); ok {
				authorName = name
			}
			if bot, ok := authorObj["bot"].(bool); ok {
				authorIsBot = bot
			}
		}

		if sourceBranch != "" {
			prMap[sourceBranch] = &models.PRInfo{
				Number:      int(iid),
				State:       state,
				Title:       title,
				Body:        description,
				URL:         webURL,
				Branch:      sourceBranch,
				Author:      author,
				AuthorName:  authorName,
				AuthorIsBot: authorIsBot,
			}
		}
	}

	return prMap, nil
}

// FetchPRMap gathers PR/MR information via supported host APIs (GitHub or GitLab).
// Returns a map keyed by branch name to PRInfo. Detects the host automatically
// based on the repository's remote URL.
func (s *Service) FetchPRMap(ctx context.Context) (map[string]*models.PRInfo, error) {
	host := s.DetectHost(ctx)

	// Skip PR fetching for repos without GitHub/GitLab remotes
	if host == gitHostUnknown {
		return make(map[string]*models.PRInfo), nil
	}

	if host == gitHostGitLab {
		return s.fetchGitLabPRs(ctx)
	}

	// Default to GitHub
	prRaw := s.RunGit(ctx, []string{
		"gh", "pr", "list",
		"--state", "all",
		"--json", "headRefName,state,number,title,body,url,author",
		"--limit", "100",
	}, "", []int{0}, false, host == gitHostUnknown)

	if prRaw == "" {
		return make(map[string]*models.PRInfo), nil
	}

	var prs []map[string]any
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
		body, _ := p["body"].(string)
		url, _ := p["url"].(string)

		author := ""
		authorName := ""
		authorIsBot := false
		if authorObj, ok := p["author"].(map[string]any); ok {
			if login, ok := authorObj["login"].(string); ok {
				author = login
			}
			if name, ok := authorObj["name"].(string); ok {
				authorName = name
			}
			if isBot, ok := authorObj["is_bot"].(bool); ok {
				authorIsBot = isBot
			}
		}

		if headRefName != "" {
			prMap[headRefName] = &models.PRInfo{
				Number:      int(number),
				State:       state,
				Title:       title,
				Body:        body,
				URL:         url,
				Branch:      headRefName,
				Author:      author,
				AuthorName:  authorName,
				AuthorIsBot: authorIsBot,
			}
		}
	}

	return prMap, nil
}

// FetchPRForWorktreeWithError fetches PR info and returns detailed error information.
func (s *Service) FetchPRForWorktreeWithError(ctx context.Context, worktreePath string) (*models.PRInfo, error) {
	host := s.DetectHost(ctx)

	switch host {
	case gitHostGithub:
		// Run gh pr view with silent=false to capture actual errors
		prRaw := s.RunGit(ctx, []string{
			"gh", "pr", "view",
			"--json", "number,state,title,body,url,headRefName,baseRefName,author",
		}, worktreePath, []int{0, 1}, false, false)

		if prRaw == "" {
			// Check if it's because gh CLI is missing
			if _, err := exec.LookPath("gh"); err != nil {
				return nil, fmt.Errorf("gh CLI not found in PATH")
			}
			// Exit code 1 typically means "no PR found", which is not an error
			return nil, nil
		}

		var pr map[string]any
		if err := json.Unmarshal([]byte(prRaw), &pr); err != nil {
			return nil, fmt.Errorf("failed to parse PR data: %w", err)
		}

		number, _ := pr["number"].(float64)
		state, _ := pr["state"].(string)
		title, _ := pr["title"].(string)
		body, _ := pr["body"].(string)
		url, _ := pr["url"].(string)
		headRefName, _ := pr["headRefName"].(string)
		baseRefName, _ := pr["baseRefName"].(string)

		author := ""
		authorName := ""
		authorIsBot := false
		if authorObj, ok := pr["author"].(map[string]any); ok {
			if login, ok := authorObj["login"].(string); ok {
				author = login
			}
			if name, ok := authorObj["name"].(string); ok {
				authorName = name
			}
			if isBot, ok := authorObj["is_bot"].(bool); ok {
				authorIsBot = isBot
			}
		}

		return &models.PRInfo{
			Number:      int(number),
			State:       state,
			Title:       title,
			Body:        body,
			URL:         url,
			Branch:      headRefName,
			BaseBranch:  baseRefName,
			Author:      author,
			AuthorName:  authorName,
			AuthorIsBot: authorIsBot,
		}, nil

	case gitHostGitLab:
		// Run glab mr view with silent=false to capture actual errors
		prRaw := s.RunGit(ctx, []string{
			"glab", "mr", "view",
			"--output", "json",
		}, worktreePath, []int{0, 1}, false, false)

		if prRaw == "" {
			// Check if it's because glab CLI is missing
			if _, err := exec.LookPath("glab"); err != nil {
				return nil, fmt.Errorf("glab CLI not found in PATH")
			}
			// Exit code 1 typically means "no MR found", which is not an error
			return nil, nil
		}

		var pr map[string]any
		if err := json.Unmarshal([]byte(prRaw), &pr); err != nil {
			return nil, fmt.Errorf("failed to parse MR data: %w", err)
		}

		iid, _ := pr["iid"].(float64)
		state, _ := pr["state"].(string)
		if state == "opened" {
			state = "OPEN"
		} else {
			state = strings.ToUpper(state)
		}
		title, _ := pr["title"].(string)
		description, _ := pr["description"].(string)
		webURL, _ := pr["web_url"].(string)
		sourceBranch, _ := pr["source_branch"].(string)
		targetBranch, _ := pr["target_branch"].(string)

		author := ""
		authorName := ""
		authorIsBot := false
		if authorObj, ok := pr["author"].(map[string]any); ok {
			if username, ok := authorObj["username"].(string); ok {
				author = username
			}
			if name, ok := authorObj["name"].(string); ok {
				authorName = name
			}
			if bot, ok := authorObj["bot"].(bool); ok {
				authorIsBot = bot
			}
		}

		return &models.PRInfo{
			Number:      int(iid),
			State:       state,
			Title:       title,
			Body:        description,
			URL:         webURL,
			Branch:      sourceBranch,
			BaseBranch:  targetBranch,
			Author:      author,
			AuthorName:  authorName,
			AuthorIsBot: authorIsBot,
		}, nil
	}

	return nil, nil
}

// FetchPRForWorktree fetches PR info for a specific worktree by running gh/glab in that directory.
// This correctly detects PRs even when the local branch name differs from the remote branch.
// Maintains backward compatibility by swallowing errors.
func (s *Service) FetchPRForWorktree(ctx context.Context, worktreePath string) *models.PRInfo {
	pr, _ := s.FetchPRForWorktreeWithError(ctx, worktreePath)
	return pr
}

// FetchAllOpenPRs fetches all open PRs/MRs and returns them as a slice.
func (s *Service) FetchAllOpenPRs(ctx context.Context) ([]*models.PRInfo, error) {
	host := s.DetectHost(ctx)
	if host == gitHostGitLab {
		return s.fetchGitLabOpenPRs(ctx)
	}

	// Default to GitHub
	prRaw := s.RunGit(ctx, []string{
		"gh", "pr", "list",
		"--state", "open",
		"--json", "headRefName,state,number,title,body,url,author,isDraft,statusCheckRollup",
		"--limit", "100",
	}, "", []int{0}, false, host == gitHostUnknown)

	if prRaw == "" {
		return []*models.PRInfo{}, nil
	}

	var prs []map[string]any
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
		body, _ := p["body"].(string)
		url, _ := p["url"].(string)
		headRefName, _ := p["headRefName"].(string)

		author := ""
		authorName := ""
		authorIsBot := false
		if authorObj, ok := p["author"].(map[string]any); ok {
			if login, ok := authorObj["login"].(string); ok {
				author = login
			}
			if name, ok := authorObj["name"].(string); ok {
				authorName = name
			}
			if isBot, ok := authorObj["is_bot"].(bool); ok {
				authorIsBot = isBot
			}
		}

		isDraft, _ := p["isDraft"].(bool)
		ciStatus := computeCIStatusFromRollup(p["statusCheckRollup"])

		result = append(result, &models.PRInfo{
			Number:      int(number),
			State:       prStateOpen,
			Title:       title,
			Body:        body,
			URL:         url,
			Branch:      headRefName,
			Author:      author,
			AuthorName:  authorName,
			AuthorIsBot: authorIsBot,
			IsDraft:     isDraft,
			CIStatus:    ciStatus,
		})
	}

	return result, nil
}

func (s *Service) fetchGitLabOpenPRs(ctx context.Context) ([]*models.PRInfo, error) {
	prRaw := s.RunGit(ctx, []string{"glab", "api", "merge_requests?state=opened&per_page=100"}, "", []int{0}, false, false)
	if prRaw == "" {
		return []*models.PRInfo{}, nil
	}

	var prs []map[string]any
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
		description, _ := p["description"].(string)
		webURL, _ := p["web_url"].(string)
		sourceBranch, _ := p["source_branch"].(string)

		author := ""
		authorName := ""
		authorIsBot := false
		if authorObj, ok := p["author"].(map[string]any); ok {
			if username, ok := authorObj["username"].(string); ok {
				author = username
			}
			if name, ok := authorObj["name"].(string); ok {
				authorName = name
			}
			if bot, ok := authorObj["bot"].(bool); ok {
				authorIsBot = bot
			}
		}

		// GitLab uses "draft" field for WIP/draft MRs
		isDraft, _ := p["draft"].(bool)
		// CI status would require additional API calls for GitLab, default to none
		ciStatus := "none"

		result = append(result, &models.PRInfo{
			Number:      int(iid),
			State:       state,
			Title:       title,
			Body:        description,
			URL:         webURL,
			Branch:      sourceBranch,
			Author:      author,
			AuthorName:  authorName,
			AuthorIsBot: authorIsBot,
			IsDraft:     isDraft,
			CIStatus:    ciStatus,
		})
	}

	return result, nil
}

// FetchPR fetches a single PR by number.
func (s *Service) FetchPR(ctx context.Context, prNumber int) (*models.PRInfo, error) {
	host := s.DetectHost(ctx)
	if host == gitHostGitLab {
		return s.fetchGitLabPR(ctx, prNumber)
	}

	// Default to GitHub
	prRaw := s.RunGit(ctx, []string{
		"gh", "pr", "view", strconv.Itoa(prNumber),
		"--json", "headRefName,baseRefName,state,number,title,body,url,author,isDraft,statusCheckRollup",
	}, "", []int{0}, false, host == gitHostUnknown)

	if prRaw == "" {
		return nil, fmt.Errorf("PR #%d not found", prNumber)
	}

	var pr map[string]any
	if err := json.Unmarshal([]byte(prRaw), &pr); err != nil {
		key := "pr_json_decode"
		s.notifyOnce(key, fmt.Sprintf("Failed to parse PR data: %v", err), "error")
		return nil, err
	}

	state, _ := pr["state"].(string)
	if !strings.EqualFold(state, prStateOpen) {
		return nil, fmt.Errorf("PR #%d is not open (state: %s)", prNumber, state)
	}

	number, _ := pr["number"].(float64)
	title, _ := pr["title"].(string)
	body, _ := pr["body"].(string)
	url, _ := pr["url"].(string)
	headRefName, _ := pr["headRefName"].(string)
	baseRefName, _ := pr["baseRefName"].(string)

	author := ""
	authorName := ""
	authorIsBot := false
	if authorObj, ok := pr["author"].(map[string]any); ok {
		if login, ok := authorObj["login"].(string); ok {
			author = login
		}
		if name, ok := authorObj["name"].(string); ok {
			authorName = name
		}
		if isBot, ok := authorObj["is_bot"].(bool); ok {
			authorIsBot = isBot
		}
	}

	isDraft, _ := pr["isDraft"].(bool)
	ciStatus := computeCIStatusFromRollup(pr["statusCheckRollup"])

	return &models.PRInfo{
		Number:      int(number),
		State:       prStateOpen,
		Title:       title,
		Body:        body,
		URL:         url,
		Branch:      headRefName,
		BaseBranch:  baseRefName,
		Author:      author,
		AuthorName:  authorName,
		AuthorIsBot: authorIsBot,
		IsDraft:     isDraft,
		CIStatus:    ciStatus,
	}, nil
}

// fetchGitLabPR fetches a single PR (merge request) by number from GitLab.
func (s *Service) fetchGitLabPR(ctx context.Context, prNumber int) (*models.PRInfo, error) {
	prRaw := s.RunGit(ctx, []string{"glab", "api", fmt.Sprintf("merge_requests/%d", prNumber)}, "", []int{0}, false, false)
	if prRaw == "" {
		return nil, fmt.Errorf("PR #%d not found", prNumber)
	}

	var pr map[string]any
	if err := json.Unmarshal([]byte(prRaw), &pr); err != nil {
		key := "pr_json_decode_glab"
		s.notifyOnce(key, fmt.Sprintf("Failed to parse GLAB PR data: %v", err), "error")
		return nil, err
	}

	state, _ := pr["state"].(string)
	state = strings.ToUpper(state)
	if state == "OPENED" {
		state = prStateOpen
	}
	if state != prStateOpen {
		return nil, fmt.Errorf("PR #%d is not open (state: %s)", prNumber, state)
	}

	iid, _ := pr["iid"].(float64)
	title, _ := pr["title"].(string)
	description, _ := pr["description"].(string)
	webURL, _ := pr["web_url"].(string)
	sourceBranch, _ := pr["source_branch"].(string)
	targetBranch, _ := pr["target_branch"].(string)

	author := ""
	authorName := ""
	authorIsBot := false
	if authorObj, ok := pr["author"].(map[string]any); ok {
		if username, ok := authorObj["username"].(string); ok {
			author = username
		}
		if name, ok := authorObj["name"].(string); ok {
			authorName = name
		}
		if bot, ok := authorObj["bot"].(bool); ok {
			authorIsBot = bot
		}
	}

	isDraft, _ := pr["draft"].(bool)
	ciStatus := "none"

	return &models.PRInfo{
		Number:      int(iid),
		State:       state,
		Title:       title,
		Body:        description,
		URL:         webURL,
		Branch:      sourceBranch,
		BaseBranch:  targetBranch,
		Author:      author,
		AuthorName:  authorName,
		AuthorIsBot: authorIsBot,
		IsDraft:     isDraft,
		CIStatus:    ciStatus,
	}, nil
}

// FetchAllOpenIssues fetches all open issues and returns them as a slice.
func (s *Service) FetchAllOpenIssues(ctx context.Context) ([]*models.IssueInfo, error) {
	host := s.DetectHost(ctx)
	if host == gitHostGitLab {
		return s.fetchGitLabOpenIssues(ctx)
	}

	// Default to GitHub
	issueRaw := s.RunGit(ctx, []string{
		"gh", "issue", "list",
		"--state", "open",
		"--json", "number,state,title,body,url,author",
		"--limit", "100",
	}, "", []int{0}, false, host == gitHostUnknown)

	if issueRaw == "" {
		return []*models.IssueInfo{}, nil
	}

	var issues []map[string]any
	if err := json.Unmarshal([]byte(issueRaw), &issues); err != nil {
		key := "issue_json_decode"
		s.notifyOnce(key, fmt.Sprintf("Failed to parse issue data: %v", err), "error")
		return nil, err
	}

	result := make([]*models.IssueInfo, 0, len(issues))
	for _, i := range issues {
		state, _ := i["state"].(string)
		if !strings.EqualFold(state, "open") {
			continue
		}
		number, _ := i["number"].(float64)
		title, _ := i["title"].(string)
		body, _ := i["body"].(string)
		url, _ := i["url"].(string)

		author := ""
		authorName := ""
		authorIsBot := false
		if authorObj, ok := i["author"].(map[string]any); ok {
			if login, ok := authorObj["login"].(string); ok {
				author = login
			}
			if name, ok := authorObj["name"].(string); ok {
				authorName = name
			}
			if isBot, ok := authorObj["is_bot"].(bool); ok {
				authorIsBot = isBot
			}
		}

		result = append(result, &models.IssueInfo{
			Number:      int(number),
			State:       "open",
			Title:       title,
			Body:        body,
			URL:         url,
			Author:      author,
			AuthorName:  authorName,
			AuthorIsBot: authorIsBot,
		})
	}

	return result, nil
}

func (s *Service) fetchGitLabOpenIssues(ctx context.Context) ([]*models.IssueInfo, error) {
	issueRaw := s.RunGit(ctx, []string{"glab", "api", "issues?state=opened&per_page=100"}, "", []int{0}, false, false)
	if issueRaw == "" {
		return []*models.IssueInfo{}, nil
	}

	var issues []map[string]any
	if err := json.Unmarshal([]byte(issueRaw), &issues); err != nil {
		key := "issue_json_decode_glab"
		s.notifyOnce(key, fmt.Sprintf("Failed to parse GLAB issue data: %v", err), "error")
		return nil, err
	}

	result := make([]*models.IssueInfo, 0, len(issues))
	for _, i := range issues {
		state, _ := i["state"].(string)
		state = strings.ToUpper(state)
		if state != issueStateOpened && state != "OPEN" {
			continue
		}

		iid, _ := i["iid"].(float64)
		title, _ := i["title"].(string)
		description, _ := i["description"].(string)
		webURL, _ := i["web_url"].(string)

		author := ""
		authorName := ""
		authorIsBot := false
		if authorObj, ok := i["author"].(map[string]any); ok {
			if username, ok := authorObj["username"].(string); ok {
				author = username
			}
			if name, ok := authorObj["name"].(string); ok {
				authorName = name
			}
			if bot, ok := authorObj["bot"].(bool); ok {
				authorIsBot = bot
			}
		}

		result = append(result, &models.IssueInfo{
			Number:      int(iid),
			State:       "open",
			Title:       title,
			Body:        description,
			URL:         webURL,
			Author:      author,
			AuthorName:  authorName,
			AuthorIsBot: authorIsBot,
		})
	}

	return result, nil
}

// FetchIssue fetches a single issue by number.
func (s *Service) FetchIssue(ctx context.Context, issueNumber int) (*models.IssueInfo, error) {
	host := s.DetectHost(ctx)
	if host == gitHostGitLab {
		return s.fetchGitLabIssue(ctx, issueNumber)
	}

	// Default to GitHub
	issueRaw := s.RunGit(ctx, []string{
		"gh", "issue", "view", strconv.Itoa(issueNumber),
		"--json", "number,state,title,body,url,author",
	}, "", []int{0}, false, host == gitHostUnknown)

	if issueRaw == "" {
		return nil, fmt.Errorf("issue #%d not found", issueNumber)
	}

	var issue map[string]any
	if err := json.Unmarshal([]byte(issueRaw), &issue); err != nil {
		key := "issue_json_decode"
		s.notifyOnce(key, fmt.Sprintf("Failed to parse issue data: %v", err), "error")
		return nil, err
	}

	state, _ := issue["state"].(string)
	if !strings.EqualFold(state, "open") {
		return nil, fmt.Errorf("issue #%d is not open (state: %s)", issueNumber, state)
	}

	number, _ := issue["number"].(float64)
	title, _ := issue["title"].(string)
	body, _ := issue["body"].(string)
	url, _ := issue["url"].(string)

	author := ""
	authorName := ""
	authorIsBot := false
	if authorObj, ok := issue["author"].(map[string]any); ok {
		if login, ok := authorObj["login"].(string); ok {
			author = login
		}
		if name, ok := authorObj["name"].(string); ok {
			authorName = name
		}
		if isBot, ok := authorObj["is_bot"].(bool); ok {
			authorIsBot = isBot
		}
	}

	return &models.IssueInfo{
		Number:      int(number),
		State:       "open",
		Title:       title,
		Body:        body,
		URL:         url,
		Author:      author,
		AuthorName:  authorName,
		AuthorIsBot: authorIsBot,
	}, nil
}

// fetchGitLabIssue fetches a single issue by number from GitLab.
func (s *Service) fetchGitLabIssue(ctx context.Context, issueNumber int) (*models.IssueInfo, error) {
	issueRaw := s.RunGit(ctx, []string{"glab", "api", fmt.Sprintf("issues/%d", issueNumber)}, "", []int{0}, false, false)
	if issueRaw == "" {
		return nil, fmt.Errorf("issue #%d not found", issueNumber)
	}

	var issue map[string]any
	if err := json.Unmarshal([]byte(issueRaw), &issue); err != nil {
		key := "issue_json_decode_glab"
		s.notifyOnce(key, fmt.Sprintf("Failed to parse GLAB issue data: %v", err), "error")
		return nil, err
	}

	state, _ := issue["state"].(string)
	state = strings.ToUpper(state)
	if state != issueStateOpened && state != "OPEN" {
		return nil, fmt.Errorf("issue #%d is not open (state: %s)", issueNumber, state)
	}

	iid, _ := issue["iid"].(float64)
	title, _ := issue["title"].(string)
	description, _ := issue["description"].(string)
	webURL, _ := issue["web_url"].(string)

	author := ""
	authorName := ""
	authorIsBot := false
	if authorObj, ok := issue["author"].(map[string]any); ok {
		if username, ok := authorObj["username"].(string); ok {
			author = username
		}
		if name, ok := authorObj["name"].(string); ok {
			authorName = name
		}
		if bot, ok := authorObj["bot"].(bool); ok {
			authorIsBot = bot
		}
	}

	return &models.IssueInfo{
		Number:      int(iid),
		State:       "open",
		Title:       title,
		Body:        description,
		URL:         webURL,
		Author:      author,
		AuthorName:  authorName,
		AuthorIsBot: authorIsBot,
	}, nil
}

// FetchCIStatus fetches CI check statuses for a PR from GitHub or GitLab.
func (s *Service) FetchCIStatus(ctx context.Context, prNumber int, branch string) ([]*models.CICheck, error) {
	host := s.DetectHost(ctx)
	switch host {
	case gitHostGithub:
		return s.fetchGitHubCI(ctx, prNumber)
	case gitHostGitLab:
		return s.fetchGitLabCI(ctx, branch)
	default:
		return nil, nil
	}
}

// FetchCIStatusByCommit fetches CI check statuses for a commit SHA (GitHub only).
// This is used for branches without an associated PR.
func (s *Service) FetchCIStatusByCommit(ctx context.Context, commitSHA, worktreePath string) ([]*models.CICheck, error) {
	if s.DetectHost(ctx) != gitHostGithub {
		return nil, nil
	}

	repoName := s.ResolveRepoName(ctx)
	if repoName == "" || repoName == "unknown" || strings.HasPrefix(repoName, "local-") {
		return nil, nil
	}

	// GitHub Check Runs API: GET /repos/{owner}/{repo}/commits/{ref}/check-runs
	apiPath := fmt.Sprintf("repos/%s/commits/%s/check-runs", repoName, commitSHA)
	out := s.RunGit(ctx, []string{"gh", "api", apiPath, "--jq", ".check_runs"},
		worktreePath, []int{0, 1}, true, true)

	if out == "" {
		return nil, nil
	}

	var checkRuns []struct {
		Name       string `json:"name"`
		Status     string `json:"status"`     // queued, in_progress, completed
		Conclusion string `json:"conclusion"` // success, failure, neutral, cancelled, skipped, timed_out, action_required
		HTMLURL    string `json:"html_url"`
		StartedAt  string `json:"started_at"`
	}

	if err := json.Unmarshal([]byte(out), &checkRuns); err != nil {
		return nil, err
	}

	result := make([]*models.CICheck, 0, len(checkRuns))
	for _, run := range checkRuns {
		conclusion := s.mapGitHubConclusion(run.Status, run.Conclusion)
		var startedAt time.Time
		if run.StartedAt != "" {
			startedAt, _ = time.Parse(time.RFC3339, run.StartedAt)
		}
		result = append(result, &models.CICheck{
			Name:       run.Name,
			Status:     strings.ToLower(run.Status),
			Conclusion: conclusion,
			Link:       run.HTMLURL,
			StartedAt:  startedAt,
		})
	}
	return result, nil
}

// mapGitHubConclusion maps GitHub check run status and conclusion to our internal format.
func (s *Service) mapGitHubConclusion(status, conclusion string) string {
	// If still in progress, return pending
	if status == "queued" || status == "in_progress" {
		return ciPending
	}
	// Map conclusion values
	switch strings.ToLower(conclusion) {
	case "success":
		return ciSuccess
	case "failure":
		return ciFailure
	case "neutral", "skipped":
		return ciSkipped
	case "cancelled", "timed_out", "action_required":
		return ciCancelled
	default:
		return conclusion
	}
}

func (s *Service) fetchGitHubCI(ctx context.Context, prNumber int) ([]*models.CICheck, error) {
	// Use gh pr checks to get CI status
	out := s.RunGit(ctx, []string{
		"gh", "pr", "checks", fmt.Sprintf("%d", prNumber),
		"--json", "name,state,bucket,link,startedAt",
	}, "", []int{0, 1, 8}, true, true) // exit code 8 = checks pending

	if out == "" {
		return nil, nil
	}

	var checks []struct {
		Name      string `json:"name"`
		State     string `json:"state"`
		Bucket    string `json:"bucket"`    // pass, fail, pending, skipping, cancel
		Link      string `json:"link"`      // URL to the check details
		StartedAt string `json:"startedAt"` // ISO 8601 format from GH CLI
	}

	if err := json.Unmarshal([]byte(out), &checks); err != nil {
		return nil, err
	}

	result := make([]*models.CICheck, 0, len(checks))
	for _, c := range checks {
		// Map bucket to our conclusion format
		conclusion := s.githubBucketToConclusion(c.Bucket)
		var startedAt time.Time
		if c.StartedAt != "" {
			startedAt, _ = time.Parse(time.RFC3339, c.StartedAt)
		}
		result = append(result, &models.CICheck{
			Name:       c.Name,
			Status:     strings.ToLower(c.State),
			Conclusion: conclusion,
			Link:       c.Link,
			StartedAt:  startedAt,
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

// computeCIStatusFromRollup computes overall CI status from GitHub statusCheckRollup data.
// Returns "success", "failure", "pending", or "none".
func computeCIStatusFromRollup(rollup any) string {
	checks, ok := rollup.([]any)
	if !ok || len(checks) == 0 {
		return "none"
	}

	hasFailure := false
	hasPending := false

	for _, check := range checks {
		checkMap, ok := check.(map[string]any)
		if !ok {
			continue
		}

		conclusion, _ := checkMap["conclusion"].(string)
		status, _ := checkMap["status"].(string)

		// Check for failure states
		switch strings.ToUpper(conclusion) {
		case "FAILURE", "CANCELLED", "TIMED_OUT", "ACTION_REQUIRED":
			hasFailure = true
		}

		// Check for pending states
		if status != "" && !strings.EqualFold(status, "COMPLETED") {
			hasPending = true
		}
	}

	if hasFailure {
		return "failure"
	}
	if hasPending {
		return "pending"
	}
	return "success"
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
			Name      string `json:"name"`
			Status    string `json:"status"`
			WebURL    string `json:"web_url"`
			StartedAt string `json:"started_at"` // ISO 8601 format from GitLab
		} `json:"jobs"`
	}

	if err := json.Unmarshal([]byte(out), &pipeline); err != nil {
		// Try parsing as array of jobs directly
		var jobs []struct {
			Name      string `json:"name"`
			Status    string `json:"status"`
			WebURL    string `json:"web_url"`
			StartedAt string `json:"started_at"` // ISO 8601 format from GitLab
		}
		if err2 := json.Unmarshal([]byte(out), &jobs); err2 != nil {
			return nil, err
		}
		result := make([]*models.CICheck, 0, len(jobs))
		for _, j := range jobs {
			var startedAt time.Time
			if j.StartedAt != "" {
				startedAt, _ = time.Parse(time.RFC3339, j.StartedAt)
			}
			result = append(result, &models.CICheck{
				Name:       j.Name,
				Status:     strings.ToLower(j.Status),
				Conclusion: s.gitlabStatusToConclusion(j.Status),
				Link:       j.WebURL,
				StartedAt:  startedAt,
			})
		}
		return result, nil
	}

	result := make([]*models.CICheck, 0, len(pipeline.Jobs))
	for _, j := range pipeline.Jobs {
		var startedAt time.Time
		if j.StartedAt != "" {
			startedAt, _ = time.Parse(time.RFC3339, j.StartedAt)
		}
		result = append(result, &models.CICheck{
			Name:       j.Name,
			Status:     strings.ToLower(j.Status),
			Conclusion: s.gitlabStatusToConclusion(j.Status),
			Link:       j.WebURL,
			StartedAt:  startedAt,
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
// It fetches the PR head commit, creates a worktree at that commit with a proper branch,
// and sets up branch tracking configuration (replicating what gh/glab pr checkout does).
func (s *Service) CreateWorktreeFromPR(ctx context.Context, prNumber int, remoteBranch, localBranch, targetPath string) bool {
	host := s.DetectHost(ctx)

	// For unknown host, fall back to manual fetch
	if host != gitHostGithub && host != gitHostGitLab {
		if !s.RunCommandChecked(ctx, []string{"git", "fetch", "origin", remoteBranch}, "", fmt.Sprintf("Failed to fetch remote branch %s", remoteBranch)) {
			return false
		}
		remoteRef := fmt.Sprintf("origin/%s", remoteBranch)
		return s.RunCommandChecked(ctx, []string{"git", "worktree", "add", "-b", localBranch, targetPath, remoteRef}, "", fmt.Sprintf("Failed to create worktree from PR branch %s", remoteBranch))
	}

	var headCommit string
	var repoURL string
	var mergeRef string

	switch host {
	case gitHostGithub:
		// Get PR info including repo URL for tracking
		prRaw := s.RunGit(ctx, []string{
			"gh", "pr", "view", fmt.Sprintf("%d", prNumber),
			"--json", "headRefOid,headRepository",
		}, "", []int{0}, true, true)
		if prRaw == "" {
			s.notify(fmt.Sprintf("Failed to get PR #%d info", prNumber), "error")
			return false
		}
		var pr map[string]any
		if err := json.Unmarshal([]byte(prRaw), &pr); err != nil {
			s.notify(fmt.Sprintf("Failed to parse PR #%d data: %v", prNumber, err), "error")
			return false
		}
		headCommit, _ = pr["headRefOid"].(string)
		if headCommit == "" {
			s.notify(fmt.Sprintf("Failed to get PR #%d head commit", prNumber), "error")
			return false
		}
		// Get repo URL from headRepository
		if headRepo, ok := pr["headRepository"].(map[string]any); ok {
			repoURL, _ = headRepo["url"].(string)
		}
		// Fallback to origin URL if headRepository not available
		if repoURL == "" {
			repoURL = strings.TrimSpace(s.RunGit(ctx, []string{"git", "remote", "get-url", "origin"}, "", []int{0}, true, true))
		}
		mergeRef = fmt.Sprintf("refs/pull/%d/head", prNumber)

		// Fetch PR ref
		if !s.RunCommandChecked(ctx, []string{"git", "fetch", "origin", fmt.Sprintf("pull/%d/head", prNumber)}, "", fmt.Sprintf("Failed to fetch PR #%d", prNumber)) {
			return false
		}

	case gitHostGitLab:
		// Get MR info including source branch for tracking
		mrRaw := s.RunGit(ctx, []string{
			"glab", "mr", "view", fmt.Sprintf("%d", prNumber),
			"--output", "json",
		}, "", []int{0}, true, true)
		if mrRaw == "" {
			s.notify(fmt.Sprintf("Failed to get MR #%d info", prNumber), "error")
			return false
		}
		var mr map[string]any
		if err := json.Unmarshal([]byte(mrRaw), &mr); err != nil {
			s.notify(fmt.Sprintf("Failed to parse MR #%d data: %v", prNumber, err), "error")
			return false
		}
		headCommit, _ = mr["sha"].(string)
		if headCommit == "" {
			s.notify(fmt.Sprintf("Failed to get MR #%d head commit", prNumber), "error")
			return false
		}
		sourceBranch, _ := mr["source_branch"].(string)
		if sourceBranch == "" {
			sourceBranch = remoteBranch
		}
		// Get repo URL from origin
		repoURL = strings.TrimSpace(s.RunGit(ctx, []string{"git", "remote", "get-url", "origin"}, "", []int{0}, true, true))
		mergeRef = fmt.Sprintf("refs/heads/%s", sourceBranch)

		// Fetch MR source branch
		if !s.RunCommandChecked(ctx, []string{"git", "fetch", "origin", fmt.Sprintf("refs/heads/%s", sourceBranch)}, "", fmt.Sprintf("Failed to fetch MR #%d", prNumber)) {
			return false
		}
	}

	// Create worktree at the PR head commit with a proper branch
	if !s.RunCommandChecked(ctx, []string{"git", "worktree", "add", "-b", localBranch, targetPath, headCommit}, "", fmt.Sprintf("Failed to create worktree at %s", targetPath)) {
		return false
	}

	// Set up branch tracking configuration (replicating gh/glab pr checkout behavior)
	// This allows git pull/push to work correctly
	if repoURL != "" {
		s.RunGit(ctx, []string{"git", "config", fmt.Sprintf("branch.%s.remote", localBranch), repoURL}, targetPath, []int{0}, true, true)
		if host == gitHostGithub {
			// GitHub also sets pushRemote
			s.RunGit(ctx, []string{"git", "config", fmt.Sprintf("branch.%s.pushRemote", localBranch), repoURL}, targetPath, []int{0}, true, true)
		}
		s.RunGit(ctx, []string{"git", "config", fmt.Sprintf("branch.%s.merge", localBranch), mergeRef}, targetPath, []int{0}, true, true)
	}

	return true
}

// CheckoutPRBranch checks out a PR branch locally without creating a worktree.
func (s *Service) CheckoutPRBranch(ctx context.Context, prNumber int, remoteBranch, localBranch string) bool {
	host := s.DetectHost(ctx)

	// For unknown host, fall back to manual fetch
	if host != gitHostGithub && host != gitHostGitLab {
		if !s.RunCommandChecked(ctx, []string{"git", "fetch", "origin", remoteBranch}, "", fmt.Sprintf("Failed to fetch remote branch %s", remoteBranch)) {
			return false
		}
		remoteRef := fmt.Sprintf("origin/%s", remoteBranch)
		if !s.RunCommandChecked(ctx, []string{"git", "branch", localBranch, remoteRef}, "", fmt.Sprintf("Failed to create branch %s", localBranch)) {
			return false
		}
		return s.RunCommandChecked(ctx, []string{"git", "switch", localBranch}, "", fmt.Sprintf("Failed to switch to branch %s", localBranch))
	}

	var headCommit string
	var repoURL string
	var mergeRef string

	switch host {
	case gitHostGithub:
		prRaw := s.RunGit(ctx, []string{
			"gh", "pr", "view", fmt.Sprintf("%d", prNumber),
			"--json", "headRefOid,headRepository",
		}, "", []int{0}, true, true)
		if prRaw == "" {
			s.notify(fmt.Sprintf("Failed to get PR #%d info", prNumber), "error")
			return false
		}
		var pr map[string]any
		if err := json.Unmarshal([]byte(prRaw), &pr); err != nil {
			s.notify(fmt.Sprintf("Failed to parse PR #%d data: %v", prNumber, err), "error")
			return false
		}
		headCommit, _ = pr["headRefOid"].(string)
		if headCommit == "" {
			s.notify(fmt.Sprintf("Failed to get PR #%d head commit", prNumber), "error")
			return false
		}
		if headRepo, ok := pr["headRepository"].(map[string]any); ok {
			repoURL, _ = headRepo["url"].(string)
		}
		if repoURL == "" {
			repoURL = strings.TrimSpace(s.RunGit(ctx, []string{"git", "remote", "get-url", "origin"}, "", []int{0}, true, true))
		}
		mergeRef = fmt.Sprintf("refs/pull/%d/head", prNumber)

		if !s.RunCommandChecked(ctx, []string{"git", "fetch", "origin", fmt.Sprintf("pull/%d/head", prNumber)}, "", fmt.Sprintf("Failed to fetch PR #%d", prNumber)) {
			return false
		}

	case gitHostGitLab:
		mrRaw := s.RunGit(ctx, []string{
			"glab", "mr", "view", fmt.Sprintf("%d", prNumber),
			"--output", "json",
		}, "", []int{0}, true, true)
		if mrRaw == "" {
			s.notify(fmt.Sprintf("Failed to get MR #%d info", prNumber), "error")
			return false
		}
		var mr map[string]any
		if err := json.Unmarshal([]byte(mrRaw), &mr); err != nil {
			s.notify(fmt.Sprintf("Failed to parse MR #%d data: %v", prNumber, err), "error")
			return false
		}
		headCommit, _ = mr["sha"].(string)
		if headCommit == "" {
			s.notify(fmt.Sprintf("Failed to get MR #%d head commit", prNumber), "error")
			return false
		}
		sourceBranch, _ := mr["source_branch"].(string)
		if sourceBranch == "" {
			sourceBranch = remoteBranch
		}
		repoURL = strings.TrimSpace(s.RunGit(ctx, []string{"git", "remote", "get-url", "origin"}, "", []int{0}, true, true))
		mergeRef = fmt.Sprintf("refs/heads/%s", sourceBranch)

		if !s.RunCommandChecked(ctx, []string{"git", "fetch", "origin", fmt.Sprintf("refs/heads/%s", sourceBranch)}, "", fmt.Sprintf("Failed to fetch MR #%d", prNumber)) {
			return false
		}
	}

	if !s.RunCommandChecked(ctx, []string{"git", "branch", localBranch, headCommit}, "", fmt.Sprintf("Failed to create branch %s", localBranch)) {
		return false
	}

	// Replicate gh/glab pr checkout tracking behaviour
	if repoURL != "" {
		s.RunGit(ctx, []string{"git", "config", fmt.Sprintf("branch.%s.remote", localBranch), repoURL}, "", []int{0}, true, true)
		if host == gitHostGithub {
			s.RunGit(ctx, []string{"git", "config", fmt.Sprintf("branch.%s.pushRemote", localBranch), repoURL}, "", []int{0}, true, true)
		}
		s.RunGit(ctx, []string{"git", "config", fmt.Sprintf("branch.%s.merge", localBranch), mergeRef}, "", []int{0}, true, true)
	}

	if !s.RunCommandChecked(ctx, []string{"git", "switch", localBranch}, "", fmt.Sprintf("Failed to switch to branch %s", localBranch)) {
		return false
	}

	return true
}

// CherryPickCommit applies a commit to a target worktree.
// Returns true on success, false on failure (including conflicts).
func (s *Service) CherryPickCommit(ctx context.Context, commitSHA, targetPath string) (bool, error) {
	// Check if there are uncommitted changes in target worktree
	statusRaw := s.RunGit(ctx, []string{"git", "status", "--porcelain"}, targetPath, []int{0}, true, false)
	if strings.TrimSpace(statusRaw) != "" {
		return false, fmt.Errorf("target worktree has uncommitted changes")
	}

	// Attempt cherry-pick
	cmd, err := s.prepareAllowedCommand(ctx, []string{"git", "cherry-pick", commitSHA})
	if err != nil {
		return false, err
	}
	cmd.Dir = targetPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Cherry-pick failed - check if it's due to conflicts
		detail := strings.TrimSpace(string(output))

		// Abort the cherry-pick to leave worktree clean
		s.RunCommandChecked(ctx, []string{"git", "cherry-pick", "--abort"}, targetPath, "Failed to abort cherry-pick")

		if strings.Contains(detail, "conflict") || strings.Contains(detail, "CONFLICT") {
			return false, fmt.Errorf("cherry-pick conflicts occurred: %s", detail)
		}
		return false, fmt.Errorf("cherry-pick failed: %s", detail)
	}

	return true, nil
}

// localRepoKey builds a stable, compact cache key when no remote name is available.
func localRepoKey(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(path))
	return fmt.Sprintf("local-%x", sum[:8])
}

// ResolveRepoName resolves the repository name using various methods.
// ResolveRepoName returns the repository identifier for caching purposes.
func (s *Service) ResolveRepoName(ctx context.Context) string {
	var repoName string

	// Try git remote get-url origin
	remoteURL := s.RunGit(ctx, []string{"git", "remote", "get-url", "origin"}, "", []int{0}, true, true)

	// Optimization: If it's a standard GitHub/GitLab URL, parse directly and avoid external tool overhead
	if remoteURL != "" {
		if strings.Contains(remoteURL, "github.com") {
			re := regexp.MustCompile(`github\.com[:/](.+)(?:\.git)?$`)
			matches := re.FindStringSubmatch(remoteURL)
			if len(matches) > 1 {
				repoName = matches[1]
			}
		} else if strings.Contains(remoteURL, "gitlab.com") {
			re := regexp.MustCompile(`gitlab\.com[:/](.+)(?:\.git)?$`)
			matches := re.FindStringSubmatch(remoteURL)
			if len(matches) > 1 {
				repoName = matches[1]
			}
		}
	}

	if repoName == "" {
		// Try gh repo view
		if out := s.RunGit(ctx, []string{"gh", "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner"}, "", []int{0}, true, true); out != "" {
			repoName = out
		}
	}

	if repoName == "" {
		// Try glab repo view
		if out := s.RunGit(ctx, []string{"glab", "repo", "view", "-F", "json"}, "", []int{0}, false, true); out != "" {
			var data map[string]any
			if err := json.Unmarshal([]byte(out), &data); err == nil {
				if path, ok := data["path_with_namespace"].(string); ok {
					repoName = path
				}
			}
		}
	}

	if repoName == "" && remoteURL != "" {
		// Fallback: Parse remote URL if we have it (even if not github/gitlab, maybe self-hosted?)
		re := regexp.MustCompile(`[:/]([^/]+/[^/]+)(?:\.git)?$`)
		matches := re.FindStringSubmatch(remoteURL)
		if len(matches) > 1 {
			repoName = matches[1]
		}
	}

	if repoName == "" {
		// Try git rev-parse --show-toplevel
		if out := s.RunGit(ctx, []string{"git", "rev-parse", "--show-toplevel"}, "", []int{0}, true, true); out != "" {
			repoName = localRepoKey(out)
		}
	}

	if repoName == "" {
		return "unknown"
	}

	return strings.TrimSuffix(repoName, ".git")
}

// BuildThreePartDiff assembles a comprehensive diff showing staged, modified, and untracked sections.
// The output is truncated according to cfg.MaxDiffChars and cfg.MaxUntrackedDiffs settings.
// Part 1: Staged changes (git diff --cached)
// Part 2: Unstaged changes (git diff)
// Part 3: Untracked files (limited by MaxUntrackedDiffs)
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

// GetCommitFiles returns the list of files changed in a specific commit.
func (s *Service) GetCommitFiles(ctx context.Context, commitSHA, worktreePath string) ([]models.CommitFile, error) {
	raw := s.RunGit(ctx, []string{
		"git", "diff-tree", "--name-status", "-r", "--no-commit-id", commitSHA,
	}, worktreePath, []int{0}, false, false)

	if raw == "" {
		return []models.CommitFile{}, nil
	}

	return parseCommitFiles(raw), nil
}

// parseCommitFiles parses the output of git diff-tree --name-status.
// Format: "M\tpath" or "R100\told\tnew" for renames.
func parseCommitFiles(raw string) []models.CommitFile {
	lines := strings.Split(raw, "\n")
	files := make([]models.CommitFile, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}

		changeType := parts[0]
		filename := parts[1]
		oldPath := ""

		// Handle renames (R100) and copies (C100)
		if len(changeType) > 1 && (changeType[0] == 'R' || changeType[0] == 'C') {
			changeType = string(changeType[0])
			if len(parts) >= 3 {
				oldPath = parts[1]
				filename = parts[2]
			}
		}

		files = append(files, models.CommitFile{
			Filename:   filename,
			ChangeType: changeType,
			OldPath:    oldPath,
		})
	}
	return files
}
