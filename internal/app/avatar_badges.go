package app

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/models"
)

const (
	avatarStateFetching = "fetching"
	avatarStateLoaded   = "loaded"
	avatarStateError    = "error"

	kittyChunkSize       = 4096
	kittyPlaceholderRune = "\U0010EEEE"
	kittyCombiningZero   = "\u0305"
	kittyCombiningOne    = "\u030d"
)

type avatarRuntimeState struct {
	image      *services.AvatarImage
	status     string
	err        string
	registered bool
}

func (m *Model) avatarBadgesEnabled() bool {
	if m == nil || m.config == nil || m.config.DisablePR {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(m.config.AvatarBadges)) {
	case "never":
		return false
	case "always":
		return true
	default:
		return kittyCompatibleTerminal()
	}
}

func kittyCompatibleTerminal() bool {
	env := func(key string) string {
		return strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	}
	switch {
	case env("KITTY_WINDOW_ID") != "":
		return true
	case env("WEZTERM_EXECUTABLE") != "":
		return true
	case strings.Contains(env("TERM_PROGRAM"), "kitty"):
		return true
	case strings.Contains(env("TERM_PROGRAM"), "wezterm"):
		return true
	case strings.Contains(env("TERM_PROGRAM"), "ghostty"):
		return true
	case strings.Contains(env("TERM"), "xterm-kitty"):
		return true
	default:
		return false
	}
}

func (m *Model) queuePRAvatarFetches() tea.Cmd {
	if !m.avatarBadgesEnabled() || m.avatarCache == nil {
		return nil
	}
	cmds := make([]tea.Cmd, 0)
	for _, wt := range m.state.data.worktrees {
		if wt == nil || wt.PR == nil {
			continue
		}
		url := strings.TrimSpace(wt.PR.AuthorAvatarURL)
		if url == "" {
			continue
		}
		if m.avatarStates == nil {
			m.avatarStates = make(map[string]*avatarRuntimeState)
		}
		if state, ok := m.avatarStates[url]; ok && state.status != "" {
			continue
		}
		m.avatarStates[url] = &avatarRuntimeState{status: avatarStateFetching}
		cmds = append(cmds, m.fetchAvatarCmd(url))
	}
	return tea.Batch(cmds...)
}

func (m *Model) fetchAvatarCmd(rawURL string) tea.Cmd {
	return func() tea.Msg {
		image, err := m.avatarCache.Fetch(m.ctx, rawURL)
		return avatarLoadedMsg{url: rawURL, image: image, err: err}
	}
}

func (m *Model) handleAvatarLoaded(msg avatarLoadedMsg) (tea.Model, tea.Cmd) {
	if m.avatarStates == nil {
		m.avatarStates = make(map[string]*avatarRuntimeState)
	}
	state := &avatarRuntimeState{}
	if msg.err != nil {
		state.status = avatarStateError
		state.err = msg.err.Error()
		m.avatarStates[msg.url] = state
		m.debugf("avatar badge fetch failed for %s: %v", msg.url, msg.err)
		return m, nil
	}
	if msg.image == nil || len(msg.image.PNG) == 0 {
		state.status = avatarStateError
		state.err = "avatar image is empty"
		m.avatarStates[msg.url] = state
		m.debugf("avatar badge fetch returned empty image for %s", msg.url)
		return m, nil
	}

	state.status = avatarStateLoaded
	state.image = msg.image
	m.avatarStates[msg.url] = state
	return m, tea.Sequence(
		tea.Raw(kittyRegisterAvatar(msg.image)),
		func() tea.Msg { return avatarRegisteredMsg{url: msg.url} },
	)
}

func (m *Model) handleAvatarRegistered(msg avatarRegisteredMsg) (tea.Model, tea.Cmd) {
	state, ok := m.avatarStates[msg.url]
	if !ok || state.status != avatarStateLoaded || state.image == nil {
		return m, nil
	}
	state.registered = true
	if wt := m.selectedWorktree(); wt != nil && wt.PR != nil && wt.PR.AuthorAvatarURL == msg.url {
		m.infoContent = m.buildInfoContent(wt)
	}
	return m, nil
}

func (m *Model) renderAvatarBadge(pr *models.PRInfo) string {
	if pr == nil || !m.avatarBadgesEnabled() {
		return ""
	}
	url := strings.TrimSpace(pr.AuthorAvatarURL)
	if url == "" || m.avatarStates == nil {
		return ""
	}
	state, ok := m.avatarStates[url]
	if !ok || state.status != avatarStateLoaded || state.image == nil || !state.registered {
		return ""
	}
	return kittyAvatarPlaceholder(kittyImageID(state.image.Key))
}

func kittyRegisterAvatar(image *services.AvatarImage) string {
	if image == nil || len(image.PNG) == 0 {
		return ""
	}
	imageID := kittyImageID(image.Key)
	data := base64.StdEncoding.EncodeToString(image.PNG)
	var b strings.Builder
	for start := 0; start < len(data); start += kittyChunkSize {
		end := min(start+kittyChunkSize, len(data))
		more := 0
		if end < len(data) {
			more = 1
		}
		if start == 0 {
			fmt.Fprintf(&b, "\x1b_Ga=T,f=100,i=%d,c=2,r=1,U=1,q=2,m=%d;%s\x1b\\", imageID, more, data[start:end])
		} else {
			fmt.Fprintf(&b, "\x1b_Gm=%d;%s\x1b\\", more, data[start:end])
		}
	}
	return wrapKittyGraphicsSequence(b.String())
}

func wrapKittyGraphicsSequence(seq string) string {
	if seq == "" || strings.TrimSpace(os.Getenv("TMUX")) == "" {
		return seq
	}
	return "\x1bPtmux;" + strings.ReplaceAll(seq, "\x1b", "\x1b\x1b") + "\x1b\\"
}

func kittyAvatarPlaceholder(imageID uint32) string {
	r := (imageID >> 16) & 0xff
	g := (imageID >> 8) & 0xff
	b := imageID & 0xff
	return fmt.Sprintf(
		"\x1b[38;2;%d;%d;%dm%s%s%s%s%s%s\x1b[39m",
		r, g, b,
		kittyPlaceholderRune, kittyCombiningZero, kittyCombiningZero,
		kittyPlaceholderRune, kittyCombiningZero, kittyCombiningOne,
	)
}

func kittyImageID(key string) uint32 {
	sum := sha256.Sum256([]byte(key))
	id := uint32(sum[0])<<16 | uint32(sum[1])<<8 | uint32(sum[2])
	if id == 0 {
		return 1
	}
	return id
}
