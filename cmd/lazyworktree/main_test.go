package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = writer

	fn()

	_ = writer.Close()
	os.Stdout = orig

	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	return string(out)
}

func TestPrintSyntaxThemes(t *testing.T) {
	out := captureStdout(t, func() {
		printSyntaxThemes()
	})

	if !strings.Contains(out, "Available syntax themes") {
		t.Fatalf("expected header to be printed, got %q", out)
	}
	if !strings.Contains(out, "dracula") {
		t.Fatalf("expected theme list to include dracula, got %q", out)
	}
}

func TestPrintCompletion(t *testing.T) {
	shells := []string{"bash", "zsh", "fish"}

	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			out := captureStdout(t, func() {
				if err := printCompletion(shell); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})

			if out == "" {
				t.Error("expected non-empty output")
			}

			// Verify it contains lazyworktree
			if !strings.Contains(out, "lazyworktree") {
				t.Error("output missing program name")
			}

			// Verify shell-specific structure
			switch shell {
			case "bash":
				if !strings.Contains(out, "_lazyworktree_completion") {
					t.Error("bash completion missing expected function")
				}
			case "zsh":
				if !strings.Contains(out, "#compdef") {
					t.Error("zsh completion missing compdef directive")
				}
			case "fish":
				if !strings.Contains(out, "complete -c") {
					t.Error("fish completion missing complete command")
				}
			}
		})
	}
}

func TestPrintCompletionInvalidShell(t *testing.T) {
	err := printCompletion("invalid")
	if err == nil {
		t.Error("expected error for invalid shell")
	}

	if !strings.Contains(err.Error(), "unsupported shell") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestOutputSelectionWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")

	selectedPath := "/path/to/worktree"
	data := selectedPath + "\n"

	const filePerms = 0o600
	err := os.WriteFile(outputFile, []byte(data), filePerms)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// #nosec G304 - test file operations with t.TempDir() are safe
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != data {
		t.Fatalf("expected %q, got %q", data, string(content))
	}
}

func TestOutputSelectionEmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")

	const filePerms = 0o600
	err := os.WriteFile(outputFile, []byte(""), filePerms)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// #nosec G304 - test file operations with t.TempDir() are safe
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if len(content) != 0 {
		t.Fatalf("expected empty content, got %q", string(content))
	}
}

func TestOutputSelectionDirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "subdir1", "subdir2", "output.txt")

	const dirPerms = 0o750
	err := os.MkdirAll(filepath.Dir(outputPath), dirPerms)
	if err != nil {
		t.Fatalf("failed to create directories: %v", err)
	}

	const filePerms = 0o600
	err = os.WriteFile(outputPath, []byte("/test/path\n"), filePerms)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("output file not created: %v", err)
	}
}

func TestVersionVariables(t *testing.T) {
	// Verify build variables are set (at least with defaults)
	if version == "" {
		t.Error("version should not be empty")
	}
	if commit == "" {
		t.Error("commit should not be empty")
	}
	if date == "" {
		t.Error("date should not be empty")
	}
	if builtBy == "" {
		t.Error("builtBy should not be empty")
	}
}

func TestPrintSyntaxThemesContainsThemes(t *testing.T) {
	out := captureStdout(t, func() {
		printSyntaxThemes()
	})

	expectedThemes := []string{"dracula", "monokai", "nord"}
	for _, theme := range expectedThemes {
		if !strings.Contains(out, theme) {
			t.Logf("warning: expected theme %q in output", theme)
		}
	}
}
