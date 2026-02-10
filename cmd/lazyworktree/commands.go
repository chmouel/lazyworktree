// Package main provides CLI command definitions for lazyworktree.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/chmouel/lazyworktree/internal/cli"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/git"
	"github.com/chmouel/lazyworktree/internal/log"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/utils"
	appiCli "github.com/urfave/cli/v3"
)

type (
	createFromBranchFuncType       func(ctx context.Context, gitSvc *git.Service, cfg *config.AppConfig, branchName, worktreeName string, withChange, silent bool) (string, error)
	createFromPRFuncType           func(ctx context.Context, gitSvc *git.Service, cfg *config.AppConfig, prNumber int, noWorkspace, silent bool) (string, error)
	createFromIssueFuncType        func(ctx context.Context, gitSvc *git.Service, cfg *config.AppConfig, issueNumber int, baseBranch string, noWorkspace, silent bool) (string, error)
	selectIssueInteractiveFuncType func(ctx context.Context, gitSvc *git.Service) (int, error)
	selectPRInteractiveFuncType    func(ctx context.Context, gitSvc *git.Service) (int, error)
)

var (
	loadCLIConfigFunc                             = loadCLIConfig
	newCLIGitServiceFunc                          = newCLIGitService
	createFromBranchFunc createFromBranchFuncType = func(ctx context.Context, gitSvc *git.Service, cfg *config.AppConfig, branchName, worktreeName string, withChange, silent bool) (string, error) {
		return cli.CreateFromBranch(ctx, gitSvc, cfg, branchName, worktreeName, withChange, silent)
	}
	createFromPRFunc createFromPRFuncType = func(ctx context.Context, gitSvc *git.Service, cfg *config.AppConfig, prNumber int, noWorkspace, silent bool) (string, error) {
		return cli.CreateFromPR(ctx, gitSvc, cfg, prNumber, noWorkspace, silent)
	}
	createFromIssueFunc createFromIssueFuncType = func(ctx context.Context, gitSvc *git.Service, cfg *config.AppConfig, issueNumber int, baseBranch string, noWorkspace, silent bool) (string, error) {
		return cli.CreateFromIssue(ctx, gitSvc, cfg, issueNumber, baseBranch, noWorkspace, silent)
	}
	selectIssueInteractiveFunc selectIssueInteractiveFuncType = func(ctx context.Context, gitSvc *git.Service) (int, error) {
		return cli.SelectIssueInteractiveFromStdio(ctx, gitSvc)
	}
	selectPRInteractiveFunc selectPRInteractiveFuncType = func(ctx context.Context, gitSvc *git.Service) (int, error) {
		return cli.SelectPRInteractiveFromStdio(ctx, gitSvc)
	}
	writeOutputSelectionFunc = writeOutputSelection
)

// handleSubcommandCompletion checks if completion is being requested and outputs flags.
// Returns true if completion was handled, false otherwise.
func handleSubcommandCompletion(cmd *appiCli.Command) bool {
	if !slices.Contains(os.Args, "--generate-shell-completion") {
		return false
	}
	outputSubcommandFlags(cmd)
	return true
}

// outputSubcommandFlags prints all visible flags for a subcommand in completion format.
func outputSubcommandFlags(cmd *appiCli.Command) {
	for _, flag := range cmd.Flags {
		if bf, ok := flag.(*appiCli.BoolFlag); ok && bf.Hidden {
			continue
		}
		if sf, ok := flag.(*appiCli.StringFlag); ok && sf.Hidden {
			continue
		}
		name := flag.Names()[0]
		usage := ""
		if df, ok := flag.(appiCli.DocGenerationFlag); ok {
			usage = df.GetUsage()
		}
		prefix := "--"
		if len(name) == 1 {
			prefix = "-"
		}
		if usage != "" {
			fmt.Printf("%s%s:%s\n", prefix, name, usage)
		} else {
			fmt.Printf("%s%s\n", prefix, name)
		}
	}
}

// subcommandShellComplete handles shell completion for subcommands.
// It handles the "--" case by outputting all flags, and filters flags for partial matches.
func subcommandShellComplete(_ context.Context, cmd *appiCli.Command) {
	args := os.Args
	argsLen := len(args)
	lastArg := ""
	if argsLen > 1 {
		lastArg = args[argsLen-2]
	}

	// Handle the "--" case by outputting all flags
	if lastArg == "--" {
		outputSubcommandFlags(cmd)
		return
	}

	// Handle partial flag matches (e.g., --n<TAB>)
	if strings.HasPrefix(lastArg, "-") {
		outputSubcommandFlagsFiltered(cmd, lastArg)
		return
	}

	// Default: output all flags
	outputSubcommandFlags(cmd)
}

// outputSubcommandFlagsFiltered prints flags matching the given prefix.
func outputSubcommandFlagsFiltered(cmd *appiCli.Command, prefix string) {
	for _, flag := range cmd.Flags {
		if bf, ok := flag.(*appiCli.BoolFlag); ok && bf.Hidden {
			continue
		}
		if sf, ok := flag.(*appiCli.StringFlag); ok && sf.Hidden {
			continue
		}
		name := flag.Names()[0]
		usage := ""
		if df, ok := flag.(appiCli.DocGenerationFlag); ok {
			usage = df.GetUsage()
		}
		flagPrefix := "--"
		if len(name) == 1 {
			flagPrefix = "-"
		}
		fullFlag := flagPrefix + name
		if !strings.HasPrefix(fullFlag, prefix) {
			continue
		}
		if usage != "" {
			fmt.Printf("%s:%s\n", fullFlag, usage)
		} else {
			fmt.Printf("%s\n", fullFlag)
		}
	}
}

// createCommand returns the create subcommand definition.
func createCommand() *appiCli.Command {
	return &appiCli.Command{
		Name:    "create",
		Aliases: []string{"wt-create"},
		Usage:   "Create a new worktree",
		Action: func(ctx context.Context, cmd *appiCli.Command) error {
			if handleSubcommandCompletion(cmd) {
				return nil
			}
			if err := validateCreateFlags(ctx, cmd); err != nil {
				return err
			}
			return handleCreateAction(ctx, cmd)
		},
		ShellComplete: subcommandShellComplete,
		Flags: []appiCli.Flag{
			&appiCli.StringFlag{
				Name:  "from-branch",
				Usage: "Create worktree from branch (defaults to current branch)",
			},
			&appiCli.IntFlag{
				Name:  "from-pr",
				Usage: "Create worktree from PR number",
			},
			&appiCli.IntFlag{
				Name:  "from-issue",
				Usage: "Create worktree from issue number",
			},
			&appiCli.BoolFlag{
				Name:    "from-issue-interactive",
				Aliases: []string{"I"},
				Usage:   "Interactively select an issue to create worktree from",
			},
			&appiCli.BoolFlag{
				Name:    "from-pr-interactive",
				Aliases: []string{"P"},
				Usage:   "Interactively select a PR to create worktree from",
			},
			&appiCli.BoolFlag{
				Name:  "generate",
				Usage: "Generate name automatically from the current branch",
			},
			&appiCli.BoolFlag{
				Name:  "with-change",
				Usage: "Carry over uncommitted changes to the new worktree",
			},
			&appiCli.BoolFlag{
				Name:    "no-workspace",
				Aliases: []string{"N"},
				Usage:   "Create local branch and switch to it without creating a worktree (requires --from-pr, --from-pr-interactive, --from-issue, or --from-issue-interactive)",
			},
			&appiCli.BoolFlag{
				Name:  "silent",
				Usage: "Suppress progress messages",
			},
			&appiCli.StringFlag{
				Name:  "output-selection",
				Usage: "Write created worktree path to a file",
			},
		},
	}
}

func deleteCommand() *appiCli.Command {
	return &appiCli.Command{
		Name:      "delete",
		Aliases:   []string{"wt-delete"},
		Usage:     "Delete a worktree",
		ArgsUsage: "[worktree-path]",
		Action: func(ctx context.Context, cmd *appiCli.Command) error {
			if handleSubcommandCompletion(cmd) {
				return nil
			}
			return handleDeleteAction(ctx, cmd)
		},
		ShellComplete: subcommandShellComplete,
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

// validateMutualExclusivity checks that at most one flag in a group is set.
func validateMutualExclusivity(checks map[string]bool, groupName string) error {
	var setFlags []string
	for name, isSet := range checks {
		if isSet {
			setFlags = append(setFlags, name)
		}
	}
	if len(setFlags) > 1 {
		return fmt.Errorf("%s are mutually exclusive: %s", groupName, strings.Join(setFlags, ", "))
	}
	return nil
}

// validateIncompatibility checks that two flags are not both set.
func validateIncompatibility(flag1Name string, flag1Set bool, flag2Name string, flag2Set bool) error {
	if flag1Set && flag2Set {
		return fmt.Errorf("%s cannot be used with %s", flag1Name, flag2Name)
	}
	return nil
}

// validateCreateFlags validates mutual exclusivity rules for the create subcommand.
func validateCreateFlags(ctx context.Context, cmd *appiCli.Command) error {
	fromBranch := cmd.String("from-branch")
	fromPR := cmd.Int("from-pr")
	fromIssue := cmd.Int("from-issue")
	fromIssueInteractive := cmd.Bool("from-issue-interactive")
	fromPRInteractive := cmd.Bool("from-pr-interactive")
	hasName := len(cmd.Args().Slice()) > 0
	generate := cmd.Bool("generate")
	withChange := cmd.Bool("with-change")
	noWorkspace := cmd.Bool("no-workspace")

	if err := validateMutualExclusivity(map[string]bool{
		"--from-pr":                fromPR > 0,
		"--from-issue":             fromIssue > 0,
		"--from-pr-interactive":    fromPRInteractive,
		"--from-issue-interactive": fromIssueInteractive,
	}, "creation mode flags"); err != nil {
		return err
	}

	incompatible := []struct {
		name1 string
		set1  bool
		name2 string
		set2  bool
	}{
		{"--from-branch", fromBranch != "", "--from-pr", fromPR > 0},
		{"--from-branch", fromBranch != "", "--from-pr-interactive", fromPRInteractive},
		{"--generate", generate, "positional name argument", hasName},
		{"--generate", generate, "--from-pr-interactive", fromPRInteractive},
		{"positional name argument", hasName, "--from-pr", fromPR > 0},
		{"positional name argument", hasName, "--from-issue", fromIssue > 0},
		{"positional name argument", hasName, "--from-issue-interactive", fromIssueInteractive},
		{"positional name argument", hasName, "--from-pr-interactive", fromPRInteractive},
	}
	for _, pair := range incompatible {
		if err := validateIncompatibility(pair.name1, pair.set1, pair.name2, pair.set2); err != nil {
			return err
		}
	}

	if withChange {
		if fromPR > 0 || fromIssue > 0 || fromIssueInteractive || fromPRInteractive {
			return fmt.Errorf("--with-change cannot be used with --from-pr, --from-issue, --from-issue-interactive, or --from-pr-interactive")
		}
	}

	if noWorkspace {
		if fromPR == 0 && !fromPRInteractive && fromIssue == 0 && !fromIssueInteractive {
			return fmt.Errorf("--no-workspace requires --from-pr, --from-pr-interactive, --from-issue, or --from-issue-interactive")
		}
		if err := validateIncompatibility("--no-workspace", true, "--with-change", withChange); err != nil {
			return err
		}
		if err := validateIncompatibility("--no-workspace", true, "--generate", generate); err != nil {
			return err
		}
		if err := validateIncompatibility("--no-workspace", true, "positional name argument", hasName); err != nil {
			return err
		}
	}

	return nil
}

// determineBaseBranch resolves the base branch to use, falling back to current branch if needed.
func determineBaseBranch(ctx context.Context, gitSvc *git.Service, fromBranch string) (string, error) {
	if fromBranch != "" {
		return fromBranch, nil
	}
	currentBranch, err := gitSvc.GetCurrentBranch(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Hint: Specify a base branch explicitly with --from-branch\n")
		_ = log.Close()
		return "", err
	}
	return currentBranch, nil
}

// handleCreateAction handles the create subcommand action.
func handleCreateAction(ctx context.Context, cmd *appiCli.Command) error {
	// Load config with global flags
	cfg, err := loadCLIConfigFunc(
		cmd.String("config-file"),
		cmd.String("worktree-dir"),
		cmd.StringSlice("config"),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return err
	}

	gitSvc := newCLIGitServiceFunc(cfg)

	// Extract command-specific flags
	fromPR := cmd.Int("from-pr")
	fromIssue := cmd.Int("from-issue")
	fromIssueInteractive := cmd.Bool("from-issue-interactive")
	fromPRInteractive := cmd.Bool("from-pr-interactive")
	fromBranch := cmd.String("from-branch")
	generate := cmd.Bool("generate")
	withChange := cmd.Bool("with-change")
	noWorkspace := cmd.Bool("no-workspace")
	silent := cmd.Bool("silent")

	// Get name from positional argument if provided
	var name string
	if len(cmd.Args().Slice()) > 0 && !generate {
		name = cmd.Args().Get(0)
	}

	var (
		opErr      error
		outputPath string
	)
	switch {
	case fromPR > 0:
		outputPath, opErr = createFromPRFunc(ctx, gitSvc, cfg, fromPR, noWorkspace, silent)
	case fromPRInteractive:
		prNumber, err := selectPRInteractiveFunc(ctx, gitSvc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			_ = log.Close()
			return err
		}
		outputPath, opErr = createFromPRFunc(ctx, gitSvc, cfg, prNumber, noWorkspace, silent)
	case fromIssueInteractive:
		issueNumber, err := selectIssueInteractiveFunc(ctx, gitSvc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			_ = log.Close()
			return err
		}
		baseBranch, err := determineBaseBranch(ctx, gitSvc, fromBranch)
		if err != nil {
			return err
		}
		outputPath, opErr = createFromIssueFunc(ctx, gitSvc, cfg, issueNumber, baseBranch, noWorkspace, silent)
	case fromIssue > 0:
		baseBranch, err := determineBaseBranch(ctx, gitSvc, fromBranch)
		if err != nil {
			return err
		}
		outputPath, opErr = createFromIssueFunc(ctx, gitSvc, cfg, fromIssue, baseBranch, noWorkspace, silent)
	default:
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

		outputPath, opErr = createFromBranchFunc(ctx, gitSvc, cfg, sourceBranch, name, withChange, silent)
	}

	if opErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", opErr)
		_ = log.Close()
		return opErr
	}

	if outputSelection := cmd.String("output-selection"); outputSelection != "" {
		if err := writeOutputSelectionFunc(outputSelection, outputPath); err != nil {
			_ = log.Close()
			return err
		}
		_ = log.Close()
		return nil
	}

	if outputPath != "" {
		fmt.Println(outputPath)
	}

	_ = log.Close()
	return nil
}

func writeOutputSelection(outputSelection, outputPath string) error {
	expanded, err := utils.ExpandPath(outputSelection)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error expanding output-selection: %v\n", err)
		return err
	}
	const defaultDirPerms = 0o750
	if err := os.MkdirAll(filepath.Dir(expanded), defaultDirPerms); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output-selection dir: %v\n", err)
		return err
	}
	const defaultFilePerms = 0o600
	data := outputPath + "\n"
	if err := os.WriteFile(expanded, []byte(data), defaultFilePerms); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output-selection: %v\n", err)
		return err
	}
	return nil
}

func listCommand() *appiCli.Command {
	return &appiCli.Command{
		Name:    "list",
		Aliases: []string{"ls"},
		Usage:   "List all worktrees",
		Action: func(ctx context.Context, cmd *appiCli.Command) error {
			if handleSubcommandCompletion(cmd) {
				return nil
			}
			return handleListAction(ctx, cmd)
		},
		ShellComplete: subcommandShellComplete,
		Flags: []appiCli.Flag{
			&appiCli.BoolFlag{
				Name:    "pristine",
				Aliases: []string{"p"},
				Usage:   "Output paths only (one per line, suitable for scripting)",
			},
			&appiCli.BoolFlag{
				Name:  "json",
				Usage: "Output as JSON",
			},
		},
	}
}

func validateListFlags(cmd *appiCli.Command) error {
	pristine := cmd.Bool("pristine")
	jsonOutput := cmd.Bool("json")
	if pristine && jsonOutput {
		return fmt.Errorf("--pristine and --json are mutually exclusive")
	}
	return nil
}

func sortWorktreesByPath(worktrees []*models.WorktreeInfo) {
	slices.SortFunc(worktrees, func(a, b *models.WorktreeInfo) int {
		return strings.Compare(a.Path, b.Path)
	})
}

// worktreeJSON represents the JSON output format for a worktree.
type worktreeJSON struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	Branch     string `json:"branch"`
	IsMain     bool   `json:"is_main"`
	Dirty      bool   `json:"dirty"`
	Ahead      int    `json:"ahead"`
	Behind     int    `json:"behind"`
	Unpushed   int    `json:"unpushed,omitempty"`
	LastActive string `json:"last_active"`
}

// handleListAction handles the list subcommand action.
func handleListAction(ctx context.Context, cmd *appiCli.Command) error {
	defer func() {
		_ = log.Close()
	}()
	if err := validateListFlags(cmd); err != nil {
		return err
	}
	cfg, err := loadCLIConfigFunc(
		cmd.String("config-file"),
		cmd.String("worktree-dir"),
		cmd.StringSlice("config"),
	)
	if err != nil {
		return err
	}

	gitSvc := newCLIGitServiceFunc(cfg)

	worktrees, err := gitSvc.GetWorktrees(ctx)
	if err != nil {
		return err
	}

	sortWorktreesByPath(worktrees)

	pristine := cmd.Bool("pristine")
	jsonOutput := cmd.Bool("json")

	if jsonOutput {
		return outputListJSON(worktrees)
	}

	if pristine {
		// Simple path output for scripting
		for _, wt := range worktrees {
			fmt.Println(wt.Path)
		}
		return nil
	}

	// Default: verbose table output
	return outputListVerbose(worktrees)
}

// outputListJSON outputs worktrees as JSON.
func outputListJSON(worktrees []*models.WorktreeInfo) error {
	output := make([]worktreeJSON, 0, len(worktrees))
	for _, wt := range worktrees {
		name := filepath.Base(wt.Path)
		output = append(output, worktreeJSON{
			Path:       wt.Path,
			Name:       name,
			Branch:     wt.Branch,
			IsMain:     wt.IsMain,
			Dirty:      wt.Dirty,
			Ahead:      wt.Ahead,
			Behind:     wt.Behind,
			Unpushed:   wt.Unpushed,
			LastActive: wt.LastActive,
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		return err
	}

	return nil
}

// outputListVerbose outputs worktrees in a formatted table.
func outputListVerbose(worktrees []*models.WorktreeInfo) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tBRANCH\tSTATUS\tLAST ACTIVE\tPATH")

	for _, wt := range worktrees {
		name := filepath.Base(wt.Path)
		status := buildStatusString(wt)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, wt.Branch, status, wt.LastActive, wt.Path)
	}

	if err := w.Flush(); err != nil {
		return err
	}

	return nil
}

// buildStatusString creates a status indicator string for a worktree.
func buildStatusString(wt *models.WorktreeInfo) string {
	var parts []string

	if wt.Dirty {
		parts = append(parts, "~")
	} else {
		parts = append(parts, "✓")
	}

	if wt.Behind > 0 {
		parts = append(parts, fmt.Sprintf("↓%d", wt.Behind))
	}
	if wt.Ahead > 0 {
		parts = append(parts, fmt.Sprintf("↑%d", wt.Ahead))
	}
	if !wt.HasUpstream && wt.Unpushed > 0 {
		parts = append(parts, fmt.Sprintf("?%d", wt.Unpushed))
	}

	return strings.Join(parts, "")
}

// handleDeleteAction handles the delete subcommand action.
func handleDeleteAction(ctx context.Context, cmd *appiCli.Command) error {
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
