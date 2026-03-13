package bootstrap

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateJSONRoundTrip(t *testing.T) {
	orig := createJSON{
		Path:        "/worktrees/myrepo/feature-x",
		Name:        "feature-x",
		Branch:      "feature/x",
		Description: "some work",
		Tags:        []string{"backend", "urgent"},
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var got createJSON
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, orig, got)
}

func TestCreateJSONOmitsEmptyFields(t *testing.T) {
	orig := createJSON{Path: "/p", Name: "n", Branch: "b"}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.NotContains(t, m, "description")
	assert.NotContains(t, m, "tags")
}

func TestDeleteJSONRoundTrip(t *testing.T) {
	orig := deleteJSON{Name: "feat", Path: "/worktrees/r/feat", BranchDeleted: true}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var got deleteJSON
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, orig, got)
}

func TestRenameJSONRoundTrip(t *testing.T) {
	orig := renameJSON{
		OldName: "feat",
		OldPath: "/worktrees/r/feat",
		NewName: "feat-v2",
		NewPath: "/worktrees/r/feat-v2",
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var got renameJSON
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, orig, got)
}

func TestNoteShowJSONRoundTrip(t *testing.T) {
	orig := noteShowJSON{
		WorktreeName: "feat",
		Path:         "/worktrees/r/feat",
		Note:         "do the thing",
		Description:  "context",
		Icon:         "🚀",
		Tags:         []string{"wip"},
		UpdatedAt:    1700000000,
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var got noteShowJSON
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, orig, got)
}

func TestNoteShowJSONOmitsEmptyFields(t *testing.T) {
	orig := noteShowJSON{WorktreeName: "n", Path: "/p"}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.NotContains(t, m, "note")
	assert.NotContains(t, m, "description")
	assert.NotContains(t, m, "icon")
	assert.NotContains(t, m, "tags")
	assert.NotContains(t, m, "updated_at")
}

func TestExecJSONRoundTrip(t *testing.T) {
	orig := execJSON{
		Name:     "feat",
		Path:     "/worktrees/r/feat",
		Command:  "make test",
		ExitCode: 1,
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var got execJSON
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, orig, got)
}

func TestWorktreeJSONExtendedRoundTrip(t *testing.T) {
	orig := worktreeJSONExtended{
		Path:          "/worktrees/r/feat",
		Name:          "feat",
		Branch:        "feature/x",
		IsMain:        false,
		Dirty:         true,
		Ahead:         2,
		Behind:        1,
		Unpushed:      0,
		LastActive:    "5 mins ago",
		Description:   "some work",
		Tags:          []string{"backend"},
		NotePresent:   true,
		NoteUpdatedAt: 1700000000,
		AgentSessions: []agentSessionJSON{
			{ID: "abc", Agent: "claude", Status: "waiting", Activity: "idle", IsOpen: true},
		},
		AgentOpen:     true,
		AgentActivity: "idle",
		AgentCount:    1,
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var got worktreeJSONExtended
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, orig, got)
}

func TestShellInvocationForExecMode(t *testing.T) {
	tests := []struct {
		name         string
		command      string
		mode         string
		shell        string
		wantExe      string
		wantArgCount int
		wantArg0     string
	}{
		{
			name:         "direct splits command",
			command:      "go test ./...",
			mode:         execModeDirect,
			wantExe:      "go",
			wantArgCount: 2,
			wantArg0:     "test",
		},
		{
			name:         "direct single word",
			command:      "ls",
			mode:         execModeDirect,
			wantExe:      "ls",
			wantArgCount: 0,
		},
		{
			name:         "shell uses -c",
			command:      "echo hello",
			mode:         execModeShell,
			shell:        "/bin/bash",
			wantExe:      "/bin/bash",
			wantArgCount: 2,
			wantArg0:     "-c",
		},
		{
			name:         "login-shell zsh uses -ilc",
			command:      "echo hello",
			mode:         execModeLoginShell,
			shell:        "/bin/zsh",
			wantExe:      "/bin/zsh",
			wantArgCount: 2,
			wantArg0:     "-ilc",
		},
		{
			name:         "login-shell bash uses -ic",
			command:      "echo hello",
			mode:         execModeLoginShell,
			shell:        "/bin/bash",
			wantExe:      "/bin/bash",
			wantArgCount: 2,
			wantArg0:     "-ic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shell != "" {
				t.Setenv("SHELL", tt.shell)
			}
			exe, args := shellInvocationForExecMode(tt.command, tt.mode)
			assert.Equal(t, tt.wantExe, exe)
			assert.Len(t, args, tt.wantArgCount)
			if tt.wantArgCount > 0 && tt.wantArg0 != "" {
				assert.Equal(t, tt.wantArg0, args[0])
			}
		})
	}
}

func TestParseTags(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"foo", []string{"foo"}},
		{"foo,bar", []string{"foo", "bar"}},
		{" foo , bar , ", []string{"foo", "bar"}},
		{",,,", nil},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, parseTags(tt.input))
		})
	}
}
