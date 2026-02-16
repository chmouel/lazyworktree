package services

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const worktreeNoteScriptTimeout = 30 * time.Second

// WorktreeNoteScriptInput contains the context passed to worktree_note_script.
type WorktreeNoteScriptInput struct {
	Content string
	Type    string
	Number  int
	Title   string
	URL     string
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
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("LAZYWORKTREE_TYPE=%s", input.Type),
		fmt.Sprintf("LAZYWORKTREE_NUMBER=%d", input.Number),
		fmt.Sprintf("LAZYWORKTREE_TITLE=%s", input.Title),
		fmt.Sprintf("LAZYWORKTREE_URL=%s", input.URL),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("worktree note script failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(stdout.String()), nil
}
