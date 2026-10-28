package cli

import (
	"testing"

	"github.com/chmouel/lazyworktree/internal/testenv"
)

func TestMain(m *testing.M) {
	testenv.ClearGitRepoEnv()
	m.Run()
}
