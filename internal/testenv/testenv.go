// Package testenv prepares the process environment for tests that run git.
package testenv

import "os"

// gitRepoEnvVars is the list printed by `git rev-parse --local-env-vars`.
// Git exports some of them to hooks, with an absolute GIT_DIR in linked
// worktrees, so tests inheriting them would run git against the caller's
// repository instead of their temporary ones.
var gitRepoEnvVars = []string{
	"GIT_ALTERNATE_OBJECT_DIRECTORIES",
	"GIT_COMMON_DIR",
	"GIT_CONFIG",
	"GIT_CONFIG_COUNT",
	"GIT_CONFIG_PARAMETERS",
	"GIT_DIR",
	"GIT_GRAFT_FILE",
	"GIT_IMPLICIT_WORK_TREE",
	"GIT_INDEX_FILE",
	"GIT_NO_REPLACE_OBJECTS",
	"GIT_OBJECT_DIRECTORY",
	"GIT_PREFIX",
	"GIT_REPLACE_REF_BASE",
	"GIT_SHALLOW_FILE",
	"GIT_WORK_TREE",
}

// ClearGitRepoEnv unsets the variables that tie git to a specific repository.
func ClearGitRepoEnv() {
	for _, name := range gitRepoEnvVars {
		_ = os.Unsetenv(name)
	}
}
