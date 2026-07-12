package bootstrap

import (
	"context"
	"fmt"
	"os"

	"github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/commands"
	"github.com/chmouel/lazyworktree/internal/models"
	appiCli "github.com/urfave/cli/v3"
)

// agentEventCommand is the hidden hook shim invoked by Claude Code, Codex
// CLI, and Copilot CLI lifecycle hooks. It reads the hook payload from stdin
// and spools a normalised event for the TUI to consume. It always exits
// successfully so a broken spool never disrupts the agent.
func agentEventCommand() *appiCli.Command {
	return &appiCli.Command{
		Name:   "agent-event",
		Usage:  "Record an agent lifecycle hook event (internal)",
		Hidden: true,
		Flags: []appiCli.Flag{
			&appiCli.StringFlag{
				Name:     "agent",
				Usage:    "Agent kind reporting the event (claude, codex, copilot, pi)",
				Required: true,
			},
		},
		Action: func(_ context.Context, cmd *appiCli.Command) error {
			agent := models.AgentKind(cmd.String("agent"))
			switch agent {
			case models.AgentKindClaude, models.AgentKindCodex, models.AgentKindCopilot, models.AgentKindPi:
			default:
				return nil
			}
			if err := services.RecordAgentHookEvent(services.AgentHookSpoolDir(), agent, os.Stdin); err != nil {
				fmt.Fprintf(os.Stderr, "lazyworktree agent-event: %v\n", err)
			}
			return nil
		},
	}
}

// setupHooksCommand installs the agent-event shim into the Claude Code,
// Codex CLI, and Copilot CLI hook configurations.
func setupHooksCommand() *appiCli.Command {
	return &appiCli.Command{
		Name:  "setup-hooks",
		Usage: "Install agent session hooks for Claude Code, Codex CLI, and Copilot CLI",
		Flags: []appiCli.Flag{
			&appiCli.BoolFlag{
				Name:  "dry-run",
				Usage: "Show the changes without writing any files",
			},
		},
		Action: func(_ context.Context, cmd *appiCli.Command) error {
			return commands.SetupAgentHooks(commands.SetupAgentHooksOptions{
				DryRun: cmd.Bool("dry-run"),
				Stdout: os.Stdout,
			})
		},
	}
}
