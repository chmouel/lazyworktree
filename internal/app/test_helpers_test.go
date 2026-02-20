package app

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func mockGitWorktreeList(t *testing.T, m *Model, paths ...string) {
	t.Helper()
	if m.state.services.git == nil {
		t.Fatal("git service must be initialised for mock setup")
	}

	porcelain := gitWorktreeListPorcelain(paths...)
	m.state.services.git.SetCommandRunner(func(_ context.Context, name string, args ...string) *exec.Cmd {
		cmd := strings.Join(append([]string{name}, args...), " ")
		if cmd == "git worktree list --porcelain" {
			// #nosec G204 -- test helper with controlled command and fixture output.
			return exec.Command("bash", "-lc", "cat <<'EOF'\n"+porcelain+"EOF")
		}
		return exec.Command("bash", "-lc", "exit 1")
	})
}

func gitWorktreeListPorcelain(paths ...string) string {
	var builder strings.Builder
	for i, path := range paths {
		builder.WriteString("worktree ")
		builder.WriteString(path)
		builder.WriteString("\n")
		fmt.Fprintf(&builder, "branch refs/heads/test-%d\n\n", i)
	}
	return builder.String()
}
