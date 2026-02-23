package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestOverlayPopupPadsToBaseWidth(t *testing.T) {
	t.Parallel()

	// Base view: 20-char wide lines (simulating a box with borders).
	baseLine := strings.Repeat("X", 20)
	base := strings.Join([]string{baseLine, baseLine, baseLine, baseLine, baseLine}, "\n")

	// Small popup (6 chars) overlaid starting at row 1.
	popup := "HELLO!"

	m := &Model{}
	result := m.overlayPopup(base, popup, 1)

	lines := strings.Split(result, "\n")
	for i, line := range lines {
		w := lipgloss.Width(line)
		assert.Equal(t, 20, w, "line %d should be padded to base width 20, got %d", i, w)
	}
}

func TestOverlayPopupPreservesBaseWhenEmpty(t *testing.T) {
	t.Parallel()
	m := &Model{}

	assert.Equal(t, "hello", m.overlayPopup("hello", "", 0))
	assert.Empty(t, m.overlayPopup("", "popup", 0))
}

func TestOverlayPopupCentresPopup(t *testing.T) {
	t.Parallel()

	// 20-char base, 4-char popup => leftPad = (20-4)/2 = 8
	baseLine := strings.Repeat(".", 20)
	base := strings.Join([]string{baseLine, baseLine, baseLine}, "\n")
	popup := "ABCD"

	m := &Model{}
	result := m.overlayPopup(base, popup, 1)

	lines := strings.Split(result, "\n")
	// Row 0 is untouched.
	assert.Equal(t, baseLine, lines[0])
	// Row 1 has the popup centred: 8 dots + ABCD + 8 dots = 20.
	assert.Contains(t, lines[1], "ABCD")
	assert.Equal(t, 20, lipgloss.Width(lines[1]))
}
