package main

import (
	"context"
	"fmt"
	"os"

	"github.com/chmouel/lazyworktree/internal/cli"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/git"
	"github.com/chmouel/lazyworktree/internal/log"
)

// handleWtCreate handles the wt-create subcommand.
func handleWtCreate(cmd *WtCreateCmd, worktreeDirFlag, configFileFlag string, configOverrides []string) {
	// Validate mutual exclusivity (Kong's xor should handle this, but we check for clarity)
	if cmd.FromBranch != "" && cmd.FromPR > 0 {
		fmt.Fprintf(os.Stderr, "Error: --from-branch and --from-pr are mutually exclusive\n")
		os.Exit(1)
	}

	if cmd.FromBranch == "" && cmd.FromPR == 0 {
		fmt.Fprintf(os.Stderr, "Error: must specify either --from-branch or --from-pr\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  lazyworktree wt-create --from-branch <branch-name> [worktree-name] [--with-change]\n")
		fmt.Fprintf(os.Stderr, "  lazyworktree wt-create --from-pr <pr-number>\n")
		os.Exit(1)
	}

	// Validate --with-change is only used with --from-branch
	if cmd.WithChange && cmd.FromPR > 0 {
		fmt.Fprintf(os.Stderr, "Error: --with-change can only be used with --from-branch\n")
		os.Exit(1)
	}

	// Validate branch name argument is only used with --from-branch
	if cmd.BranchName != "" && cmd.FromPR > 0 {
		fmt.Fprintf(os.Stderr, "Error: branch name argument cannot be used with --from-pr\n")
		os.Exit(1)
	}

	ctx := context.Background()

	cfg, err := loadCLIConfig(configFileFlag, worktreeDirFlag, configOverrides)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	gitSvc := newCLIGitService(cfg)

	// Execute appropriate operation
	var opErr error
	if cmd.FromBranch != "" {
		opErr = cli.CreateFromBranch(ctx, gitSvc, cfg, cmd.FromBranch, cmd.BranchName, cmd.WithChange, cmd.Silent)
	} else if cmd.FromPR > 0 {
		opErr = cli.CreateFromPR(ctx, gitSvc, cfg, cmd.FromPR, cmd.Silent)
	}

	if opErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", opErr)
		_ = log.Close()
		os.Exit(1)
	}

	_ = log.Close()
}

// handleWtDelete handles the wt-delete subcommand.
func handleWtDelete(cmd *WtDeleteCmd, worktreeDirFlag, configFileFlag string, configOverrides []string) {
	ctx := context.Background()

	cfg, err := loadCLIConfig(configFileFlag, worktreeDirFlag, configOverrides)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	gitSvc := newCLIGitService(cfg)

	// Execute delete operation
	deleteBranch := !cmd.NoBranch
	if err := cli.DeleteWorktree(ctx, gitSvc, cfg, cmd.WorktreePath, deleteBranch, cmd.Silent); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		_ = log.Close()
		os.Exit(1)
	}

	_ = log.Close()
}

func loadCLIConfig(configFileFlag, worktreeDirFlag string, configOverrides []string) (*config.AppConfig, error) {
	cfg, err := config.LoadConfig(configFileFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		cfg = config.DefaultConfig()
	}

	if err := applyWorktreeDirConfig(cfg, worktreeDirFlag); err != nil {
		return nil, err
	}

	if len(configOverrides) > 0 {
		if err := cfg.ApplyCLIOverrides(configOverrides); err != nil {
			return nil, fmt.Errorf("error applying config overrides: %w", err)
		}
	}

	return cfg, nil
}

func newCLIGitService(cfg *config.AppConfig) *git.Service {
	gitSvc := git.NewService(cliNotify, cliNotifyOnce)
	gitSvc.SetGitPager(cfg.GitPager)
	gitSvc.SetGitPagerArgs(cfg.GitPagerArgs)
	return gitSvc
}

func cliNotify(message, severity string) {
	if severity == "error" {
		fmt.Fprintf(os.Stderr, "Error: %s\n", message)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\n", message)
}

func cliNotifyOnce(_, message, severity string) {
	cliNotify(message, severity)
}
