package git

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chmouel/lazyworktree/internal/models"
)

// fakeCommand returns a command that prints stdout and exits with code.
func fakeCommand(stdout, stderr string, code int) *exec.Cmd {
	script := ""
	if stdout != "" {
		script += "printf '%s' " + shellQuote(stdout) + "; "
	}
	if stderr != "" {
		script += "printf '%s' " + shellQuote(stderr) + " >&2; "
	}
	script += "exit " + strconv.Itoa(code)
	return exec.Command("sh", "-c", script) //#nosec G204 -- test helper with controlled args
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func newReviewerRepo(t *testing.T, remote string) *Service {
	t.Helper()

	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "remote", "add", "origin", remote)
	withCwd(t, repo)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	return service
}

func TestFetchPRReviewersArgv(t *testing.T) {
	t.Run("github uses raw fields for strings and a typed field for the number", func(t *testing.T) {
		service := newReviewerRepo(t, "git@github.com:chmouel/lazyworktree.git")
		var got []string
		service.SetCommandRunner(func(_ context.Context, name string, args ...string) *exec.Cmd {
			if name == "gh" {
				got = append([]string{name}, args...)
				return fakeCommand(`{"data":{"repository":{"pullRequest":{"latestReviews":{"totalCount":0,"nodes":[]}}}}}`, "", 0)
			}
			return exec.CommandContext(context.Background(), name, args...) //#nosec G204 -- test passthrough
		})

		_, err := service.FetchPRReviewers(context.Background(), 42)
		require.NoError(t, err)

		require.NotEmpty(t, got)
		assert.Equal(t, []string{"gh", "api", "graphql"}, got[:3])
		assert.Contains(t, got, "-f")
		assert.Contains(t, got, "owner=chmouel")
		assert.Contains(t, got, "name=lazyworktree")
		assert.Contains(t, got, "number=42")
		// The number is the only typed field.
		assert.Equal(t, 1, countOccurrences(got, "-F"))
		assert.Equal(t, "-F", got[len(got)-2])
	})

	t.Run("gitlab passes the iid as a raw field", func(t *testing.T) {
		service := newReviewerRepo(t, "git@gitlab.com:group/project.git")
		var got []string
		service.SetCommandRunner(func(_ context.Context, name string, args ...string) *exec.Cmd {
			if name == "glab" {
				got = append([]string{name}, args...)
				return fakeCommand(`{"data":{"project":{"mergeRequest":{"webUrl":"","reviewers":{"nodes":[]}}}}}`, "", 0)
			}
			return exec.CommandContext(context.Background(), name, args...) //#nosec G204 -- test passthrough
		})

		_, err := service.FetchPRReviewers(context.Background(), 7)
		require.NoError(t, err)

		require.NotEmpty(t, got)
		assert.Equal(t, []string{"glab", "api", "graphql"}, got[:3])
		assert.Contains(t, got, "fullPath=group/project")
		assert.Contains(t, got, "iid=7")
		assert.Equal(t, 0, countOccurrences(got, "-F"))
	})
}

func countOccurrences(values []string, want string) int {
	count := 0
	for _, value := range values {
		if value == want {
			count++
		}
	}
	return count
}

func TestFetchPRReviewersFailures(t *testing.T) {
	t.Run("a non-zero exit surfaces stderr instead of looking like no reviewers", func(t *testing.T) {
		service := newReviewerRepo(t, "git@github.com:chmouel/lazyworktree.git")
		service.SetCommandRunner(func(_ context.Context, name string, args ...string) *exec.Cmd {
			if name == "gh" {
				return fakeCommand("", "gh: not authenticated", 1)
			}
			return exec.CommandContext(context.Background(), name, args...) //#nosec G204 -- test passthrough
		})

		summary, err := service.FetchPRReviewers(context.Background(), 1)
		require.Error(t, err)
		assert.Nil(t, summary)
		assert.Contains(t, err.Error(), "not authenticated")
	})

	t.Run("malformed json is an error", func(t *testing.T) {
		service := newReviewerRepo(t, "git@github.com:chmouel/lazyworktree.git")
		service.SetCommandRunner(func(_ context.Context, name string, args ...string) *exec.Cmd {
			if name == "gh" {
				return fakeCommand("not json", "", 0)
			}
			return exec.CommandContext(context.Background(), name, args...) //#nosec G204 -- test passthrough
		})

		_, err := service.FetchPRReviewers(context.Background(), 1)
		require.Error(t, err)
	})

	t.Run("no reviewers for an unknown host", func(t *testing.T) {
		service := newReviewerRepo(t, "git@example.com:group/project.git")
		summary, err := service.FetchPRReviewers(context.Background(), 1)
		require.NoError(t, err)
		assert.Nil(t, summary)
	})

	t.Run("no reviewers without a change request number", func(t *testing.T) {
		service := NewService(func(string, string) {}, func(string, string, string) {})
		summary, err := service.FetchPRReviewers(context.Background(), 0)
		require.NoError(t, err)
		assert.Nil(t, summary)
	})
}

func TestParseGitHubReviewers(t *testing.T) {
	t.Parallel()

	t.Run("bots, humans and a deleted author", func(t *testing.T) {
		t.Parallel()
		raw := []byte(`{"data":{"repository":{"pullRequest":{"latestReviews":{
			"totalCount":3,
			"nodes":[
				{"state":"APPROVED","author":{"__typename":"User","login":"alice","name":"Alice","avatarUrl":"https://avatars.example/a.png"}},
				{"state":"COMMENTED","author":{"__typename":"Bot","login":"copilot","avatarUrl":"https://avatars.example/b.png"}},
				{"state":"CHANGES_REQUESTED","author":null}
			]}}}}}`)

		summary, err := parseGitHubReviewers(raw)
		require.NoError(t, err)
		require.NotNil(t, summary)

		assert.Equal(t, 3, summary.Total)
		require.Len(t, summary.Reviewers, 2)
		assert.Equal(t, "alice", summary.Reviewers[0].Login)
		assert.False(t, summary.Reviewers[0].IsBot)
		assert.Equal(t, models.ReviewStateApproved, summary.Reviewers[0].State)
		assert.True(t, summary.Reviewers[1].IsBot)
		assert.Equal(t, models.ReviewStateCommented, summary.Reviewers[1].State)
	})

	t.Run("pending reviews are not shown", func(t *testing.T) {
		t.Parallel()
		raw := []byte(`{"data":{"repository":{"pullRequest":{"latestReviews":{
			"totalCount":1,
			"nodes":[{"state":"PENDING","author":{"__typename":"User","login":"alice"}}]}}}}}`)

		summary, err := parseGitHubReviewers(raw)
		require.NoError(t, err)
		assert.Empty(t, summary.Reviewers)
	})

	t.Run("the count never falls below what is displayed", func(t *testing.T) {
		t.Parallel()
		raw := []byte(`{"data":{"repository":{"pullRequest":{"latestReviews":{
			"totalCount":0,
			"nodes":[{"state":"APPROVED","author":{"__typename":"User","login":"alice"}}]}}}}}`)

		summary, err := parseGitHubReviewers(raw)
		require.NoError(t, err)
		assert.Equal(t, 1, summary.Total)
	})

	t.Run("an errors array is an error even on a successful response", func(t *testing.T) {
		t.Parallel()
		_, err := parseGitHubReviewers([]byte(`{"errors":[{"message":"Could not resolve to a Repository"}],"data":null}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Could not resolve")
	})

	t.Run("no reviews at all", func(t *testing.T) {
		t.Parallel()
		summary, err := parseGitHubReviewers([]byte(`{"data":{"repository":{"pullRequest":{"latestReviews":{"totalCount":0,"nodes":[]}}}}}`))
		require.NoError(t, err)
		assert.Equal(t, 0, summary.Total)
		assert.Empty(t, summary.Reviewers)
	})
}

func TestParseGitLabReviewers(t *testing.T) {
	t.Parallel()

	t.Run("every review state", func(t *testing.T) {
		t.Parallel()
		raw := []byte(`{"data":{"project":{"mergeRequest":{
			"webUrl":"https://gitlab.example.com/group/project/-/merge_requests/1",
			"reviewers":{"nodes":[
				{"username":"alice","name":"Alice","bot":false,"avatarUrl":"https://gitlab.example.com/a.png","mergeRequestInteraction":{"reviewState":"APPROVED"}},
				{"username":"bob","bot":false,"mergeRequestInteraction":{"reviewState":"REQUESTED_CHANGES"}},
				{"username":"carol","bot":false,"mergeRequestInteraction":{"reviewState":"REVIEWED"}},
				{"username":"dave","bot":false,"mergeRequestInteraction":{"reviewState":"UNAPPROVED"}},
				{"username":"erin","bot":false,"mergeRequestInteraction":{"reviewState":"UNREVIEWED"}},
				{"username":"frank","bot":false,"mergeRequestInteraction":{"reviewState":"REVIEW_STARTED"}},
				{"username":"grace","bot":false,"mergeRequestInteraction":{"reviewState":"SOMETHING_NEW"}},
				{"username":"bot-user","bot":true,"mergeRequestInteraction":{"reviewState":"APPROVED"}}
			]}}}}}`)

		summary, err := parseGitLabReviewers(raw)
		require.NoError(t, err)
		require.NotNil(t, summary)

		logins := make([]string, 0, len(summary.Reviewers))
		states := make(map[string]string, len(summary.Reviewers))
		for _, reviewer := range summary.Reviewers {
			logins = append(logins, reviewer.Login)
			states[reviewer.Login] = reviewer.State
		}
		assert.Equal(t, []string{"alice", "bob", "carol", "dave", "bot-user"}, logins)
		assert.Equal(t, 5, summary.Total)
		assert.Equal(t, models.ReviewStateApproved, states["alice"])
		assert.Equal(t, models.ReviewStateChangesRequested, states["bob"])
		assert.Equal(t, models.ReviewStateCommented, states["carol"])
		assert.Equal(t, models.ReviewStateDismissed, states["dave"])
		assert.True(t, summary.Reviewers[4].IsBot)
	})

	t.Run("a relative avatar path resolves against the merge request origin", func(t *testing.T) {
		t.Parallel()
		raw := []byte(`{"data":{"project":{"mergeRequest":{
			"webUrl":"https://gitlab.example.com/group/project/-/merge_requests/1",
			"reviewers":{"nodes":[
				{"username":"alice","avatarUrl":"/uploads/-/system/user/avatar/1/avatar.png","mergeRequestInteraction":{"reviewState":"APPROVED"}}
			]}}}}}`)

		summary, err := parseGitLabReviewers(raw)
		require.NoError(t, err)
		require.Len(t, summary.Reviewers, 1)
		assert.Equal(t, "https://gitlab.example.com/uploads/-/system/user/avatar/1/avatar.png", summary.Reviewers[0].AvatarURL)
	})

	t.Run("a reviewer without interaction data is skipped", func(t *testing.T) {
		t.Parallel()
		raw := []byte(`{"data":{"project":{"mergeRequest":{"webUrl":"","reviewers":{"nodes":[
			{"username":"alice","mergeRequestInteraction":null}
		]}}}}}`)

		summary, err := parseGitLabReviewers(raw)
		require.NoError(t, err)
		assert.Equal(t, 0, summary.Total)
	})

	t.Run("an errors array is an error", func(t *testing.T) {
		t.Parallel()
		_, err := parseGitLabReviewers([]byte(`{"errors":[{"message":"Field 'reviewState' doesn't exist"}]}`))
		require.Error(t, err)
	})
}

func TestSanitiseAvatarURL(t *testing.T) {
	t.Parallel()

	base := "https://gitlab.example.com/group/project/-/merge_requests/1"
	tests := []struct {
		name string
		raw  string
		base string
		want string
	}{
		{"absolute https is kept", "https://avatars.example/a.png", "", "https://avatars.example/a.png"},
		{"relative resolves against the origin", "/uploads/a.png", base, "https://gitlab.example.com/uploads/a.png"},
		{"relative without a base is dropped", "/uploads/a.png", "", ""},
		{"relative with an http base is dropped", "/uploads/a.png", "http://gitlab.example.com/x", ""},
		{"scheme-relative is dropped", "//attacker.example/a.png", base, ""},
		{"plain http is dropped", "http://avatars.example/a.png", "", ""},
		{"data urls are dropped", "data:image/png;base64,AAAA", "", ""},
		{"javascript urls are dropped", "javascript:alert(1)", "", ""},
		{"unparseable is dropped", "https://exa mple.com/\x7f", "", ""},
		{"empty is dropped", "   ", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, sanitiseAvatarURL(tc.raw, tc.base))
		})
	}
}

func TestUsableForgeRepoName(t *testing.T) {
	t.Parallel()

	assert.True(t, usableForgeRepoName("owner/repo"))
	assert.False(t, usableForgeRepoName(""))
	assert.False(t, usableForgeRepoName("  "))
	assert.False(t, usableForgeRepoName("local-abc123"))
}
