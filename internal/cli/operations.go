package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/git"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/security"
	"github.com/chmouel/lazyworktree/internal/utils"
)

var (
	osStat     = os.Stat
	osMkdirAll = os.MkdirAll
	osGetwd    = os.Getwd
)

type gitService interface {
	CreateWorktreeFromPR(ctx context.Context, prNumber int, branch string, worktreeName string, targetPath string) bool
	ExecuteCommands(ctx context.Context, cmdList []string, cwd string, env map[string]string) error
	FetchAllOpenPRs(ctx context.Context) ([]*models.PRInfo, error)
	GetMainWorktreePath(ctx context.Context) string
	GetWorktrees(ctx context.Context) ([]*models.WorktreeInfo, error)
	ResolveRepoName(ctx context.Context) string
	RunCommandChecked(ctx context.Context, args []string, cwd string, errorMsg string) bool
	RunGit(ctx context.Context, args []string, cwd string, exitCodes []int, silent bool, ignoreErrors bool) string
}

var _ gitService = (*git.Service)(nil)

// CreateFromBranch creates a worktree from a branch name.
func CreateFromBranch(ctx context.Context, gitSvc gitService, cfg *config.AppConfig, branchName string, withChange, silent bool) error {
	// Validate branch exists
	if !branchExists(ctx, gitSvc, branchName) {
		return fmt.Errorf("branch %q does not exist", branchName)
	}

	// Get current worktree if --with-change is specified
	var currentWt *models.WorktreeInfo
	var hasChanges bool
	if withChange {
		var err error
		currentWt, hasChanges, err = getCurrentWorktreeWithChanges(ctx, gitSvc)
		if err != nil {
			return err
		}

		if !hasChanges {
			if !silent {
				fmt.Fprintf(os.Stderr, "No uncommitted changes found, proceeding without --with-change\n")
			}
			withChange = false
		}
	}

	// Use sanitized branch name as worktree name
	worktreeName := utils.SanitizeBranchName(branchName, 100)

	// Construct target path
	repoName := gitSvc.ResolveRepoName(ctx)
	targetPath := filepath.Join(cfg.WorktreeDir, repoName, worktreeName)

	// Check for path conflicts
	if _, err := osStat(targetPath); err == nil {
		return fmt.Errorf("path already exists: %s", targetPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check path %s: %w", targetPath, err)
	}

	// Create parent directory
	if err := osMkdirAll(filepath.Dir(targetPath), utils.DefaultDirPerms); err != nil {
		return fmt.Errorf("failed to create worktree directory: %w", err)
	}

	if !silent {
		fmt.Fprintf(os.Stderr, "\nCreating worktree at: %s\n", targetPath)
	}

	// Create worktree with or without changes
	if withChange && hasChanges && currentWt != nil {
		if err := createWorktreeWithChanges(ctx, gitSvc, cfg, currentWt, branchName, worktreeName, targetPath, silent); err != nil {
			return err
		}
	} else {
		// Create worktree normally
		args := []string{"git", "worktree", "add"}

		// Only use -b flag if we're creating a new branch from a remote or if local branch doesn't exist
		if strings.Contains(branchName, "/") {
			// Remote branch - create new local branch with tracking
			args = append(args, "-b", worktreeName, "--track", targetPath, branchName)
		} else {
			// Local branch - check if it already exists
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
	}

	// Output only the path to stdout
	fmt.Println(targetPath)

	return nil
}

// CreateFromPR creates a worktree from a PR number.
func CreateFromPR(ctx context.Context, gitSvc gitService, cfg *config.AppConfig, prNumber int, silent bool) error {
	if !silent {
		fmt.Fprintf(os.Stderr, "Fetching PR #%d...\n", prNumber)
	}

	// Fetch all PRs to find the specific one
	prs, err := gitSvc.FetchAllOpenPRs(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch PRs: %w", err)
	}

	// Find the PR with the specified number
	var selectedPR *models.PRInfo
	for _, pr := range prs {
		if pr.Number == prNumber {
			selectedPR = pr
			break
		}
	}

	if selectedPR == nil {
		return fmt.Errorf("PR #%d not found (must be an open PR)", prNumber)
	}

	// Generate branch name using template
	template := cfg.PRBranchNameTemplate
	if template == "" {
		template = "pr-{number}-{title}"
	}
	branchName := utils.GeneratePRWorktreeName(selectedPR, template, "")

	// Construct target path
	repoName := gitSvc.ResolveRepoName(ctx)
	targetPath := filepath.Join(cfg.WorktreeDir, repoName, branchName)

	// Check for path conflicts
	if _, err := osStat(targetPath); err == nil {
		return fmt.Errorf("path already exists: %s", targetPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check path %s: %w", targetPath, err)
	}

	// Create parent directory
	if err := osMkdirAll(filepath.Dir(targetPath), utils.DefaultDirPerms); err != nil {
		return fmt.Errorf("failed to create worktree directory: %w", err)
	}

	// Create worktree from PR
	if !gitSvc.CreateWorktreeFromPR(ctx, selectedPR.Number, selectedPR.Branch, branchName, targetPath) {
		return fmt.Errorf("failed to create worktree from PR #%d", selectedPR.Number)
	}

	// Run init commands
	if err := runInitCommands(ctx, gitSvc, cfg, branchName, targetPath, silent); err != nil {
		// Clean up the worktree if init commands fail
		gitSvc.RunCommandChecked(ctx, []string{"git", "worktree", "remove", "--force", targetPath}, "", "Failed to cleanup worktree")
		return err
	}

	// Output only the path to stdout
	fmt.Println(targetPath)

	return nil
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
		fmt.Fprintf(os.Stderr, "\nUsage: lazyworktree wt-delete <worktree-name-or-path>\n")
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

// getCurrentWorktreeWithChanges returns the current worktree and whether it has uncommitted changes.
func getCurrentWorktreeWithChanges(ctx context.Context, gitSvc gitService) (*models.WorktreeInfo, bool, error) {
	pwd, err := osGetwd()
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
	prevStashHash := strings.TrimSpace(gitSvc.RunGit(ctx, []string{"git", "stash", "list", "-1", "--format=%H"}, currentWt.Path, []int{0}, true, false))

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
	newStashHash := strings.TrimSpace(gitSvc.RunGit(ctx, []string{"git", "stash", "list", "-1", "--format=%H"}, currentWt.Path, []int{0}, true, false))
	if newStashHash == "" || newStashHash == prevStashHash {
		return fmt.Errorf("failed to create stash for moving changes: no new entry created")
	}

	// Get the stash reference
	stashRef := strings.TrimSpace(gitSvc.RunGit(ctx, []string{"git", "stash", "list", "-1", "--format=%gd"}, currentWt.Path, []int{0}, true, false))
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
