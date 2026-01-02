package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeStub(t *testing.T, name, script string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		t.Fatalf("failed to write stub: %v", err)
	}
	// #nosec G302 -- test stub needs executable permissions.
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatalf("failed to chmod stub: %v", err)
	}
	return dir
}

func withStubbedPath(t *testing.T, dir string) {
	t.Helper()

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath)
}

func TestFetchGitLabPRs(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"api\" ]; then\n" +
		"  echo '[{\"iid\":1,\"state\":\"opened\",\"title\":\"One\",\"web_url\":\"https://example.com/1\",\"source_branch\":\"feature\"},{\"iid\":2,\"state\":\"closed\",\"title\":\"Two\",\"web_url\":\"https://example.com/2\",\"source_branch\":\"closed\"}]'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
	dir := writeStub(t, "glab", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	prs, err := service.fetchGitLabPRs(context.Background())
	require.NoError(t, err)
	require.NotNil(t, prs)

	pr, ok := prs["feature"]
	require.True(t, ok)
	assert.Equal(t, 1, pr.Number)
	assert.Equal(t, prStateOpen, pr.State)
	assert.Equal(t, "One", pr.Title)
}

func TestFetchGitLabOpenPRs(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"api\" ]; then\n" +
		"  echo '[{\"iid\":1,\"state\":\"opened\",\"title\":\"One\",\"web_url\":\"https://example.com/1\",\"source_branch\":\"feature\"},{\"iid\":2,\"state\":\"closed\",\"title\":\"Two\",\"web_url\":\"https://example.com/2\",\"source_branch\":\"closed\"}]'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
	dir := writeStub(t, "glab", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	prs, err := service.fetchGitLabOpenPRs(context.Background())
	require.NoError(t, err)
	require.Len(t, prs, 1)
	assert.Equal(t, "feature", prs[0].Branch)
	assert.Equal(t, prStateOpen, prs[0].State)
}

func TestFetchGitLabCI(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"ci\" ]; then\n" +
		"  echo '{\"jobs\":[{\"name\":\"build\",\"status\":\"success\"},{\"name\":\"test\",\"status\":\"failed\"}]}'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
	dir := writeStub(t, "glab", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	checks, err := service.fetchGitLabCI(context.Background(), "main")
	require.NoError(t, err)
	require.Len(t, checks, 2)
	assert.Equal(t, ciSuccess, checks[0].Conclusion)
	assert.Equal(t, ciFailure, checks[1].Conclusion)
}

func TestFetchGitLabCIFallbackArray(t *testing.T) {
	stub := "#!/bin/sh\n" +
		"if [ \"$1\" = \"ci\" ]; then\n" +
		"  echo '[{\"name\":\"lint\",\"status\":\"skipped\"}]'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
	dir := writeStub(t, "glab", stub)
	withStubbedPath(t, dir)

	service := NewService(func(string, string) {}, func(string, string, string) {})
	checks, err := service.fetchGitLabCI(context.Background(), "main")
	require.NoError(t, err)
	require.Len(t, checks, 1)
	assert.Equal(t, ciSkipped, checks[0].Conclusion)
}
