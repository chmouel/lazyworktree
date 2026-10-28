package multiplexer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/config"
	log "github.com/chmouel/lazyworktree/internal/log"
	"github.com/chmouel/lazyworktree/internal/models"
)

// HerdrWorktreeCreator creates worktrees with `herdr worktree create`, so each
// one starts as a Herdr workspace grouped under its repository. Lazyworktree
// still owns the branches: they are prepared with git and Herdr only checks
// them out. Herdr failures never fail the creation; git takes over instead.
//
// New workspaces stay unfocused until FinaliseWorktree runs once the init
// commands are done. It applies the Layout custom command, if any, and focuses
// the workspace. A worktree rolled back before that closes its workspace.
type HerdrWorktreeCreator struct {
	Bin           string
	SessionPrefix string
	// RepoKey resolves the repository key for herdr_session expansion.
	RepoKey func(ctx context.Context) string
	// Layout is a workspace-mode Herdr custom command laying out new
	// workspaces. Nil keeps Herdr's plain worktree workspace.
	Layout *config.CustomCommand
	Warn   func(message string)
	// NoFocus keeps new workspaces in the background, for callers that must
	// stay in front until they exit, such as a TUI started by a shell helper.
	NoFocus bool

	mu    sync.Mutex
	fresh map[string]herdrFreshWorktree
}

// NewHerdrWorktreeCreator returns the creator enabled by
// use_herdr_for_workspace_create, or nil when the setting is off or Herdr is
// not installed. Configuration problems are reported through warn.
func NewHerdrWorktreeCreator(cfg *config.AppConfig, repoKey func(context.Context) string, warn func(string)) *HerdrWorktreeCreator {
	if cfg == nil || !cfg.UseHerdrForWorkspaceCreate {
		return nil
	}
	creator := &HerdrWorktreeCreator{SessionPrefix: cfg.SessionPrefix, RepoKey: repoKey, Warn: warn}
	bin, err := exec.LookPath("herdr")
	if err != nil {
		creator.warn("use_herdr_for_workspace_create is enabled but herdr is not installed; worktrees are created with git")
		return nil
	}
	creator.Bin = bin
	layout, warning := cfg.HerdrCreateLayout()
	if warning != "" {
		creator.warn(warning)
	}
	creator.Layout = layout
	return creator
}

// herdrFreshWorktree is a workspace created for a worktree whose setup is not
// finished yet.
type herdrFreshWorktree struct {
	session string
	created herdrWorkspaceCreateResult
}

type herdrPaneGetResponse struct {
	Result struct {
		Pane struct {
			Focused bool `json:"focused"`
		} `json:"pane"`
	} `json:"result"`
}

type herdrProcessInfoResponse struct {
	Result struct {
		ProcessInfo struct {
			ForegroundProcessGroupID int `json:"foreground_process_group_id"`
			ShellPID                 int `json:"shell_pid"`
		} `json:"process_info"`
	} `json:"result"`
}

// AddWorktree implements the git worktree hook.
func (c *HerdrWorktreeCreator) AddWorktree(ctx context.Context, add models.WorktreeAdd, runGit func() error) error {
	dir := add.Dir
	if dir == "" {
		dir = "."
	}
	main := HerdrSourceCheckout(ctx, dir)
	if main == "" {
		return runGit()
	}

	branch := add.NewBranch
	if branch == "" {
		branch = add.Ref
	}
	session, err := c.resolveSession(ctx, add.Path, branch, main)
	if err != nil {
		c.warn(fmt.Sprintf("Herdr could not prepare a workspace for %s, creating the worktree with git: %v", add.Path, err))
		return runGit()
	}
	h := &Herdr{Bin: c.Bin, Session: session}

	createdSHA := ""
	switch {
	case add.NewBranch != "":
		args := []string{"branch"}
		if add.Track {
			args = append(args, "--track")
		}
		if _, err := herdrGit(ctx, dir, append(args, add.NewBranch, add.Ref)...); err != nil {
			// Let git report the problem, such as an existing branch, as usual.
			return runGit()
		}
		createdSHA, _ = herdrGit(ctx, main, "rev-parse", "refs/heads/"+add.NewBranch)
	case !herdrLocalBranch(ctx, main, add.Ref):
		// Herdr only checks out branches, so detached commits and tags are
		// added by git and then opened in Herdr.
		if err := runGit(); err != nil {
			return err
		}
		c.openWorkspace(ctx, h, main, add.Path)
		return nil
	}

	var created herdrWorktreeOpenResponse
	err = h.runJSON(ctx, &created, "worktree", "create", "--cwd", main, "--branch", branch, "--path", add.Path,
		"--label", filepath.Base(add.Path), "--trust-repository", "--no-focus")
	if err == nil && created.Result.Workspace.ID != "" {
		c.track(ctx, h, add.Path, created.Result.herdrWorkspaceCreateResult)
		return nil
	}
	if err == nil {
		err = fmt.Errorf("herdr worktree create for %s returned no workspace ID", add.Path)
	}

	// A Herdr error does not prove the checkout is missing.
	if herdrCheckedOut(ctx, main, add.Path, branch) {
		c.warn(fmt.Sprintf("Herdr created the worktree %s but reported an error: %v", add.Path, err))
		c.openWorkspace(ctx, h, main, add.Path)
		return nil
	}
	if createdSHA != "" {
		herdrDeleteUnusedBranch(ctx, main, add.NewBranch, createdSHA)
	}
	c.warn(fmt.Sprintf("Herdr could not create the worktree %s, creating it with git: %v", add.Path, err))
	return runGit()
}

// WorktreeRemoved implements the git worktree hook. A worktree removed before its
// setup finished was rolled back, so its workspace goes with it. When the
// removal failed, the workspace stays open and is still closed by a later
// successful removal.
func (c *HerdrWorktreeCreator) WorktreeRemoved(ctx context.Context, path string, removed bool) {
	if !removed {
		if entry, ok := c.lookup(path); ok {
			c.warn(fmt.Sprintf("Herdr workspace %s for %s stays open until the worktree is removed", entry.created.Workspace.ID, path))
		}
		return
	}
	entry, ok := c.take(path)
	if !ok {
		return
	}
	h := &Herdr{Bin: c.Bin, Session: entry.session}
	if err := h.CloseWorkspace(ctx, entry.created.Workspace.ID); err != nil {
		c.warn(fmt.Sprintf("Could not close the Herdr workspace of the removed worktree %s: %v", path, err))
	}
}

// ForgetWorktree implements the git worktree hook. The workspace of a worktree
// kept after its setup failed stays open without the configured layout.
func (c *HerdrWorktreeCreator) ForgetWorktree(path string) {
	if entry, ok := c.take(path); ok {
		c.warn(fmt.Sprintf("Herdr workspace %s for %s keeps its basic setup without the configured layout", entry.created.Workspace.ID, path))
	}
}

// FinaliseWorktree implements the git worktree hook. It applies the layout with the
// full command environment and focuses the workspace when the current Herdr
// client can show it. Worktrees not created by Herdr are ignored.
func (c *HerdrWorktreeCreator) FinaliseWorktree(ctx context.Context, path string, env map[string]string) error {
	entry, ok := c.take(path)
	if !ok {
		return nil
	}
	h := &Herdr{Bin: c.Bin, Session: entry.session}
	var errs []error
	focus := true
	if c.Layout != nil && c.Layout.Herdr != nil {
		focus = c.Layout.Herdr.Attach
		if err := c.applyLayout(ctx, h, entry, path, env); err != nil {
			errs = append(errs, fmt.Errorf("herdr workspace %s is partially configured: %w", entry.created.Workspace.ID, err))
		}
	}
	if focus && !c.NoFocus && herdrInsideClient() && HerdrSessionIsAmbient(entry.session) {
		if err := h.FocusWorkspace(ctx, entry.created.Workspace.ID); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (c *HerdrWorktreeCreator) applyLayout(ctx context.Context, h *Herdr, entry herdrFreshWorktree, path string, env map[string]string) error {
	cfg := c.Layout.Herdr
	workspaceID := entry.created.Workspace.ID
	label := strings.TrimSpace(services.ExpandWithEnv(cfg.SessionName, env))
	label, _ = HerdrWorkspaceLabel(cfg.Mode, label, c.SessionPrefix, path, env)
	if label != "" && label != entry.created.Workspace.Label {
		if err := h.run(ctx, "workspace", "rename", workspaceID, label); err != nil {
			return err
		}
	}

	windows := ResolveHerdrWindows(cfg.Windows, env, path)
	if c.Layout.Container != nil {
		var err error
		windows = DefaultHerdrContainerCommands(windows)
		if windows, err = WrapWindowCommandsForContainer(windows, c.Layout.Container, env); err != nil {
			return err
		}
	}
	if len(windows) == 0 {
		return nil
	}
	if h.implicitTabIdle(ctx, entry.created.RootPane.ID) {
		return h.configureWorkspace(ctx, entry.created, windows, env, true)
	}
	// The first tab may already be in use, so the layout goes alongside it.
	return h.addTabs(ctx, workspaceID, windows, env)
}

// implicitTabIdle reports whether the pane of the tab Herdr opened with the
// workspace is unfocused and only runs its shell, so replacing it loses
// nothing. Any doubt keeps the tab.
func (h *Herdr) implicitTabIdle(ctx context.Context, paneID string) bool {
	if paneID == "" {
		return false
	}
	var pane herdrPaneGetResponse
	if err := h.runJSON(ctx, &pane, "pane", "get", paneID); err != nil || pane.Result.Pane.Focused {
		return false
	}
	var info herdrProcessInfoResponse
	if err := h.runJSON(ctx, &info, "pane", "process-info", "--pane", paneID); err != nil {
		return false
	}
	process := info.Result.ProcessInfo
	return process.ShellPID != 0 && process.ForegroundProcessGroupID == process.ShellPID
}

// resolveSession returns the Herdr session for new workspaces and makes sure
// its server runs. Only the base environment exists before the worktree does,
// so contextual LAZYWORKTREE_* values in herdr_session expand to nothing.
func (c *HerdrWorktreeCreator) resolveSession(ctx context.Context, path, branch, main string) (string, error) {
	session := ""
	if c.Layout != nil && c.Layout.Herdr != nil {
		repoKey := ""
		if c.RepoKey != nil {
			repoKey = c.RepoKey(ctx)
		}
		env := services.BuildCommandEnv(branch, path, repoKey, main)
		configured := strings.TrimSpace(services.ExpandWithEnv(c.Layout.Herdr.HerdrSession, env))
		if session = SanitizeHerdrSessionName(configured); configured != "" && session == "" {
			return "", HerdrInvalidNameError("herdr_session", configured)
		}
	}
	h := &Herdr{Bin: c.Bin, Session: session}
	return session, h.EnsureServer(ctx)
}

// openWorkspace opens an existing checkout as a grouped workspace.
func (c *HerdrWorktreeCreator) openWorkspace(ctx context.Context, h *Herdr, main, path string) {
	var opened herdrWorktreeOpenResponse
	err := h.runJSON(ctx, &opened, "worktree", "open", "--cwd", main, "--path", path,
		"--label", filepath.Base(path), "--trust-repository", "--no-focus")
	if err == nil && opened.Result.Workspace.ID == "" {
		err = fmt.Errorf("herdr worktree open for %s returned no workspace ID", path)
	}
	if err != nil {
		c.warn(fmt.Sprintf("Could not open the worktree %s in Herdr: %v", path, err))
		return
	}
	// A workspace that was already open may not be ours, so it is never laid
	// out or closed on rollback.
	if opened.Result.AlreadyOpen {
		c.warn(fmt.Sprintf("Herdr workspace %s for %s was already open and is left as it is", opened.Result.Workspace.ID, path))
		return
	}
	c.track(ctx, h, path, opened.Result.herdrWorkspaceCreateResult)
}

func (c *HerdrWorktreeCreator) track(ctx context.Context, h *Herdr, path string, created herdrWorkspaceCreateResult) {
	if err := h.TagWorkspace(ctx, created.Workspace.ID, HerdrWorktreeKey(path)); err != nil {
		c.warn(fmt.Sprintf("Could not tag the Herdr workspace of %s: %v", path, err))
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fresh == nil {
		c.fresh = map[string]herdrFreshWorktree{}
	}
	c.fresh[WorktreeKey(path)] = herdrFreshWorktree{session: h.Session, created: created}
}

func (c *HerdrWorktreeCreator) lookup(path string) (herdrFreshWorktree, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.fresh[WorktreeKey(path)]
	return entry, ok
}

func (c *HerdrWorktreeCreator) take(path string) (herdrFreshWorktree, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := WorktreeKey(path)
	entry, ok := c.fresh[key]
	delete(c.fresh, key)
	return entry, ok
}

func (c *HerdrWorktreeCreator) warn(message string) {
	log.Printf("herdr: %s", message)
	if c.Warn != nil {
		c.Warn(message)
	}
}

func herdrInsideClient() bool {
	return strings.TrimSpace(os.Getenv("HERDR_SESSION")) != "" || HerdrClientSocket() != ""
}

func herdrGit(ctx context.Context, dir string, args ...string) (string, error) {
	// #nosec G204 -- arguments are branch names and paths chosen by lazyworktree
	out, err := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

func herdrLocalBranch(ctx context.Context, main, ref string) bool {
	_, err := herdrGit(ctx, main, "show-ref", "--verify", "--quiet", "refs/heads/"+ref)
	return err == nil
}

// herdrWorktreeBranches maps each checkout of the repository to its branch.
func herdrWorktreeBranches(ctx context.Context, main string) map[string]string {
	out, err := herdrGit(ctx, main, "worktree", "list", "--porcelain")
	if err != nil {
		return nil
	}
	branches := map[string]string{}
	current := ""
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			current = herdrRealPath(strings.TrimPrefix(line, "worktree "))
			branches[current] = ""
		case strings.HasPrefix(line, "branch ") && current != "":
			branches[current] = strings.TrimPrefix(line, "branch refs/heads/")
		}
	}
	return branches
}

func herdrCheckedOut(ctx context.Context, main, path, branch string) bool {
	got, ok := herdrWorktreeBranches(ctx, main)[herdrRealPath(path)]
	return ok && got == branch
}

// herdrDeleteUnusedBranch removes a branch prepared for a failed creation, but
// only while it still points where it was created and no checkout uses it.
func herdrDeleteUnusedBranch(ctx context.Context, main, branch, sha string) {
	if current, err := herdrGit(ctx, main, "rev-parse", "refs/heads/"+branch); err != nil || current != sha {
		return
	}
	for _, used := range herdrWorktreeBranches(ctx, main) {
		if used == branch {
			return
		}
	}
	_, _ = herdrGit(ctx, main, "branch", "-D", branch)
}

func herdrRealPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
}
