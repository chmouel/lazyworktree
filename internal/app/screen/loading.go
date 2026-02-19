package screen

import (
	"math/rand/v2"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/chmouel/lazyworktree/internal/theme"
	"github.com/muesli/reflow/wrap"
)

// TipCategory classifies loading tips by area of functionality.
type TipCategory string

// TipOperation identifies the operation currently running.
type TipOperation string

const (
	// TipCategoryNavigation groups navigation-focused tips.
	TipCategoryNavigation TipCategory = "navigation"
	// TipCategoryWorktree groups worktree-management tips.
	TipCategoryWorktree TipCategory = "worktree"
	// TipCategoryRepo groups repository-operation tips.
	TipCategoryRepo TipCategory = "repo"
	// TipCategoryTools groups tool-integration tips.
	TipCategoryTools TipCategory = "tools"
	// TipCategoryTips groups generic guidance tips.
	TipCategoryTips TipCategory = "tips"

	// TipOperationGeneral is used when no specific operation context is available.
	TipOperationGeneral TipOperation = "general"
	// TipOperationCreate is used during worktree creation flows.
	TipOperationCreate TipOperation = "create"
	// TipOperationRefresh is used during refresh operations.
	TipOperationRefresh TipOperation = "refresh"
	// TipOperationFetch is used during fetch operations.
	TipOperationFetch TipOperation = "fetch"
	// TipOperationSync is used during synchronisation operations.
	TipOperationSync TipOperation = "sync"
	// TipOperationPush is used during push operations.
	TipOperationPush TipOperation = "push"
	// TipOperationRerun is used during CI rerun operations.
	TipOperationRerun TipOperation = "rerun"
	// TipOperationCommand is used during command execution operations.
	TipOperationCommand TipOperation = "command"
)

// Tip defines a single loading tip.
type Tip struct {
	ID         string
	Text       string
	Category   TipCategory
	Operations []TipOperation
	Priority   int
	ShowInHelp bool
}

// LoadingTips is the built-in catalogue of loading tips.
var LoadingTips = []Tip{
	{ID: "help", Text: "Press '?' to open the help guide at any time.", Category: TipCategoryTips, Operations: []TipOperation{TipOperationGeneral}, Priority: 1, ShowInHelp: true},
	{ID: "search", Text: "Use '/' for incremental search in the focused pane.", Category: TipCategoryNavigation, Operations: []TipOperation{TipOperationGeneral, TipOperationRefresh}, Priority: 1, ShowInHelp: true},
	{ID: "filter", Text: "Press 'f' to filter the focused pane; press Esc to clear an active filter.", Category: TipCategoryNavigation, Operations: []TipOperation{TipOperationGeneral, TipOperationRefresh}, Priority: 1, ShowInHelp: true},
	{ID: "create", Text: "Press 'c' to create a worktree from a branch, PR/MR, issue, or custom flow.", Category: TipCategoryWorktree, Operations: []TipOperation{TipOperationGeneral, TipOperationCreate}, Priority: 2, ShowInHelp: true},
	{ID: "layout", Text: "Press 'L' to toggle between default and top pane layouts.", Category: TipCategoryNavigation, Operations: []TipOperation{TipOperationGeneral, TipOperationRefresh}, Priority: 1, ShowInHelp: true},
	{ID: "panes", Text: "Use '1', '2', '3', '[', ']', or Tab to move between panes quickly.", Category: TipCategoryNavigation, Operations: []TipOperation{TipOperationGeneral}, Priority: 1, ShowInHelp: true},
	{ID: "zoom", Text: "Press '=' to zoom the focused pane, then press '=' again to unzoom.", Category: TipCategoryNavigation, Operations: []TipOperation{TipOperationGeneral}, Priority: 1, ShowInHelp: false},
	{ID: "palette", Text: "Press ':' or Ctrl+P to open the Command Palette, including active tmux and zellij sessions.", Category: TipCategoryTools, Operations: []TipOperation{TipOperationGeneral, TipOperationCommand}, Priority: 1, ShowInHelp: true},
	{ID: "notes", Text: "Press 'i' to open worktree notes; existing notes open in the viewer first.", Category: TipCategoryWorktree, Operations: []TipOperation{TipOperationGeneral, TipOperationCreate}, Priority: 1, ShowInHelp: true},
	{ID: "taskboard", Text: "Press 'T' to open Taskboard and toggle markdown checkbox tasks across worktrees.", Category: TipCategoryWorktree, Operations: []TipOperation{TipOperationGeneral, TipOperationCreate}, Priority: 1, ShowInHelp: true},
	{ID: "sync", Text: "Use 'S' to synchronise with upstream (pull then push) when the worktree is clean.", Category: TipCategoryRepo, Operations: []TipOperation{TipOperationGeneral, TipOperationSync}, Priority: 2, ShowInHelp: true},
	{ID: "push", Text: "Use 'P' to push the current branch to its upstream; set upstream when prompted.", Category: TipCategoryRepo, Operations: []TipOperation{TipOperationGeneral, TipOperationPush}, Priority: 2, ShowInHelp: false},
	{ID: "fetch", Text: "Press 'R' to fetch all remotes and refresh upstream tracking information.", Category: TipCategoryRepo, Operations: []TipOperation{TipOperationGeneral, TipOperationFetch}, Priority: 2, ShowInHelp: false},
	{ID: "refresh", Text: "Press 'r' to refresh worktrees and, on GitHub/GitLab, refresh PR and CI data.", Category: TipCategoryRepo, Operations: []TipOperation{TipOperationGeneral, TipOperationRefresh}, Priority: 2, ShowInHelp: false},
	{ID: "ci", Text: "Press 'v' to open CI checks, Enter to open a job, and Ctrl+v to view logs in the pager.", Category: TipCategoryRepo, Operations: []TipOperation{TipOperationGeneral, TipOperationRefresh, TipOperationRerun}, Priority: 2, ShowInHelp: true},
	{ID: "status-jump", Text: "In the Status pane, use Ctrl+Left and Ctrl+Right to jump between folders.", Category: TipCategoryNavigation, Operations: []TipOperation{TipOperationGeneral, TipOperationRefresh}, Priority: 1, ShowInHelp: true},
	{ID: "run", Text: "Press '!' to run a command in the selected worktree with command history support.", Category: TipCategoryTools, Operations: []TipOperation{TipOperationGeneral, TipOperationCommand}, Priority: 1, ShowInHelp: false},
	{ID: "lazygit", Text: "Press 'g' to open LazyGit in the selected worktree.", Category: TipCategoryTools, Operations: []TipOperation{TipOperationGeneral}, Priority: 1, ShowInHelp: false},
	{ID: "jump", Text: "Press Enter on a worktree to jump there and change directory via shell integration.", Category: TipCategoryWorktree, Operations: []TipOperation{TipOperationGeneral, TipOperationCreate}, Priority: 1, ShowInHelp: false},
}

var lastLoadingTipID string

// TipOperationFromContext maps app loading context to a tip operation.
func TipOperationFromContext(loadingOperation, message string) TipOperation {
	switch strings.ToLower(strings.TrimSpace(loadingOperation)) {
	case string(TipOperationPush):
		return TipOperationPush
	case string(TipOperationSync):
		return TipOperationSync
	case string(TipOperationRerun):
		return TipOperationRerun
	}

	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "refresh"):
		return TipOperationRefresh
	case strings.Contains(lower, "fetch"):
		return TipOperationFetch
	case strings.Contains(lower, "push"):
		return TipOperationPush
	case strings.Contains(lower, "sync") || strings.Contains(lower, "synchron") || strings.Contains(lower, "updating"):
		return TipOperationSync
	case strings.Contains(lower, "create"):
		return TipOperationCreate
	case strings.Contains(lower, "running"):
		return TipOperationCommand
	default:
		return TipOperationGeneral
	}
}

// SelectLoadingTip selects a contextual tip and avoids immediate repeats when possible.
func SelectLoadingTip(operation TipOperation, previousTipID string) Tip {
	if len(LoadingTips) == 0 {
		return Tip{}
	}

	contextual := make([]Tip, 0, len(LoadingTips))
	fallback := make([]Tip, 0, len(LoadingTips))

	for _, tip := range LoadingTips {
		fallback = append(fallback, tip)
		if slices.Contains(tip.Operations, operation) {
			contextual = append(contextual, tip)
		}
	}

	pool := contextual
	if len(pool) == 0 {
		pool = fallback
	}

	if len(pool) > 1 && previousTipID != "" {
		filtered := make([]Tip, 0, len(pool)-1)
		for _, tip := range pool {
			if tip.ID != previousTipID {
				filtered = append(filtered, tip)
			}
		}
		if len(filtered) > 0 {
			pool = filtered
		}
	}

	return pool[rand.IntN(len(pool))] //nolint:gosec
}

// HelpTips returns curated tips for the help screen.
func HelpTips() []string {
	lines := make([]string, 0, len(LoadingTips))
	for _, tip := range LoadingTips {
		if tip.ShowInHelp {
			lines = append(lines, "- "+tip.Text)
		}
	}
	return lines
}

// LoadingScreen displays a modal with a spinner and a contextual tip.
type LoadingScreen struct {
	Message        string
	FrameIdx       int
	BorderColorIdx int
	Tip            string
	TipID          string
	Operation      TipOperation
	Thm            *theme.Theme
	SpinnerFrames  []string
	ShowIcons      bool
}

// DefaultSpinnerFrames returns the text-only spinner frames.
func DefaultSpinnerFrames() []string {
	return []string{"...", ".. ", ".  "}
}

// NewLoadingScreen creates a loading modal with the given message.
// spinnerFrames should be provided by the caller; if nil, text fallback is used.
func NewLoadingScreen(message string, operation TipOperation, thm *theme.Theme, spinnerFrames []string, showIcons bool) *LoadingScreen {
	frames := spinnerFrames
	if len(frames) == 0 {
		frames = DefaultSpinnerFrames()
	}

	tip := SelectLoadingTip(operation, lastLoadingTipID)
	lastLoadingTipID = tip.ID

	return &LoadingScreen{
		Message:       message,
		Tip:           tip.Text,
		TipID:         tip.ID,
		Operation:     operation,
		Thm:           thm,
		SpinnerFrames: frames,
		ShowIcons:     showIcons,
	}
}

// Type returns the screen type.
func (s *LoadingScreen) Type() Type {
	return TypeLoading
}

// Update handles key events. Loading screen does not respond to keys.
func (s *LoadingScreen) Update(msg tea.KeyMsg) (Screen, tea.Cmd) {
	// Loading screen ignores key input
	return s, nil
}

// loadingBorderColors returns the colour cycle for the pulsing border.
func (s *LoadingScreen) loadingBorderColors() []lipgloss.Color {
	return []lipgloss.Color{
		s.Thm.Accent,
		s.Thm.SuccessFg,
		s.Thm.WarnFg,
		s.Thm.Accent,
	}
}

// LoadingBorderColours exposes the border colours for tests.
func (s *LoadingScreen) LoadingBorderColours() []lipgloss.Color {
	return s.loadingBorderColors()
}

// Tick advances the loading animation (spinner frame and border colour).
func (s *LoadingScreen) Tick() {
	s.FrameIdx = (s.FrameIdx + 1) % len(s.SpinnerFrames)
	colours := s.loadingBorderColors()
	s.BorderColorIdx = (s.BorderColorIdx + 1) % len(colours)
}

func formatTipBody(text string, width, maxLines int) string {
	if width <= 0 || maxLines <= 0 {
		return ""
	}

	wrapped := wrap.String(text, width)
	lines := strings.Split(wrapped, "\n")
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}

	lines = lines[:maxLines]
	truncateWidth := width - 1
	if truncateWidth < 1 {
		truncateWidth = 1
	}
	truncatedLine := ansi.Truncate(lines[maxLines-1], truncateWidth, "")
	lines[maxLines-1] = truncatedLine + "…"
	return strings.Join(lines, "\n")
}

// View renders the loading modal with spinner, message, and a contextual tip.
func (s *LoadingScreen) View() string {
	width := 60
	height := 10

	colours := s.loadingBorderColors()
	borderColour := colours[s.BorderColorIdx%len(colours)]

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColour).
		Padding(1, 2).
		Width(width).
		Height(height)

	spinnerFrame := s.SpinnerFrames[s.FrameIdx%len(s.SpinnerFrames)]
	spinnerStyle := lipgloss.NewStyle().
		Foreground(s.Thm.Accent).
		Bold(true)

	messageStyle := lipgloss.NewStyle().
		Foreground(s.Thm.TextFg).
		Bold(true)

	separatorStyle := lipgloss.NewStyle().Foreground(s.Thm.BorderDim)
	separator := separatorStyle.Render(strings.Repeat("-", width-6))

	tipLabelStyle := lipgloss.NewStyle().
		Foreground(s.Thm.Cyan).
		Bold(true)
	tipStyle := lipgloss.NewStyle().
		Foreground(s.Thm.MutedFg).
		Italic(true)

	tipLabel := "Tip"
	if s.ShowIcons {
		tipLabel = labelWithIcon(UIIconTip, "Tip", s.ShowIcons)
		tipLabel = strings.TrimSpace(tipLabel)
	}
	tipBody := formatTipBody(s.Tip, width-8, 2)

	content := lipgloss.JoinVertical(lipgloss.Center,
		spinnerStyle.Render(spinnerFrame),
		"",
		messageStyle.Render(s.Message),
		separator,
		tipLabelStyle.Render(tipLabel+":"),
		tipStyle.Render(tipBody),
	)

	return boxStyle.Render(content)
}

// SetTheme updates the theme for this screen.
func (s *LoadingScreen) SetTheme(thm *theme.Theme) {
	s.Thm = thm
}

// SetSpinnerFrames updates the spinner frames.
func (s *LoadingScreen) SetSpinnerFrames(frames []string) {
	if len(frames) > 0 {
		s.SpinnerFrames = frames
	}
}
