package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appiCli "github.com/urfave/cli/v3"
)

// buildTestApp builds a minimal CLI app matching the real app structure for introspection tests.
func buildTestApp() *appiCli.Command {
	app := &appiCli.Command{
		Name:  "lazyworktree",
		Usage: "A TUI tool to manage git worktrees",
		Commands: []*appiCli.Command{
			createCommand(),
			renameCommand(),
			deleteCommand(),
			listCommand(),
			execCommand(),
			noteCommand(),
			describeCommand(),
		},
	}
	return app
}

// captureDescribeOutput runs describe with the given args and returns the JSON output.
func captureDescribeOutput(t *testing.T, args []string) []byte {
	t.Helper()

	app := buildTestApp()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
		_ = w.Close()
		_ = r.Close()
	})

	fullArgs := append([]string{"lazyworktree", "describe"}, args...)
	err = app.Run(context.Background(), fullArgs)
	require.NoError(t, err)

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.Bytes()
}

func TestDescribeRootEmitsAllCommands(t *testing.T) {
	data := captureDescribeOutput(t, nil)

	var desc commandDescJSON
	require.NoError(t, json.Unmarshal(data, &desc))

	assert.Equal(t, "lazyworktree", desc.Name)

	names := make([]string, 0, len(desc.Subcommands))
	for _, sub := range desc.Subcommands {
		names = append(names, sub.Name)
	}
	assert.Contains(t, names, "create")
	assert.Contains(t, names, "delete")
	assert.Contains(t, names, "rename")
	assert.Contains(t, names, "list")
	assert.Contains(t, names, "exec")
	assert.Contains(t, names, "note")
	assert.Contains(t, names, "describe")
}

func TestDescribeCreateIncludesExpectedFlags(t *testing.T) {
	data := captureDescribeOutput(t, []string{"create"})

	var desc commandDescJSON
	require.NoError(t, json.Unmarshal(data, &desc))

	assert.Equal(t, "create", desc.Name)
	assert.Equal(t, "[worktree-name]", desc.ArgsUsage)

	flagNames := make([]string, 0, len(desc.Flags))
	for _, f := range desc.Flags {
		flagNames = append(flagNames, f.Name)
	}

	expected := []string{"from-branch", "from-pr", "from-issue", "json", "note", "description", "tags", "exec-mode"}
	for _, name := range expected {
		assert.Contains(t, flagNames, name, "expected flag %q in create", name)
	}
}

func TestDescribeNoteShowIncludesJSONFlag(t *testing.T) {
	data := captureDescribeOutput(t, []string{"note", "show"})

	var desc commandDescJSON
	require.NoError(t, json.Unmarshal(data, &desc))

	assert.Equal(t, "show", desc.Name)

	flagNames := make([]string, 0, len(desc.Flags))
	for _, f := range desc.Flags {
		flagNames = append(flagNames, f.Name)
	}
	assert.Contains(t, flagNames, "json")
}

func TestDescribeFlagTypes(t *testing.T) {
	data := captureDescribeOutput(t, []string{"create"})

	var desc commandDescJSON
	require.NoError(t, json.Unmarshal(data, &desc))

	typeByName := make(map[string]string)
	for _, f := range desc.Flags {
		typeByName[f.Name] = f.Type
	}

	assert.Equal(t, "string", typeByName["from-branch"])
	assert.Equal(t, "int", typeByName["from-pr"])
	assert.Equal(t, "bool", typeByName["json"])
}

func TestDescribeFlagAliasesIncluded(t *testing.T) {
	data := captureDescribeOutput(t, []string{"create"})

	var desc commandDescJSON
	require.NoError(t, json.Unmarshal(data, &desc))

	var fromBranchFlag *flagDescJSON
	for i := range desc.Flags {
		if desc.Flags[i].Name == "from-branch" {
			fromBranchFlag = &desc.Flags[i]
			break
		}
	}
	require.NotNil(t, fromBranchFlag)
	assert.Contains(t, fromBranchFlag.Aliases, "branch")
}

func TestDescribeUnknownCommandErrors(t *testing.T) {
	app := buildTestApp()

	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = oldStderr
		_ = w.Close()
	})

	err := app.Run(context.Background(), []string{"lazyworktree", "describe", "nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestDescribeFlagDescJSON(t *testing.T) {
	tests := []struct {
		name    string
		flag    appiCli.Flag
		wantTyp string
	}{
		{"string flag", &appiCli.StringFlag{Name: "foo", Usage: "a string"}, "string"},
		{"bool flag", &appiCli.BoolFlag{Name: "bar", Usage: "a bool"}, "bool"},
		{"int flag", &appiCli.IntFlag{Name: "baz", Usage: "an int"}, "int"},
		{"slice flag", &appiCli.StringSliceFlag{Name: "qux"}, "string-slice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := describeFlag(tt.flag)
			assert.Equal(t, tt.wantTyp, got.Type)
		})
	}
}
