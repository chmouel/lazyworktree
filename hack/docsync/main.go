package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

type flagSpec struct {
	Name    string
	Aliases []string
	Usage   string
	Kind    string
}

type commandSpec struct {
	Name      string
	Usage     string
	ArgsUsage string
	Aliases   []string
	Flags     []flagSpec
}

type configKeySpec struct {
	Key         string
	Type        string
	Default     string
	Description string
}

type actionSpec struct {
	ID          string
	Label       string
	Shortcut    string
	Description string
	Section     string
}

type docsSyncData struct {
	GlobalFlags []flagSpec
	Commands    []commandSpec
	ConfigKeys  []configKeySpec
	Actions     []actionSpec
}

func main() {
	root := flag.String("root", ".", "repository root")
	check := flag.Bool("check", false, "check generated docs are up to date")
	verify := flag.Bool("verify", false, "run docs/man/help synchronisation checks")
	flag.Parse()

	data, err := collectDocsSyncData(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docsync: %v\n", err)
		os.Exit(1)
	}

	outputs := map[string]string{
		filepath.Join(*root, "docs", "cli", "commands.md"):            renderCLICommandsPage(data.Commands),
		filepath.Join(*root, "docs", "cli", "flags.md"):               renderCLIFlagsPage(data.GlobalFlags, data.Commands),
		filepath.Join(*root, "docs", "configuration", "reference.md"): renderConfigReferencePage(data.ConfigKeys),
		filepath.Join(*root, "docs", "action-ids.md"):                 renderActionIDsPage(data.Actions),
	}

	var stale []string
	for path, content := range outputs {
		changed, syncErr := syncFile(path, content, *check)
		if syncErr != nil {
			fmt.Fprintf(os.Stderr, "docsync: %v\n", syncErr)
			os.Exit(1)
		}
		if changed {
			stale = append(stale, path)
		}
	}

	if *check && len(stale) > 0 {
		sort.Strings(stale)
		fmt.Fprintln(os.Stderr, "docsync: generated documentation is stale:")
		for _, path := range stale {
			fmt.Fprintf(os.Stderr, "  - %s\n", path)
		}
		fmt.Fprintln(os.Stderr, "run `make docs-sync` to update generated docs")
		os.Exit(1)
	}

	if *verify {
		if err := verifySync(*root, data); err != nil {
			fmt.Fprintf(os.Stderr, "docsync: verification failed: %v\n", err)
			os.Exit(1)
		}
	}
}

func collectDocsSyncData(root string) (*docsSyncData, error) {
	flagsPath := filepath.Join(root, "internal", "bootstrap", "flags.go")
	commandsPath := filepath.Join(root, "internal", "bootstrap", "commands.go")
	configPath := filepath.Join(root, "internal", "config", "config.go")
	registryPath := filepath.Join(root, "internal", "app", "commands", "registry.go")

	globalFlags, err := parseGlobalFlags(flagsPath)
	if err != nil {
		return nil, err
	}

	commands, err := parseCommands(commandsPath)
	if err != nil {
		return nil, err
	}

	configKeys, err := parseConfigKeys(configPath)
	if err != nil {
		return nil, err
	}

	actions, err := parseActionIDs(registryPath)
	if err != nil {
		return nil, err
	}

	return &docsSyncData{
		GlobalFlags: globalFlags,
		Commands:    commands,
		ConfigKeys:  configKeys,
		Actions:     actions,
	}, nil
}

func syncFile(path, content string, check bool) (bool, error) {
	// #nosec G304 -- path is constrained to repository targets by caller.
	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		if string(existing) == content {
			return false, nil
		}
		if check {
			return true, nil
		}
	case !errors.Is(err, os.ErrNotExist):
		return false, fmt.Errorf("reading %s: %w", path, err)
	case check:
		return true, nil
	}

	if check {
		return true, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return false, fmt.Errorf("creating directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return false, fmt.Errorf("writing %s: %w", path, err)
	}
	return true, nil
}

func parseGlobalFlags(path string) ([]flagSpec, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse global flags file: %w", err)
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "globalFlags" {
			continue
		}
		lit := extractReturnedCompositeLit(fn)
		if lit == nil {
			return nil, errors.New("could not extract return literal from globalFlags")
		}

		flags := make([]flagSpec, 0, len(lit.Elts))
		for _, elt := range lit.Elts {
			fl, ok := parseFlagLiteralExpr(elt)
			if !ok || fl.Name == "" {
				continue
			}
			flags = append(flags, fl)
		}
		sort.Slice(flags, func(i, j int) bool { return flags[i].Name < flags[j].Name })
		return flags, nil
	}

	return nil, errors.New("globalFlags function not found")
}

func parseCommands(path string) ([]commandSpec, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse commands file: %w", err)
	}

	var commands []commandSpec
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if !strings.HasSuffix(fn.Name.Name, "Command") {
			continue
		}

		lit := extractReturnedCompositeLit(fn)
		if lit == nil {
			continue
		}

		cmd := parseCommandLiteral(lit)
		if cmd.Name == "" {
			continue
		}
		if !slices.Contains([]string{"create", "delete", "rename", "list", "exec", "note"}, cmd.Name) {
			continue
		}
		sort.Slice(cmd.Flags, func(i, j int) bool { return cmd.Flags[i].Name < cmd.Flags[j].Name })
		commands = append(commands, cmd)
	}

	// Parse note subcommands (show, edit) and attach their flags to the parent.
	noteCmd := parseNoteSubcommands(file, commands)
	if noteCmd != nil {
		for i, cmd := range commands {
			if cmd.Name == "note" {
				commands[i] = *noteCmd
				break
			}
		}
	}

	if len(commands) == 0 {
		return nil, errors.New("no command definitions found")
	}

	order := map[string]int{"list": 0, "create": 1, "delete": 2, "rename": 3, "exec": 4, "note": 5}
	sort.Slice(commands, func(i, j int) bool {
		return order[commands[i].Name] < order[commands[j].Name]
	})
	return commands, nil
}

// parseNoteSubcommands extracts the show and edit subcommand definitions and
// merges their flags into a single "note" commandSpec for documentation.
func parseNoteSubcommands(file *ast.File, _ []commandSpec) *commandSpec {
	subFuncs := map[string]string{
		"noteShowCommand": "show",
		"noteEditCommand": "edit",
	}
	var subs []commandSpec
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		subName, ok := subFuncs[fn.Name.Name]
		if !ok {
			continue
		}
		lit := extractReturnedCompositeLit(fn)
		if lit == nil {
			continue
		}
		sub := parseCommandLiteral(lit)
		sub.Name = subName
		subs = append(subs, sub)
	}
	if len(subs) == 0 {
		return nil
	}

	cmd := &commandSpec{
		Name:  "note",
		Usage: "Show or edit worktree notes",
	}
	// Merge subcommand flags with subcommand prefix for documentation clarity.
	for _, sub := range subs {
		for _, fl := range sub.Flags {
			fl.Usage = fmt.Sprintf("(%s) %s", sub.Name, fl.Usage)
			cmd.Flags = append(cmd.Flags, fl)
		}
	}
	sort.Slice(cmd.Flags, func(i, j int) bool { return cmd.Flags[i].Name < cmd.Flags[j].Name })
	return cmd
}

func parseConfigKeys(path string) ([]configKeySpec, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	keys, err := parseConfigDataKeys(file)
	if err != nil {
		return nil, err
	}
	sort.Strings(keys)

	defaults := defaultConfigMap(file)

	typeByKey := map[string]string{
		"worktree_dir":                 "string",
		"debug_log":                    "string",
		"pager":                        "string",
		"ci_script_pager":              "string",
		"editor":                       "string",
		"commit.auto_generate_command": "string",
		"init_commands":                "[]string",
		"terminate_commands":           "[]string",
		"sort_mode":                    "enum(path|active|switched)",
		"sort_by_active":               "bool (legacy)",
		"auto_fetch_prs":               "bool",
		"disable_pr":                   "bool",
		"prune_stale_branches":         "bool",
		"auto_refresh":                 "bool",
		"ci_auto_refresh":              "bool",
		"refresh_interval":             "int",
		"search_auto_select":           "bool",
		"fuzzy_finder_input":           "bool",
		"icon_set":                     "enum(nerd-font-v3|text)",
		"max_untracked_diffs":          "int",
		"max_diff_chars":               "int",
		"max_name_length":              "int",
		"git_pager_args":               "[]string",
		"delta_args":                   "[]string (legacy)",
		"git_pager":                    "string",
		"delta_path":                   "string (legacy)",
		"trust_mode":                   "enum(tofu|never|always)",
		"theme":                        "string",
		"git_pager_interactive":        "bool",
		"git_pager_command_mode":       "bool",
		"branch_name_script":           "string",
		"worktree_note_script":         "string",
		"worktree_notes_path":          "string",
		"issue_branch_name_template":   "string",
		"pr_branch_name_template":      "string",
		"merge_method":                 "enum(rebase|merge)",
		"session_prefix":               "string",
		"palette_mru":                  "bool",
		"palette_mru_limit":            "int",
		"layout":                       "enum(default|top)",
		"custom_commands":              "map[string]map[string]object",
		"keybindings":                  "map[string]map[string]string",
		"custom_create_menus":          "[]object",
		"custom_themes":                "map[string]object",
	}

	descByKey := map[string]string{
		"worktree_dir":                 "Root directory for managed worktrees. Supports `$LWT_REPO_PATH` (auto-set to the git repository root) for repo-local placement, e.g. `$LWT_REPO_PATH/.worktrees`. When the directory is inside the repository, the `<repoName>` path segment is omitted automatically.",
		"debug_log":                    "Debug log file path.",
		"pager":                        "Pager for command output views.",
		"ci_script_pager":              "Dedicated pager for CI logs.",
		"editor":                       "Editor used in file open actions.",
		"commit.auto_generate_command": "Command used by Ctrl+O in the commit screen to generate a message from the staged diff.",
		"init_commands":                "Global commands run after worktree creation.",
		"terminate_commands":           "Global commands run before worktree removal.",
		"sort_mode":                    "Primary sort behaviour in the worktree list.",
		"sort_by_active":               "Compatibility key for older sort configuration.",
		"auto_fetch_prs":               "Automatically fetch PR/MR data.",
		"disable_pr":                   "Disable PR/MR integration.",
		"prune_stale_branches":         "Include merged branches without worktrees in prune.",
		"auto_refresh":                 "Enable background refresh of repository state.",
		"ci_auto_refresh":              "Enable periodic CI refresh for GitHub repositories.",
		"refresh_interval":             "Background refresh cadence in seconds.",
		"search_auto_select":           "Focus filter and auto-select first match.",
		"fuzzy_finder_input":           "Enable fuzzy helper input in selection dialogues.",
		"icon_set":                     "Icon rendering mode for terminal compatibility.",
		"max_untracked_diffs":          "Limit number of untracked file diffs rendered.",
		"max_diff_chars":               "Maximum characters read from diff output.",
		"max_name_length":              "Maximum displayed worktree name length.",
		"git_pager_args":               "Extra arguments passed to configured git pager.",
		"delta_args":                   "Legacy alias for git_pager_args.",
		"git_pager":                    "Diff formatter/pager command.",
		"delta_path":                   "Legacy alias for git_pager.",
		"trust_mode":                   "Trust policy for repository `.wt` commands.",
		"theme":                        "UI theme selection.",
		"git_pager_interactive":        "Use interactive pager mode for terminal-native tools.",
		"git_pager_command_mode":       "Use command mode for pagers that run git themselves.",
		"branch_name_script":           "Script to generate branch naming suggestions.",
		"worktree_note_script":         "Script to prefill worktree notes from issue/PR context.",
		"worktree_notes_path":          "Optional shared JSON file path for notes storage.",
		"issue_branch_name_template":   "Template for issue-based branch naming.",
		"pr_branch_name_template":      "Template for PR-based branch naming.",
		"merge_method":                 "Absorb strategy for integrating a worktree.",
		"session_prefix":               "Prefix for tmux/zellij session names.",
		"palette_mru":                  "Enable MRU sorting in command palette.",
		"palette_mru_limit":            "Maximum MRU items in command palette.",
		"layout":                       "Pane layout strategy.",
		"custom_commands":              "Pane-scoped custom key bindings. Use `universal` for all panes or a pane name (`worktrees`, `info`, `status`, `log`, `notes`) for context-specific commands.",
		"keybindings":                  "Pane-scoped bindings to built-in palette action IDs. Use `universal` for all panes or a pane name for context-specific bindings (see docs/action-ids.md).",
		"custom_create_menus":          "Custom create menu entries.",
		"custom_themes":                "Custom theme definitions.",
	}

	defaultByKey := map[string]string{
		"sort_mode":                  defaults["SortMode"],
		"auto_fetch_prs":             defaults["AutoFetchPRs"],
		"disable_pr":                 "false",
		"prune_stale_branches":       "false",
		"auto_refresh":               defaults["AutoRefresh"],
		"ci_auto_refresh":            "false",
		"refresh_interval":           defaults["RefreshIntervalSeconds"],
		"search_auto_select":         defaults["SearchAutoSelect"],
		"fuzzy_finder_input":         "false",
		"icon_set":                   defaults["IconSet"],
		"max_untracked_diffs":        defaults["MaxUntrackedDiffs"],
		"max_diff_chars":             defaults["MaxDiffChars"],
		"max_name_length":            defaults["MaxNameLength"],
		"git_pager":                  defaults["GitPager"],
		"git_pager_interactive":      defaults["GitPagerInteractive"],
		"git_pager_command_mode":     "false",
		"trust_mode":                 defaults["TrustMode"],
		"theme":                      "auto-detect",
		"merge_method":               defaults["MergeMethod"],
		"issue_branch_name_template": defaults["IssueBranchNameTemplate"],
		"pr_branch_name_template":    defaults["PRBranchNameTemplate"],
		"session_prefix":             defaults["SessionPrefix"],
		"layout":                     defaults["Layout"],
		"palette_mru":                defaults["PaletteMRU"],
		"palette_mru_limit":          defaults["PaletteMRULimit"],
		"custom_commands":            "universal: t, Z",
		"git_pager_args":             "auto-matched delta syntax theme",
	}

	var specs []configKeySpec
	for _, key := range orderedConfigKeys(keys) {
		specs = append(specs, configKeySpec{
			Key:         key,
			Type:        fallback(typeByKey[key], "string"),
			Default:     fallback(defaultByKey[key], "none"),
			Description: fallback(descByKey[key], "See config.example.yaml for usage details."),
		})
	}

	return specs, nil
}

func parseActionIDs(path string) ([]actionSpec, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse registry file: %w", err)
	}

	consts := collectStringConsts(file)

	var actions []actionSpec
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !strings.HasPrefix(fn.Name.Name, "Register") {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Register" {
				return true
			}
			for _, arg := range call.Args {
				lit, ok := arg.(*ast.CompositeLit)
				if !ok {
					continue
				}
				spec := parseActionLiteral(lit, consts)
				if spec.ID != "" {
					actions = append(actions, spec)
				}
			}
			return true
		})
	}

	if len(actions) == 0 {
		return nil, errors.New("no action registrations found in registry")
	}

	return actions, nil
}

func collectStringConsts(file *ast.File) map[string]string {
	consts := make(map[string]string)
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) == 0 || len(vs.Values) == 0 {
				continue
			}
			if val, ok := stringLiteral(vs.Values[0]); ok {
				consts[vs.Names[0].Name] = val
			}
		}
	}
	return consts
}

func parseActionLiteral(lit *ast.CompositeLit, consts map[string]string) actionSpec {
	spec := actionSpec{
		ID:          extractStringField(lit.Elts, "ID"),
		Label:       extractStringField(lit.Elts, "Label"),
		Description: extractStringField(lit.Elts, "Description"),
		Shortcut:    extractStringField(lit.Elts, "Shortcut"),
	}
	// Section may be a const identifier rather than a string literal.
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		keyIdent, ok := kv.Key.(*ast.Ident)
		if !ok || keyIdent.Name != "Section" {
			continue
		}
		if ident, ok := kv.Value.(*ast.Ident); ok {
			spec.Section = consts[ident.Name]
		} else {
			spec.Section, _ = stringLiteral(kv.Value)
		}
		break
	}
	return spec
}

func parseConfigDataKeys(file *ast.File) ([]string, error) {
	var parseFn *ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "parseConfig" {
			continue
		}
		parseFn = fn
		break
	}
	if parseFn == nil {
		return nil, errors.New("parseConfig function not found")
	}

	seen := map[string]struct{}{}
	ast.Inspect(parseFn.Body, func(node ast.Node) bool {
		idx, ok := node.(*ast.IndexExpr)
		if !ok {
			return true
		}
		ident, ok := idx.X.(*ast.Ident)
		if !ok || ident.Name != "data" {
			return true
		}
		if key, ok := stringLiteral(idx.Index); ok {
			seen[key] = struct{}{}
		}
		return true
	})

	keys := make([]string, 0, len(seen))
	if _, ok := seen["commit.auto_generate_command"]; ok {
		delete(seen, "commit")
	}
	for key := range seen {
		keys = append(keys, key)
	}
	return keys, nil
}

func defaultConfigMap(file *ast.File) map[string]string {
	defaults := make(map[string]string)

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "DefaultConfig" {
			continue
		}
		lit := extractReturnedCompositeLit(fn)
		if lit == nil {
			return defaults
		}
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			keyIdent, ok := kv.Key.(*ast.Ident)
			if !ok {
				continue
			}
			defaults[keyIdent.Name] = exprString(kv.Value)
		}
	}

	return defaults
}

func extractReturnedCompositeLit(fn *ast.FuncDecl) *ast.CompositeLit {
	if fn.Body == nil {
		return nil
	}
	for _, stmt := range fn.Body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok || len(ret.Results) == 0 {
			continue
		}
		switch result := ret.Results[0].(type) {
		case *ast.CompositeLit:
			return result
		case *ast.UnaryExpr:
			if lit, ok := result.X.(*ast.CompositeLit); ok {
				return lit
			}
		}
	}
	return nil
}

func parseCommandLiteral(lit *ast.CompositeLit) commandSpec {
	cmd := commandSpec{
		Name:      extractStringField(lit.Elts, "Name"),
		Usage:     extractStringField(lit.Elts, "Usage"),
		ArgsUsage: extractStringField(lit.Elts, "ArgsUsage"),
		Aliases:   extractStringSliceField(lit.Elts, "Aliases"),
	}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		keyIdent, ok := kv.Key.(*ast.Ident)
		if !ok || keyIdent.Name != "Flags" {
			continue
		}
		cmd.Flags = parseFlagsComposite(kv.Value)
		break
	}
	return cmd
}

func parseFlagsComposite(expr ast.Expr) []flagSpec {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	flags := make([]flagSpec, 0, len(lit.Elts))
	for _, elt := range lit.Elts {
		fl, ok := parseFlagLiteralExpr(elt)
		if !ok || fl.Name == "" {
			continue
		}
		flags = append(flags, fl)
	}
	return flags
}

func parseFlagLiteralExpr(expr ast.Expr) (flagSpec, bool) {
	switch value := expr.(type) {
	case *ast.UnaryExpr:
		lit, ok := value.X.(*ast.CompositeLit)
		if !ok {
			return flagSpec{}, false
		}
		return parseFlagCompositeLiteral(lit)
	case *ast.CompositeLit:
		return parseFlagCompositeLiteral(value)
	default:
		return flagSpec{}, false
	}
}

func parseFlagCompositeLiteral(lit *ast.CompositeLit) (flagSpec, bool) {
	spec := flagSpec{
		Name:    extractStringField(lit.Elts, "Name"),
		Aliases: extractStringSliceField(lit.Elts, "Aliases"),
		Usage:   extractStringField(lit.Elts, "Usage"),
	}
	if sel, ok := lit.Type.(*ast.SelectorExpr); ok {
		spec.Kind = strings.ToLower(strings.TrimSuffix(sel.Sel.Name, "Flag"))
	}
	return spec, spec.Name != ""
}

// extractStringField returns the string literal value for a named key in a
// composite literal field list, or "" if the key is absent or not a string.
func extractStringField(fields []ast.Expr, key string) string {
	for _, elt := range fields {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		keyIdent, ok := kv.Key.(*ast.Ident)
		if !ok || keyIdent.Name != key {
			continue
		}
		val, _ := stringLiteral(kv.Value)
		return val
	}
	return ""
}

// extractStringSliceField returns the []string literal for a named key, or nil.
func extractStringSliceField(fields []ast.Expr, key string) []string {
	for _, elt := range fields {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		keyIdent, ok := kv.Key.(*ast.Ident)
		if !ok || keyIdent.Name != key {
			continue
		}
		return stringSliceLiteral(kv.Value)
	}
	return nil
}

func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}

func stringSliceLiteral(expr ast.Expr) []string {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	values := make([]string, 0, len(lit.Elts))
	for _, elt := range lit.Elts {
		value, ok := stringLiteral(elt)
		if ok {
			values = append(values, value)
		}
	}
	return values
}

func exprString(expr ast.Expr) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, token.NewFileSet(), expr); err != nil {
		return ""
	}
	value := strings.TrimSpace(buf.String())
	if unquoted, err := strconv.Unquote(value); err == nil {
		return unquoted
	}
	return value
}

func orderedConfigKeys(keys []string) []string {
	preferred := []string{
		"worktree_dir",
		"theme",
		"icon_set",
		"layout",
		"sort_mode",
		"sort_by_active",
		"auto_refresh",
		"refresh_interval",
		"ci_auto_refresh",
		"auto_fetch_prs",
		"disable_pr",
		"prune_stale_branches",
		"search_auto_select",
		"fuzzy_finder_input",
		"max_name_length",
		"max_untracked_diffs",
		"max_diff_chars",
		"git_pager",
		"git_pager_args",
		"delta_path",
		"delta_args",
		"git_pager_interactive",
		"git_pager_command_mode",
		"pager",
		"ci_script_pager",
		"editor",
		"commit.auto_generate_command",
		"merge_method",
		"trust_mode",
		"branch_name_script",
		"issue_branch_name_template",
		"pr_branch_name_template",
		"worktree_note_script",
		"worktree_notes_path",
		"session_prefix",
		"palette_mru",
		"palette_mru_limit",
		"init_commands",
		"terminate_commands",
		"custom_commands",
		"keybindings",
		"custom_create_menus",
		"custom_themes",
		"debug_log",
	}

	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keySet[key] = struct{}{}
	}

	ordered := make([]string, 0, len(keys))
	for _, key := range preferred {
		if _, ok := keySet[key]; ok {
			ordered = append(ordered, key)
			delete(keySet, key)
		}
	}

	var remaining []string
	for key := range keySet {
		remaining = append(remaining, key)
	}
	sort.Strings(remaining)
	ordered = append(ordered, remaining...)
	return ordered
}

func renderCLICommandsPage(commands []commandSpec) string {
	var b strings.Builder
	b.WriteString("# CLI Commands Reference\n\n")
	b.WriteString("This page is generated from `internal/bootstrap/commands.go`. Run `make docs-sync` after changing command definitions.\n\n")
	b.WriteString("<!-- BEGIN GENERATED:cli-commands -->\n")
	b.WriteString("| Command | Usage | Args | Aliases | Guide |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, cmd := range commands {
		args := cmd.ArgsUsage
		if args == "" {
			args = "-"
		}
		aliases := "-"
		if len(cmd.Aliases) > 0 {
			aliases = strings.Join(wrapCode(cmd.Aliases), ", ")
		}
		guide := fmt.Sprintf("[`%s`](%s.md)", cmd.Name, cmd.Name)
		fmt.Fprintf(&b, "| `%s` | %s | `%s` | %s | %s |\n", cmd.Name, escapePipe(cmd.Usage), escapePipe(args), aliases, guide)
	}

	for _, cmd := range commands {
		b.WriteString("\n")
		fmt.Fprintf(&b, "## `%s`\n\n", cmd.Name)
		fmt.Fprintf(&b, "%s\n\n", cmd.Usage)
		if len(cmd.Flags) == 0 {
			b.WriteString("No command-specific flags.\n")
			continue
		}
		b.WriteString("| Flag | Type | Usage |\n")
		b.WriteString("| --- | --- | --- |\n")
		for _, flag := range cmd.Flags {
			fmt.Fprintf(&b, "| %s | `%s` | %s |\n", formatFlag(flag), flag.Kind, escapePipe(flag.Usage))
		}
	}
	b.WriteString("\n<!-- END GENERATED:cli-commands -->\n")
	return b.String()
}

func renderCLIFlagsPage(global []flagSpec, commands []commandSpec) string {
	var b strings.Builder
	b.WriteString("# CLI Flags Reference\n\n")
	b.WriteString("This page is generated from `internal/bootstrap/flags.go` and `internal/bootstrap/commands.go`.\n")
	b.WriteString("Run `make docs-sync` after changing flag definitions.\n\n")

	b.WriteString("## Global Flags\n\n")
	b.WriteString("<!-- BEGIN GENERATED:global-flags -->\n")
	b.WriteString("| Flag | Type | Usage |\n")
	b.WriteString("| --- | --- | --- |\n")
	for _, flag := range global {
		fmt.Fprintf(&b, "| %s | `%s` | %s |\n", formatFlag(flag), flag.Kind, escapePipe(flag.Usage))
	}
	b.WriteString("<!-- END GENERATED:global-flags -->\n\n")

	b.WriteString("## Command Flags\n\n")
	b.WriteString("<!-- BEGIN GENERATED:command-flags -->\n")
	for _, cmd := range commands {
		if len(cmd.Flags) == 0 {
			continue
		}
		fmt.Fprintf(&b, "### `%s`\n\n", cmd.Name)
		b.WriteString("| Flag | Type | Usage |\n")
		b.WriteString("| --- | --- | --- |\n")
		for _, flag := range cmd.Flags {
			fmt.Fprintf(&b, "| %s | `%s` | %s |\n", formatFlag(flag), flag.Kind, escapePipe(flag.Usage))
		}
		b.WriteString("\n")
	}
	b.WriteString("<!-- END GENERATED:command-flags -->\n\n")

	b.WriteString("## Validation Rules\n\n")
	b.WriteString("These runtime rules are enforced in `internal/bootstrap/commands.go`:\n\n")
	b.WriteString("- `create`: `--from-pr`, `--from-issue`, `--from-pr-interactive`, and `--from-issue-interactive` are mutually exclusive.\n")
	b.WriteString("- `create`: `--query` requires `--from-pr-interactive` or `--from-issue-interactive`.\n")
	b.WriteString("- `create`: `--no-workspace` requires PR/issue creation mode and cannot be combined with `--with-change` or `--generate`.\n")
	b.WriteString("- `list`: `--pristine` and `--json` are mutually exclusive.\n")
	b.WriteString("- `exec`: use either positional command or `--key`, never both.\n")
	return b.String()
}

func renderConfigReferencePage(keys []configKeySpec) string {
	var b strings.Builder
	b.WriteString("# Configuration Reference\n\n")
	b.WriteString("This page is generated from `internal/config/config.go`. Run `make docs-sync` after changing config parsing/defaults.\n\n")
	b.WriteString("<!-- BEGIN GENERATED:config-reference -->\n")
	b.WriteString("| Key | Type | Default | Description |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, key := range keys {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %s |\n",
			key.Key,
			escapePipe(key.Type),
			escapePipe(key.Default),
			escapePipe(key.Description))
	}
	b.WriteString("<!-- END GENERATED:config-reference -->\n\n")
	b.WriteString("For examples and grouped explanations, see:\n\n")
	b.WriteString("- [Configuration Overview](overview.md)\n")
	b.WriteString("- [Display and Themes](display-and-themes.md)\n")
	b.WriteString("- [Refresh and Performance](refresh-and-performance.md)\n")
	b.WriteString("- [Diff, Pager, and Editor](diff-pager-and-editor.md)\n")
	b.WriteString("- [Lifecycle Hooks](lifecycle-hooks.md)\n")
	b.WriteString("- [Branch Naming](branch-naming.md)\n")
	b.WriteString("- [Custom Themes](custom-themes.md)\n")
	return b.String()
}

func renderActionIDsPage(actions []actionSpec) string {
	var b strings.Builder
	b.WriteString("# Action IDs Reference\n\n")
	b.WriteString("Use these IDs in the `keybindings:` section of your configuration file to bind any key to a built-in palette action. Keybindings use a pane-scoped structure where `universal` bindings apply everywhere and pane-specific sections override them when that pane is focused.\n\n")
	b.WriteString("**Valid pane scope names:** `universal`, `worktrees`, `info`, `status`, `log`, `notes`, `agent_sessions`\n\n")
	b.WriteString("```yaml\nkeybindings:\n  universal:\n    G: git-lazygit\n    ctrl+d: worktree-delete\n    F: git-fetch\n  worktrees:\n    x: worktree-delete\n  log:\n    d: git-diff\n```\n\n")
	b.WriteString("Keys defined in `keybindings:` take priority over `custom_commands` and built-in keys. The bound key is also displayed as the shortcut in the command palette. Pane-specific bindings override universal ones for the same key.\n\n")
	b.WriteString("---\n")

	type sectionGroup struct {
		name    string
		actions []actionSpec
	}
	var sections []sectionGroup
	sectionIdx := make(map[string]int)
	for _, a := range actions {
		if idx, ok := sectionIdx[a.Section]; ok {
			sections[idx].actions = append(sections[idx].actions, a)
		} else {
			sectionIdx[a.Section] = len(sections)
			sections = append(sections, sectionGroup{name: a.Section, actions: []actionSpec{a}})
		}
	}

	for _, sec := range sections {
		fmt.Fprintf(&b, "\n## %s\n\n", sec.name)
		b.WriteString("| ID | Label | Default Key | Description |\n")
		b.WriteString("|----|-------|-------------|-------------|\n")
		for _, a := range sec.actions {
			shortcut := "—"
			if a.Shortcut != "" {
				shortcut = "`" + a.Shortcut + "`"
			}
			desc := a.Description
			if desc == "" {
				desc = a.Label
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", a.ID, a.Label, shortcut, desc)
		}
	}

	return b.String()
}

func verifySync(root string, data *docsSyncData) error {
	if err := verifyCommandDocs(root, data.Commands); err != nil {
		return err
	}
	if err := verifyManPage(root, data.GlobalFlags, data.Commands); err != nil {
		return err
	}
	if err := verifyKeybindingDocs(root); err != nil {
		return err
	}
	if err := verifyRawHTMLLocalLinks(root); err != nil {
		return err
	}
	return nil
}

var rawHTMLURLAttrPattern = regexp.MustCompile(`(?i)\b(?:src|href)\s*=\s*["']([^"']+)["']`)

func verifyRawHTMLLocalLinks(root string) error {
	docsRoot := filepath.Join(root, "docs")
	var failures []string

	err := filepath.WalkDir(docsRoot, func(filePath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || filepath.Ext(filePath) != ".md" {
			return nil
		}
		// #nosec G304 -- path is discovered under repository docs root.
		content, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(docsRoot, filePath)
		if err != nil {
			return err
		}

		base := markdownRouteBase(relPath)
		inFence := false
		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}
			matches := rawHTMLURLAttrPattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) < 2 {
					continue
				}
				ref := strings.TrimSpace(match[1])
				if skipRawHTMLLocalLinkCheck(ref) {
					continue
				}
				target, resolveErr := resolveRouteRelativePath(base, ref)
				if resolveErr != nil {
					failures = append(failures, fmt.Sprintf("%s:%d invalid link %q: %v", filepath.ToSlash(relPath), i+1, ref, resolveErr))
					continue
				}
				if !routeTargetExists(docsRoot, target) {
					failures = append(failures, fmt.Sprintf("%s:%d unresolved %q (resolved to /%s)", filepath.ToSlash(relPath), i+1, ref, target))
				}
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("raw HTML link verification failed: %w", err)
	}
	if len(failures) > 0 {
		sort.Strings(failures)
		return fmt.Errorf("raw HTML links with unresolved local targets:\n- %s", strings.Join(failures, "\n- "))
	}
	return nil
}

func markdownRouteBase(relPath string) string {
	p := filepath.ToSlash(relPath)
	switch {
	case strings.HasSuffix(p, "/index.md"):
		return "/" + strings.TrimSuffix(p, "index.md")
	case strings.HasSuffix(p, ".md"):
		return "/" + strings.TrimSuffix(p, ".md") + "/"
	default:
		return "/"
	}
}

func skipRawHTMLLocalLinkCheck(ref string) bool {
	if ref == "" {
		return true
	}
	lower := strings.ToLower(ref)
	return strings.HasPrefix(lower, "#") ||
		strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "tel:") ||
		strings.HasPrefix(lower, "data:") ||
		strings.HasPrefix(lower, "javascript:") ||
		strings.HasPrefix(lower, "//")
}

func resolveRouteRelativePath(base, ref string) (string, error) {
	u, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	resolved := (&url.URL{Path: base}).ResolveReference(u).Path
	if resolved == "" || resolved == "/" {
		return "", nil
	}
	return strings.TrimPrefix(pathpkg.Clean(resolved), "/"), nil
}

func routeTargetExists(docsRoot, target string) bool {
	if target == "" {
		return true
	}
	candidates := []string{
		filepath.Join(docsRoot, filepath.FromSlash(target)),
		filepath.Join(docsRoot, filepath.FromSlash(target)+".md"),
		filepath.Join(docsRoot, filepath.FromSlash(target), "index.md"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func verifyCommandDocs(root string, commands []commandSpec) error {
	for _, cmd := range commands {
		path := filepath.Join(root, "docs", "cli", cmd.Name+".md")
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("missing command guide %s", path)
		}
	}
	return nil
}

func verifyManPage(root string, global []flagSpec, commands []commandSpec) error {
	manPath := filepath.Join(root, "lazyworktree.1")
	// #nosec G304 -- path is fixed to repository man page.
	content, err := os.ReadFile(manPath)
	if err != nil {
		return fmt.Errorf("reading man page: %w", err)
	}
	normalised := strings.ReplaceAll(string(content), "\\", "")

	for _, cmd := range commands {
		if !strings.Contains(normalised, "lazyworktree "+cmd.Name) {
			return fmt.Errorf("man page missing command synopsis for %q", cmd.Name)
		}
	}

	for _, flag := range global {
		needle := "--" + flag.Name
		if !strings.Contains(normalised, needle) {
			return fmt.Errorf("man page missing global flag %q", needle)
		}
	}

	for _, cmd := range commands {
		for _, flag := range cmd.Flags {
			needle := "--" + flag.Name
			if !strings.Contains(normalised, needle) {
				return fmt.Errorf("man page missing %s flag %q", cmd.Name, needle)
			}
		}
	}

	return nil
}

func verifyKeybindingDocs(root string) error {
	helpPath := filepath.Join(root, "internal", "app", "screen", "help.go")
	keybindingsPath := filepath.Join(root, "docs", "keybindings.md")
	navigationPath := filepath.Join(root, "docs", "core", "navigation-and-keybindings.md")

	// #nosec G304 -- paths are fixed to repository files.
	help, err := os.ReadFile(helpPath)
	if err != nil {
		return fmt.Errorf("reading help text source: %w", err)
	}
	// #nosec G304 -- path is fixed to repository docs file.
	keybindings, err := os.ReadFile(keybindingsPath)
	if err != nil {
		return fmt.Errorf("reading docs keybindings page: %w", err)
	}
	// #nosec G304 -- path is fixed to repository docs file.
	navigation, err := os.ReadFile(navigationPath)
	if err != nil {
		return fmt.Errorf("reading docs navigation page: %w", err)
	}

	helpText := strings.ToLower(string(help))
	docsText := strings.ToLower(string(keybindings) + "\n" + string(navigation))

	required := []struct {
		helpNeed string
		docsNeed string
	}{
		{helpNeed: "toggle layout", docsNeed: "toggle layout"},
		{helpNeed: "absorb worktree", docsNeed: "absorb worktree"},
		{helpNeed: "prune merged worktrees", docsNeed: "prune merged worktrees"},
		{helpNeed: "command palette", docsNeed: "command palette"},
		{helpNeed: "copy selected worktree branch name", docsNeed: "copy selected worktree branch name"},
		{helpNeed: "view selected ci check logs", docsNeed: "view selected ci check logs"},
		{helpNeed: "restart selected ci job", docsNeed: "restart ci job"},
	}

	for _, check := range required {
		if !strings.Contains(helpText, check.helpNeed) {
			return fmt.Errorf("help text missing phrase %q", check.helpNeed)
		}
		if !strings.Contains(docsText, check.docsNeed) {
			return fmt.Errorf("docs keybinding pages missing phrase %q", check.docsNeed)
		}
	}

	return nil
}

func formatFlag(spec flagSpec) string {
	parts := []string{"--" + spec.Name}
	for _, alias := range spec.Aliases {
		prefix := "--"
		if len(alias) == 1 {
			prefix = "-"
		}
		parts = append(parts, prefix+alias)
	}
	for i, part := range parts {
		parts[i] = "`" + part + "`"
	}
	return strings.Join(parts, ", ")
}

func wrapCode(items []string) []string {
	values := make([]string, len(items))
	for i, item := range items {
		values[i] = "`" + item + "`"
	}
	return values
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

func escapePipe(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return strings.ReplaceAll(value, "|", "\\|")
}
