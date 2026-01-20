// Package main provides CLI command definitions for lazyworktree.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/chmouel/lazyworktree/internal/cli"
	"github.com/chmouel/lazyworktree/internal/log"
	appiCli "github.com/urfave/cli/v3"
)

// wtCreateCommand returns the wt-create subcommand definition.
func wtCreateCommand() *appiCli.Command {
	return &appiCli.Command{
		Name:  "wt-create",
		Usage: "Create a new worktree",
		Action: func(ctx context.Context, cmd *appiCli.Command) error {
			if err := validateWtCreateFlags(ctx, cmd); err != nil {
				return err
			}
			return handleWtCreateAction(ctx, cmd)
		},
		Flags: []appiCli.Flag{
			&appiCli.StringFlag{
				Name:  "from-branch",
				Usage: "Create worktree from branch (defaults to current branch)",
			},
			&appiCli.IntFlag{
				Name:  "from-pr",
				Usage: "Create worktree from PR number",
			},
			&appiCli.StringFlag{
				Name:  "name",
				Usage: "Name for the new worktree/branch (defaults to sanitised source branch name)",
			},
			&appiCli.BoolFlag{
				Name:  "with-change",
				Usage: "Carry over uncommitted changes to the new worktree",
			},
			&appiCli.BoolFlag{
				Name:  "silent",
				Usage: "Suppress progress messages",
			},
		},
	}
}

func wtDeleteCommand() *appiCli.Command {
	return &appiCli.Command{
		Name:      "wt-delete",
		Usage:     "Delete a worktree",
		ArgsUsage: "[worktree-path]",
		Action:    handleWtDeleteAction,
		Flags: []appiCli.Flag{
			&appiCli.BoolFlag{
				Name:  "no-branch",
				Usage: "Skip branch deletion",
			},
			&appiCli.BoolFlag{
				Name:  "silent",
				Usage: "Suppress progress messages",
			},
		},
	}
}

// validateWtCreateFlags validates mutual exclusivity rules for wt-create flags.
func validateWtCreateFlags(ctx context.Context, cmd *appiCli.Command) error {
	fromBranch := cmd.String("from-branch")
	fromPR := cmd.Int("from-pr")
	name := cmd.String("name")
	withChange := cmd.Bool("with-change")

	// Mutual exclusivity: from-branch and from-pr
	if fromBranch != "" && fromPR > 0 {
		return fmt.Errorf("--from-branch and --from-pr are mutually exclusive")
	}

	// Name cannot be used with from-pr
	if name != "" && fromPR > 0 {
		return fmt.Errorf("--name cannot be used with --from-pr")
	}

	// with-change cannot be used with from-pr
	if withChange && fromPR > 0 {
		return fmt.Errorf("--with-change cannot be used with --from-pr")
	}

	return nil
}

// handleWtCreateAction handles the wt-create subcommand action.
func handleWtCreateAction(ctx context.Context, cmd *appiCli.Command) error {
	// Load config with global flags
	cfg, err := loadCLIConfig(
		cmd.String("config-file"),
		cmd.String("worktree-dir"),
		cmd.StringSlice("config"),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return err
	}

	gitSvc := newCLIGitService(cfg)

	// Extract command-specific flags
	fromPR := cmd.Int("from-pr")
	fromBranch := cmd.String("from-branch")
	name := cmd.String("name")
	withChange := cmd.Bool("with-change")
	silent := cmd.Bool("silent")

	// Execute appropriate operation
	var opErr error
	if fromPR > 0 {
		// Create from PR
		opErr = cli.CreateFromPR(ctx, gitSvc, cfg, fromPR, silent)
	} else {
		// Create from branch (either specified or current)
		sourceBranch := fromBranch

		// If no branch specified, use current branch
		if sourceBranch == "" {
			currentBranch, err := gitSvc.GetCurrentBranch(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				fmt.Fprintf(os.Stderr, "Hint: Specify a branch explicitly with --from-branch\n")
				_ = log.Close()
				return err
			}
			sourceBranch = currentBranch

			if !silent {
				fmt.Fprintf(os.Stderr, "Creating worktree from current branch: %s\n", sourceBranch)
			}
		}

		opErr = cli.CreateFromBranch(ctx, gitSvc, cfg, sourceBranch, name, withChange, silent)
	}

	if opErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", opErr)
		_ = log.Close()
		return opErr
	}

	_ = log.Close()
	return nil
}

// handleWtDeleteAction handles the wt-delete subcommand action.
func handleWtDeleteAction(ctx context.Context, cmd *appiCli.Command) error {
	// Load config with global flags
	cfg, err := loadCLIConfig(
		cmd.String("config-file"),
		cmd.String("worktree-dir"),
		cmd.StringSlice("config"),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return err
	}

	gitSvc := newCLIGitService(cfg)

	// Get worktree path from args
	worktreePath := ""
	if cmd.NArg() > 0 {
		worktreePath = cmd.Args().Get(0)
	}

	// Extract command-specific flags
	noBranch := cmd.Bool("no-branch")
	silent := cmd.Bool("silent")

	// Execute delete operation
	deleteBranch := !noBranch
	if err := cli.DeleteWorktree(ctx, gitSvc, cfg, worktreePath, deleteBranch, silent); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		_ = log.Close()
		return err
	}

	_ = log.Close()
	return nil
}
