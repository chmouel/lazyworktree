package services

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const worktreeNoteScriptTimeout = 30 * time.Second

// WorktreeNoteScriptInput contains the context passed to worktree_note_script.
type WorktreeNoteScriptInput struct {
	Content     string
	Type        string
	Number      int
	Title       string
	URL         string
	Description string
}

// RunWorktreeNoteScript executes worktree_note_script and returns the generated note text.
func RunWorktreeNoteScript(ctx context.Context, script string, input WorktreeNoteScriptInput) (string, error) {
	script = strings.TrimSpace(script)
	if script == "" {
		return "", nil
	}

	ctx, cancel := context.WithTimeout(ctx, worktreeNoteScriptTimeout)
	defer cancel()

	// #nosec G204 -- script is user-configured and trusted
	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	cmd.Stdin = strings.NewReader(input.Content)
	cmd.Env = AppendCommandEnv(os.Environ(), BuildCommandEnvWithContext("", "", "", "", LazyWorktreeContext{
		Type:        input.Type,
		Number:      strconv.Itoa(input.Number),
		Title:       input.Title,
		URL:         input.URL,
		Description: input.Description,
	}))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("worktree note script failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(stdout.String()), nil
}
