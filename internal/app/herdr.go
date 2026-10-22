package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/chmouel/lazyworktree/internal/multiplexer"
)

const herdrWorkspaceLabel = "herdr workspace"

type herdrReadyMsg struct {
	label       string
	session     string
	workspaceID string
	mode        string
	attach      bool
	insideHerdr bool
	newTab      bool
	bin         string
	wtPath      string
	wtBranch    string
}

func sanitizeHerdrSessionName(name string) string {
	return multiplexer.SanitizeHerdrSessionName(name)
}

func herdrInsideSession() bool {
	return strings.TrimSpace(os.Getenv("HERDR_SESSION")) != "" || strings.TrimSpace(os.Getenv("HERDR_SOCKET_PATH")) != ""
}

func buildHerdrWorkspaceInfoMessage(label, attachCommand string) string {
	return fmt.Sprintf("herdr workspace %q ready.\n\nSelect it in Herdr, or attach with:\n\n  %s", label, attachCommand)
}

func buildHerdrSessionInfoMessage(session string) string {
	quoted := multiplexer.ShellQuote(session)
	return fmt.Sprintf("herdr session ready.\n\nAttach with:\n\n  herdr session attach %s", quoted)
}

func buildHerdrNestedInfoMessage(session string) string {
	return fmt.Sprintf("herdr session %q was created outside the current Herdr session.\n\nHerdr cannot switch sessions from inside a running client.\n\nAttach from another terminal:\n\n  herdr session attach %s", session, multiplexer.ShellQuote(session))
}

func buildHerdrNestedWorkspaceInfoMessage(label, session string) string {
	return fmt.Sprintf("herdr workspace %q belongs to session %q, outside the current Herdr session.\n\nHerdr cannot switch sessions from inside a running client.\n\nAttach from another terminal:\n\n  herdr session attach %s", label, session, multiplexer.ShellQuote(session))
}

// herdrSessionIsAmbient reports whether the current Herdr client can display a
// target session. An empty target inherits the ambient session.
func herdrSessionIsAmbient(session string) bool {
	return session == "" || session == multiplexer.HerdrAmbientSession()
}

func (m *Model) attachHerdrCmd(msg herdrReadyMsg) tea.Cmd {
	h := &multiplexer.Herdr{Bin: msg.bin, Session: msg.session}
	args := h.AttachArgs()
	// #nosec G204 -- herdr binary and session come from user configuration
	c := m.commandRunner(m.ctx, args[0], args[1:]...)
	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	})
}

func (m *Model) openHerdrWorkspaceCmd(workspaceID string) tea.Cmd {
	session := strings.TrimSpace(os.Getenv("HERDR_SESSION"))
	insideHerdr := herdrInsideSession()
	return func() tea.Msg {
		h := &multiplexer.Herdr{Session: session}
		if err := h.FocusWorkspace(m.ctx, workspaceID); err != nil {
			return errMsg{err: err}
		}
		if insideHerdr {
			return refreshCompleteMsg{}
		}
		return herdrReadyMsg{
			session:     session,
			workspaceID: workspaceID,
			attach:      true,
			mode:        config.HerdrModeWorkspace,
		}
	}
}

// openHerdrSessionByNameCmd attaches to an active session discovered by the
// palette. name is the full session name, so no prefix is reapplied.
func (m *Model) openHerdrSessionByNameCmd(name string) tea.Cmd {
	session := name
	if herdrInsideSession() {
		m.showInfo(buildHerdrNestedInfoMessage(session), nil)
		return nil
	}
	return m.attachHerdrCmd(herdrReadyMsg{
		session: session,
		attach:  true,
		mode:    config.HerdrModeSession,
	})
}

func (m *Model) handleHerdrReady(msg herdrReadyMsg) tea.Cmd {
	if msg.mode == config.HerdrModeSession {
		if msg.insideHerdr {
			m.showInfo(buildHerdrNestedInfoMessage(msg.session), nil)
			return nil
		}
		if !msg.attach {
			m.showInfo(buildHerdrSessionInfoMessage(msg.session), nil)
			return nil
		}
	} else {
		if !msg.attach {
			h := &multiplexer.Herdr{Bin: msg.bin, Session: msg.session}
			m.showInfo(buildHerdrWorkspaceInfoMessage(msg.label, h.DisplayAttachCommand()), nil)
			return nil
		}
		if msg.insideHerdr {
			if !herdrSessionIsAmbient(msg.session) {
				m.showInfo(buildHerdrNestedWorkspaceInfoMessage(msg.label, msg.session), nil)
				return nil
			}
			if msg.workspaceID != "" {
				return m.focusHerdrWorkspaceCmd(msg.bin, msg.session, msg.workspaceID)
			}
		}
	}

	if msg.newTab {
		return m.openHerdrInNewTab(msg)
	}
	return m.attachHerdrCmd(msg)
}

func (m *Model) focusHerdrWorkspaceCmd(bin, session, workspaceID string) tea.Cmd {
	return func() tea.Msg {
		h := &multiplexer.Herdr{Bin: bin, Session: session}
		if err := h.FocusWorkspace(m.ctx, workspaceID); err != nil {
			return errMsg{err: err}
		}
		return refreshCompleteMsg{}
	}
}

// herdrAttachCommand renders an attach command with every argument quoted, so
// an executable installed under a path containing spaces survives shell
// interpretation by terminal launchers.
func herdrAttachCommand(bin, session string) string {
	h := &multiplexer.Herdr{Bin: bin, Session: session}
	args := h.AttachArgs()
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = multiplexer.ShellQuote(arg)
	}
	return strings.Join(quoted, " ")
}

func (m *Model) openHerdrInNewTab(msg herdrReadyMsg) tea.Cmd {
	command := herdrAttachCommand(msg.bin, msg.session)
	wt := &models.WorktreeInfo{Path: msg.wtPath, Branch: msg.wtBranch}
	return m.openTerminalTab(&config.CustomCommand{Command: command, Description: msg.label}, wt)
}

func (m *Model) openHerdrSession(customCmd *config.CustomCommand, wt *models.WorktreeInfo) tea.Cmd {
	if customCmd == nil || customCmd.Herdr == nil {
		return nil
	}

	bin, err := exec.LookPath("herdr")
	if err != nil {
		m.showInfo("herdr is not installed. Install it from https://herdr.dev to use this feature.", nil)
		return nil
	}

	cfg := customCmd.Herdr
	env := m.buildCommandEnvForWorktree(wt)
	label := strings.TrimSpace(expandWithEnv(cfg.SessionName, env))

	var nameErr error
	session := ""
	if cfg.Mode == config.HerdrModeSession {
		session, nameErr = multiplexer.HerdrResolvedName("session_name", label, fmt.Sprintf("%s%s", m.config.SessionPrefix, filepath.Base(wt.Path)))
		if nameErr == nil {
			label, nameErr = multiplexer.HerdrResolvedName("workspace label", "", filepath.Base(wt.Path))
		}
	} else {
		label, nameErr = multiplexer.HerdrResolvedName("session_name", label, m.config.SessionPrefix+filepath.Base(wt.Path))
		if nameErr == nil {
			configuredSession := strings.TrimSpace(expandWithEnv(cfg.HerdrSession, env))
			if session = sanitizeHerdrSessionName(configuredSession); configuredSession != "" && session == "" {
				nameErr = multiplexer.HerdrInvalidNameError("herdr_session", configuredSession)
			}
		}
	}
	if nameErr != nil {
		return func() tea.Msg { return errMsg{err: nameErr} }
	}

	windows := multiplexer.ResolveHerdrWindows(cfg.Windows, env, wt.Path)
	if customCmd.Container != nil {
		var wrapErr error
		windows, wrapErr = multiplexer.WrapWindowCommandsForContainer(windows, customCmd.Container, env)
		if wrapErr != nil {
			return func() tea.Msg { return errMsg{err: wrapErr} }
		}
	}

	insideHerdr := herdrInsideSession()
	m.debugf("Opening herdr %s %q (session %q)", cfg.Mode, label, session)

	return func() tea.Msg {
		return m.createHerdrLayout(bin, cfg, wt, label, session, windows, env, insideHerdr, customCmd.NewTab)
	}
}

func (m *Model) createHerdrLayout(bin string, cfg *config.HerdrCommand, wt *models.WorktreeInfo, label, session string, windows []multiplexer.ResolvedWindow, env map[string]string, insideHerdr, newTab bool) tea.Msg {
	ctx := m.ctx
	h := &multiplexer.Herdr{Bin: bin, Session: session}
	ready := herdrReadyMsg{
		label:       label,
		session:     session,
		mode:        cfg.Mode,
		attach:      cfg.Attach,
		insideHerdr: insideHerdr,
		newTab:      newTab,
		bin:         bin,
		wtPath:      wt.Path,
		wtBranch:    wt.Branch,
	}

	if cfg.Mode == config.HerdrModeSession {
		running := h.Running(ctx)
		if running {
			switch strings.ToLower(strings.TrimSpace(cfg.OnExists)) {
			case multiplexer.OnExistsKill:
				if err := h.StopSession(ctx); err != nil {
					return errMsg{err: err}
				}
			case multiplexer.OnExistsNew:
				session, err := multiplexer.UniqueHerdrSessionName(ctx, h.Bin, h.Session)
				if err != nil {
					return errMsg{err: err}
				}
				h.Session = session
				ready.session = session
			}
		}
	}

	if err := h.EnsureServer(ctx); err != nil {
		return errMsg{err: err}
	}

	if cfg.Mode == config.HerdrModeSession {
		// A session left running by a failed attempt may hold a partial layout,
		// so reuse the worktree workspace when it already exists rather than
		// assuming the layout completed.
		existing, found, err := h.FindWorkspaceByLabel(ctx, ready.label)
		if err != nil {
			return errMsg{err: err}
		}
		if found {
			ready.workspaceID = existing.ID
			ready.label = existing.Label
			return m.focusReusedHerdrWorkspace(ctx, h, cfg, ready, insideHerdr)
		}
	} else {
		if err := m.resolveHerdrWorkspaceTarget(ctx, h, cfg, &ready); err != nil {
			var reused herdrWorkspaceReused
			if errors.As(err, &reused) {
				return m.focusReusedHerdrWorkspace(ctx, h, cfg, ready, insideHerdr)
			}
			return errMsg{err: err}
		}
	}

	workspace, err := h.CreateWorkspace(ctx, ready.label, wt.Path, windows, env, cfg.Attach)
	if err != nil {
		return errMsg{err: err}
	}
	ready.workspaceID = workspace.ID
	ready.label = workspace.Label
	return ready
}

// herdrWorkspaceReused signals that an existing workspace satisfied on_exists
// without creating a new one.
type herdrWorkspaceReused struct{}

func (herdrWorkspaceReused) Error() string { return "herdr workspace reused" }

// focusReusedHerdrWorkspace focuses a reused workspace before the client
// attaches, unless Herdr already hosts the TUI or attaching is disabled.
func (m *Model) focusReusedHerdrWorkspace(ctx context.Context, h *multiplexer.Herdr, cfg *config.HerdrCommand, ready herdrReadyMsg, insideHerdr bool) tea.Msg {
	if !insideHerdr && cfg.Attach && ready.workspaceID != "" {
		if err := h.FocusWorkspace(ctx, ready.workspaceID); err != nil {
			return errMsg{err: err}
		}
	}
	return ready
}

func (m *Model) resolveHerdrWorkspaceTarget(ctx context.Context, h *multiplexer.Herdr, cfg *config.HerdrCommand, ready *herdrReadyMsg) error {
	existing, found, err := h.FindWorkspaceByLabel(ctx, ready.label)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	switch strings.ToLower(strings.TrimSpace(cfg.OnExists)) {
	case multiplexer.OnExistsKill:
		if err := h.CloseWorkspace(ctx, existing.ID); err != nil {
			return err
		}
		return nil
	case multiplexer.OnExistsNew:
		label, err := h.UniqueWorkspaceLabel(ctx, ready.label)
		if err != nil {
			return err
		}
		ready.label = label
		return nil
	default:
		ready.workspaceID = existing.ID
		return herdrWorkspaceReused{}
	}
}

func (m *Model) getHerdrWorkspaces() []multiplexer.HerdrWorkspace {
	if _, err := exec.LookPath("herdr"); err != nil {
		return nil
	}
	h := &multiplexer.Herdr{Session: strings.TrimSpace(os.Getenv("HERDR_SESSION"))}
	workspaces, err := h.Workspaces(m.ctx)
	if err != nil {
		return nil
	}
	prefixes := herdrPrefixCandidates(m.config.SessionPrefix)
	filtered := make([]multiplexer.HerdrWorkspace, 0, len(workspaces))
	for _, workspace := range workspaces {
		if len(prefixes) > 0 &&
			!hasAnyHerdrPrefix(workspace.Label, prefixes) &&
			!hasAnyHerdrPrefix(sanitizeHerdrSessionName(workspace.Label), prefixes) {
			continue
		}
		filtered = append(filtered, workspace)
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Label < filtered[j].Label })
	return filtered
}

// herdrSessionEntry pairs a discovered Herdr session with the label shown in
// the palette. Name keeps the full session name so attaching targets the same
// session, even when the prefix was matched in sanitised form.
type herdrSessionEntry struct {
	Name  string
	Label string
}

func (m *Model) getHerdrActiveSessions() []herdrSessionEntry {
	if _, err := exec.LookPath("herdr"); err != nil {
		return nil
	}
	sessions, err := multiplexer.ListHerdrSessions(m.ctx, "")
	if err != nil {
		return nil
	}
	prefixes := herdrPrefixCandidates(m.config.SessionPrefix)
	entries := make([]herdrSessionEntry, 0, len(sessions))
	for _, session := range sessions {
		if !session.Running || session.Default {
			continue
		}
		prefix, matched := matchingHerdrPrefix(session.Name, prefixes)
		if len(prefixes) > 0 && !matched {
			continue
		}
		label := strings.TrimPrefix(session.Name, prefix)
		if label != "" {
			entries = append(entries, herdrSessionEntry{Name: session.Name, Label: label})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}

func herdrPrefixCandidates(prefix string) []string {
	if prefix == "" {
		return nil
	}
	sanitized := multiplexer.SanitizeHerdrPrefix(prefix)
	if sanitized == "" || sanitized == prefix {
		return []string{prefix}
	}
	return []string{prefix, sanitized}
}

func hasAnyHerdrPrefix(name string, prefixes []string) bool {
	_, matched := matchingHerdrPrefix(name, prefixes)
	return matched
}

func matchingHerdrPrefix(name string, prefixes []string) (string, bool) {
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return prefix, true
		}
	}
	return "", false
}
