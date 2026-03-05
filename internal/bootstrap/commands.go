// Package bootstrap provides CLI command definitions for lazyworktree.
package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/cli"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/git"
	"github.com/chmouel/lazyworktree/internal/log"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/multiplexer"
	"github.com/chmouel/lazyworktree/internal/utils"
	appiCli "github.com/urfave/cli/v3"
)

type (
	createFromBranchFuncType       func(ctx context.Context, gitSvc *git.Service, cfg *config.AppConfig, branchName, worktreeName string, withChange, silent bool) (string, error)
	createFromPRFuncType           func(ctx context.Context, gitSvc *git.Service, cfg *config.AppConfig, prNumber int, noWorkspace, silent bool) (string, error)
	createFromIssueFuncType        func(ctx context.Context, gitSvc *git.Service, cfg *config.AppConfig, issueNumber int, baseBranch string, noWorkspace, silent bool) (string, error)
	renameWorktreeFuncType         func(ctx context.Context, gitSvc *git.Service, cfg *config.AppConfig, worktreePath, newName string, silent bool) error
	selectIssueInteractiveFuncType func(ctx context.Context, gitSvc *git.Service, query string) (int, error)
	selectPRInteractiveFuncType    func(ctx context.Context, gitSvc *git.Service, query string) (int, error)
	runCreateExecFuncType          func(ctx context.Context, command, cwd string) error
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
	renameWorktreeFunc renameWorktreeFuncType = func(ctx context.Context, gitSvc *git.Service, cfg *config.AppConfig, worktreePath, newName string, silent bool) error {
		return cli.RenameWorktree(ctx, gitSvc, cfg, worktreePath, newName, silent)
	}
	selectIssueInteractiveFunc selectIssueInteractiveFuncType = func(ctx context.Context, gitSvc *git.Service, query string) (int, error) {
		return cli.SelectIssueInteractiveFromStdio(ctx, gitSvc, query)
	}
	selectPRInteractiveFunc selectPRInteractiveFuncType = func(ctx context.Context, gitSvc *git.Service, query string) (int, error) {
		return cli.SelectPRInteractiveFromStdio(ctx, gitSvc, query)
	}
	writeOutputSelectionFunc                       = writeOutputSelection
	runCreateExecFunc        runCreateExecFuncType = runCreateExec
)

// handleSubcommandCompletion checks if completion is being requested and outputs flags.
// Returns true if completion was handled, false otherwise.
func handleSubcommandCompletion(ctx context.Context, cmd *appiCli.Command) bool {
	if !slices.Contains(os.Args, "--generate-shell-completion") {
		return false
	}
	outputSubcommandCompletions(ctx, cmd)
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

var listSubcommandWorktreeNamesFunc = listSubcommandWorktreeNames

// outputSubcommandCompletions routes shell completion output for subcommands.
func outputSubcommandCompletions(ctx context.Context, cmd *appiCli.Command) {
	lastArg := completionTokenFromArgs(os.Args)

	// Handle the "--" case by outputting all flags
	if lastArg == "--" {
		outputSubcommandFlags(cmd)
		return
	}

	// Handle partial flag matches (e.g., --n<TAB>)
	if strings.HasPrefix(lastArg, "-") && !isExactSubcommandFlag(cmd, lastArg) {
		outputSubcommandFlagsFiltered(cmd, lastArg)
		return
	}

	if names := completionWorktreeBasenames(ctx, cmd); len(names) > 0 {
		outputCompletionLines(names)
		return
	}

	// Default: output all flags
	outputSubcommandFlags(cmd)
}

func completionTokenFromArgs(args []string) string {
	if len(args) <= 1 {
		return ""
	}
	return args[len(args)-2]
}

func isExactSubcommandFlag(cmd *appiCli.Command, candidate string) bool {
	for _, flag := range cmd.Flags {
		for _, name := range flag.Names() {
			prefix := "--"
			if len(name) == 1 {
				prefix = "-"
			}
			if candidate == prefix+name {
				return true
			}
		}
	}
	return false
}

func completionWorktreeBasenames(ctx context.Context, cmd *appiCli.Command) []string {
	// For exec command, complete anytime (handled by custom ShellComplete)
	if cmd.Name == "exec" {
		return listSubcommandWorktreeNamesFunc(ctx, cmd)
	}

	if (cmd.Name != "delete" && cmd.Name != "rename") || cmd.NArg() != 0 {
		return nil
	}

	return listSubcommandWorktreeNamesFunc(ctx, cmd)
}

func outputCompletionLines(lines []string) {
	for _, line := range lines {
		fmt.Println(line)
	}
}

func listSubcommandWorktreeNames(ctx context.Context, cmd *appiCli.Command) []string {
	cfg, err := loadCompletionConfig(cmd)
	if err != nil {
		return nil
	}

	gitSvc := newCLIGitServiceFunc(cfg)
	worktrees, err := gitSvc.GetWorktrees(ctx)
	if err != nil {
		return nil
	}

	return uniqueSortedWorktreeBasenames(worktrees)
}

func loadCompletionConfig(cmd *appiCli.Command) (*config.AppConfig, error) {
	cfg, err := config.LoadConfig(cmd.String("config-file"))
	if err != nil {
		cfg = config.DefaultConfig()
	}

	if err := applyWorktreeDirConfig(cfg, cmd.String("worktree-dir")); err != nil {
		return nil, err
	}

	if configOverrides := cmd.StringSlice("config"); len(configOverrides) > 0 {
		if err := cfg.ApplyCLIOverrides(configOverrides); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func uniqueSortedWorktreeBasenames(worktrees []*models.WorktreeInfo) []string {
	seen := make(map[string]struct{}, len(worktrees))
	names := make([]string, 0, len(worktrees))

	for _, wt := range worktrees {
		if wt == nil || wt.IsMain {
			continue
		}

		name := filepath.Base(strings.TrimSpace(wt.Path))
		if name == "" || name == "." || name == string(filepath.Separator) {
			continue
		}

		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	slices.Sort(names)
	return names
}

// subcommandShellComplete handles shell completion for subcommands.
func subcommandShellComplete(ctx context.Context, cmd *appiCli.Command) {
	outputSubcommandCompletions(ctx, cmd)
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
		Name:  "create",
		Usage: "Create a new worktree",
		Action: func(ctx context.Context, cmd *appiCli.Command) error {
			if handleSubcommandCompletion(ctx, cmd) {
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
			&appiCli.StringFlag{
				Aliases: []string{"x"},
				Name:    "exec",
				Usage:   "Run a shell command after creation (in the created worktree, or current directory with --no-workspace)",
			},
			&appiCli.StringFlag{
				Name:    "query",
				Aliases: []string{"q"},
				Usage:   "Pre-filter interactive selection (pre-fills fzf search or filters numbered list); requires --from-pr-interactive or --from-issue-interactive",
			},
		},
	}
}

func deleteCommand() *appiCli.Command {
	return &appiCli.Command{
		Name:      "delete",
		Usage:     "Delete a worktree",
		ArgsUsage: "[worktree-path]",
		Action: func(ctx context.Context, cmd *appiCli.Command) error {
			if handleSubcommandCompletion(ctx, cmd) {
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

func renameCommand() *appiCli.Command {
	return &appiCli.Command{
		Name:      "rename",
		Usage:     "Rename a worktree",
		ArgsUsage: "<new-name> | <worktree> <new-name>",
		Action: func(ctx context.Context, cmd *appiCli.Command) error {
			if handleSubcommandCompletion(ctx, cmd) {
				return nil
			}
			return handleRenameAction(ctx, cmd)
		},
		ShellComplete: subcommandShellComplete,
		Flags: []appiCli.Flag{
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

	query := cmd.String("query")
	if query != "" && !fromPRInteractive && !fromIssueInteractive {
		return fmt.Errorf("--query requires --from-pr-interactive or --from-issue-interactive")
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
	execCommand := strings.TrimSpace(cmd.String("exec"))
	query := cmd.String("query")

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
		prNumber, err := selectPRInteractiveFunc(ctx, gitSvc, query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			_ = log.Close()
			return err
		}
		outputPath, opErr = createFromPRFunc(ctx, gitSvc, cfg, prNumber, noWorkspace, silent)
	case fromIssueInteractive:
		issueNumber, err := selectIssueInteractiveFunc(ctx, gitSvc, query)
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

	if execCommand != "" {
		execCWD, err := resolveCreateExecCWD(outputPath, noWorkspace)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			_ = log.Close()
			return err
		}
		if err := runCreateExecFunc(ctx, execCommand, execCWD); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			_ = log.Close()
			return err
		}
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

func resolveCreateExecCWD(outputPath string, noWorkspace bool) (string, error) {
	if noWorkspace {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to determine current directory for --exec: %w", err)
		}
		return cwd, nil
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to access created worktree for --exec: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("created worktree path is not a directory: %s", outputPath)
	}

	return outputPath, nil
}

func runCreateExec(ctx context.Context, command, cwd string) error {
	shellPath, shellArgs := shellInvocationForExec(command)
	// #nosec G204 -- --exec is an explicit user-provided command executed by request
	execCmd := exec.CommandContext(ctx, shellPath, shellArgs...)
	execCmd.Dir = cwd
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("--exec command failed: %w", err)
	}

	return nil
}

func shellInvocationForExec(command string) (string, []string) {
	shellPath := strings.TrimSpace(os.Getenv("SHELL"))
	if shellPath == "" {
		return "bash", []string{"-lc", command}
	}

	switch strings.ToLower(filepath.Base(shellPath)) {
	case "zsh":
		return shellPath, []string{"-ilc", command}
	case "bash":
		return shellPath, []string{"-ic", command}
	default:
		return shellPath, []string{"-lc", command}
	}
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
			if handleSubcommandCompletion(ctx, cmd) {
				return nil
			}
			return handleListAction(ctx, cmd)
		},
		ShellComplete: subcommandShellComplete,
		Flags: []appiCli.Flag{
			&appiCli.BoolFlag{
				Name:    "main",
				Aliases: []string{"m"},
				Usage:   "Show only the main branch worktree",
			},
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

	main := cmd.Bool("main")
	pristine := cmd.Bool("pristine")
	jsonOutput := cmd.Bool("json")

	// Filter to main worktree if --main flag is set
	if main {
		filtered := make([]*models.WorktreeInfo, 0, 1)
		for _, wt := range worktrees {
			if wt.IsMain {
				filtered = append(filtered, wt)
				break
			}
		}
		worktrees = filtered
	}

	if jsonOutput {
		return outputListJSON(worktrees, main)
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
func outputListJSON(worktrees []*models.WorktreeInfo, mainOnly bool) error {
	// If --main flag is set and we have exactly one worktree, output just the object
	if mainOnly && len(worktrees) == 1 {
		wt := worktrees[0]
		name := filepath.Base(wt.Path)
		output := worktreeJSON{
			Path:       wt.Path,
			Name:       name,
			Branch:     wt.Branch,
			IsMain:     wt.IsMain,
			Dirty:      wt.Dirty,
			Ahead:      wt.Ahead,
			Behind:     wt.Behind,
			Unpushed:   wt.Unpushed,
			LastActive: wt.LastActive,
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(output); err != nil {
			return err
		}
		return nil
	}

	// Otherwise output as array
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

// handleRenameAction handles the rename subcommand action.
func handleRenameAction(ctx context.Context, cmd *appiCli.Command) error {
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

	if cmd.NArg() > 2 {
		err := fmt.Errorf("too many arguments: expected <worktree-name-or-path> <new-name>")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		_ = log.Close()
		return err
	}

	worktreePath := ""
	newName := ""
	switch {
	case cmd.NArg() == 1:
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			_ = log.Close()
			return fmt.Errorf("failed to determine current directory: %w", err)
		}
		worktreePath = cwd
		newName = cmd.Args().Get(0)
	case cmd.NArg() == 2:
		worktreePath = cmd.Args().Get(0)
		newName = cmd.Args().Get(1)
	}

	silent := cmd.Bool("silent")
	if err := renameWorktreeFunc(ctx, gitSvc, cfg, worktreePath, newName, silent); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		_ = log.Close()
		return err
	}

	_ = log.Close()
	return nil
}

func execCommand() *appiCli.Command {
	return &appiCli.Command{
		Name:      "exec",
		Usage:     "Run a command or trigger a key action in a worktree",
		ArgsUsage: "[command]",
		Action: func(ctx context.Context, cmd *appiCli.Command) error {
			if handleSubcommandCompletion(ctx, cmd) {
				return nil
			}
			return handleExecAction(ctx, cmd)
		},
		ShellComplete: func(ctx context.Context, cmd *appiCli.Command) {
			// Handle workspace flag completion
			if len(os.Args) >= 2 {
				prevArg := os.Args[len(os.Args)-2]
				if prevArg == "--workspace" || prevArg == "-w" {
					completionWorktreeBasenames(ctx, cmd)
					return
				}
			}
			subcommandShellComplete(ctx, cmd)
		},
		Flags: []appiCli.Flag{
			&appiCli.StringFlag{
				Name:    "workspace",
				Aliases: []string{"w"},
				Usage:   "Target worktree name or path",
			},
			&appiCli.StringFlag{
				Name:    "key",
				Aliases: []string{"k"},
				Usage:   "Custom command key to trigger (e.g. 't' for tmux)",
			},
		},
	}
}

func handleExecAction(ctx context.Context, cmd *appiCli.Command) error {
	key := cmd.String("key")
	workspace := cmd.String("workspace")
	var command string
	if cmd.NArg() > 0 {
		command = cmd.Args().Get(0)
	}

	// Validate: key and command are mutually exclusive
	if key != "" && command != "" {
		fmt.Fprintf(os.Stderr, "Error: --key and command argument are mutually exclusive\n")
		return fmt.Errorf("--key and command argument are mutually exclusive")
	}
	if key == "" && command == "" {
		fmt.Fprintf(os.Stderr, "Error: either --key or command argument is required\n")
		return fmt.Errorf("either --key or command argument is required")
	}

	// Load config
	cfg, err := loadCLIConfigFunc(
		cmd.String("config-file"),
		cmd.String("worktree-dir"),
		cmd.StringSlice("config-override"),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		return err
	}

	// Create git service
	gitSvc := newCLIGitServiceFunc(cfg)

	// Get worktrees
	worktrees, err := gitSvc.GetWorktrees(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting worktrees: %v\n", err)
		return err
	}

	// Resolve target worktree
	var targetWorktree *models.WorktreeInfo
	if workspace != "" {
		// User provided workspace flag
		targetWorktree, err = cli.FindWorktreeByPathOrName(workspace, worktrees, cfg.WorktreeDir, gitSvc.ResolveRepoName(ctx))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
	} else {
		// Auto-detect from cwd
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
			return err
		}
		for _, wt := range worktrees {
			if strings.HasPrefix(cwd, wt.Path) {
				targetWorktree = wt
				break
			}
		}
		if targetWorktree == nil {
			fmt.Fprintf(os.Stderr, "Error: could not auto-detect worktree from current directory\n")
			fmt.Fprintf(os.Stderr, "Use --workspace to specify a worktree\n")
			return fmt.Errorf("could not auto-detect worktree")
		}
	}

	// Get main worktree path
	var mainWorktreePath string
	for _, wt := range worktrees {
		if wt.IsMain {
			mainWorktreePath = wt.Path
			break
		}
	}

	// Build environment variables
	env := services.BuildCommandEnv(targetWorktree.Branch, targetWorktree.Path, gitSvc.ResolveRepoName(ctx), mainWorktreePath)

	// Execute command or key action
	if command != "" {
		// Command mode - run shell command
		return executeShellCommand(ctx, command, targetWorktree.Path, env)
	}

	// Key mode - trigger custom command
	return executeKeyAction(ctx, key, cfg, targetWorktree, env)
}

func executeShellCommand(ctx context.Context, command, cwd string, env map[string]string) error {
	shellPath, shellArgs := shellInvocationForExec(command)
	// #nosec G204 -- explicit user-provided command executed by request
	execCmd := exec.CommandContext(ctx, shellPath, shellArgs...)
	execCmd.Dir = cwd
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Env = append(os.Environ(), services.EnvMapToList(env)...)

	if err := execCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: command failed: %v\n", err)
		return err
	}
	return nil
}

func executeKeyAction(ctx context.Context, key string, cfg *config.AppConfig, wt *models.WorktreeInfo, env map[string]string) error {
	customCmd, exists := cfg.CustomCommands[key]
	if !exists {
		fmt.Fprintf(os.Stderr, "Error: custom command key %q not found in config\n", key)
		return fmt.Errorf("custom command key %q not found", key)
	}

	// Determine command type based on config fields
	if customCmd.Tmux != nil {
		return executeTmuxAction(ctx, customCmd.Tmux, customCmd.Container, cfg, wt, env)
	}

	if customCmd.Zellij != nil {
		return executeZellijAction(ctx, customCmd.Zellij, customCmd.Container, cfg, wt, env)
	}

	if customCmd.NewTab {
		fmt.Fprintf(os.Stderr, "Error: new-tab commands are not supported in CLI mode\n")
		return fmt.Errorf("new-tab commands are not supported in CLI mode")
	}

	if customCmd.ShowOutput {
		command := customCmd.Command
		if customCmd.Container != nil && (command != "" || customCmd.Container.Entrypoint != "") {
			var err error
			command, err = multiplexer.BuildContainerCommand(customCmd.Container, command, wt.Path, env, false)
			if err != nil {
				return err
			}
		}
		return executeShowOutputAction(ctx, command, cfg, wt, env)
	}

	// Default: shell command
	command := customCmd.Command
	if customCmd.Container != nil && (command != "" || customCmd.Container.Entrypoint != "") {
		var err error
		command, err = multiplexer.BuildContainerCommand(customCmd.Container, command, wt.Path, env, true)
		if err != nil {
			return err
		}
	}
	return executeShellCommand(ctx, command, wt.Path, env)
}

func executeTmuxAction(ctx context.Context, tmuxCfg *config.TmuxCommand, containerCfg *config.ContainerCommand, cfg *config.AppConfig, wt *models.WorktreeInfo, env map[string]string) error {
	if tmuxCfg == nil {
		return fmt.Errorf("tmux configuration is nil")
	}

	// Expand and sanitise session name
	sessionName := strings.TrimSpace(services.ExpandWithEnv(tmuxCfg.SessionName, env))
	if sessionName == "" {
		sessionName = fmt.Sprintf("%s%s", cfg.SessionPrefix, filepath.Base(wt.Path))
	}
	sessionName = multiplexer.SanitizeTmuxSessionName(sessionName)

	// Resolve windows
	windows, ok := multiplexer.ResolveTmuxWindows(tmuxCfg.Windows, env, wt.Path)
	if !ok {
		return fmt.Errorf("failed to resolve tmux windows")
	}

	// Wrap window commands in container if configured
	if containerCfg != nil {
		var err error
		windows, err = multiplexer.WrapWindowCommandsForContainer(windows, containerCfg, env)
		if err != nil {
			return err
		}
	}

	// Create session file for script to write final session name
	sessionFile, err := os.CreateTemp("", "lazyworktree-tmux-")
	if err != nil {
		return err
	}
	sessionPath := sessionFile.Name()
	if closeErr := sessionFile.Close(); closeErr != nil {
		return closeErr
	}
	defer func() { _ = os.Remove(sessionPath) }() //#nosec G703 -- controlled temp file cleanup

	// Build script with attach=true
	scriptCfg := *tmuxCfg
	scriptCfg.Attach = true
	env["LW_TMUX_SESSION_FILE"] = sessionPath
	script := multiplexer.BuildTmuxScript(sessionName, &scriptCfg, windows, env)

	// Execute script
	// #nosec G204 -- command is built from user-configured tmux session settings
	execCmd := exec.CommandContext(ctx, "bash", "-lc", script)
	execCmd.Dir = wt.Path
	execCmd.Env = append(os.Environ(), services.EnvMapToList(env)...)
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	return execCmd.Run()
}

func executeZellijAction(ctx context.Context, zellijCfg *config.TmuxCommand, containerCfg *config.ContainerCommand, cfg *config.AppConfig, wt *models.WorktreeInfo, env map[string]string) error {
	if zellijCfg == nil {
		return fmt.Errorf("zellij configuration is nil")
	}

	// Expand and sanitise session name
	sessionName := strings.TrimSpace(services.ExpandWithEnv(zellijCfg.SessionName, env))
	if sessionName == "" {
		sessionName = fmt.Sprintf("%s%s", cfg.SessionPrefix, filepath.Base(wt.Path))
	}
	sessionName = multiplexer.SanitizeZellijSessionName(sessionName)

	// Resolve windows
	windows, ok := multiplexer.ResolveTmuxWindows(zellijCfg.Windows, env, wt.Path)
	if !ok {
		return fmt.Errorf("failed to resolve zellij windows")
	}

	// Wrap window commands in container if configured
	if containerCfg != nil {
		var err error
		windows, err = multiplexer.WrapWindowCommandsForContainer(windows, containerCfg, env)
		if err != nil {
			return err
		}
	}

	// Write layout files
	layoutPaths, err := multiplexer.WriteZellijLayouts(windows)
	if err != nil {
		return err
	}
	defer multiplexer.CleanupZellijLayouts(layoutPaths)

	// Create session file
	sessionFile, err := os.CreateTemp("", "lazyworktree-zellij-")
	if err != nil {
		return err
	}
	sessionPath := sessionFile.Name()
	if closeErr := sessionFile.Close(); closeErr != nil {
		return closeErr
	}
	defer func() { _ = os.Remove(sessionPath) }() //#nosec G703 -- controlled temp file cleanup

	// Build script
	scriptCfg := *zellijCfg
	scriptCfg.Attach = false
	env["LW_ZELLIJ_SESSION_FILE"] = sessionPath
	script := multiplexer.BuildZellijScript(sessionName, &scriptCfg, layoutPaths)

	// Execute script
	// #nosec G204 -- command is built from user-configured zellij session settings
	execCmd := exec.CommandContext(ctx, "bash", "-lc", script)
	execCmd.Dir = wt.Path
	execCmd.Env = append(os.Environ(), services.EnvMapToList(env)...)
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		return err
	}

	// Read final session name and attach
	finalSession := multiplexer.ReadSessionFile(sessionPath, sessionName)
	// #nosec G204 G702 -- zellij session name comes from user configuration
	attachCmd := exec.CommandContext(ctx, "zellij", "attach", finalSession)
	attachCmd.Stdin = os.Stdin
	attachCmd.Stdout = os.Stdout
	attachCmd.Stderr = os.Stderr

	return attachCmd.Run()
}

func executeShowOutputAction(ctx context.Context, command string, cfg *config.AppConfig, wt *models.WorktreeInfo, env map[string]string) error {
	// Run command and capture output
	shellPath, shellArgs := shellInvocationForExec(command)
	// #nosec G204 -- explicit user-provided command executed by request
	execCmd := exec.CommandContext(ctx, shellPath, shellArgs...)
	execCmd.Dir = wt.Path
	execCmd.Env = append(os.Environ(), services.EnvMapToList(env)...)

	output, err := execCmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: command failed: %v\n", err)
		return err
	}

	// Get pager command
	pagerCmd := services.PagerCommand(cfg)

	// Pipe output through pager
	// #nosec G204 -- pager command comes from config or environment
	pager := exec.CommandContext(ctx, "sh", "-c", pagerCmd)
	pager.Stdin = strings.NewReader(string(output))
	pager.Stdout = os.Stdout
	pager.Stderr = os.Stderr

	return pager.Run()
}
