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

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to read home dir: %v", err)
	}

	result, err := expandPath("~/worktrees")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(home, "worktrees")
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}

	t.Setenv("LW_TEST_DIR", "/tmp/lw")
	result, err = expandPath("$LW_TEST_DIR/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "/tmp/lw/path" {
		t.Fatalf("expected env expansion, got %q", result)
	}
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
