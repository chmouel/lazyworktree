package theme

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
)

// DetectBackground attempts to detect the terminal background color.
// It returns the detected theme name (DefaultLight or DefaultDark) and an error if detection fails.
func DetectBackground(timeout time.Duration) (string, error) {
	// Open /dev/tty to communicate directly with the controlling terminal
	// This avoids issues where stdin/stdout are redirected (e.g. pipes)
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return DefaultDark(), err
	}
	defer func() {
		_ = tty.Close()
	}()

	fd := int(tty.Fd()) //#nosec G115 -- fd conversion is safe on supported platforms
	if !term.IsTerminal(fd) {
		return DefaultDark(), fmt.Errorf("not a terminal")
	}

	// Save current terminal state
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return DefaultDark(), err
	}
	defer func() {
		_ = term.Restore(fd, oldState)
	}()

	// Send OSC 11 query
	_, err = tty.WriteString("\x1b]11;?\x1b\\")
	if err != nil {
		return DefaultDark(), err
	}

	// Channel to receive the response
	responseCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		var buf []byte
		readBuf := make([]byte, 1)

		for {
			// Set a read deadline logic if needed, but we rely on the outer select for timeout.
			// However, TTY reads are blocking. To cleanly exit on timeout, we might need
			// non-blocking I/O or simply rely on the fact that the main goroutine will
			// return and close the program/tty eventually.
			// For a cleaner implementation, we could rely on SetReadDeadline if *os.File supported it for TTYs well,
			// or just let the goroutine leak in the rare timeout case (acceptable for a short-lived CLI command startup).

			n, err := tty.Read(readBuf)
			if err != nil {
				errCh <- err
				return
			}
			if n > 0 {
				buf = append(buf, readBuf[:n]...)

				// Check for terminators: ST (\x1b\) or BEL (\x07)
				if bytes.HasSuffix(buf, []byte("\x1b\\")) || bytes.HasSuffix(buf, []byte("\x07")) {
					responseCh <- string(buf)
					return
				}

				// Sanity check to prevent infinite accumulation
				if len(buf) > 1024 {
					errCh <- fmt.Errorf("response too long")
					return
				}
			}
		}
	}()

	select {
	case resp := <-responseCh:
		// Parse response: \x1b]11;rgb:rrrr/gggg/bbbb\x1b\\
		if !strings.HasPrefix(resp, "\x1b]11;rgb:") {
			return DefaultDark(), fmt.Errorf("invalid response format")
		}

		// Strip prefix and suffix
		content := strings.TrimPrefix(resp, "\x1b]11;rgb:")
		content = strings.TrimSuffix(content, "\x1b\\")
		content = strings.TrimSuffix(content, "\x07") // Some terminals use BEL instead of ST

		parts := strings.Split(content, "/")
		if len(parts) != 3 {
			return DefaultDark(), fmt.Errorf("invalid rgb format")
		}

		r, err := parseColorComponent(parts[0])
		if err != nil {
			return DefaultDark(), err
		}
		g, err := parseColorComponent(parts[1])
		if err != nil {
			return DefaultDark(), err
		}
		b, err := parseColorComponent(parts[2])
		if err != nil {
			return DefaultDark(), err
		}

		// Calculate luminance (standard formula)
		// Y = 0.299*R + 0.587*G + 0.114*B
		// Normalize to 0-1 range first (assuming 16-bit color values from some terminals, or 8-bit)
		// parseColorComponent handles 8-bit, 12-bit, 16-bit normalization to 0-65535

		luminance := (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 65535.0

		if luminance > 0.5 {
			return DefaultLight(), nil
		}
		return DefaultDark(), nil

	case <-time.After(timeout):
		return DefaultDark(), fmt.Errorf("timeout waiting for terminal response")
	case err := <-errCh:
		return DefaultDark(), err
	}
}

func parseColorComponent(s string) (int, error) {
	// Hex string, length varies (1 to 4 hex digits typically)
	// We want to normalize to 16-bit (0-65535)

	val, err := strconv.ParseUint(s, 16, 16)
	if err != nil {
		return 0, err
	}

	switch len(s) {
	case 1: // 4-bit (0-15) -> 16-bit
		return int(val * 0x1111), nil //nolint:gosec
	case 2: // 8-bit (0-255) -> 16-bit
		return int(val * 0x101), nil //nolint:gosec
	case 3: // 12-bit (0-4095) -> 16-bit
		// 0xFFF -> 0xFFFF
		// val / 4095 * 65535 ?
		// approximation: val << 4 | val >> 8
		return int(val*0x10 + val/0x100), nil //nolint:gosec
	case 4: // 16-bit
		return int(val), nil //nolint:gosec
	default:
		// Fallback for unexpected lengths, just treat as value if parses
		return int(val), nil //nolint:gosec
	}
}
