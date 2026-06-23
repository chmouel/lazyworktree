package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/git"
	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appiCli "github.com/urfave/cli/v3"
)

func TestDoctorJSONReportsRepository(t *testing.T) {
	repoRoot, worktreeRoot, _, _ := initMachineTestRepo(t)

	output, errOutput, err := runMachineCommand(t, repoRoot, []string{
		"lazyworktree",
		"--worktree-dir", worktreeRoot,
		"doctor",
		"--json",
	})
	require.NoError(t, err, errOutput)

	var payload doctorJSON
	require.NoError(t, json.Unmarshal(output, &payload))
	assert.True(t, payload.Tools.Git.Available)
	assert.Equal(t, normalizePathForTest(t, repoRoot), payload.Repository.GitTopLevel)
	assert.Equal(t, 2, payload.Repository.WorktreeCount)
	assert.True(t, payload.Checks.CanListWorktrees)
}

func TestWorktreesResolveAndNotesGetJSON(t *testing.T) {
	repoRoot, worktreeRoot, featurePath, gitSvc := initMachineTestRepo(t)
	_ = resolveRepoKeyForTest(t, repoRoot, gitSvc)

	resolveOutput, errOutput, err := runMachineCommand(t, repoRoot, []string{
		"lazyworktree",
		"--worktree-dir", worktreeRoot,
		"worktrees",
		"resolve",
		"--name", "feature",
		"--json",
	})
	require.NoError(t, err, errOutput)

	var resolved machineWorktreeResolveJSON
	require.NoError(t, json.Unmarshal(resolveOutput, &resolved))
	assert.Equal(t, normalizePathForTest(t, featurePath), resolved.Worktree.Path)
	assert.Equal(t, "feature", resolved.Worktree.Name)
	assert.Equal(t, "branch", resolved.ResolvedBy)

	noteOutput, noteErrOutput, err := runMachineCommand(t, repoRoot, []string{
		"lazyworktree",
		"--worktree-dir", worktreeRoot,
		"notes",
		"get",
		"feature",
		"--json",
	})
	require.NoError(t, err, noteErrOutput)

	var note noteShowJSON
	require.NoError(t, json.Unmarshal(noteOutput, &note))
	assert.Equal(t, normalizePathForTest(t, featurePath), note.Path)
	assert.Equal(t, "feature", note.WorktreeName)
	assert.Empty(t, note.Note)
}

func TestWorktreesResolveNormalizesTrailingSlashInputs(t *testing.T) {
	repoRoot, worktreeRoot, featurePath, _ := initMachineTestRepo(t)

	for _, tc := range []struct {
		args           []string
		wantResolvedBy string
	}{
		{
			args: []string{
				"lazyworktree",
				"--worktree-dir", worktreeRoot,
				"worktrees",
				"resolve",
				"--cwd", featurePath + string(filepath.Separator),
				"--json",
			},
			wantResolvedBy: "cwd",
		},
		{
			args: []string{
				"lazyworktree",
				"--worktree-dir", worktreeRoot,
				"worktrees",
				"resolve",
				"--path", featurePath + string(filepath.Separator),
				"--json",
			},
			wantResolvedBy: "path",
		},
	} {
		output, errOutput, err := runMachineCommand(t, repoRoot, tc.args)
		require.NoError(t, err, errOutput)

		var resolved machineWorktreeResolveJSON
		require.NoError(t, json.Unmarshal(output, &resolved))
		assert.Equal(t, normalizePathForTest(t, featurePath), resolved.Worktree.Path)
		assert.Equal(t, tc.wantResolvedBy, resolved.ResolvedBy)
	}
}

func TestWorktreesContextUsesConfiguredAgentRoots(t *testing.T) {
	repoRoot, worktreeRoot, featurePath, _ := initMachineTestRepo(t)

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	claudeRoot := filepath.Join(tempHome, "custom-claude")
	sessionDir := filepath.Join(claudeRoot, "project-a")
	sessionPath := filepath.Join(sessionDir, "session-1.jsonl")
	require.NoError(t, os.MkdirAll(sessionDir, 0o750))

	ts := time.Now().UTC().Format(time.RFC3339Nano)
	lines := []map[string]any{
		{
			"type":      "user",
			"cwd":       featurePath,
			"timestamp": ts,
			"message": map[string]any{
				"role":    "user",
				"content": "Inspect machine output",
			},
		},
		{
			"type":      "assistant",
			"cwd":       featurePath,
			"timestamp": ts,
			"message": map[string]any{
				"role":  "assistant",
				"model": "claude-sonnet-4",
				"content": []map[string]any{
					{"type": "tool_use", "name": "Read", "input": map[string]any{"file_path": filepath.Join(featurePath, "README.md")}},
				},
			},
		},
	}
	writeJSONLLinesForTest(t, sessionPath, lines...)

	configPath := filepath.Join(tempHome, "lazyworktree.yml")
	configBody := []byte("agent_sessions:\n  claude_root: " + claudeRoot + "\n")
	require.NoError(t, os.WriteFile(configPath, configBody, 0o600))

	output, errOutput, err := runMachineCommand(t, repoRoot, []string{
		"lazyworktree",
		"--config-file", configPath,
		"--worktree-dir", worktreeRoot,
		"worktrees",
		"context",
		"feature",
		"--include", "agents",
		"--json",
	})
	require.NoError(t, err, errOutput)

	var payload machineWorktreeContextJSON
	require.NoError(t, json.Unmarshal(output, &payload))
	require.Len(t, payload.AgentSessions, 1)
	assert.Equal(t, "session-1", payload.AgentSessions[0].ID)
	assert.Equal(t, "claude", payload.AgentSessions[0].Agent)
	assert.Equal(t, "feature", payload.Worktree.Name)
}

func initMachineTestRepo(t *testing.T) (string, string, string, *git.Service) {
	t.Helper()

	tempDir := t.TempDir()
	repoRoot := filepath.Join(tempDir, "repo")
	worktreeRoot := filepath.Join(tempDir, "worktrees")
	featurePath := filepath.Join(worktreeRoot, "feature")

	require.NoError(t, os.MkdirAll(repoRoot, 0o750))
	require.NoError(t, os.MkdirAll(worktreeRoot, 0o750))

	runGit(t, repoRoot, "init")
	runGit(t, repoRoot, "config", "user.name", "Lazy Worktree Test")
	runGit(t, repoRoot, "config", "user.email", "lazyworktree@example.com")
	runGit(t, repoRoot, "config", "commit.gpgsign", "false")
	runGit(t, repoRoot, "branch", "-m", "main")
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("hello\n"), 0o600))
	runGit(t, repoRoot, "add", "README.md")
	runGit(t, repoRoot, "commit", "-m", "initial commit")
	runGit(t, repoRoot, "branch", "feature")
	runGit(t, repoRoot, "worktree", "add", featurePath, "feature")

	repoRoot = normalizePathForTest(t, repoRoot)
	worktreeRoot = normalizePathForTest(t, worktreeRoot)
	featurePath = normalizePathForTest(t, featurePath)

	gitSvc := git.NewService(nil, nil)
	return repoRoot, worktreeRoot, featurePath, gitSvc
}

func resolveRepoKeyForTest(t *testing.T, cwd string, gitSvc *git.Service) string {
	t.Helper()

	oldWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(cwd))
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	return gitSvc.ResolveRepoName(context.Background())
}

func normalizePathForTest(t *testing.T, path string) string {
	t.Helper()

	normalized, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return normalized
}

func runGit(t *testing.T, cwd string, args ...string) {
	t.Helper()
	// #nosec G204 -- test helper executes controlled git commands against temp repositories.
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func runMachineCommand(t *testing.T, cwd string, args []string) ([]byte, string, error) {
	t.Helper()

	app := &appiCli.Command{
		Name:  "lazyworktree",
		Usage: "A TUI tool to manage git worktrees",
		Flags: globalFlags(),
		Commands: []*appiCli.Command{
			doctorCommand(),
			worktreesCommand(),
			notesCommand(),
		},
	}

	oldWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(cwd))
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	outR, outW, err := os.Pipe()
	require.NoError(t, err)
	errR, errW, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = outW
	os.Stderr = errW
	t.Cleanup(func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = outW.Close()
		_ = outR.Close()
		_ = errW.Close()
		_ = errR.Close()
	})

	runErr := app.Run(context.Background(), args)

	require.NoError(t, outW.Close())
	require.NoError(t, errW.Close())

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	_, _ = io.Copy(&outBuf, outR)
	_, _ = io.Copy(&errBuf, errR)

	return outBuf.Bytes(), errBuf.String(), runErr
}

func writeJSONLLinesForTest(t *testing.T, path string, entries ...map[string]any) {
	t.Helper()

	var payload bytes.Buffer
	for _, entry := range entries {
		line, err := json.Marshal(entry)
		require.NoError(t, err)
		payload.Write(line)
		payload.WriteByte('\n')
	}

	require.NoError(t, os.WriteFile(path, payload.Bytes(), 0o600))
}

func TestBuildNoteJSONSupportsLegacySharedKey(t *testing.T) {
	cfg := &config.AppConfig{
		WorktreeDir:       "/tmp/worktrees",
		WorktreeNotesPath: "/tmp/shared-notes.json",
	}
	wtPath := "/tmp/worktrees/repo/feature"
	note := models.WorktreeNote{Note: "legacy", UpdatedAt: 1}

	got := buildNoteJSON(cfg, "repo/name", map[string]models.WorktreeNote{
		filepath.Clean(wtPath): note,
	}, &models.WorktreeInfo{Path: wtPath})

	require.NotNil(t, got)
	assert.Equal(t, "legacy", got.Note)
}

func TestWriteMaybeJSONErrorEnvelope(t *testing.T) {
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	outR, outW, err := os.Pipe()
	require.NoError(t, err)
	errR, errW, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = outW
	os.Stderr = errW
	t.Cleanup(func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = outW.Close()
		_ = outR.Close()
		_ = errW.Close()
		_ = errR.Close()
	})

	runErr := writeMaybeJSONError(true, "invalid_input", assert.AnError, map[string]string{"flag": "name"})
	require.Error(t, runErr)

	require.NoError(t, outW.Close())
	require.NoError(t, errW.Close())

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	_, _ = io.Copy(&outBuf, outR)
	_, _ = io.Copy(&errBuf, errR)

	var envelope jsonErrorEnvelope
	require.NoError(t, json.Unmarshal(outBuf.Bytes(), &envelope))
	assert.Equal(t, "invalid_input", envelope.Error.Code)
	assert.Contains(t, envelope.Error.Message, assert.AnError.Error())
	assert.Contains(t, errBuf.String(), assert.AnError.Error())
}
