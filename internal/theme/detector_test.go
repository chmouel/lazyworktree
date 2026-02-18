package theme

import (
	"os"
	"testing"
	"time"

	"golang.org/x/term"
)

func TestParseColorComponent(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "4-bit low", input: "0", want: 0x0000, wantErr: false},
		{name: "4-bit mid", input: "8", want: 0x8888, wantErr: false},
		{name: "4-bit high", input: "f", want: 0xffff, wantErr: false},
		{name: "4-bit uppercase", input: "A", want: 0xaaaa, wantErr: false},
		{name: "8-bit low", input: "00", want: 0x0000, wantErr: false},
		{name: "8-bit mid", input: "80", want: 0x8080, wantErr: false},
		{name: "8-bit high", input: "ff", want: 0xffff, wantErr: false},
		{name: "8-bit uppercase", input: "FF", want: 0xffff, wantErr: false},
		{name: "12-bit low", input: "000", want: 0x0000, wantErr: false},
		{name: "12-bit mid", input: "800", want: 0x8008, wantErr: false}, // 0x800 * 16 + 0x800 / 256
		{name: "12-bit high", input: "fff", want: 0xffff, wantErr: false},
		{name: "16-bit low", input: "0000", want: 0x0000, wantErr: false},
		{name: "16-bit mid", input: "8080", want: 0x8080, wantErr: false},
		{name: "16-bit high", input: "ffff", want: 0xffff, wantErr: false},
		{name: "16-bit uppercase", input: "FFFF", want: 0xffff, wantErr: false},
		{name: "empty string", input: "", want: 0, wantErr: true},
		{name: "invalid hex", input: "xyz", want: 0, wantErr: true},
		{name: "invalid hex with numbers", input: "12g", want: 0, wantErr: true},
		{name: "mixed case", input: "aBcD", want: 0xabcd, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseColorComponent(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseColorComponent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseColorComponent() = %x, want %x", got, tt.want)
			}
		})
	}
}

// DetectBackground requires a terminal and user interaction, so we skip it in unit tests.
// However, we can mock the behavior if we refactor DetectBackground to take an interface or a reader/writer.
// For now, we just ensure it returns an error or default when not in a terminal (which is the case in CI/tests).
func TestDetectBackgroundNonTerminal(t *testing.T) {
	// Skip if we are running in a real terminal to avoid escape sequences in test output
	// Check /dev/tty first since that's what DetectBackground uses
	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		isTerm := term.IsTerminal(int(tty.Fd())) //#nosec G115 -- fd conversion is safe on supported platforms
		_ = tty.Close()
		if isTerm {
			t.Skip("skipping terminal detection test in interactive mode to avoid escape sequences")
		}
	}
	// Also check stdout as a fallback
	if term.IsTerminal(int(os.Stdout.Fd())) { //#nosec G115 -- fd conversion is safe on supported platforms
		t.Skip("skipping terminal detection test in interactive mode to avoid escape sequences")
	}
	// Check if TERM is set (indicates we're in a terminal environment)
	if os.Getenv("TERM") != "" && os.Getenv("CI") == "" {
		// In a real terminal but not CI - skip to avoid escape sequences
		t.Skip("skipping terminal detection test in interactive mode to avoid escape sequences")
	}

	// This test assumes it's running in a non-interactive environment (go test usually is).
	// If it somehow thinks it's a terminal, it might hang waiting for input if we aren't careful.
	// But DetectBackground checks term.IsTerminal(stdin).

	// In a test environment, stdin is typically not a terminal.
	// If it is (e.g. running go test manually), this test might fail or hang.
	// We'll set a very short timeout.

	// We can't easily force IsTerminal to be false without mocking syscalls or 'term' package.
	// But commonly `go test` redirects stdin from /dev/null or a pipe.

	theme, err := DetectBackground(10 * time.Millisecond)

	// We expect either "dracula" (default fallback) and an error, OR a valid theme if it miraculously works.
	// But most likely it returns error "stdin is not a terminal".

	if theme == "" {
		t.Error("DetectBackground returned empty theme")
	}

	// Just ensure it doesn't panic.
	if err == nil {
		// If no error, it means it somehow detected something or timed out gracefully.
		// That's acceptable.
	} else if theme != DefaultDark() {
		// Verify it returns the default dark theme on error
		t.Errorf("DetectBackground returned %q on error, expected default dark %q", theme, DefaultDark())
	}
}

func TestDetectBackgroundTimeout(t *testing.T) {
	// Skip if we are running in a real terminal to avoid escape sequences in test output
	// Check /dev/tty first since that's what DetectBackground uses
	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		isTerm := term.IsTerminal(int(tty.Fd())) //#nosec G115 -- fd conversion is safe on supported platforms
		_ = tty.Close()
		if isTerm {
			t.Skip("skipping terminal detection test in interactive mode to avoid escape sequences")
		}
	}
	// Also check stdout as a fallback
	if term.IsTerminal(int(os.Stdout.Fd())) { //#nosec G115 -- fd conversion is safe on supported platforms
		t.Skip("skipping terminal detection test in interactive mode to avoid escape sequences")
	}
	// Check if TERM is set (indicates we're in a terminal environment)
	if os.Getenv("TERM") != "" && os.Getenv("CI") == "" {
		// In a real terminal but not CI - skip to avoid escape sequences
		t.Skip("skipping terminal detection test in interactive mode to avoid escape sequences")
	}

	// Test with very short timeout to trigger timeout path
	theme, err := DetectBackground(1 * time.Millisecond)

	if theme == "" {
		t.Error("DetectBackground returned empty theme")
	}

	// Should return default dark theme
	if theme != DefaultDark() {
		t.Errorf("DetectBackground returned %q on timeout, expected default dark %q", theme, DefaultDark())
	}

	// Should have an error (timeout or not a terminal)
	if err == nil {
		t.Log("DetectBackground returned no error (might have detected in time)")
	}
}

func TestDetectBackgroundInvalidTTY(t *testing.T) {
	// Test that DetectBackground handles /dev/tty open failure gracefully
	// We can't easily mock this without refactoring, but we can test the error path
	// by using a non-existent path (though the function hardcodes /dev/tty)
	// For now, we just verify it doesn't panic and returns a default theme

	// Skip if we are running in a real terminal to avoid escape sequences in test output
	// Check /dev/tty first since that's what DetectBackground uses
	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		isTerm := term.IsTerminal(int(tty.Fd())) //#nosec G115 -- fd conversion is safe on supported platforms
		_ = tty.Close()
		if isTerm {
			t.Skip("skipping terminal detection test in interactive mode to avoid escape sequences")
		}
	}
	// Also check stdout as a fallback
	if term.IsTerminal(int(os.Stdout.Fd())) { //#nosec G115 -- fd conversion is safe on supported platforms
		t.Skip("skipping terminal detection test in interactive mode to avoid escape sequences")
	}
	// Check if TERM is set (indicates we're in a terminal environment)
	if os.Getenv("TERM") != "" && os.Getenv("CI") == "" {
		// In a real terminal but not CI - skip to avoid escape sequences
		t.Skip("skipping terminal detection test in interactive mode to avoid escape sequences")
	}

	// This will likely fail to open /dev/tty or detect it's not a terminal
	theme, err := DetectBackground(10 * time.Millisecond)

	if theme == "" {
		t.Error("DetectBackground returned empty theme")
	}

	// Should return default dark theme on any error
	if theme != DefaultDark() {
		t.Errorf("DetectBackground returned %q on error, expected default dark %q", theme, DefaultDark())
	}

	// Should have an error
	if err == nil {
		t.Log("DetectBackground returned no error (unexpected but acceptable)")
	}
}
