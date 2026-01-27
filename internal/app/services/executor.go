package services

import (
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

// CommandExecutor runs shell commands for the application.
type CommandExecutor interface {
	Run(cmd string, cwd string, env map[string]string) tea.Cmd
	RunWithPager(cmd string, cwd string, env map[string]string) tea.Cmd
	RunInteractive(cmd *exec.Cmd, callback tea.ExecCallback) tea.Cmd
}
