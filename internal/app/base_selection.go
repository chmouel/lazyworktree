package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/config"
)

const (
	commitListLimit = 25
	originMain      = "origin/main"
	originMaster    = "origin/master"
)

type branchOption struct {
	name          string
	isRemote      bool
	isTag         bool
	committerDate time.Time
}

type commitOption struct {
	fullHash  string
	shortHash string
	date      string
	subject   string
}

func (m *Model) showBaseSelection(defaultBase string) tea.Cmd {
	// Determine current branch for menu label
	currentBranch := m.getCurrentBranchForMenu()

	// Format label with ellipsis if needed
	fromCurrentLabel := formatCreateFromCurrentLabel(currentBranch)

	// Check for changes in current worktree
	hasChanges := false
	currentWt := m.determineCurrentWorktree()
	if currentWt != nil {
		statusRaw := m.git.RunGit(m.ctx, []string{"git", "status", "--porcelain"}, currentWt.Path, []int{0}, true, false)
		hasChanges = strings.TrimSpace(statusRaw) != ""
	}

	// Set description based on whether changes exist
	fromCurrentDesc := "Create from current branch"
	if hasChanges {
		fromCurrentDesc += " (with or without changes)"
	}

	items := []selectionItem{
		{id: "from-current", label: fromCurrentLabel, description: fromCurrentDesc},
		{id: "branch-list", label: "Pick a base branch or tag", description: "Branches, tags, and remotes"},
		{id: "commit-list", label: "Pick a base commit", description: "Choose a branch, then a commit"},
		{id: "from-pr", label: "Create from PR/MR", description: "Create from a pull/merge request"},
		{id: "from-issue", label: "Create from Issue", description: "Create from a GitHub/GitLab issue"},
		{id: "freeform", label: "Enter base ref manually", description: "Type a branch or commit"},
	}

	// Append custom create menu items from global config
	for i, menu := range m.config.CustomCreateMenus {
		items = append(items, selectionItem{
			id:          fmt.Sprintf("custom-%d", i),
			label:       menu.Label,
			description: menu.Description,
		})
	}

	title := "Select base for new worktree"

	m.listScreen = NewListSelectionScreen(items, title, "Filter options...", "No base options available.", m.windowWidth, m.windowHeight, "", m.theme)
	m.listSubmit = func(item selectionItem) tea.Cmd {
		switch {
		case item.id == "from-current":
			return m.showCreateFromCurrent()
		case item.id == "branch-list":
			return m.showBranchSelection(
				"Select base branch",
				"Filter branches...",
				"No branches found.",
				defaultBase,
				func(branch string) tea.Cmd {
					suggestedName := stripRemotePrefix(branch)
					return m.showBranchNameInput(branch, suggestedName)
				},
			)
		case item.id == "commit-list":
			return m.showCommitSelection(defaultBase)
		case item.id == "freeform":
			return m.showFreeformBaseInput(defaultBase)
		case item.id == "from-pr":
			return m.showCreateFromPR()
		case item.id == "from-issue":
			return m.showCreateFromIssue()
		case strings.HasPrefix(item.id, "custom-"):
			idxStr := strings.TrimPrefix(item.id, "custom-")
			var idx int
			if _, err := fmt.Sscanf(idxStr, "%d", &idx); err == nil {
				if idx >= 0 && idx < len(m.config.CustomCreateMenus) {
					return m.showBaseBranchForCustomCreateMenu(m.config.CustomCreateMenus[idx])
				}
			}
			return nil
		default:
			return nil
		}
	}

	m.currentScreen = screenListSelect
	return textinput.Blink
}

func (m *Model) showFreeformBaseInput(defaultBase string) tea.Cmd {
	m.clearListSelection()
	m.inputScreen = NewInputScreen("Base ref", defaultBase, defaultBase, m.theme)
	m.inputSubmit = func(baseVal string, checked bool) (tea.Cmd, bool) {
		baseRef := strings.TrimSpace(baseVal)
		if baseRef == "" {
			m.inputScreen.errorMsg = "Base ref cannot be empty."
			return nil, false
		}
		if !m.baseRefExists(baseRef) {
			m.inputScreen.errorMsg = "Base ref not found."
			return nil, false
		}
		m.inputScreen.errorMsg = ""
		return m.showBranchNameInput(baseRef, ""), false
	}
	m.currentScreen = screenInput
	return textinput.Blink
}

func (m *Model) showBranchSelection(title, placeholder, noResults, preferred string, onSelect func(string) tea.Cmd) tea.Cmd {
	items := m.branchSelectionItems(preferred)
	m.listScreen = NewListSelectionScreen(items, title, placeholder, noResults, m.windowWidth, m.windowHeight, preferred, m.theme)
	m.listSubmit = func(item selectionItem) tea.Cmd {
		return onSelect(item.id)
	}
	m.currentScreen = screenListSelect
	return textinput.Blink
}

func stripRemotePrefix(branch string) string {
	if idx := strings.Index(branch, "/"); idx > 0 {
		return branch[idx+1:]
	}
	return branch
}

func (m *Model) showCommitSelection(baseBranch string) tea.Cmd {
	raw := m.git.RunGit(
		m.ctx,
		[]string{
			"git", "log",
			fmt.Sprintf("--max-count=%d", commitListLimit),
			"--date=short",
			"--pretty=format:%H%x1f%h%x1f%ad%x1f%s",
			baseBranch,
		},
		"",
		[]int{0},
		true,
		false,
	)
	commits := parseCommitOptions(raw)
	items := buildCommitItems(commits)
	commitLookup := make(map[string]commitOption, len(commits))
	for _, commit := range commits {
		commitLookup[commit.fullHash] = commit
	}
	title := fmt.Sprintf("Select commit from %q", baseBranch)
	noResults := fmt.Sprintf("No commits found on %s.", baseBranch)

	m.listScreen = NewListSelectionScreen(items, title, "Filter commits...", noResults, m.windowWidth, m.windowHeight, "", m.theme)
	m.listSubmit = func(item selectionItem) tea.Cmd {
		m.clearListSelection()
		commit, ok := commitLookup[item.id]
		if !ok {
			commit = commitOption{fullHash: item.id}
		}

		var commitMessage string
		if strings.TrimSpace(m.config.BranchNameScript) != "" {
			commitMessage = m.commitLog(item.id)
		}

		defaultName := ""
		scriptErr := ""
		if strings.TrimSpace(m.config.BranchNameScript) != "" && commitMessage != "" {
			if generatedName, err := runBranchNameScript(m.ctx, m.config.BranchNameScript, commitMessage, "diff", "", "", ""); err != nil {
				scriptErr = fmt.Sprintf("Branch name script error: %v", err)
			} else if generatedName != "" {
				defaultName = generatedName
			}
		}

		if defaultName == "" {
			subject := strings.TrimSpace(commit.subject)
			if subject == "" && commitMessage != "" {
				subject = strings.TrimSpace(strings.Split(commitMessage, "\n")[0])
			}
			defaultName = sanitizeBranchNameFromTitle(subject, commit.shortHash)
		}

		if scriptErr != "" {
			m.showInfo(scriptErr, func() tea.Msg {
				cmd := m.showBranchNameInput(item.id, defaultName)
				if cmd != nil {
					return cmd()
				}
				return nil
			})
			return nil
		}
		return m.showBranchNameInput(item.id, defaultName)
	}
	m.currentScreen = screenListSelect
	return textinput.Blink
}

func (m *Model) showBranchNameInput(baseRef, defaultName string) tea.Cmd {
	m.clearListSelection()
	suggested := strings.TrimSpace(defaultName)
	if suggested != "" {
		suggested = m.suggestBranchName(suggested)
	}
	m.inputScreen = NewInputScreen("Create worktree: branch name", "feature/my-branch", suggested, m.theme)
	m.inputSubmit = func(value string, checked bool) (tea.Cmd, bool) {
		newBranch := strings.TrimSpace(value)
		newBranch = sanitizeBranchNameFromTitle(newBranch, "")
		if newBranch == "" {
			m.inputScreen.errorMsg = errBranchEmpty
			return nil, false
		}

		// Prevent duplicates
		for _, wt := range m.worktrees {
			if wt.Branch == newBranch {
				m.inputScreen.errorMsg = fmt.Sprintf("Branch %q already exists.", newBranch)
				return nil, false
			}
		}

		targetPath := filepath.Join(m.getRepoWorktreeDir(), newBranch)
		if _, err := os.Stat(targetPath); err == nil {
			m.inputScreen.errorMsg = fmt.Sprintf("Path already exists: %s", targetPath)
			return nil, false
		}

		// Show loading screen immediately (before returning from inputSubmit)
		if err := os.MkdirAll(m.getRepoWorktreeDir(), defaultDirPerms); err != nil {
			return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to create worktree directory: %w", err)} }, true
		}
		m.loading = true
		m.statusContent = fmt.Sprintf("Creating worktree from %s...", baseRef)
		m.loadingScreen = NewLoadingScreen(m.statusContent, m.theme)
		m.currentScreen = screenLoading

		return m.createWorktreeFromBaseAsync(newBranch, targetPath, baseRef), true
	}
	m.currentScreen = screenInput
	return textinput.Blink
}

func (m *Model) suggestBranchName(baseName string) string {
	existing := make(map[string]struct{})
	for _, wt := range m.worktrees {
		if wt.Branch == "" || wt.Branch == "(detached)" {
			continue
		}
		existing[wt.Branch] = struct{}{}
	}

	raw := m.git.RunGit(
		m.ctx,
		[]string{
			"git", "for-each-ref",
			"--format=%(refname:short)",
			"refs/heads",
		},
		"",
		[]int{0},
		true,
		false,
	)
	for line := range strings.SplitSeq(strings.TrimSpace(raw), "\n") {
		branch := strings.TrimSpace(line)
		if branch == "" {
			continue
		}
		existing[branch] = struct{}{}
	}

	return suggestBranchNameWithExisting(baseName, existing)
}

func suggestBranchNameWithExisting(baseName string, existing map[string]struct{}) string {
	baseName = strings.TrimSpace(baseName)
	if baseName == "" {
		return ""
	}
	if _, ok := existing[baseName]; !ok {
		return baseName
	}
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s-%d", baseName, i)
		if _, ok := existing[candidate]; !ok {
			return candidate
		}
	}
}

func (m *Model) commitLog(hash string) string {
	return m.git.RunGit(
		m.ctx,
		[]string{"git", "show", "--quiet", "--pretty=format:%s%n%b", hash},
		"",
		[]int{0},
		true,
		false,
	)
}

func (m *Model) baseRefExists(ref string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return false
	}
	refQuery := fmt.Sprintf("%s^{commit}", ref)
	out := m.git.RunGit(
		m.ctx,
		[]string{"git", "rev-parse", "--verify", refQuery},
		"",
		[]int{0, 1},
		true,
		true,
	)
	return strings.TrimSpace(out) != ""
}

func sanitizeBranchNameFromTitle(title, fallback string) string {
	sanitized := strings.ToLower(strings.TrimSpace(title))
	sanitized = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(sanitized, "-")
	sanitized = strings.Trim(sanitized, "-")
	sanitized = regexp.MustCompile(`-+`).ReplaceAllString(sanitized, "-")
	if len(sanitized) > 50 {
		sanitized = strings.TrimRight(sanitized[:50], "-")
	}
	if sanitized == "" {
		sanitized = strings.TrimSpace(fallback)
	}
	if sanitized == "" {
		sanitized = "commit"
	}
	return sanitized
}

// createWorktreeFromBaseAsync performs the actual async worktree creation.
// The LoadingScreen should be set up before calling this.
func (m *Model) createWorktreeFromBaseAsync(newBranch, targetPath, baseRef string) tea.Cmd {
	return func() tea.Msg {
		args := []string{"git", "worktree", "add", "-b", newBranch}
		if strings.Contains(baseRef, "/") {
			args = append(args, "--track")
		}
		args = append(args, targetPath, baseRef)

		ok := m.git.RunCommandChecked(
			m.ctx,
			args,
			"",
			fmt.Sprintf("Failed to create worktree %s", newBranch),
		)
		if !ok {
			return errMsg{err: fmt.Errorf("failed to create worktree %s", newBranch)}
		}

		env := m.buildCommandEnv(newBranch, targetPath)
		initCmds := m.collectInitCommands()

		// Run init commands with trust checks, passing after callback
		after := func() tea.Msg {
			// If there's a custom menu with post-command, run it
			if m.pendingCustomMenu != nil && m.pendingCustomMenu.PostCommand != "" {
				return customPostCommandPendingMsg{
					targetPath: targetPath,
					env:        env,
				}
			}

			// Otherwise just reload worktrees
			worktrees, err := m.git.GetWorktrees(m.ctx)
			return worktreesLoadedMsg{
				worktrees: worktrees,
				err:       err,
			}
		}

		// Return the init commands execution, which will handle the 'after' callback
		cmd := m.runCommandsWithTrust(initCmds, targetPath, env, after)
		if cmd != nil {
			return cmd()
		}
		return after()
	}
}

// createWorktreeFromBase is kept for backward compatibility (e.g., custom create menus)
func (m *Model) createWorktreeFromBase(newBranch, targetPath, baseRef string) tea.Cmd {
	if err := os.MkdirAll(m.getRepoWorktreeDir(), defaultDirPerms); err != nil {
		return func() tea.Msg { return errMsg{err: fmt.Errorf("failed to create worktree directory: %w", err)} }
	}

	// Show loading screen while creating worktree (can take time, so do it async with a loading pulse)
	m.loading = true
	m.statusContent = fmt.Sprintf("Creating worktree from %s...", baseRef)
	m.loadingScreen = NewLoadingScreen(m.statusContent, m.theme)
	m.currentScreen = screenLoading

	return m.createWorktreeFromBaseAsync(newBranch, targetPath, baseRef)
}

func (m *Model) clearListSelection() {
	m.listScreen = nil
	m.listSubmit = nil
	if m.currentScreen == screenListSelect {
		m.currentScreen = screenNone
	}
}

func (m *Model) branchSelectionItems(preferred string) []selectionItem {
	raw := m.git.RunGit(
		m.ctx,
		[]string{
			"git", "for-each-ref",
			"--sort=-committerdate",
			"--format=%(refname:short)\t%(refname)\t%(committerdate:unix)",
			"refs/heads",
			"refs/remotes",
			"refs/tags",
		},
		"",
		[]int{0},
		true,
		false,
	)

	options := parseBranchOptionsWithDate(raw)
	options = sortBranchOptions(options)
	items := make([]selectionItem, 0, len(options))
	for _, opt := range options {
		desc := ""
		if opt.isRemote {
			desc = "remote"
		} else if opt.isTag {
			desc = "tag"
		}
		items = append(items, selectionItem{
			id:          opt.name,
			label:       opt.name,
			description: desc,
		})
	}
	return items
}

func parseBranchOptionsWithDate(raw string) []branchOption {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) == 1 && strings.TrimSpace(lines[0]) == "" {
		return nil
	}

	options := make([]branchOption, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		fullRef := strings.TrimSpace(parts[1])
		timestampStr := strings.TrimSpace(parts[2])
		if name == "" || fullRef == "" {
			continue
		}
		if strings.HasSuffix(name, "/HEAD") {
			continue
		}

		var commitDate time.Time
		if timestamp, err := strconv.ParseInt(timestampStr, 10, 64); err == nil {
			commitDate = time.Unix(timestamp, 0)
		}

		options = append(options, branchOption{
			name:          name,
			isRemote:      strings.HasPrefix(fullRef, "refs/remotes/"),
			isTag:         strings.HasPrefix(fullRef, "refs/tags/"),
			committerDate: commitDate,
		})
	}
	return options
}

func sortBranchOptions(options []branchOption) []branchOption {
	if len(options) == 0 {
		return options
	}

	var localMain, localMaster, remoteOriginMain, remoteOriginMaster *branchOption
	others := make([]branchOption, 0, len(options))

	for i := range options {
		opt := &options[i]
		switch {
		case opt.name == "main" && !opt.isRemote && !opt.isTag:
			localMain = opt
		case opt.name == "master" && !opt.isRemote && !opt.isTag:
			localMaster = opt
		case opt.name == originMain && opt.isRemote && !opt.isTag:
			remoteOriginMain = opt
		case opt.name == originMaster && opt.isRemote && !opt.isTag:
			remoteOriginMaster = opt
		default:
			others = append(others, *opt)
		}
	}

	// Sort others by commit date (descending), then alphabetically
	for i := 0; i < len(others); i++ {
		for j := i + 1; j < len(others); j++ {
			// Sort by date descending (newer first)
			if others[i].committerDate.Before(others[j].committerDate) {
				others[i], others[j] = others[j], others[i]
			} else if others[i].committerDate.Equal(others[j].committerDate) {
				// If dates are equal, sort alphabetically
				if others[i].name > others[j].name {
					others[i], others[j] = others[j], others[i]
				}
			}
		}
	}

	// Build result in priority order
	result := make([]branchOption, 0, len(options))
	if localMain != nil {
		result = append(result, *localMain)
	} else if localMaster != nil {
		result = append(result, *localMaster)
	}

	if remoteOriginMain != nil {
		result = append(result, *remoteOriginMain)
	}
	if remoteOriginMaster != nil {
		result = append(result, *remoteOriginMaster)
	}

	result = append(result, others...)
	return result
}

func parseCommitOptions(raw string) []commitOption {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) == 1 && strings.TrimSpace(lines[0]) == "" {
		return nil
	}

	options := make([]commitOption, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1f", 4)
		if len(parts) < 4 {
			continue
		}
		full := strings.TrimSpace(parts[0])
		short := strings.TrimSpace(parts[1])
		date := strings.TrimSpace(parts[2])
		subject := strings.TrimSpace(parts[3])
		if full == "" {
			continue
		}
		options = append(options, commitOption{
			fullHash:  full,
			shortHash: short,
			date:      date,
			subject:   subject,
		})
	}
	return options
}

func buildCommitItems(options []commitOption) []selectionItem {
	items := make([]selectionItem, 0, len(options))
	for _, opt := range options {
		label := strings.TrimSpace(fmt.Sprintf("%s %s", opt.shortHash, opt.subject))
		items = append(items, selectionItem{
			id:          opt.fullHash,
			label:       label,
			description: opt.date,
		})
	}
	return items
}

// executeCustomCreateCommand runs a custom create menu command and returns the result.
func (m *Model) executeCustomCreateCommand(menu *config.CustomCreateMenu) tea.Cmd {
	m.clearListSelection()

	// Get main worktree path for command execution
	mainWorktreePath := ""
	if len(m.worktrees) > 0 {
		for _, wt := range m.worktrees {
			if wt.IsMain {
				mainWorktreePath = wt.Path
				break
			}
		}
	}

	if menu.Interactive {
		// Interactive mode: suspend TUI, run command in terminal, capture stdout via temp file
		return m.executeCustomCreateCommandInteractive(menu, mainWorktreePath)
	}

	// Non-interactive mode: capture stdout directly
	m.loadingScreen = NewLoadingScreen(fmt.Sprintf("Running: %s", menu.Label), m.theme)
	m.currentScreen = screenLoading

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
		defer cancel()

		// #nosec G204 -- user-configured command from trusted config
		cmd := exec.CommandContext(ctx, "bash", "-c", menu.Command)
		cmd.Dir = mainWorktreePath

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			errMsg := strings.TrimSpace(stderr.String())
			if errMsg == "" {
				errMsg = err.Error()
			}
			return customCreateResultMsg{err: fmt.Errorf("%s", errMsg)}
		}

		output := strings.TrimSpace(stdout.String())
		if output == "" {
			return customCreateResultMsg{err: fmt.Errorf("command produced no output")}
		}

		// Take the first line of output
		if idx := strings.Index(output, "\n"); idx > 0 {
			output = output[:idx]
		}

		// Use output as-is (preserve case, no lowercasing)
		branchName := strings.TrimSpace(output)
		branchName = sanitizeBranchNameFromTitle(branchName, "")

		return customCreateResultMsg{branchName: branchName}
	}
}

// executeCustomCreateCommandInteractive runs a custom create command interactively.
// The TUI suspends, the command runs in the terminal, and stdout is captured to a temp file.
func (m *Model) executeCustomCreateCommandInteractive(menu *config.CustomCreateMenu, workDir string) tea.Cmd {
	// Create temp file for capturing stdout
	tmpFile, err := os.CreateTemp("", "lazyworktree-custom-create-*")
	if err != nil {
		return func() tea.Msg {
			return customCreateResultMsg{err: fmt.Errorf("failed to create temp file: %w", err)}
		}
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()

	// Wrap command to redirect stdout to temp file
	// Interactive commands typically write UI to stderr or /dev/tty
	wrappedCmd := fmt.Sprintf("%s > %s", menu.Command, tmpPath)

	// #nosec G204 -- user-configured command from trusted config
	c := m.commandRunner("bash", "-c", wrappedCmd)
	c.Dir = workDir

	return m.execProcess(c, func(err error) tea.Msg {
		defer func() { _ = os.Remove(tmpPath) }()

		if err != nil {
			return customCreateResultMsg{err: err}
		}

		// Read branch name from temp file
		// #nosec G304 -- tmpPath is created by os.CreateTemp, not user input
		content, readErr := os.ReadFile(tmpPath)
		if readErr != nil {
			return customCreateResultMsg{err: fmt.Errorf("failed to read output: %w", readErr)}
		}

		output := strings.TrimSpace(string(content))
		if output == "" {
			return customCreateResultMsg{err: fmt.Errorf("command produced no output")}
		}

		// Take the first line of output
		if idx := strings.Index(output, "\n"); idx > 0 {
			output = output[:idx]
		}

		// Use output as-is (preserve case, no lowercasing)
		branchName := strings.TrimSpace(output)
		branchName = sanitizeBranchNameFromTitle(branchName, "")

		return customCreateResultMsg{branchName: branchName}
	})
}

// showBaseBranchForCustomCreateMenu shows the branch picker before running a custom create command.
func (m *Model) showBaseBranchForCustomCreateMenu(menu *config.CustomCreateMenu) tea.Cmd {
	return m.showBranchSelection(
		"Select base branch for worktree",
		"Filter branches...",
		"No branches found.",
		"",
		func(branch string) tea.Cmd {
			// Store base branch and menu for later use
			m.pendingCustomBaseRef = branch
			m.pendingCustomMenu = menu
			// Now run the command
			return m.executeCustomCreateCommand(menu)
		},
	)
}

// executeCustomPostCommand runs a non-interactive post-creation command in the new worktree directory.
func (m *Model) executeCustomPostCommand(script, targetPath string, env map[string]string) tea.Cmd {
	m.loadingScreen = NewLoadingScreen("Running post-creation command...", m.theme)
	m.currentScreen = screenLoading

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
		defer cancel()

		// #nosec G204 -- user-configured command from trusted config
		cmd := exec.CommandContext(ctx, "bash", "-c", script)
		cmd.Dir = targetPath

		// Merge environment variables
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}

		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			errMsg := strings.TrimSpace(stderr.String())
			if errMsg == "" {
				errMsg = err.Error()
			}
			return customPostCommandResultMsg{err: fmt.Errorf("%s", errMsg)}
		}

		return customPostCommandResultMsg{err: nil}
	}
}

// executeCustomPostCommandInteractive runs an interactive post-creation command in the new worktree directory.
// The TUI suspends and the command runs in the terminal.
func (m *Model) executeCustomPostCommandInteractive(script, targetPath string, env map[string]string) tea.Cmd {
	// Build environment for command
	envList := os.Environ()
	for k, v := range env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}

	// #nosec G204 -- user-configured command from trusted config
	c := m.commandRunner("bash", "-c", script)
	c.Dir = targetPath
	c.Env = envList

	return m.execProcess(c, func(err error) tea.Msg {
		if err != nil {
			return customPostCommandResultMsg{err: err}
		}
		return customPostCommandResultMsg{err: nil}
	})
}
