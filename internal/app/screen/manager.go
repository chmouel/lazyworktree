package screen

// Manager handles screen state and provides a stack-based interface for modal overlays.
type Manager struct {
	current Screen
	stack   []Screen
}

// NewManager creates a new screen manager.
func NewManager() *Manager {
	return &Manager{
		stack: make([]Screen, 0),
	}
}

// Push adds a screen to the stack and sets it as the current screen.
func (m *Manager) Push(s Screen) {
	if s == nil {
		return
	}
	if m.current != nil {
		m.stack = append(m.stack, m.current)
	}
	m.current = s
}

// Pop removes the current screen and restores the previous one.
// Returns the screen that was removed, or nil if no screen was active.
func (m *Manager) Pop() Screen {
	removed := m.current
	if len(m.stack) > 0 {
		m.current = m.stack[len(m.stack)-1]
		m.stack = m.stack[:len(m.stack)-1]
	} else {
		m.current = nil
	}
	return removed
}

// Current returns the currently active screen, or nil if none.
func (m *Manager) Current() Screen {
	return m.current
}

// IsActive returns true if there is a screen currently displayed.
func (m *Manager) IsActive() bool {
	return m.current != nil
}

// Type returns the type of the current screen, or TypeNone if no screen is active.
func (m *Manager) Type() Type {
	if m.current == nil {
		return TypeNone
	}
	return m.current.Type()
}

// Clear removes all screens from the stack.
func (m *Manager) Clear() {
	m.current = nil
	m.stack = m.stack[:0]
}

// Set replaces the current screen without affecting the stack.
// This is useful for replacing the current screen without pushing/popping.
func (m *Manager) Set(s Screen) {
	m.current = s
}

// StackDepth returns the number of screens in the stack (excluding current).
func (m *Manager) StackDepth() int {
	return len(m.stack)
}
