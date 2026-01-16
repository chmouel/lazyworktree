package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/chmouel/lazyworktree/internal/cli"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/git"
	"github.com/chmouel/lazyworktree/internal/log"
)

// handleWtCreate handles the wt-create subcommand.
func handleWtCreate(args []string, worktreeDirFlag, configFileFlag string, configOverrides configOverrides) {
	fs := flag.NewFlagSet("wt-create", flag.ExitOnError)
	fromBranch := fs.String("from-branch", "", "Create worktree from branch")
	fromPR := fs.Int("from-pr", 0, "Create worktree from PR number")
	withChange := fs.Bool("with-change", false, "Carry over uncommitted changes to the new worktree (only with --from-branch)")
	silent := fs.Bool("silent", false, "Suppress progress messages")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Validate mutual exclusivity
	if *fromBranch != "" && *fromPR > 0 {
		fmt.Fprintf(os.Stderr, "Error: --from-branch and --from-pr are mutually exclusive\n")
		os.Exit(1)
	}

	if *fromBranch == "" && *fromPR == 0 {
		fmt.Fprintf(os.Stderr, "Error: must specify either --from-branch or --from-pr\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  lazyworktree wt-create --from-branch <branch-name> [--with-change]\n")
		fmt.Fprintf(os.Stderr, "  lazyworktree wt-create --from-pr <pr-number>\n")
		os.Exit(1)
	}

	// Validate --with-change is only used with --from-branch
	if *withChange && *fromPR > 0 {
		fmt.Fprintf(os.Stderr, "Error: --with-change can only be used with --from-branch\n")
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
	if *fromBranch != "" {
		opErr = cli.CreateFromBranch(ctx, gitSvc, cfg, *fromBranch, *withChange, *silent)
	} else if *fromPR > 0 {
		opErr = cli.CreateFromPR(ctx, gitSvc, cfg, *fromPR, *silent)
	}

	if opErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", opErr)
		_ = log.Close()
		os.Exit(1)
	}

	_ = log.Close()
}

// handleWtDelete handles the wt-delete subcommand.
func handleWtDelete(args []string, worktreeDirFlag, configFileFlag string, configOverrides configOverrides) {
	fs := flag.NewFlagSet("wt-delete", flag.ExitOnError)
	noBranch := fs.Bool("no-branch", false, "Skip branch deletion")
	silent := fs.Bool("silent", false, "Suppress progress messages")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Get optional positional argument (worktree path/name)
	var worktreePath string
	if len(fs.Args()) > 0 {
		worktreePath = fs.Args()[0]
	}

	ctx := context.Background()

	cfg, err := loadCLIConfig(configFileFlag, worktreeDirFlag, configOverrides)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	gitSvc := newCLIGitService(cfg)

	// Execute delete operation
	deleteBranch := !*noBranch
	if err := cli.DeleteWorktree(ctx, gitSvc, cfg, worktreePath, deleteBranch, *silent); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		_ = log.Close()
		os.Exit(1)
	}

	_ = log.Close()
}

func loadCLIConfig(configFileFlag, worktreeDirFlag string, configOverrides configOverrides) (*config.AppConfig, error) {
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
