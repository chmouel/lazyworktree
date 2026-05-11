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
	"github.com/chmouel/lazyworktree/internal/buildinfo"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/git"
	"github.com/chmouel/lazyworktree/internal/models"
	appiCli "github.com/urfave/cli/v3"
)

type commandExitError struct {
	err      error
	exitCode int
	quiet    bool
}

func (e *commandExitError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *commandExitError) Unwrap() error { return e.err }

func (e *commandExitError) ExitCode() int {
	if e == nil || e.exitCode == 0 {
		return 1
	}
	return e.exitCode
}

func (e *commandExitError) Quiet() bool { return e != nil && e.quiet }

type resolvedWorktree struct {
	worktree   *models.WorktreeInfo
	resolvedBy string
}

type worktreeReadDeps struct {
	cfg      *config.AppConfig
	repoKey  string
	notesMap map[string]models.WorktreeNote
	agentSvc *services.AgentSessionService
}

func doctorCommand() *appiCli.Command {
	return &appiCli.Command{
		Name:  "doctor",
		Usage: "Report CLI, repository, and tooling health for automation",
		Flags: []appiCli.Flag{
			&appiCli.BoolFlag{
				Name:  "json",
				Usage: "Output result as JSON",
			},
		},
		Action: handleDoctorAction,
	}
}

func worktreesCommand() *appiCli.Command {
	return &appiCli.Command{
		Name:  "worktrees",
		Usage: "Discover and inspect worktrees with stable machine-readable output",
		Commands: []*appiCli.Command{
			worktreesListCommand(),
			worktreesResolveCommand(),
			worktreesGetCommand(),
			worktreesContextCommand(),
		},
	}
}

func worktreesListCommand() *appiCli.Command {
	return &appiCli.Command{
		Name:  "list",
		Usage: "List worktrees for the current repository",
		Flags: []appiCli.Flag{
			&appiCli.BoolFlag{
				Name:  "json",
				Usage: "Output result as JSON",
			},
			&appiCli.BoolFlag{
				Name:  "main",
				Usage: "Show only the main worktree",
			},
			&appiCli.IntFlag{
				Name:  "limit",
				Usage: "Limit the number of returned worktrees",
			},
			&appiCli.BoolFlag{
				Name:  "no-agent",
				Usage: "Skip agent session data",
			},
		},
		Action: handleWorktreesListAction,
	}
}

func worktreesResolveCommand() *appiCli.Command {
	return &appiCli.Command{
		Name:  "resolve",
		Usage: "Resolve a worktree name, path, or cwd to its canonical path",
		Flags: []appiCli.Flag{
			&appiCli.BoolFlag{
				Name:  "json",
				Usage: "Output result as JSON",
			},
			&appiCli.StringFlag{
				Name:  "name",
				Usage: "Resolve a worktree by name, basename, or branch",
			},
			&appiCli.StringFlag{
				Name:  "path",
				Usage: "Resolve a worktree by absolute path or a path inside it",
			},
			&appiCli.StringFlag{
				Name:  "cwd",
				Usage: "Resolve a worktree from the provided working directory",
			},
			&appiCli.BoolFlag{
				Name:  "no-agent",
				Usage: "Skip agent session data",
			},
		},
		Action: handleWorktreesResolveAction,
	}
}

func worktreesGetCommand() *appiCli.Command {
	return &appiCli.Command{
		Name:      "get",
		Usage:     "Read one exact worktree by canonical path, name, or branch",
		ArgsUsage: "<worktree>",
		ShellComplete: func(ctx context.Context, cmd *appiCli.Command) {
			if cmd.NArg() == 0 {
				outputCompletionLines(listSubcommandWorktreeNamesFunc(ctx, cmd))
			}
		},
		Flags: []appiCli.Flag{
			&appiCli.BoolFlag{
				Name:  "json",
				Usage: "Output result as JSON",
			},
			&appiCli.BoolFlag{
				Name:  "no-agent",
				Usage: "Skip agent session data",
			},
		},
		Action: handleWorktreesGetAction,
	}
}

func worktreesContextCommand() *appiCli.Command {
	return &appiCli.Command{
		Name:      "context",
		Usage:     "Read note and agent-session context for one worktree",
		ArgsUsage: "<worktree>",
		ShellComplete: func(ctx context.Context, cmd *appiCli.Command) {
			if cmd.NArg() == 0 {
				outputCompletionLines(listSubcommandWorktreeNamesFunc(ctx, cmd))
			}
		},
		Flags: []appiCli.Flag{
			&appiCli.BoolFlag{
				Name:  "json",
				Usage: "Output result as JSON",
			},
			&appiCli.StringFlag{
				Name:  "include",
				Usage: "Comma-separated context sections: notes,agents (default: notes,agents)",
				Value: "notes,agents",
			},
		},
		Action: handleWorktreesContextAction,
	}
}

func notesCommand() *appiCli.Command {
	return &appiCli.Command{
		Name:  "notes",
		Usage: "Read worktree notes with machine-readable output",
		Commands: []*appiCli.Command{
			notesGetCommand(),
		},
	}
}

func notesGetCommand() *appiCli.Command {
	return &appiCli.Command{
		Name:      "get",
		Usage:     "Read the note for one worktree",
		ArgsUsage: "<worktree>",
		ShellComplete: func(ctx context.Context, cmd *appiCli.Command) {
			if cmd.NArg() == 0 {
				outputCompletionLines(listSubcommandWorktreeNamesFunc(ctx, cmd))
			}
		},
		Flags: []appiCli.Flag{
			&appiCli.BoolFlag{
				Name:  "json",
				Usage: "Output result as JSON including metadata",
			},
		},
		Action: handleNotesGetAction,
	}
}

func handleDoctorAction(ctx context.Context, cmd *appiCli.Command) error {
	cfg, cfgErr := loadCLIConfigFunc(
		cmd.String("config-file"),
		cmd.String("worktree-dir"),
		cmd.StringSlice("config"),
	)
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	gitSvc := newCLIGitServiceFunc(cfg)
	cwd, _ := os.Getwd()
	topLevel := gitToplevel()
	worktrees, wtErr := gitSvc.GetWorktrees(ctx)
	if wtErr != nil {
		worktrees = nil
	}

	currentWorktree := detectWorktreeFromPath(cwd, worktrees)
	mainWorktree := gitSvc.GetMainWorktreePath(ctx)

	buildinfo.Enrich()
	payload := doctorJSON{
		Version: buildinfo.Version(),
		Build: doctorBuildJSON{
			Commit:  buildinfo.Commit(),
			Date:    buildinfo.Date(),
			BuiltBy: buildinfo.BuiltBy(),
		},
		Config: doctorConfigJSON{
			Path:        cfg.ConfigPath,
			Loaded:      strings.TrimSpace(cfg.ConfigPath) != "",
			WorktreeDir: cfg.WorktreeDir,
			NoteType:    cfg.WorktreeNoteType,
			NotesPath:   cfg.WorktreeNotesPath,
		},
		Repository: doctorRepoJSON{
			CWD:             cwd,
			GitTopLevel:     topLevel,
			Repo:            repoNameOrEmpty(gitSvc.ResolveRepoName(ctx)),
			Host:            emptyWhenUnknown(gitSvc.DetectHost(ctx)),
			MainWorktree:    mainWorktree,
			InWorktree:      currentWorktree != nil,
			WorktreeCount:   len(worktrees),
			CurrentWorktree: pathOrEmpty(currentWorktree),
		},
		Tools: doctorToolsJSON{
			Git:  doctorToolStatus("git"),
			GH:   doctorToolStatus("gh"),
			GLab: doctorToolStatus("glab"),
		},
		Checks: doctorChecksJSON{
			CanListWorktrees: wtErr == nil,
		},
	}
	if cfgErr != nil {
		payload.Checks.ConfigError = cfgErr.Error()
	}
	if wtErr != nil {
		payload.Checks.WorktreeError = wtErr.Error()
	}

	if cmd.Bool("json") {
		return encodeJSON(os.Stdout, payload)
	}

	fmt.Fprintf(os.Stdout, "Version: %s\n", payload.Version)
	if payload.Config.Loaded {
		fmt.Fprintf(os.Stdout, "Config: %s\n", payload.Config.Path)
	} else {
		fmt.Fprintln(os.Stdout, "Config: defaults only")
	}
	if payload.Repository.GitTopLevel != "" {
		fmt.Fprintf(os.Stdout, "Repository: %s\n", payload.Repository.GitTopLevel)
	} else {
		fmt.Fprintln(os.Stdout, "Repository: not detected")
	}
	if payload.Repository.CurrentWorktree != "" {
		fmt.Fprintf(os.Stdout, "Worktree: %s\n", payload.Repository.CurrentWorktree)
	}
	fmt.Fprintf(os.Stdout, "Tools: git=%t gh=%t glab=%t\n", payload.Tools.Git.Available, payload.Tools.GH.Available, payload.Tools.GLab.Available)
	if payload.Checks.ConfigError != "" {
		fmt.Fprintf(os.Stdout, "Config warning: %s\n", payload.Checks.ConfigError)
	}
	if payload.Checks.WorktreeError != "" {
		fmt.Fprintf(os.Stdout, "Worktree warning: %s\n", payload.Checks.WorktreeError)
	}
	return nil
}

func handleWorktreesListAction(ctx context.Context, cmd *appiCli.Command) error {
	state, err := loadWorktreeCommandState(ctx, cmd, !cmd.Bool("no-agent"))
	if err != nil {
		return writeMaybeJSONError(cmd.Bool("json"), "config_error", err, nil)
	}

	worktrees := slices.Clone(state.worktrees)
	sortWorktreesByPath(worktrees)
	if cmd.Bool("main") {
		worktrees = filterMainWorktrees(worktrees)
	}
	if limit := cmd.Int("limit"); limit > 0 && len(worktrees) > limit {
		worktrees = worktrees[:limit]
	}

	items := make([]machineWorktreeJSON, 0, len(worktrees))
	for _, wt := range worktrees {
		items = append(items, buildMachineWorktreeJSON(wt, state.deps))
	}

	if cmd.Bool("json") {
		return encodeJSON(os.Stdout, machineWorktreeListJSON{
			Repo:  state.repoKey,
			Count: len(items),
			Limit: cmd.Int("limit"),
			Items: items,
		})
	}

	return outputMachineWorktreesTable(items)
}

func handleWorktreesResolveAction(ctx context.Context, cmd *appiCli.Command) error {
	input, kind, err := validateResolveFlags(cmd)
	if err != nil {
		return writeMaybeJSONError(cmd.Bool("json"), "invalid_input", err, nil)
	}

	state, err := loadWorktreeCommandState(ctx, cmd, !cmd.Bool("no-agent"))
	if err != nil {
		return writeMaybeJSONError(cmd.Bool("json"), "config_error", err, nil)
	}

	resolved, err := resolveWorktreeForMachine(state.cfg, state.repoKey, state.mainWorktree, state.worktrees, input, kind)
	if err != nil {
		return writeMaybeJSONError(cmd.Bool("json"), "worktree_not_found", err, map[string]string{"input": input})
	}

	view := buildMachineWorktreeJSON(resolved.worktree, state.deps)
	if cmd.Bool("json") {
		return encodeJSON(os.Stdout, machineWorktreeResolveJSON{
			Input:      input,
			ResolvedBy: resolved.resolvedBy,
			Worktree:   view,
		})
	}

	fmt.Println(view.Path)
	return nil
}

func handleWorktreesGetAction(ctx context.Context, cmd *appiCli.Command) error {
	if cmd.NArg() != 1 {
		return writeMaybeJSONError(cmd.Bool("json"), "invalid_input", fmt.Errorf("expected exactly one worktree argument"), nil)
	}

	state, err := loadWorktreeCommandState(ctx, cmd, !cmd.Bool("no-agent"))
	if err != nil {
		return writeMaybeJSONError(cmd.Bool("json"), "config_error", err, nil)
	}

	resolved, err := resolveWorktreeForMachine(state.cfg, state.repoKey, state.mainWorktree, state.worktrees, cmd.Args().Get(0), "name")
	if err != nil {
		return writeMaybeJSONError(cmd.Bool("json"), "worktree_not_found", err, nil)
	}

	view := buildMachineWorktreeJSON(resolved.worktree, state.deps)
	if cmd.Bool("json") {
		return encodeJSON(os.Stdout, view)
	}

	return outputMachineWorktreesTable([]machineWorktreeJSON{view})
}

func handleWorktreesContextAction(ctx context.Context, cmd *appiCli.Command) error {
	if cmd.NArg() != 1 {
		return writeMaybeJSONError(cmd.Bool("json"), "invalid_input", fmt.Errorf("expected exactly one worktree argument"), nil)
	}

	includeNotes, includeAgents := parseIncludeFlags(cmd.String("include"))
	if !includeNotes && !includeAgents {
		return writeMaybeJSONError(cmd.Bool("json"), "invalid_input", fmt.Errorf("--include must contain notes, agents, or both"), nil)
	}

	state, err := loadWorktreeCommandState(ctx, cmd, includeAgents)
	if err != nil {
		return writeMaybeJSONError(cmd.Bool("json"), "config_error", err, nil)
	}

	resolved, err := resolveWorktreeForMachine(state.cfg, state.repoKey, state.mainWorktree, state.worktrees, cmd.Args().Get(0), "name")
	if err != nil {
		return writeMaybeJSONError(cmd.Bool("json"), "worktree_not_found", err, nil)
	}

	payload := machineWorktreeContextJSON{
		Worktree: buildMachineWorktreeJSON(resolved.worktree, state.deps),
	}
	if includeNotes {
		payload.Note = buildNoteJSON(state.cfg, state.repoKey, state.deps.notesMap, resolved.worktree)
	}
	if includeAgents {
		payload.AgentSessions = buildAgentSessionJSONs(state.deps.agentSvc, resolved.worktree.Path)
	}

	if cmd.Bool("json") {
		return encodeJSON(os.Stdout, payload)
	}

	fmt.Fprintf(os.Stdout, "Path: %s\n", payload.Worktree.Path)
	fmt.Fprintf(os.Stdout, "Branch: %s\n", payload.Worktree.Branch)
	if payload.Note != nil && strings.TrimSpace(payload.Note.Note) != "" {
		fmt.Fprintf(os.Stdout, "\nNote:\n%s\n", payload.Note.Note)
	}
	if len(payload.AgentSessions) > 0 {
		fmt.Fprintln(os.Stdout, "\nAgent sessions:")
		for _, session := range payload.AgentSessions {
			fmt.Fprintf(os.Stdout, "- %s %s %s\n", session.Agent, session.Status, session.TaskLabel)
		}
	}
	return nil
}

func handleNotesGetAction(ctx context.Context, cmd *appiCli.Command) error {
	if cmd.NArg() != 1 {
		return writeMaybeJSONError(cmd.Bool("json"), "invalid_input", fmt.Errorf("expected exactly one worktree argument"), nil)
	}

	state, err := loadWorktreeCommandState(ctx, cmd, false)
	if err != nil {
		return writeMaybeJSONError(cmd.Bool("json"), "config_error", err, nil)
	}

	resolved, err := resolveWorktreeForMachine(state.cfg, state.repoKey, state.mainWorktree, state.worktrees, cmd.Args().Get(0), "name")
	if err != nil {
		return writeMaybeJSONError(cmd.Bool("json"), "worktree_not_found", err, nil)
	}

	note := buildNoteJSON(state.cfg, state.repoKey, state.deps.notesMap, resolved.worktree)
	if note == nil {
		note = &noteShowJSON{
			WorktreeName: filepath.Base(resolved.worktree.Path),
			Path:         resolved.worktree.Path,
		}
	}

	if cmd.Bool("json") {
		return encodeJSON(os.Stdout, note)
	}

	if strings.TrimSpace(note.Note) != "" {
		fmt.Print(note.Note)
		if !strings.HasSuffix(note.Note, "\n") {
			fmt.Println()
		}
	}
	return nil
}

type worktreeCommandState struct {
	cfg          *config.AppConfig
	gitSvc       *git.Service
	repoKey      string
	worktrees    []*models.WorktreeInfo
	mainWorktree string
	deps         worktreeReadDeps
}

func loadWorktreeCommandState(ctx context.Context, cmd *appiCli.Command, includeAgents bool) (*worktreeCommandState, error) {
	cfg, err := loadCLIConfigFunc(
		cmd.String("config-file"),
		cmd.String("worktree-dir"),
		cmd.StringSlice("config"),
	)
	if err != nil {
		return nil, err
	}

	gitSvc := newCLIGitServiceFunc(cfg)
	worktrees, err := gitSvc.GetWorktrees(ctx)
	if err != nil {
		return nil, err
	}
	sortWorktreesByPath(worktrees)

	repoKey := gitSvc.ResolveRepoName(ctx)
	mainWorktree := gitSvc.GetMainWorktreePath(ctx)
	deps := worktreeReadDeps{
		cfg:     cfg,
		repoKey: repoKey,
	}
	if mainEnv := buildMainWorktreeEnv(ctx, gitSvc, worktrees, repoKey); mainEnv != nil {
		deps.notesMap, _ = services.LoadWorktreeNotes(repoKey, cfg.WorktreeDir, cfg.WorktreeNotesPath, cfg.WorktreeNoteType, mainEnv)
	}
	if includeAgents {
		agentSvc := services.NewAgentSessionServiceFromConfig(
			cfg.AgentSessionClaudeRoot,
			cfg.AgentSessionPiRoot,
			nil,
		)
		_, _ = agentSvc.Refresh()
		deps.agentSvc = agentSvc
	}

	return &worktreeCommandState{
		cfg:          cfg,
		gitSvc:       gitSvc,
		repoKey:      repoKey,
		worktrees:    worktrees,
		mainWorktree: mainWorktree,
		deps:         deps,
	}, nil
}

func validateResolveFlags(cmd *appiCli.Command) (string, string, error) {
	options := []struct {
		kind  string
		value string
	}{
		{kind: "name", value: strings.TrimSpace(cmd.String("name"))},
		{kind: "path", value: strings.TrimSpace(cmd.String("path"))},
		{kind: "cwd", value: strings.TrimSpace(cmd.String("cwd"))},
	}
	var chosen []struct {
		kind  string
		value string
	}
	for _, option := range options {
		if option.value != "" {
			chosen = append(chosen, option)
		}
	}
	if len(chosen) != 1 {
		return "", "", fmt.Errorf("specify exactly one of --name, --path, or --cwd")
	}
	return chosen[0].value, chosen[0].kind, nil
}

func resolveWorktreeForMachine(cfg *config.AppConfig, repoKey, mainWorktree string, worktrees []*models.WorktreeInfo, input, kind string) (*resolvedWorktree, error) {
	switch kind {
	case "cwd":
		for _, wt := range worktrees {
			if wt.Path == input || strings.HasPrefix(input, wt.Path+string(filepath.Separator)) {
				return &resolvedWorktree{worktree: wt, resolvedBy: "cwd"}, nil
			}
		}
		return nil, fmt.Errorf("worktree not found for cwd: %s", input)
	case "path":
		for _, wt := range worktrees {
			if wt.Path == input {
				return &resolvedWorktree{worktree: wt, resolvedBy: "path"}, nil
			}
		}
		for _, wt := range worktrees {
			if strings.HasPrefix(input, wt.Path+string(filepath.Separator)) {
				return &resolvedWorktree{worktree: wt, resolvedBy: "path-prefix"}, nil
			}
		}
		return nil, fmt.Errorf("worktree not found for path: %s", input)
	default:
		for _, wt := range worktrees {
			if wt.Branch == input {
				return &resolvedWorktree{worktree: wt, resolvedBy: "branch"}, nil
			}
		}
		constructedPath := filepath.Join(resolveMachineWorktreeBaseDir(cfg.WorktreeDir, mainWorktree, repoKey), input)
		for _, wt := range worktrees {
			if wt.Path == constructedPath {
				return &resolvedWorktree{worktree: wt, resolvedBy: "constructed-path"}, nil
			}
		}
		for _, wt := range worktrees {
			if filepath.Base(wt.Path) == input {
				return &resolvedWorktree{worktree: wt, resolvedBy: "basename"}, nil
			}
		}
		for _, wt := range worktrees {
			if wt.Path == input {
				return &resolvedWorktree{worktree: wt, resolvedBy: "path"}, nil
			}
		}
	}
	return nil, fmt.Errorf("worktree not found: %s", input)
}

func buildMachineWorktreeJSON(wt *models.WorktreeInfo, deps worktreeReadDeps) machineWorktreeJSON {
	view := machineWorktreeJSON{
		Path:       wt.Path,
		Name:       filepath.Base(wt.Path),
		Branch:     wt.Branch,
		Repo:       deps.repoKey,
		IsMain:     wt.IsMain,
		Dirty:      wt.Dirty,
		Ahead:      wt.Ahead,
		Behind:     wt.Behind,
		Unpushed:   wt.Unpushed,
		LastActive: wt.LastActive,
	}

	if deps.notesMap != nil && deps.cfg != nil {
		if note, ok := findNoteForWorktree(deps.cfg, deps.repoKey, deps.notesMap, wt.Path); ok {
			view.NotePresent = true
			view.NoteUpdatedAt = note.UpdatedAt
			view.Description = note.Description
			view.Tags = note.Tags
		}
	}

	if deps.agentSvc != nil {
		sessions := deps.agentSvc.SessionsForWorktree(wt.Path)
		view.AgentCount = len(sessions)
		for _, session := range sessions {
			if session.IsOpen {
				view.AgentOpen = true
				view.AgentActivity = string(session.Activity)
			}
		}
	}

	return view
}

func buildNoteJSON(cfg *config.AppConfig, repoKey string, notesMap map[string]models.WorktreeNote, wt *models.WorktreeInfo) *noteShowJSON {
	if notesMap == nil || cfg == nil || wt == nil {
		return nil
	}

	note, ok := findNoteForWorktree(cfg, repoKey, notesMap, wt.Path)
	if !ok {
		return nil
	}

	return &noteShowJSON{
		WorktreeName: filepath.Base(wt.Path),
		Path:         wt.Path,
		Note:         note.Note,
		Description:  note.Description,
		Icon:         note.Icon,
		Tags:         note.Tags,
		UpdatedAt:    note.UpdatedAt,
	}
}

func buildAgentSessionJSONs(agentSvc *services.AgentSessionService, wtPath string) []agentSessionJSON {
	if agentSvc == nil {
		return nil
	}

	sessions := agentSvc.SessionsForWorktree(wtPath)
	if len(sessions) == 0 {
		return nil
	}

	output := make([]agentSessionJSON, 0, len(sessions))
	for _, session := range sessions {
		output = append(output, agentSessionJSON{
			ID:           session.ID,
			Agent:        string(session.Agent),
			Status:       string(session.Status),
			Activity:     string(session.Activity),
			IsOpen:       session.IsOpen,
			LastActivity: session.LastActivity.Format("2006-01-02T15:04:05Z07:00"),
			TaskLabel:    session.TaskLabel,
			Model:        session.Model,
		})
	}
	return output
}

func parseIncludeFlags(raw string) (bool, bool) {
	var includeNotes, includeAgents bool
	for _, part := range strings.Split(raw, ",") {
		switch strings.TrimSpace(strings.ToLower(part)) {
		case "notes":
			includeNotes = true
		case "agents":
			includeAgents = true
		}
	}
	return includeNotes, includeAgents
}

func outputMachineWorktreesTable(items []machineWorktreeJSON) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tBRANCH\tSTATUS\tLAST ACTIVE\tPATH")
	for i := range items {
		item := &items[i]
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", item.Name, item.Branch, buildMachineStatusString(item), item.LastActive, item.Path)
	}
	return w.Flush()
}

func buildMachineStatusString(item *machineWorktreeJSON) string {
	var parts []string
	if item.Dirty {
		parts = append(parts, "~")
	} else {
		parts = append(parts, "✓")
	}
	if item.Behind > 0 {
		parts = append(parts, fmt.Sprintf("↓%d", item.Behind))
	}
	if item.Ahead > 0 {
		parts = append(parts, fmt.Sprintf("↑%d", item.Ahead))
	}
	if item.Unpushed > 0 {
		parts = append(parts, fmt.Sprintf("?%d", item.Unpushed))
	}
	return strings.Join(parts, "")
}

func filterMainWorktrees(worktrees []*models.WorktreeInfo) []*models.WorktreeInfo {
	filtered := make([]*models.WorktreeInfo, 0, 1)
	for _, wt := range worktrees {
		if wt.IsMain {
			filtered = append(filtered, wt)
		}
	}
	return filtered
}

func doctorToolStatus(name string) doctorToolJSON {
	path, err := exec.LookPath(name)
	if err != nil {
		return doctorToolJSON{}
	}
	return doctorToolJSON{Available: true, Path: path}
}

func detectWorktreeFromPath(path string, worktrees []*models.WorktreeInfo) *models.WorktreeInfo {
	for _, wt := range worktrees {
		if wt.Path == path || strings.HasPrefix(path, wt.Path+string(filepath.Separator)) {
			return wt
		}
	}
	return nil
}

func emptyWhenUnknown(v string) string {
	if v == "" || v == "unknown" {
		return ""
	}
	return v
}

func repoNameOrEmpty(v string) string {
	if v == "" || v == "unknown" {
		return ""
	}
	return v
}

func pathOrEmpty(wt *models.WorktreeInfo) string {
	if wt == nil {
		return ""
	}
	return wt.Path
}

func resolveMachineWorktreeBaseDir(worktreeDir, mainWorktreePath, repoName string) string {
	if mainWorktreePath != "" && worktreeDir != "" && strings.HasPrefix(filepath.Clean(worktreeDir)+string(filepath.Separator), filepath.Clean(mainWorktreePath)+string(filepath.Separator)) {
		return worktreeDir
	}
	return filepath.Join(worktreeDir, repoName)
}

func findNoteForWorktree(cfg *config.AppConfig, repoKey string, notesMap map[string]models.WorktreeNote, wtPath string) (models.WorktreeNote, bool) {
	noteKey := worktreeNoteKey(cfg, repoKey, wtPath)
	note, ok := notesMap[noteKey]
	if ok {
		return note, true
	}
	if cfg.WorktreeNoteType != config.NoteTypeSplitted && strings.TrimSpace(cfg.WorktreeNotesPath) != "" {
		note, ok = notesMap[filepath.Clean(wtPath)]
	}
	return note, ok
}

func encodeJSON(w *os.File, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func writeMaybeJSONError(jsonOutput bool, code string, err error, details any) error {
	if !jsonOutput {
		return err
	}
	if err != nil && err.Error() != "" {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}

	payload := jsonErrorEnvelope{
		Error: jsonError{
			Code:    code,
			Message: err.Error(),
			Details: marshalJSONDetails(details),
		},
	}
	if encodeErr := encodeJSON(os.Stdout, payload); encodeErr != nil {
		return encodeErr
	}
	return &commandExitError{err: err, exitCode: 1, quiet: true}
}

func marshalJSONDetails(details any) json.RawMessage {
	if details == nil {
		return nil
	}
	data, err := json.Marshal(details)
	if err != nil {
		return nil
	}
	return json.RawMessage(data)
}
