package config

import "strings"

// CustomCommand represents a user-defined command binding.
type CustomCommand struct {
	Command     string
	Description string
	ShowHelp    bool
	Wait        bool
	ShowOutput  bool
	NewTab      bool // Launch command in a new terminal tab (Kitty, etc.)
	Tmux        *TmuxCommand
	Zellij      *TmuxCommand
	Herdr       *HerdrCommand
	Container   *ContainerCommand
}

// CustomCommandsConfig maps pane names to key→*CustomCommand maps. "universal" applies to all panes.
type CustomCommandsConfig map[string]map[string]*CustomCommand

// Lookup returns the custom command for the given pane and key,
// checking pane-specific first, then universal.
func (c CustomCommandsConfig) Lookup(paneName, key string) (*CustomCommand, bool) {
	if m, ok := c[paneName]; ok {
		if cmd, ok := m[key]; ok && cmd != nil {
			return cmd, true
		}
	}
	if m, ok := c[PaneUniversal]; ok {
		if cmd, ok := m[key]; ok && cmd != nil {
			return cmd, true
		}
	}
	return nil, false
}

// AllForPane returns a merged map of universal + pane-specific commands (pane wins on conflict).
func (c CustomCommandsConfig) AllForPane(paneName string) map[string]*CustomCommand {
	result := make(map[string]*CustomCommand)
	for key, cmd := range c[PaneUniversal] {
		if cmd != nil {
			result[key] = cmd
		}
	}
	if paneName != PaneUniversal {
		for key, cmd := range c[paneName] {
			if cmd != nil {
				result[key] = cmd
			}
		}
	}
	return result
}

// AllUniversal returns only the universal commands.
func (c CustomCommandsConfig) AllUniversal() map[string]*CustomCommand {
	return c[PaneUniversal]
}

const paletteOnlyCommandPrefix = "_"

// IsPaletteOnlyCommandKey reports whether a custom command is palette-only.
// Keys prefixed with "_" remain available in the command palette, but they are
// not bound to a direct keyboard shortcut in the main TUI.
func IsPaletteOnlyCommandKey(key string) bool {
	return strings.HasPrefix(key, paletteOnlyCommandPrefix)
}

// PaletteOnlyCommandName returns the palette-visible identifier without the
// leading "_" marker.
func PaletteOnlyCommandName(key string) string {
	if !IsPaletteOnlyCommandKey(key) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(key, paletteOnlyCommandPrefix))
}

// CustomCommandHasKeyBinding reports whether the key should be matched against
// direct keyboard input.
func CustomCommandHasKeyBinding(key string) bool {
	return !IsPaletteOnlyCommandKey(key)
}

// CustomCreateMenu defines a custom entry in the worktree creation menu.
// The command should output a branch name that will be sanitized and used.
type CustomCreateMenu struct {
	Label           string // Display label in the menu
	Description     string // Help text shown next to label
	Command         string // Shell command that outputs branch name
	Interactive     bool   // Run interactively (TUI suspends, captures stdout via temp file)
	PostCommand     string // Command to run after worktree creation (optional)
	PostInteractive bool   // Run post-command interactively (default: false)
}

// commandSpecificFields are YAML keys that appear directly inside a custom command
// definition (as opposed to a pane name). Used to detect the old flat format.
var commandSpecificFields = map[string]bool{
	"command": true, "description": true, "show_help": true,
	"wait": true, "show_output": true, "new_tab": true,
	"tmux": true, "zellij": true, "herdr": true, "container": true,
}

// isOldFlatCommandEntry reports whether val looks like an old-style flat
// custom_command entry (i.e. a map whose keys are command fields, not key→cmd maps).
func isOldFlatCommandEntry(val any) bool {
	m, ok := val.(map[string]any)
	if !ok {
		return false
	}
	for k := range m {
		if commandSpecificFields[k] {
			return true
		}
	}
	return false
}

func parseCustomCommands(data map[string]any) (CustomCommandsConfig, []string) {
	raw, ok := data["custom_commands"].(map[string]any)
	if !ok {
		return make(CustomCommandsConfig), nil
	}

	var warnings []string
	migrated := false
	result := make(CustomCommandsConfig)

	for pane, val := range raw {
		if isOldFlatCommandEntry(val) {
			migrated = true
			cmdData, _ := val.(map[string]any)
			if cmd := parseOneCustomCommand(cmdData); cmd != nil {
				if result[PaneUniversal] == nil {
					result[PaneUniversal] = make(map[string]*CustomCommand)
				}
				result[PaneUniversal][pane] = cmd
			}
			continue
		}

		paneMap, ok := val.(map[string]any)
		if !ok {
			continue
		}
		cmds := make(map[string]*CustomCommand)
		for key, cmdVal := range paneMap {
			cmdData, ok := cmdVal.(map[string]any)
			if !ok {
				continue
			}
			if cmd := parseOneCustomCommand(cmdData); cmd != nil {
				cmds[key] = cmd
			}
		}
		if len(cmds) > 0 {
			result[pane] = cmds
		}
	}

	if migrated {
		warnings = append(warnings, "`custom_commands` uses the old flat format. "+
			"Wrap your commands under a pane key (e.g. `universal:`). "+
			"Old entries have been migrated automatically — please update your config file.")
	}

	return result, warnings
}

func parseOneCustomCommand(cmdData map[string]any) *CustomCommand {
	cmd := &CustomCommand{
		Command:     getString(cmdData, "command"),
		Description: getString(cmdData, "description"),
		ShowHelp:    coerceBool(cmdData["show_help"], false),
		Wait:        coerceBool(cmdData["wait"], false),
		ShowOutput:  coerceBool(cmdData["show_output"], false),
		NewTab:      coerceBool(cmdData["new_tab"], false),
	}

	if tmux, ok := cmdData["tmux"].(map[string]any); ok {
		cmd.Tmux = parseTmuxCommand(tmux)
	}
	if zellij, ok := cmdData["zellij"].(map[string]any); ok {
		cmd.Zellij = parseTmuxCommand(zellij)
	}
	if herdr, ok := cmdData["herdr"].(map[string]any); ok {
		cmd.Herdr = parseHerdrCommand(herdr)
	}
	if container, ok := cmdData["container"].(map[string]any); ok {
		cmd.Container = parseContainerCommand(container)
	}

	if cmd.Command != "" || cmd.Tmux != nil || cmd.Zellij != nil || cmd.Herdr != nil || cmd.Container != nil {
		return cmd
	}
	return nil
}

func parseCustomCreateMenus(data map[string]any) []*CustomCreateMenu {
	raw, ok := data["custom_create_menus"].([]any)
	if !ok {
		return nil
	}

	menus := make([]*CustomCreateMenu, 0, len(raw))
	for _, val := range raw {
		mData, ok := val.(map[string]any)
		if !ok {
			continue
		}

		menu := &CustomCreateMenu{
			Label:           getString(mData, "label"),
			Description:     getString(mData, "description"),
			Command:         getString(mData, "command"),
			Interactive:     coerceBool(mData["interactive"], false),
			PostCommand:     getString(mData, "post_command"),
			PostInteractive: coerceBool(mData["post_interactive"], false),
		}
		if menu.Label != "" && menu.Command != "" {
			menus = append(menus, menu)
		}
	}
	return menus
}
