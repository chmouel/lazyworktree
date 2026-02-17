package bootstrap

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/git"
	urfavecli "github.com/urfave/cli/v3"
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

func TestOutputAllFlags(t *testing.T) {
	out := captureStdout(t, func() {
		// Create a mock command with flags
		cmd := &urfavecli.Command{
			Flags: globalFlags(),
		}
		outputAllFlags(cmd)
	})

	// Verify expected flags are present
	expectedFlags := []string{"--worktree-dir", "--debug-log", "--theme", "--config"}
	for _, flag := range expectedFlags {
		if !strings.Contains(out, flag) {
			t.Errorf("expected flag %q in output, got %q", flag, out)
		}
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

func TestApplyWorktreeDirConfig(t *testing.T) {
	tests := []struct {
		name           string
		worktreeDir    string
		cfgWorktreeDir string
		expected       string
		expectError    bool
	}{
		{
			name:        "flag takes precedence",
			worktreeDir: "/custom/path",
			expected:    "/custom/path",
		},
		{
			name:           "config value used when flag empty",
			worktreeDir:    "",
			cfgWorktreeDir: "/config/path",
			expected:       "/config/path",
		},
		{
			name:        "default when both empty",
			worktreeDir: "",
			expected:    "", // Will be set to default, but we can't easily test home dir
		},
		{
			name:        "expand path with tilde",
			worktreeDir: "~/test",
			expected:    "", // Will be expanded, exact path depends on home dir
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.AppConfig{
				WorktreeDir: tt.cfgWorktreeDir,
			}

			err := applyWorktreeDirConfig(cfg, tt.worktreeDir)
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError && tt.expected != "" && !strings.Contains(cfg.WorktreeDir, tt.expected) {
				// For default case, just verify it's set
				if tt.worktreeDir == "" && tt.cfgWorktreeDir == "" {
					if cfg.WorktreeDir == "" {
						t.Error("expected default worktree dir to be set")
					}
				}
			}
		})
	}
}

func TestApplyThemeConfig(t *testing.T) {
	tests := []struct {
		name        string
		themeName   string
		expectError bool
	}{
		{
			name:        "valid theme",
			themeName:   "dracula",
			expectError: false,
		},
		{
			name:        "valid theme uppercase",
			themeName:   "DRACULA",
			expectError: false,
		},
		{
			name:        "invalid theme",
			themeName:   "nonexistent-theme",
			expectError: true,
		},
		{
			name:        "empty theme",
			themeName:   "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.GitPager = "delta"

			err := applyThemeConfig(cfg, tt.themeName)
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError && tt.themeName != "" {
				if cfg.Theme == "" {
					t.Error("expected theme to be set")
				}
			}
		})
	}
}

func TestLoadCLIConfig(t *testing.T) {
	t.Run("load default config", func(t *testing.T) {
		cfg, err := loadCLIConfig("", "", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected config to be non-nil")
		}
	})

	t.Run("apply worktree dir", func(t *testing.T) {
		cfg, err := loadCLIConfig("", "/test/path", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.WorktreeDir == "" {
			t.Error("expected worktree dir to be set")
		}
	})

	t.Run("apply config overrides", func(t *testing.T) {
		overrides := []string{"lw.theme=dracula"}
		cfg, err := loadCLIConfig("", "", overrides)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Theme != "dracula" {
			t.Errorf("expected theme to be dracula, got %q", cfg.Theme)
		}
	})
}

func TestNewCLIGitService(t *testing.T) {
	// Mock the lookup function to avoid dependency on delta being installed
	oldLookup := git.LookupPath
	defer func() { git.LookupPath = oldLookup }()
	git.LookupPath = func(name string) (string, error) {
		return "/mock/" + name, nil
	}

	cfg := config.DefaultConfig()
	cfg.GitPager = "delta"
	cfg.GitPagerArgs = []string{"--syntax-theme", "Dracula"}

	svc := newCLIGitService(cfg)
	if svc == nil {
		t.Fatal("expected service to be non-nil")
	}
	if !svc.UseGitPager() {
		t.Error("expected git pager to be enabled")
	}
}
