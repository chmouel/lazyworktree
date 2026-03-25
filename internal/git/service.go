// Package git wraps git commands and helpers used by lazyworktree.
package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"sync"

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
	notify               NotifyFn
	notifyOnce           NotifyOnceFn
	semaphore            chan struct{}
	mainBranch           string
	gitHost              string
	remoteURL            string
	mainWorktreePath     string
	mainBranchOnce       sync.Once
	remoteURLOnce        sync.Once
	gitHostOnce          sync.Once
	mainWorktreePathOnce sync.Once
	notifiedSet          map[string]bool
	useGitPager          bool
	gitPagerArgs         []string
	gitPager             string
	commandRunner        func(ctx context.Context, name string, args ...string) *exec.Cmd
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

// BuildThreePartDiff assembles a comprehensive diff showing staged, modified, and untracked sections.
// The output is truncated according to cfg.MaxDiffChars and cfg.MaxUntrackedDiffs settings.
// Part 1: Staged changes (git diff --cached)
// Part 2: Unstaged changes (git diff)
// Part 3: Untracked files (limited by MaxUntrackedDiffs)
func (s *Service) BuildThreePartDiff(ctx context.Context, path string, cfg *config.AppConfig) string {
	var parts []string
	totalChars := 0

	var stagedDiff, unstagedDiff, untrackedRaw string
	var wg sync.WaitGroup
	wg.Go(func() {
		stagedDiff = s.RunGit(ctx, []string{"git", "diff", "--cached", "--patch", "--no-color"}, path, []int{0}, false, false)
	})
	wg.Go(func() {
		unstagedDiff = s.RunGit(ctx, []string{"git", "diff", "--patch", "--no-color"}, path, []int{0}, false, false)
	})
	wg.Go(func() {
		untrackedRaw = s.RunGit(ctx, []string{"git", "ls-files", "--others", "--exclude-standard"}, path, []int{0}, false, false)
	})
	wg.Wait()

	// Part 1: Staged changes
	if stagedDiff != "" {
		header := "=== Staged Changes ===\n"
		parts = append(parts, header+stagedDiff)
		totalChars += len(header) + len(stagedDiff)
	}

	// Part 2: Unstaged changes
	if totalChars < cfg.MaxDiffChars && unstagedDiff != "" {
		header := "=== Unstaged Changes ===\n"
		parts = append(parts, header+unstagedDiff)
		totalChars += len(header) + len(unstagedDiff)
	}

	// Part 3: Untracked files (limited by config)
	if totalChars < cfg.MaxDiffChars && cfg.MaxUntrackedDiffs > 0 {
		untrackedFiles := []string{}
		for line := range strings.SplitSeq(untrackedRaw, "\n") {
			file := strings.TrimSpace(line)
			if file != "" {
				untrackedFiles = append(untrackedFiles, file)
			}
		}
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
