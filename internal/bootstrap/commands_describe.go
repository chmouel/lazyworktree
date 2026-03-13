package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	appiCli "github.com/urfave/cli/v3"
)

func describeCommand() *appiCli.Command {
	return &appiCli.Command{
		Name:      "describe",
		Usage:     "Describe the CLI structure as JSON for machine-readable introspection",
		ArgsUsage: "[command] [subcommand]",
		Action:    handleDescribeAction,
		Flags: []appiCli.Flag{
			&appiCli.BoolFlag{
				Name:  "all",
				Usage: "Describe all commands and their flags",
			},
		},
	}
}

func handleDescribeAction(_ context.Context, cmd *appiCli.Command) error {
	root := cmd.Root()
	all := cmd.Bool("all")
	args := cmd.Args().Slice()

	if all || len(args) == 0 {
		return encodeDescribeJSON(describeCommandNode(root))
	}

	// Find named top-level command.
	target := findDescribeTarget(root, args[0])
	if target == nil {
		fmt.Fprintf(os.Stderr, "Error: command %q not found\n", args[0])
		return fmt.Errorf("command %q not found", args[0])
	}

	// Optionally find a nested subcommand.
	if len(args) > 1 {
		sub := findDescribeTarget(target, args[1])
		if sub == nil {
			fmt.Fprintf(os.Stderr, "Error: subcommand %q not found under %q\n", args[1], args[0])
			return fmt.Errorf("subcommand %q not found under %q", args[1], args[0])
		}
		target = sub
	}

	return encodeDescribeJSON(describeCommandNode(target))
}

// findDescribeTarget finds a direct subcommand of parent by name or alias.
func findDescribeTarget(parent *appiCli.Command, name string) *appiCli.Command {
	for _, sub := range parent.Commands {
		if sub.Name == name {
			return sub
		}
		for _, alias := range sub.Aliases {
			if alias == name {
				return sub
			}
		}
	}
	return nil
}

// describeCommandNode recursively builds a commandDescJSON for cmd and its subcommands.
func describeCommandNode(cmd *appiCli.Command) commandDescJSON {
	desc := commandDescJSON{
		Name:      cmd.Name,
		Usage:     cmd.Usage,
		ArgsUsage: cmd.ArgsUsage,
		Flags:     describeFlags(cmd.Flags),
	}
	for _, sub := range cmd.Commands {
		desc.Subcommands = append(desc.Subcommands, describeCommandNode(sub))
	}
	return desc
}

// describeFlags converts a slice of Flag into flagDescJSON entries.
func describeFlags(flags []appiCli.Flag) []flagDescJSON {
	if len(flags) == 0 {
		return nil
	}
	result := make([]flagDescJSON, 0, len(flags))
	for _, f := range flags {
		result = append(result, describeFlag(f))
	}
	return result
}

// describeFlag converts a single Flag to a flagDescJSON.
func describeFlag(f appiCli.Flag) flagDescJSON {
	names := f.Names()
	name := names[0]
	var aliases []string
	if len(names) > 1 {
		aliases = names[1:]
	}

	desc := flagDescJSON{Name: name, Aliases: aliases}

	if df, ok := f.(appiCli.DocGenerationFlag); ok {
		desc.Usage = df.GetUsage()
	}

	switch v := f.(type) {
	case *appiCli.StringFlag:
		desc.Type = "string"
		desc.Default = v.Value
	case *appiCli.BoolFlag:
		desc.Type = "bool"
		if v.Value {
			desc.Default = "true"
		}
	case *appiCli.IntFlag:
		desc.Type = "int"
		if v.Value != 0 {
			desc.Default = fmt.Sprintf("%d", v.Value)
		}
	case *appiCli.StringSliceFlag:
		desc.Type = "string-slice"
	default:
		desc.Type = "unknown"
	}

	return desc
}

func encodeDescribeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
