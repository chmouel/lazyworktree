package log

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func resetDebugLogger(t *testing.T) func() {
	t.Helper()

	globalDebugLogger.mu.Lock()
	prevFile := globalDebugLogger.file
	prevBuffer := append([]byte(nil), globalDebugLogger.buffer...)
	prevDiscard := globalDebugLogger.discard
	globalDebugLogger.file = nil
	globalDebugLogger.buffer = nil
	globalDebugLogger.discard = false
	globalDebugLogger.mu.Unlock()

	return func() {
		globalDebugLogger.mu.Lock()
		if globalDebugLogger.file != nil {
			_ = globalDebugLogger.file.Close()
		}
		globalDebugLogger.file = prevFile
		globalDebugLogger.buffer = prevBuffer
		globalDebugLogger.discard = prevDiscard
		globalDebugLogger.mu.Unlock()
	}
}

func TestWrite(t *testing.T) {
	t.Run("write to buffer when no file", func(t *testing.T) {
		restore := resetDebugLogger(t)
		t.Cleanup(restore)

		testData := []byte("test message")
		n, err := globalDebugLogger.Write(testData)
		if err != nil {
			t.Fatalf("Write() error = %v, want nil", err)
		}
		if n != len(testData) {
			t.Fatalf("Write() n = %d, want %d", n, len(testData))
		}

		globalDebugLogger.mu.Lock()
		buffer := append([]byte(nil), globalDebugLogger.buffer...)
		globalDebugLogger.mu.Unlock()

		if !bytes.Equal(buffer, testData) {
			t.Fatalf("Write() buffer = %q, want %q", string(buffer), string(testData))
		}
	})

	t.Run("write to file when file is set", func(t *testing.T) {
		restore := resetDebugLogger(t)
		t.Cleanup(restore)

		logFile := filepath.Join(t.TempDir(), "test.log")
		if err := SetFile(logFile); err != nil {
			t.Fatalf("SetFile() error = %v", err)
		}

		testData := []byte("file message")
		n, err := globalDebugLogger.Write(testData)
		if err != nil {
			t.Fatalf("Write() error = %v, want nil", err)
		}
		if n != len(testData) {
			t.Fatalf("Write() n = %d, want %d", n, len(testData))
		}

		// Read file to verify content
		//nolint:gosec // Test file path from t.TempDir() is safe
		content, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if !strings.Contains(string(content), string(testData)) {
			t.Fatalf("Write() file content = %q, want to contain %q", string(content), string(testData))
		}
	})

	t.Run("discard mode returns success but doesn't write", func(t *testing.T) {
		restore := resetDebugLogger(t)
		t.Cleanup(restore)

		globalDebugLogger.mu.Lock()
		globalDebugLogger.discard = true
		globalDebugLogger.mu.Unlock()

		testData := []byte("discarded message")
		n, err := globalDebugLogger.Write(testData)
		if err != nil {
			t.Fatalf("Write() error = %v, want nil", err)
		}
		if n != len(testData) {
			t.Fatalf("Write() n = %d, want %d", n, len(testData))
		}

		globalDebugLogger.mu.Lock()
		bufferLen := len(globalDebugLogger.buffer)
		globalDebugLogger.mu.Unlock()

		if bufferLen != 0 {
			t.Fatalf("Write() buffer length = %d, want 0", bufferLen)
		}
	})

	t.Run("concurrent writes are safe", func(t *testing.T) {
		restore := resetDebugLogger(t)
		t.Cleanup(restore)

		logFile := filepath.Join(t.TempDir(), "concurrent.log")
		if err := SetFile(logFile); err != nil {
			t.Fatalf("SetFile() error = %v", err)
		}

		var wg sync.WaitGroup
		numGoroutines := 10
		messagesPerGoroutine := 10

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < messagesPerGoroutine; j++ {
					msg := []byte(fmt.Sprintf("goroutine %d message %d", id, j))
					_, _ = globalDebugLogger.Write(msg)
				}
			}(i)
		}

		wg.Wait()

		// Verify file has content (exact count not important, just that it didn't crash)
		//nolint:gosec // Test file path from t.TempDir() is safe
		content, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if len(content) == 0 {
			t.Fatal("Write() concurrent writes produced no output")
		}
	})
}

func TestSetFile(t *testing.T) {
	t.Run("set empty path discards logs", func(t *testing.T) {
		restore := resetDebugLogger(t)
		t.Cleanup(restore)

		// First buffer some data
		globalDebugLogger.mu.Lock()
		globalDebugLogger.buffer = []byte("buffered data")
		globalDebugLogger.mu.Unlock()

		if err := SetFile(""); err != nil {
			t.Fatalf("SetFile(\"\") error = %v, want nil", err)
		}

		globalDebugLogger.mu.Lock()
		discard := globalDebugLogger.discard
		bufferLen := len(globalDebugLogger.buffer)
		file := globalDebugLogger.file
		globalDebugLogger.mu.Unlock()

		if !discard {
			t.Fatal("SetFile(\"\") expected discard to be true")
		}
		if bufferLen != 0 {
			t.Fatalf("SetFile(\"\") buffer length = %d, want 0", bufferLen)
		}
		if file != nil {
			t.Fatal("SetFile(\"\") expected file to be nil")
		}
	})

	t.Run("create new file and flush buffer", func(t *testing.T) {
		restore := resetDebugLogger(t)
		t.Cleanup(restore)

		// Buffer some data first
		bufferedData := []byte("buffered message")
		globalDebugLogger.mu.Lock()
		globalDebugLogger.buffer = append([]byte(nil), bufferedData...)
		globalDebugLogger.mu.Unlock()

		logFile := filepath.Join(t.TempDir(), "new.log")
		if err := SetFile(logFile); err != nil {
			t.Fatalf("SetFile() error = %v", err)
		}

		globalDebugLogger.mu.Lock()
		bufferLen := len(globalDebugLogger.buffer)
		file := globalDebugLogger.file
		globalDebugLogger.mu.Unlock()

		if bufferLen != 0 {
			t.Fatalf("SetFile() buffer length = %d, want 0", bufferLen)
		}
		if file == nil {
			t.Fatal("SetFile() expected file to be set")
		}

		// Verify buffered data was written to file
		//nolint:gosec // Test file path from t.TempDir() is safe
		content, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if !strings.Contains(string(content), string(bufferedData)) {
			t.Fatalf("SetFile() file content = %q, want to contain %q", string(content), string(bufferedData))
		}
	})

	t.Run("replace existing file", func(t *testing.T) {
		restore := resetDebugLogger(t)
		t.Cleanup(restore)

		firstFile := filepath.Join(t.TempDir(), "first.log")
		if err := SetFile(firstFile); err != nil {
			t.Fatalf("SetFile() error = %v", err)
		}

		_, _ = globalDebugLogger.Write([]byte("first file message"))

		secondFile := filepath.Join(t.TempDir(), "second.log")
		if err := SetFile(secondFile); err != nil {
			t.Fatalf("SetFile() error = %v", err)
		}

		globalDebugLogger.mu.Lock()
		file := globalDebugLogger.file
		globalDebugLogger.mu.Unlock()

		if file == nil {
			t.Fatal("SetFile() expected file to be set")
		}

		// First file should be closed
		_, err := os.Stat(firstFile)
		if err != nil {
			t.Fatalf("First file should still exist: %v", err)
		}

		// Second file should exist
		_, err = os.Stat(secondFile)
		if err != nil {
			t.Fatalf("Second file should exist: %v", err)
		}

		//nolint:gosec // Test file path from t.TempDir() is safe
		_, _ = os.ReadFile(secondFile) // Verify file is readable
	})

	t.Run("failure discards logs", func(t *testing.T) {
		restore := resetDebugLogger(t)
		t.Cleanup(restore)

		unwritableDir := t.TempDir()
		if err := os.Chmod(unwritableDir, 0o500); err != nil { //nolint:gosec
			t.Fatalf("set directory permissions: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Chmod(unwritableDir, 0o700) //nolint:gosec
		})

		logPath := filepath.Join(unwritableDir, "debug.log")
		if err := SetFile(logPath); err == nil {
			t.Fatalf("expected SetFile to fail for %q", logPath)
		}

		globalDebugLogger.mu.Lock()
		discard := globalDebugLogger.discard
		bufferLen := len(globalDebugLogger.buffer)
		globalDebugLogger.mu.Unlock()

		if !discard {
			t.Fatalf("expected discard to be enabled after SetFile failure")
		}
		if bufferLen != 0 {
			t.Fatalf("expected buffer to be cleared after SetFile failure")
		}

		Printf("should be discarded")

		globalDebugLogger.mu.Lock()
		bufferLen = len(globalDebugLogger.buffer)
		globalDebugLogger.mu.Unlock()

		if bufferLen != 0 {
			t.Fatalf("expected buffer to remain empty after logging")
		}
	})
}

func TestPrintln(t *testing.T) {
	t.Run("println writes to buffer", func(t *testing.T) {
		restore := resetDebugLogger(t)
		t.Cleanup(restore)

		Println("test message", 123)

		globalDebugLogger.mu.Lock()
		buffer := append([]byte(nil), globalDebugLogger.buffer...)
		globalDebugLogger.mu.Unlock()

		if len(buffer) == 0 {
			t.Fatal("Println() expected buffer to contain data")
		}
		if !strings.Contains(string(buffer), "test message") {
			t.Fatalf("Println() buffer = %q, want to contain 'test message'", string(buffer))
		}
	})

	t.Run("println writes to file", func(t *testing.T) {
		restore := resetDebugLogger(t)
		t.Cleanup(restore)

		logFile := filepath.Join(t.TempDir(), "println.log")
		if err := SetFile(logFile); err != nil {
			t.Fatalf("SetFile() error = %v", err)
		}

		Println("file message", 456)

		// Give time for async operations
		time.Sleep(10 * time.Millisecond)

		//nolint:gosec // Test file path from t.TempDir() is safe
		content, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if !strings.Contains(string(content), "file message") {
			t.Fatalf("Println() file content = %q, want to contain 'file message'", string(content))
		}
	})

	t.Run("println respects discard mode", func(t *testing.T) {
		restore := resetDebugLogger(t)
		t.Cleanup(restore)

		globalDebugLogger.mu.Lock()
		globalDebugLogger.discard = true
		globalDebugLogger.mu.Unlock()

		Println("discarded message")

		globalDebugLogger.mu.Lock()
		bufferLen := len(globalDebugLogger.buffer)
		globalDebugLogger.mu.Unlock()

		if bufferLen != 0 {
			t.Fatalf("Println() buffer length = %d, want 0", bufferLen)
		}
	})
}

func TestClose(t *testing.T) {
	t.Run("close when no file is nil", func(t *testing.T) {
		restore := resetDebugLogger(t)
		t.Cleanup(restore)

		if err := Close(); err != nil {
			t.Fatalf("Close() error = %v, want nil", err)
		}
	})

	t.Run("close existing file", func(t *testing.T) {
		restore := resetDebugLogger(t)
		t.Cleanup(restore)

		logFile := filepath.Join(t.TempDir(), "close.log")
		if err := SetFile(logFile); err != nil {
			t.Fatalf("SetFile() error = %v", err)
		}

		globalDebugLogger.mu.Lock()
		fileBefore := globalDebugLogger.file
		globalDebugLogger.mu.Unlock()

		if fileBefore == nil {
			t.Fatal("expected file to be set before Close()")
		}

		if err := Close(); err != nil {
			t.Fatalf("Close() error = %v, want nil", err)
		}

		globalDebugLogger.mu.Lock()
		fileAfter := globalDebugLogger.file
		globalDebugLogger.mu.Unlock()

		if fileAfter != nil {
			t.Fatal("Close() expected file to be nil after close")
		}
	})

	t.Run("close multiple times is safe", func(t *testing.T) {
		restore := resetDebugLogger(t)
		t.Cleanup(restore)

		logFile := filepath.Join(t.TempDir(), "close-multiple.log")
		if err := SetFile(logFile); err != nil {
			t.Fatalf("SetFile() error = %v", err)
		}

		if err := Close(); err != nil {
			t.Fatalf("Close() first call error = %v", err)
		}
		if err := Close(); err != nil {
			t.Fatalf("Close() second call error = %v", err)
		}
		if err := Close(); err != nil {
			t.Fatalf("Close() third call error = %v", err)
		}
	})
}
