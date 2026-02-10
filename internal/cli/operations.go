package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/git"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/security"
	"github.com/chmouel/lazyworktree/internal/utils"
)

// OSFilesystem abstracts filesystem operations for dependency injection.
type OSFilesystem interface {
	Stat(name string) (os.FileInfo, error)
	MkdirAll(path string, perm os.FileMode) error
	Getwd() (string, error)
}

// RealFilesystem implements OSFilesystem using the real os package.
type RealFilesystem struct{}

// Stat wraps os.Stat.
func (RealFilesystem) Stat(name string) (os.FileInfo, error) { return os.Stat(name) }

// MkdirAll wraps os.MkdirAll.
func (RealFilesystem) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }

// Getwd wraps os.Getwd.
func (RealFilesystem) Getwd() (string, error) { return os.Getwd() }

// DefaultFS is the default filesystem implementation using the real os package.
var DefaultFS OSFilesystem = RealFilesystem{}

type gitService interface {
	CheckoutPRBranch(ctx context.Context, prNumber int, remoteBranch string, localBranch string) bool
	CreateWorktreeFromPR(ctx context.Context, prNumber int, branch string, worktreeName string, targetPath string) bool
	ExecuteCommands(ctx context.Context, cmdList []string, cwd string, env map[string]string) error
	FetchAllOpenIssues(ctx context.Context) ([]*models.IssueInfo, error)
	FetchAllOpenPRs(ctx context.Context) ([]*models.PRInfo, error)
	FetchIssue(ctx context.Context, issueNumber int) (*models.IssueInfo, error)
	FetchPR(ctx context.Context, prNumber int) (*models.PRInfo, error)
	GetCurrentBranch(ctx context.Context) (string, error)
	GetMainWorktreePath(ctx context.Context) string
	GetWorktrees(ctx context.Context) ([]*models.WorktreeInfo, error)
	ResolveRepoName(ctx context.Context) string
	RunCommandChecked(ctx context.Context, args []string, cwd string, errorMsg string) bool
	RunGit(ctx context.Context, args []string, cwd string, exitCodes []int, silent bool, ignoreErrors bool) string
}

var _ gitService = (*git.Service)(nil)

// CreateFromBranch creates a worktree from a branch name.
func CreateFromBranch(ctx context.Context, gitSvc gitService, cfg *config.AppConfig, branchName, worktreeName string, withChange, silent bool) (string, error) {
	return CreateFromBranchWithFS(ctx, gitSvc, cfg, branchName, worktreeName, withChange, silent, DefaultFS)
}

// CreateFromBranchWithFS creates a worktree from a branch name using the provided filesystem.
func CreateFromBranchWithFS(ctx context.Context, gitSvc gitService, cfg *config.AppConfig, branchName, worktreeName string, withChange, silent bool, fs OSFilesystem) (string, error) {
	// Validate branch exists
	if !branchExists(ctx, gitSvc, branchName) {
		return "", fmt.Errorf("branch %q does not exist", branchName)
	}

	// Get current worktree if --with-change is specified
	var currentWt *models.WorktreeInfo
	var hasChanges bool
	if withChange {
		var err error
		currentWt, hasChanges, err = getCurrentWorktreeWithChangesFS(ctx, gitSvc, fs)
		if err != nil {
			return "", err
		}

		if !hasChanges {
			if !silent {
				fmt.Fprintf(os.Stderr, "No uncommitted changes found, proceeding without --with-change\n")
			}
			withChange = false
		}
	}

	// Construct target path based on worktree name
	repoName := gitSvc.ResolveRepoName(ctx)

	// Generate random name if not provided, or validate user-provided name
	if worktreeName == "" {
		// Generate random name with retry for uniqueness
		sanitizedBranch := utils.SanitizeBranchName(branchName, 50)
		worktreeName = generateUniqueWorktreeNameFS(cfg, repoName, sanitizedBranch, fs)
	} else {
		// Validate and sanitise user-provided name
		sanitised := utils.SanitizeBranchName(worktreeName, 100)
		if sanitised == "" {
			return "", fmt.Errorf("invalid worktree name: must contain at least one alphanumeric character")
		}
		worktreeName = sanitised
	}

	targetPath := filepath.Join(cfg.WorktreeDir, repoName, worktreeName)

	// Check for path conflicts
	if _, err := fs.Stat(targetPath); err == nil {
		return "", fmt.Errorf("path already exists: %s", targetPath)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to check path %s: %w", targetPath, err)
	}

	// Create parent directory
	if err := fs.MkdirAll(filepath.Dir(targetPath), utils.DefaultDirPerms); err != nil {
		return "", fmt.Errorf("failed to create worktree directory: %w", err)
	}

	if !silent {
		fmt.Fprintf(os.Stderr, "\nCreating worktree at: %s\n", targetPath)
	}

	// Create worktree with or without changes
	if withChange && hasChanges && currentWt != nil {
		if err := createWorktreeWithChanges(ctx, gitSvc, cfg, currentWt, branchName, worktreeName, targetPath, silent); err != nil {
			return "", err
		}
	} else {
		if err := createWorktreeFromBranch(ctx, gitSvc, cfg, branchName, worktreeName, targetPath, silent); err != nil {
			return "", err
		}
	}

	return targetPath, nil
}

func createWorktreeFromBranch(ctx context.Context, gitSvc gitService, cfg *config.AppConfig, branchName, worktreeName, targetPath string, silent bool) error {
	// Create worktree normally
	args := []string{"git", "worktree", "add"}

	// Determine if we need to create a new branch
	switch {
	case strings.Contains(branchName, "/"):
		// Remote branch - create new local branch with tracking
		args = append(args, "-b", worktreeName, "--track", targetPath, branchName)
	case worktreeName != branchName:
		// Creating a new branch with a different name (e.g., random name)
		// Always use -b to create the new branch based on the source branch
		args = append(args, "-b", worktreeName, targetPath, branchName)
	default:
		// Worktree name matches branch name - check if branch already exists
		localBranchExists := gitSvc.RunGit(
			ctx,
			[]string{"git", "show-ref", "--verify", fmt.Sprintf("refs/heads/%s", branchName)},
			"",
			[]int{0, 1},
			true,
			true,
		)

		if strings.TrimSpace(localBranchExists) != "" {
			// Local branch exists - checkout without creating new branch
			args = append(args, targetPath, branchName)
		} else {
			// Local branch doesn't exist - create it
			args = append(args, "-b", worktreeName, targetPath, branchName)
		}
	}

	if !gitSvc.RunCommandChecked(ctx, args, "", fmt.Sprintf("Failed to create worktree from branch %s", branchName)) {
		return fmt.Errorf("failed to create worktree")
	}

	// Run init commands
	if err := runInitCommands(ctx, gitSvc, cfg, worktreeName, targetPath, silent); err != nil {
		// Clean up the worktree if init commands fail
		gitSvc.RunCommandChecked(ctx, []string{"git", "worktree", "remove", "--force", targetPath}, "", "Failed to cleanup worktree")
		return err
	}

	return nil
}

// generateUniqueWorktreeNameFS generates a unique worktree name with retries.
// Format: <branch>-<random-adjective>-<random-noun>
// Retries up to 10 times if path already exists.
func generateUniqueWorktreeNameFS(cfg *config.AppConfig, repoName, branchName string, fs OSFilesystem) string {
	const maxRetries = 10

	for range maxRetries {
		randomPart := utils.RandomBranchName()
		candidate := fmt.Sprintf("%s-%s", branchName, randomPart)
		targetPath := filepath.Join(cfg.WorktreeDir, repoName, candidate)

		// Check if path exists
		if _, err := fs.Stat(targetPath); os.IsNotExist(err) {
			return candidate // Found unique name
		}
	}

	// Fallback: append timestamp if all retries fail (extremely unlikely)
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s-%s-%d", branchName, utils.RandomBranchName(), timestamp)
}

// CreateFromPR creates a worktree from a PR number.
func CreateFromPR(ctx context.Context, gitSvc gitService, cfg *config.AppConfig, prNumber int, noWorkspace, silent bool) (string, error) {
	return CreateFromPRWithFS(ctx, gitSvc, cfg, prNumber, noWorkspace, silent, DefaultFS)
}

// CreateFromPRWithFS creates a worktree from a PR number using the provided filesystem.
func CreateFromPRWithFS(ctx context.Context, gitSvc gitService, cfg *config.AppConfig, prNumber int, noWorkspace, silent bool, fs OSFilesystem) (string, error) {
	if !silent {
		fmt.Fprintf(os.Stderr, "Fetching PR #%d...\n", prNumber)
	}

	selectedPR, err := gitSvc.FetchPR(ctx, prNumber)
	if err != nil {
		return "", fmt.Errorf("failed to fetch PR: %w", err)
	}

	// Generate branch name using template
	template := cfg.PRBranchNameTemplate
	if template == "" {
		template = "pr-{number}-{title}"
	}
	branchName := utils.GeneratePRWorktreeName(selectedPR, template, "")

	if noWorkspace {
		if !gitSvc.CheckoutPRBranch(ctx, selectedPR.Number, selectedPR.Branch, branchName) {
			return "", fmt.Errorf("failed to checkout branch for PR #%d", selectedPR.Number)
		}
		return branchName, nil
	}

	// Construct target path
	repoName := gitSvc.ResolveRepoName(ctx)
	targetPath := filepath.Join(cfg.WorktreeDir, repoName, branchName)

	// Check for path conflicts
	if _, err := fs.Stat(targetPath); err == nil {
		return "", fmt.Errorf("path already exists: %s", targetPath)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to check path %s: %w", targetPath, err)
	}

	// Create parent directory
	if err := fs.MkdirAll(filepath.Dir(targetPath), utils.DefaultDirPerms); err != nil {
		return "", fmt.Errorf("failed to create worktree directory: %w", err)
	}

	// Create worktree from PR
	if !gitSvc.CreateWorktreeFromPR(ctx, selectedPR.Number, selectedPR.Branch, branchName, targetPath) {
		return "", fmt.Errorf("failed to create worktree from PR #%d", selectedPR.Number)
	}

	// Run init commands
	if err := runInitCommands(ctx, gitSvc, cfg, branchName, targetPath, silent); err != nil {
		// Clean up the worktree if init commands fail
		gitSvc.RunCommandChecked(ctx, []string{"git", "worktree", "remove", "--force", targetPath}, "", "Failed to cleanup worktree")
		return "", err
	}

	return targetPath, nil
}

// CreateFromIssue creates a worktree from an issue number.
func CreateFromIssue(ctx context.Context, gitSvc gitService, cfg *config.AppConfig, issueNumber int, baseBranch string, noWorkspace, silent bool) (string, error) {
	return CreateFromIssueWithFS(ctx, gitSvc, cfg, issueNumber, baseBranch, noWorkspace, silent, DefaultFS)
}

// CreateFromIssueWithFS creates a worktree from an issue number using the provided filesystem.
func CreateFromIssueWithFS(ctx context.Context, gitSvc gitService, cfg *config.AppConfig, issueNumber int, baseBranch string, noWorkspace, silent bool, fs OSFilesystem) (string, error) {
	if !silent {
		fmt.Fprintf(os.Stderr, "Fetching issue #%d...\n", issueNumber)
	}

	selectedIssue, err := gitSvc.FetchIssue(ctx, issueNumber)
	if err != nil {
		return "", fmt.Errorf("failed to fetch issue: %w", err)
	}

	// Generate branch name using template
	template := cfg.IssueBranchNameTemplate
	if template == "" {
		template = "issue-{number}-{title}"
	}
	branchName := utils.GenerateIssueWorktreeName(selectedIssue, template, "")

	if noWorkspace {
		if !gitSvc.RunCommandChecked(ctx,
			[]string{"git", "branch", branchName, baseBranch},
			"",
			fmt.Sprintf("Failed to create branch from issue #%d", issueNumber),
		) {
			return "", fmt.Errorf("failed to create branch from issue #%d", issueNumber)
		}
		if !gitSvc.RunCommandChecked(ctx,
			[]string{"git", "switch", branchName},
			"",
			fmt.Sprintf("Failed to switch to branch %s", branchName),
		) {
			return "", fmt.Errorf("failed to switch to branch %s", branchName)
		}
		return branchName, nil
	}

	// Construct target path
	repoName := gitSvc.ResolveRepoName(ctx)
	targetPath := filepath.Join(cfg.WorktreeDir, repoName, branchName)

	// Check for path conflicts
	if _, err := fs.Stat(targetPath); err == nil {
		return "", fmt.Errorf("path already exists: %s", targetPath)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to check path %s: %w", targetPath, err)
	}

	// Create parent directory
	if err := fs.MkdirAll(filepath.Dir(targetPath), utils.DefaultDirPerms); err != nil {
		return "", fmt.Errorf("failed to create worktree directory: %w", err)
	}

	// Create worktree from base branch
	if !gitSvc.RunCommandChecked(ctx,
		[]string{"git", "worktree", "add", "-b", branchName, targetPath, baseBranch},
		"",
		fmt.Sprintf("Failed to create worktree from issue #%d", issueNumber),
	) {
		return "", fmt.Errorf("failed to create worktree from issue #%d", issueNumber)
	}

	// Run init commands
	if err := runInitCommands(ctx, gitSvc, cfg, branchName, targetPath, silent); err != nil {
		// Clean up the worktree if init commands fail
		gitSvc.RunCommandChecked(ctx, []string{"git", "worktree", "remove", "--force", targetPath}, "", "Failed to cleanup worktree")
		return "", err
	}

	return targetPath, nil
}

// DeleteWorktree deletes a worktree. If worktreePath is empty, lists available worktrees.
func DeleteWorktree(ctx context.Context, gitSvc gitService, cfg *config.AppConfig, worktreePath string, deleteBranch, silent bool) error {
	worktrees, err := gitSvc.GetWorktrees(ctx)
	if err != nil {
		return fmt.Errorf("failed to get worktrees: %w", err)
	}

	// Filter out main worktree
	nonMainWorktrees := make([]*models.WorktreeInfo, 0, len(worktrees))
	for _, wt := range worktrees {
		if !wt.IsMain {
			nonMainWorktrees = append(nonMainWorktrees, wt)
		}
	}

	if len(nonMainWorktrees) == 0 {
		return fmt.Errorf("no worktrees to delete")
	}

	// If no path specified, list available worktrees
	if worktreePath == "" {
		fmt.Fprintf(os.Stderr, "Available worktrees:\n")
		for _, wt := range nonMainWorktrees {
			fmt.Fprintf(os.Stderr, "  %s\n", formatWorktreeForList(wt))
		}
		fmt.Fprintf(os.Stderr, "\nUsage: lazyworktree delete <worktree-name-or-path>\n")
		return nil
	}

	// Find the worktree to delete
	selectedWorktree, err := findWorktreeByPathOrName(worktreePath, nonMainWorktrees, cfg.WorktreeDir, gitSvc.ResolveRepoName(ctx))
	if err != nil {
		return err
	}

	// Run terminate commands
	if err := runTerminateCommands(ctx, gitSvc, cfg, selectedWorktree.Branch, selectedWorktree.Path, silent); err != nil {
		// Log error but continue with deletion
		if !silent {
			fmt.Fprintf(os.Stderr, "Warning: terminate commands failed: %v\n", err)
		}
	}

	// Delete worktree
	if !gitSvc.RunCommandChecked(
		ctx,
		[]string{"git", "worktree", "remove", "--force", selectedWorktree.Path},
		"",
		fmt.Sprintf("Failed to remove worktree %s", selectedWorktree.Path),
	) {
		return fmt.Errorf("failed to remove worktree")
	}

	// Delete branch only if worktree name matches branch name (unless --no-branch was specified)
	if deleteBranch {
		worktreeName := filepath.Base(selectedWorktree.Path)
		if worktreeName == selectedWorktree.Branch {
			if !gitSvc.RunCommandChecked(
				ctx,
				[]string{"git", "branch", "-D", selectedWorktree.Branch},
				"",
				fmt.Sprintf("Failed to delete branch %s", selectedWorktree.Branch),
			) {
				fmt.Fprintf(os.Stderr, "Warning: failed to delete branch %s\n", selectedWorktree.Branch)
			}
		} else if !silent {
			fmt.Fprintf(os.Stderr, "Skipping branch deletion: worktree name %q != branch %q\n", worktreeName, selectedWorktree.Branch)
		}
	}

	return nil
}

// findWorktreeByPathOrName finds a worktree by its path or name.
func findWorktreeByPathOrName(pathOrName string, worktrees []*models.WorktreeInfo, worktreeDir, repoName string) (*models.WorktreeInfo, error) {
	// Try to match by exact path
	for _, wt := range worktrees {
		if wt.Path == pathOrName {
			return wt, nil
		}
	}

	// Try to match by branch name
	for _, wt := range worktrees {
		if wt.Branch == pathOrName {
			return wt, nil
		}
	}

	// Try to construct the path from worktree name and match
	constructedPath := filepath.Join(worktreeDir, repoName, pathOrName)
	for _, wt := range worktrees {
		if wt.Path == constructedPath {
			return wt, nil
		}
	}

	// Try basename match
	for _, wt := range worktrees {
		if filepath.Base(wt.Path) == pathOrName {
			return wt, nil
		}
	}

	return nil, fmt.Errorf("worktree not found: %s", pathOrName)
}

// runInitCommands runs init commands with TOFU trust checks.
func runInitCommands(ctx context.Context, gitSvc gitService, cfg *config.AppConfig, branch, wtPath string, silent bool) error {
	// Collect init commands from global and repo config
	commands := make([]string, 0)
	commands = append(commands, cfg.InitCommands...)

	// Load repo config from main worktree
	mainPath := gitSvc.GetMainWorktreePath(ctx)
	repoConfig, wtFilePath, err := config.LoadRepoConfig(mainPath)
	if err != nil {
		return fmt.Errorf("failed to load repo config: %w", err)
	}

	if repoConfig != nil {
		commands = append(commands, repoConfig.InitCommands...)
	}

	if len(commands) == 0 {
		return nil
	}

	// Check trust for .wt file commands
	if repoConfig != nil && len(repoConfig.InitCommands) > 0 {
		if err := checkTrust(ctx, cfg, wtFilePath); err != nil {
			return err
		}
	}

	// Build environment
	repoName := gitSvc.ResolveRepoName(ctx)
	env := buildCommandEnv(branch, wtPath, mainPath, repoName)

	// Run commands
	if !silent {
		fmt.Fprintf(os.Stderr, "Running init commands...\n")
	}
	if err := gitSvc.ExecuteCommands(ctx, commands, wtPath, env); err != nil {
		return fmt.Errorf("init commands failed: %w", err)
	}

	return nil
}

// runTerminateCommands runs terminate commands with TOFU trust checks.
func runTerminateCommands(ctx context.Context, gitSvc gitService, cfg *config.AppConfig, branch, wtPath string, silent bool) error {
	// Collect terminate commands
	commands := make([]string, 0)
	commands = append(commands, cfg.TerminateCommands...)

	// Load repo config
	mainPath := gitSvc.GetMainWorktreePath(ctx)
	repoConfig, wtFilePath, err := config.LoadRepoConfig(mainPath)
	if err != nil {
		// Don't fail if repo config can't be loaded during deletion
		fmt.Fprintf(os.Stderr, "Warning: failed to load repo config: %v\n", err)
	} else if repoConfig != nil {
		commands = append(commands, repoConfig.TerminateCommands...)
	}

	if len(commands) == 0 {
		return nil
	}

	// Check trust for .wt file commands
	if repoConfig != nil && len(repoConfig.TerminateCommands) > 0 {
		if err := checkTrust(ctx, cfg, wtFilePath); err != nil {
			return err
		}
	}

	// Build environment
	repoName := gitSvc.ResolveRepoName(ctx)
	env := buildCommandEnv(branch, wtPath, mainPath, repoName)

	// Run commands
	if !silent {
		fmt.Fprintf(os.Stderr, "Running terminate commands...\n")
	}
	if err := gitSvc.ExecuteCommands(ctx, commands, wtPath, env); err != nil {
		return fmt.Errorf("terminate commands failed: %w", err)
	}

	return nil
}

// checkTrust verifies TOFU trust for .wt file commands.
func checkTrust(_ context.Context, cfg *config.AppConfig, wtFilePath string) error {
	trustMode := strings.ToLower(cfg.TrustMode)

	// If trust mode is "always", no check needed
	if trustMode == "always" {
		return nil
	}

	// If trust mode is "never", reject
	if trustMode == "never" {
		return fmt.Errorf("trust mode is set to 'never', cannot run commands from .wt file")
	}

	// TOFU mode (default)
	tm := security.NewTrustManager()
	trustStatus := tm.CheckTrust(wtFilePath)

	if trustStatus == security.TrustStatusUntrusted {
		// .wt file is not trusted - this should have been handled before CLI execution
		// For CLI mode, we require manual trust setup via the TUI
		return fmt.Errorf(".wt file is not trusted. Please run lazyworktree in TUI mode to review and trust the file at: %s", wtFilePath)
	}

	return nil
}

// buildCommandEnv builds the environment variables for commands.
func buildCommandEnv(branch, wtPath, mainPath, repoName string) map[string]string {
	return map[string]string{
		"WORKTREE_BRANCH":    branch,
		"MAIN_WORKTREE_PATH": mainPath,
		"WORKTREE_PATH":      wtPath,
		"WORKTREE_NAME":      filepath.Base(wtPath),
		"REPO_NAME":          repoName,
	}
}

// branchExists checks if a branch exists.
func branchExists(ctx context.Context, gitSvc gitService, branch string) bool {
	// Try to verify the branch exists
	output := gitSvc.RunGit(ctx, []string{"git", "rev-parse", "--verify", branch}, "", []int{0}, true, true)
	return strings.TrimSpace(output) != ""
}

// getCurrentWorktreeWithChangesFS returns the current worktree and whether it has uncommitted changes.
func getCurrentWorktreeWithChangesFS(ctx context.Context, gitSvc gitService, fs OSFilesystem) (*models.WorktreeInfo, bool, error) {
	pwd, err := fs.Getwd()
	if err != nil {
		return nil, false, fmt.Errorf("failed to get working directory: %w", err)
	}

	worktrees, err := gitSvc.GetWorktrees(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get worktrees: %w", err)
	}

	// Find current worktree
	var currentWt *models.WorktreeInfo
	for _, wt := range worktrees {
		if pwd == wt.Path || strings.HasPrefix(pwd, wt.Path+string(filepath.Separator)) {
			currentWt = wt
			break
		}
	}

	if currentWt == nil {
		return nil, false, fmt.Errorf("not currently in a worktree")
	}

	// Check for uncommitted changes
	statusOutput := gitSvc.RunGit(ctx, []string{"git", "status", "--porcelain"}, currentWt.Path, []int{0}, true, false)
	hasChanges := strings.TrimSpace(statusOutput) != ""

	return currentWt, hasChanges, nil
}

// createWorktreeWithChanges creates a worktree and carries over uncommitted changes from the current worktree.
func createWorktreeWithChanges(ctx context.Context, gitSvc gitService, cfg *config.AppConfig, currentWt *models.WorktreeInfo, baseBranch, newBranch, targetPath string, silent bool) error {
	if !silent {
		fmt.Fprintf(os.Stderr, "Stashing uncommitted changes...\n")
	}

	// Get previous stash hash to detect if stash creation succeeded
	prevStashHash := strings.TrimSpace(gitSvc.RunGit(ctx, []string{"git", "stash", "list", "-1", "--format=%H"}, "", []int{0}, true, false))

	// Stash changes with descriptive message
	stashMessage := fmt.Sprintf("git-wt-create move-current: %s", newBranch)
	if !gitSvc.RunCommandChecked(
		ctx,
		[]string{"git", "stash", "push", "-u", "-m", stashMessage},
		currentWt.Path,
		"Failed to create stash for moving changes",
	) {
		return fmt.Errorf("failed to create stash for moving changes")
	}

	// Verify stash was created
	newStashHash := strings.TrimSpace(gitSvc.RunGit(ctx, []string{"git", "stash", "list", "-1", "--format=%H"}, "", []int{0}, true, false))
	if newStashHash == "" || newStashHash == prevStashHash {
		return fmt.Errorf("failed to create stash for moving changes: no new entry created")
	}

	// Get the stash reference (run from main repo context, not worktree path, since stashes are stored in main repo)
	stashRef := strings.TrimSpace(gitSvc.RunGit(ctx, []string{"git", "stash", "list", "-1", "--format=%gd"}, "", []int{0}, true, false))
	if stashRef == "" || !strings.HasPrefix(stashRef, "stash@{") {
		// Try to restore stash if we can't get the ref
		gitSvc.RunCommandChecked(ctx, []string{"git", "stash", "pop"}, currentWt.Path, "Failed to restore stash")
		return fmt.Errorf("failed to get stash reference")
	}

	if !silent {
		fmt.Fprintf(os.Stderr, "✓ Changes stashed\n")
	}

	// Create the new worktree from the base branch
	if !gitSvc.RunCommandChecked(
		ctx,
		[]string{"git", "worktree", "add", "-b", newBranch, targetPath, baseBranch},
		"",
		fmt.Sprintf("Failed to create worktree %s", newBranch),
	) {
		// If worktree creation fails, try to restore the stash
		gitSvc.RunCommandChecked(ctx, []string{"git", "stash", "pop"}, currentWt.Path, "Failed to restore stash")
		return fmt.Errorf("failed to create worktree %s", newBranch)
	}

	if !silent {
		fmt.Fprintf(os.Stderr, "✓ Worktree created\n")
	}

	// Apply stash to the new worktree
	if !silent {
		fmt.Fprintf(os.Stderr, "Applying changes to new worktree...\n")
	}
	if !gitSvc.RunCommandChecked(
		ctx,
		[]string{"git", "stash", "apply", "--index", stashRef},
		targetPath,
		"Failed to apply stash to new worktree",
	) {
		// If stash apply fails, clean up the worktree and try to restore stash to original location
		gitSvc.RunCommandChecked(ctx, []string{"git", "worktree", "remove", "--force", targetPath}, "", "Failed to remove worktree")
		gitSvc.RunCommandChecked(ctx, []string{"git", "stash", "pop"}, currentWt.Path, "Failed to restore stash")
		return fmt.Errorf("failed to apply stash to new worktree")
	}

	if !silent {
		fmt.Fprintf(os.Stderr, "✓ Changes applied\n")
	}

	// Drop the stash from the original location
	gitSvc.RunCommandChecked(ctx, []string{"git", "stash", "drop", stashRef}, currentWt.Path, "Failed to drop stash")

	// Run init commands
	if err := runInitCommands(ctx, gitSvc, cfg, newBranch, targetPath, silent); err != nil {
		return err
	}

	return nil
}
