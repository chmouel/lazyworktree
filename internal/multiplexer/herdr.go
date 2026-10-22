package multiplexer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/config"
)

const (
	herdrDefaultSession    = "default"
	herdrMaxSessionNameLen = 64
	herdrServerReadyWait   = 10 * time.Second
	herdrServerReadyPoll   = 100 * time.Millisecond
)

// HerdrWorkspace is the subset of a Herdr workspace record needed to reuse or
// focus a worktree workspace.
type HerdrWorkspace struct {
	ID    string `json:"workspace_id"`
	Label string `json:"label"`
}

// HerdrSession is the subset of a Herdr session record used for discovery.
type HerdrSession struct {
	Name    string `json:"name"`
	Default bool   `json:"default"`
	Running bool   `json:"running"`
}

type herdrPane struct {
	ID string `json:"pane_id"`
}

type herdrTab struct {
	ID string `json:"tab_id"`
}

type herdrWorkspaceListResponse struct {
	Result struct {
		Workspaces []HerdrWorkspace `json:"workspaces"`
	} `json:"result"`
}

type herdrWorkspaceCreateResponse struct {
	Result struct {
		Workspace HerdrWorkspace `json:"workspace"`
		Tab       herdrTab       `json:"tab"`
		RootPane  herdrPane      `json:"root_pane"`
	} `json:"result"`
}

type herdrTabCreateResponse struct {
	Result struct {
		RootPane herdrPane `json:"root_pane"`
	} `json:"result"`
}

type herdrSessionListResponse struct {
	Sessions []HerdrSession `json:"sessions"`
}

// Herdr runs Herdr CLI commands against one Herdr session. An empty Session
// targets the current or default Herdr session.
type Herdr struct {
	Bin     string
	Session string
}

// SanitizeHerdrSessionName keeps only characters accepted by Herdr session
// names and limits the result to Herdr's 64 byte limit.
func SanitizeHerdrSessionName(name string) string {
	sanitized := strings.Trim(sanitizeHerdrName(name), "-")
	if sanitized == "." || sanitized == ".." {
		return ""
	}
	if len(sanitized) > herdrMaxSessionNameLen {
		sanitized = strings.TrimRight(sanitized[:herdrMaxSessionNameLen], "-")
	}
	return sanitized
}

// SanitizeHerdrPrefix maps a session-name prefix the same way as full names,
// but preserves leading and trailing separators such as the dash in "wt-".
func SanitizeHerdrPrefix(prefix string) string {
	return sanitizeHerdrName(prefix)
}

func sanitizeHerdrName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}

// HerdrInvalidNameError reports a name whose sanitised form is empty. Such a
// name would silently target the default session or an empty workspace label,
// so callers reject it before running any Herdr command.
func HerdrInvalidNameError(field, value string) error {
	return fmt.Errorf("herdr %s %q sanitises to an empty name; use letters, digits, '.', '_' or '-'", field, value)
}

// HerdrResolvedName sanitises a configured name, or a fallback derived from the
// worktree when none is configured. An empty result is rejected rather than
// silently targeting the default session or an empty label.
func HerdrResolvedName(field, configured, fallback string) (string, error) {
	if configured != "" {
		if name := SanitizeHerdrSessionName(configured); name != "" {
			return name, nil
		}
		return "", HerdrInvalidNameError(field, configured)
	}
	if name := SanitizeHerdrSessionName(fallback); name != "" {
		return name, nil
	}
	return "", fmt.Errorf("herdr %s derived from %q sanitises to an empty name; use a name containing letters, digits, '.', '_' or '-'", field, fallback)
}

// ResolveHerdrWindows expands names and working directories for Herdr tabs.
// Unlike tmux, Herdr receives environment variables through --env and types tab
// commands into the pane's existing shell, so commands keep their original
// form and tabs without one stay with their native shell.
func ResolveHerdrWindows(windows []config.TmuxWindow, env map[string]string, defaultCwd string) []ResolvedWindow {
	resolved := make([]ResolvedWindow, 0, len(windows))
	for i, window := range windows {
		name := strings.TrimSpace(services.ExpandWithEnv(window.Name, env))
		if name == "" {
			name = fmt.Sprintf("window-%d", i+1)
		}
		cwd := strings.TrimSpace(services.ExpandWithEnv(window.Cwd, env))
		if cwd == "" {
			cwd = defaultCwd
		}
		command := strings.TrimSpace(window.Command)
		resolved = append(resolved, ResolvedWindow{
			Name:    name,
			Command: command,
			Cwd:     cwd,
		})
	}
	return resolved
}

// DefaultHerdrShellCommands fills commandless tabs with the default shell so
// container wrapping has a command to replace. It must only run for container
// layouts: Herdr types commands into the pane's existing shell, so a plain tab
// keeps its native shell and its own syntax.
func DefaultHerdrShellCommands(windows []ResolvedWindow) []ResolvedWindow {
	resolved := make([]ResolvedWindow, len(windows))
	for i, window := range windows {
		resolved[i] = window
		if strings.TrimSpace(window.Command) == "" {
			resolved[i].Command = defaultWindowShellCommand
		}
	}
	return resolved
}

// BuildHerdrEnvArgs renders the repeated --env flags passed to workspace and
// tab creation.
func BuildHerdrEnvArgs(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	args := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		args = append(args, "--env", key+"="+env[key])
	}
	return args
}

// AttachArgs returns the command used to attach a Herdr client to this session.
// Only an empty session omits the attach target; an explicit "default" still
// names the default session.
func (h *Herdr) AttachArgs() []string {
	if h.Session == "" {
		return []string{h.bin()}
	}
	return []string{h.bin(), "session", "attach", h.Session}
}

// InfoCommand renders the attach command for display in a modal.
func (h *Herdr) InfoCommand() string {
	return strings.Join(h.AttachArgs(), " ")
}

// DisplayAttachCommand renders an attach hint that names the effective ambient
// session, so the copied command still works from a shell without HERDR_SESSION.
func (h *Herdr) DisplayAttachCommand() string {
	if h.Session == "" && strings.TrimSpace(os.Getenv("HERDR_SESSION")) != "" {
		return strings.Join([]string{h.bin(), "session", "attach", h.displaySession()}, " ")
	}
	return h.InfoCommand()
}

// SessionName returns the effective session, including an ambient HERDR_SESSION
// and the default session.
func (h *Herdr) SessionName() string {
	return h.displaySession()
}

func (h *Herdr) bin() string {
	if h.Bin != "" {
		return h.Bin
	}
	return "herdr"
}

func (h *Herdr) args(rest ...string) []string {
	args := make([]string, 0, len(rest)+2)
	if h.Session != "" {
		args = append(args, "--session", h.Session)
	}
	return append(args, rest...)
}

func (h *Herdr) command(ctx context.Context, args ...string) *exec.Cmd {
	// #nosec G204 -- Herdr arguments come from user configuration
	cmd := exec.CommandContext(ctx, h.bin(), args...)
	cmd.Env = os.Environ()
	return cmd
}

func (h *Herdr) runJSON(ctx context.Context, out any, args ...string) error {
	cmd := h.command(ctx, h.args(args...)...)
	stdout, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
				return fmt.Errorf("herdr %s: %s", strings.Join(args, " "), stderr)
			}
		}
		return fmt.Errorf("herdr %s: %w", strings.Join(args, " "), err)
	}
	if err := json.Unmarshal(stdout, out); err != nil {
		return fmt.Errorf("herdr %s returned invalid JSON: %w", strings.Join(args, " "), err)
	}
	return nil
}

func (h *Herdr) runGlobal(ctx context.Context, out any, args ...string) error {
	cmd := h.command(ctx, args...)
	stdout, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
				return fmt.Errorf("herdr %s: %s", strings.Join(args, " "), stderr)
			}
		}
		return fmt.Errorf("herdr %s: %w", strings.Join(args, " "), err)
	}
	if out != nil {
		if err := json.Unmarshal(stdout, out); err != nil {
			return fmt.Errorf("herdr %s returned invalid JSON: %w", strings.Join(args, " "), err)
		}
	}
	return nil
}

func (h *Herdr) run(ctx context.Context, args ...string) error {
	cmd := h.command(ctx, h.args(args...)...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return fmt.Errorf("herdr %s: %s", strings.Join(args, " "), message)
		}
		return fmt.Errorf("herdr %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

// ListHerdrSessions lists all Herdr sessions known to this machine.
func ListHerdrSessions(ctx context.Context, bin string) ([]HerdrSession, error) {
	h := &Herdr{Bin: bin}
	var response herdrSessionListResponse
	if err := h.runGlobal(ctx, &response, "session", "list", "--json"); err != nil {
		return nil, err
	}
	return response.Sessions, nil
}

// UniqueHerdrSessionName returns base when no Herdr session uses that name, or
// the first free -2, -3, ... variant. The numeric suffix is reserved before the
// 64 byte limit is applied so long base names keep distinct candidates.
func UniqueHerdrSessionName(ctx context.Context, bin, base string) (string, error) {
	sessions, err := ListHerdrSessions(ctx, bin)
	if err != nil {
		return "", err
	}
	used := make(map[string]struct{}, len(sessions))
	for _, session := range sessions {
		used[session.Name] = struct{}{}
	}
	base = SanitizeHerdrSessionName(base)
	if base == "" {
		return "", HerdrInvalidNameError("session_name", base)
	}
	if _, exists := used[base]; !exists {
		return base, nil
	}
	for suffix := 2; ; suffix++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		candidate := herdrNameCandidate(base, suffix)
		if _, exists := used[candidate]; !exists {
			return candidate, nil
		}
	}
}

// herdrNameCandidate appends a numeric suffix while keeping the result within
// Herdr's 64 byte name limit, so truncation never removes the suffix.
func herdrNameCandidate(base string, suffix int) string {
	marker := fmt.Sprintf("-%d", suffix)
	room := herdrMaxSessionNameLen - len(marker)
	stem := strings.TrimRight(base, "-")
	if len(stem) > room {
		stem = strings.TrimRight(stem[:room], "-")
	}
	return stem + marker
}

// Running reports whether this Herdr session already has a live server.
func (h *Herdr) Running(ctx context.Context) bool {
	sessions, err := ListHerdrSessions(ctx, h.bin())
	if err != nil {
		return false
	}
	name := h.displaySession()
	for _, session := range sessions {
		if session.Name == name {
			return session.Running
		}
	}
	return false
}

// EnsureServer starts a detached Herdr server when this session is not running
// and waits until its API accepts commands.
func (h *Herdr) EnsureServer(ctx context.Context) error {
	if h.Running(ctx) {
		return nil
	}

	// #nosec G204 -- herdr binary and session come from user configuration
	cmd := exec.CommandContext(ctx, h.bin(), h.args("server")...)
	cmd.Dir = string(os.PathSeparator)
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer func() { _ = devNull.Close() }()
	cmd.Stdin = devNull
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	configureDetachedHerdrCommand(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start herdr server: %w", err)
	}
	_ = cmd.Process.Release()

	deadline := time.Now().Add(herdrServerReadyWait)
	for {
		if _, err := h.Workspaces(ctx); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("herdr server for session %q did not become ready", h.displaySession())
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(herdrServerReadyPoll):
		}
	}
}

// displaySession resolves the session these commands act on: an explicit
// Session wins, otherwise the ambient session.
func (h *Herdr) displaySession() string {
	if h.Session != "" {
		return h.Session
	}
	return HerdrAmbientSession()
}

// HerdrAmbientSession returns the session the current Herdr client is attached
// to: HERDR_SESSION when set, otherwise Herdr's default session.
func HerdrAmbientSession() string {
	if ambient := strings.TrimSpace(os.Getenv("HERDR_SESSION")); ambient != "" {
		return ambient
	}
	return herdrDefaultSession
}

// Workspaces lists the workspaces in this Herdr session.
func (h *Herdr) Workspaces(ctx context.Context) ([]HerdrWorkspace, error) {
	var response herdrWorkspaceListResponse
	if err := h.runJSON(ctx, &response, "workspace", "list"); err != nil {
		return nil, err
	}
	return response.Result.Workspaces, nil
}

// FindWorkspaceByLabel returns the workspace whose label matches exactly.
func (h *Herdr) FindWorkspaceByLabel(ctx context.Context, label string) (HerdrWorkspace, bool, error) {
	workspaces, err := h.Workspaces(ctx)
	if err != nil {
		return HerdrWorkspace{}, false, err
	}
	for _, workspace := range workspaces {
		if workspace.Label == label {
			return workspace, true, nil
		}
	}
	return HerdrWorkspace{}, false, nil
}

// UniqueWorkspaceLabel appends a numeric suffix until the label is unused.
func (h *Herdr) UniqueWorkspaceLabel(ctx context.Context, label string) (string, error) {
	workspaces, err := h.Workspaces(ctx)
	if err != nil {
		return "", err
	}
	used := make(map[string]struct{}, len(workspaces))
	for _, workspace := range workspaces {
		used[workspace.Label] = struct{}{}
	}
	if _, exists := used[label]; !exists {
		return label, nil
	}
	for suffix := 2; ; suffix++ {
		candidate := herdrNameCandidate(label, suffix)
		if _, exists := used[candidate]; !exists {
			return candidate, nil
		}
	}
}

// CreateWorkspace creates a workspace plus one tab per additional window,
// running each configured command in the tab's root pane. Herdr does not report
// a reliable completion marker, so a failure after the workspace exists is
// reported as a partial configuration and the workspace is left in place.
func (h *Herdr) CreateWorkspace(ctx context.Context, label, cwd string, windows []ResolvedWindow, env map[string]string, focus bool) (HerdrWorkspace, error) {
	if len(windows) == 0 {
		return HerdrWorkspace{}, fmt.Errorf("herdr requires at least one window")
	}

	workspaceCwd := windows[0].Cwd
	if strings.TrimSpace(workspaceCwd) == "" {
		workspaceCwd = cwd
	}
	args := []string{"workspace", "create", "--label", label, "--cwd", workspaceCwd}
	args = append(args, BuildHerdrEnvArgs(env)...)
	args = append(args, herdrFocusFlag(focus))
	var created herdrWorkspaceCreateResponse
	if err := h.runJSON(ctx, &created, args...); err != nil {
		return HerdrWorkspace{}, err
	}
	workspace := created.Result.Workspace
	if workspace.ID == "" {
		return HerdrWorkspace{}, fmt.Errorf("herdr workspace create for %q returned no workspace ID", label)
	}
	if err := h.configureWorkspace(ctx, created, windows, env); err != nil {
		return workspace, fmt.Errorf("herdr workspace %q (%s) is partially configured: %w", label, workspace.ID, err)
	}
	return workspace, nil
}

// configureWorkspace names the implicit first tab, runs its command and adds a
// tab per remaining window. Missing IDs are fatal whenever the matching command
// or name would otherwise be dropped silently.
func (h *Herdr) configureWorkspace(ctx context.Context, created herdrWorkspaceCreateResponse, windows []ResolvedWindow, env map[string]string) error {
	first := windows[0]
	if strings.TrimSpace(first.Name) != "" {
		if created.Result.Tab.ID == "" {
			return fmt.Errorf("herdr workspace create returned no tab ID for the first tab")
		}
		if err := h.renameTab(ctx, created.Result.Tab.ID, first.Name); err != nil {
			return err
		}
	}
	if strings.TrimSpace(first.Command) != "" {
		if created.Result.RootPane.ID == "" {
			return fmt.Errorf("herdr workspace create returned no root pane ID for the first tab")
		}
		if err := h.runPaneCommand(ctx, created.Result.RootPane.ID, first.Command); err != nil {
			return err
		}
	}

	for _, window := range windows[1:] {
		tabArgs := []string{"tab", "create", "--workspace", created.Result.Workspace.ID, "--label", window.Name, "--cwd", window.Cwd}
		tabArgs = append(tabArgs, BuildHerdrEnvArgs(env)...)
		tabArgs = append(tabArgs, "--no-focus")
		var tab herdrTabCreateResponse
		if err := h.runJSON(ctx, &tab, tabArgs...); err != nil {
			return err
		}
		if strings.TrimSpace(window.Command) != "" {
			if tab.Result.RootPane.ID == "" {
				return fmt.Errorf("herdr tab create for %q returned no root pane ID", window.Name)
			}
			if err := h.runPaneCommand(ctx, tab.Result.RootPane.ID, window.Command); err != nil {
				return err
			}
		}
	}
	return nil
}

// renameTab applies the first configured window's name to the tab that Herdr
// created implicitly for the new workspace.
func (h *Herdr) renameTab(ctx context.Context, tabID, label string) error {
	return h.run(ctx, "tab", "rename", tabID, label)
}

func (h *Herdr) runPaneCommand(ctx context.Context, paneID, command string) error {
	if strings.TrimSpace(command) == "" {
		return nil
	}
	return h.run(ctx, "pane", "run", paneID, command)
}

// FocusWorkspace brings an existing workspace to the foreground.
func (h *Herdr) FocusWorkspace(ctx context.Context, workspaceID string) error {
	return h.run(ctx, "workspace", "focus", workspaceID)
}

// CloseWorkspace closes an existing workspace.
func (h *Herdr) CloseWorkspace(ctx context.Context, workspaceID string) error {
	return h.run(ctx, "workspace", "close", workspaceID)
}

// StopSession stops this Herdr session's server.
func (h *Herdr) StopSession(ctx context.Context) error {
	return h.runGlobal(ctx, nil, "session", "stop", h.displaySession())
}

func herdrFocusFlag(focus bool) string {
	if focus {
		return "--focus"
	}
	return "--no-focus"
}
