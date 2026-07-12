package services

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/chmouel/lazyworktree/internal/models"
)

type agentProcessRunner func(name string, args ...string) *exec.Cmd

// AgentProcess describes a live agent process that may map to a session.
type AgentProcess struct {
	PID       int
	Agent     models.AgentKind
	Source    string
	Command   string
	Args      string
	CWD       string
	OpenFiles []string
}

// AgentProcessService snapshots live agent CLI processes (Claude, Codex,
// Copilot, and pi). The ps/lsof process scan is deprecated and opt-in via
// the agent_sessions.process_scan configuration key; hook events installed
// with `lazyworktree setup-hooks` are the default liveness source.
type AgentProcessService struct {
	mu           sync.RWMutex
	process      []*AgentProcess
	detailsValid bool
	runCmd       agentProcessRunner
	logf         func(string, ...any)
}

// NewAgentProcessService builds a process service with the default command runner.
func NewAgentProcessService(logf func(string, ...any)) *AgentProcessService {
	return NewAgentProcessServiceWithRunner(exec.Command, logf)
}

// NewAgentProcessServiceWithRunner builds a process service with an injected runner for tests.
func NewAgentProcessServiceWithRunner(runCmd agentProcessRunner, logf func(string, ...any)) *AgentProcessService {
	if runCmd == nil {
		runCmd = exec.Command
	}
	return &AgentProcessService{
		runCmd: runCmd,
		logf:   logf,
	}
}

// Refresh discovers live Claude, Codex, and pi processes.
func (s *AgentProcessService) Refresh() ([]*AgentProcess, error) {
	if runtime.GOOS == "windows" {
		s.mu.Lock()
		s.process = nil
		s.mu.Unlock()
		return nil, nil
	}

	processes, err := s.listProcesses()
	if err != nil {
		return nil, err
	}
	detailsValid := false
	if len(processes) > 0 {
		s.mu.RLock()
		previous := s.process
		previousValid := s.detailsValid
		s.mu.RUnlock()
		// lsof is expensive (especially on macOS); when the process set is
		// unchanged since the last scan, reuse the cached details instead.
		// Per-process cwd/open-file changes may lag one refresh interval.
		if previousValid && reuseAgentProcessDetails(processes, previous) {
			detailsValid = true
		} else if err := s.populateProcessDetails(processes); err != nil {
			if s.logf != nil {
				s.logf("agent processes: detail lookup failed: %v", err)
			}
		} else {
			detailsValid = true
		}
	}

	sort.Slice(processes, func(i, j int) bool {
		if processes[i].Agent == processes[j].Agent {
			return processes[i].PID < processes[j].PID
		}
		return processes[i].Agent < processes[j].Agent
	})

	s.mu.Lock()
	s.process = cloneAgentProcesses(processes)
	s.detailsValid = detailsValid
	s.mu.Unlock()
	return cloneAgentProcesses(processes), nil
}

// reuseAgentProcessDetails copies cached lsof details onto the fresh snapshot
// when it describes the exact same set of processes as the previous one.
// It reports false without modifying anything when the sets differ.
func reuseAgentProcessDetails(processes, previous []*AgentProcess) bool {
	if len(processes) != len(previous) {
		return false
	}
	byPID := make(map[int]*AgentProcess, len(previous))
	for _, prev := range previous {
		if prev != nil {
			byPID[prev.PID] = prev
		}
	}
	for _, process := range processes {
		if process == nil {
			return false
		}
		prev, ok := byPID[process.PID]
		if !ok || prev.Command != process.Command || prev.Args != process.Args {
			return false
		}
	}
	for _, process := range processes {
		prev := byPID[process.PID]
		process.CWD = prev.CWD
		if len(prev.OpenFiles) > 0 {
			process.OpenFiles = append([]string(nil), prev.OpenFiles...)
		}
	}
	return true
}

// Processes returns the last live-process snapshot.
func (s *AgentProcessService) Processes() []*AgentProcess {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneAgentProcesses(s.process)
}

func (s *AgentProcessService) listProcesses() ([]*AgentProcess, error) {
	out, err := s.runCmd("ps", "-axo", "pid=,comm=,args=").Output()
	if err != nil {
		return nil, err
	}
	return parseAgentProcessesPS(string(out)), nil
}

func (s *AgentProcessService) populateProcessDetails(processes []*AgentProcess) error {
	args := []string{"-n", "-P", "-F", "pfn"}
	for _, process := range processes {
		if process == nil || process.PID <= 0 {
			continue
		}
		args = append(args, "-p", strconv.Itoa(process.PID))
	}
	out, err := s.runCmd("lsof", args...).Output()
	if err != nil {
		return err
	}
	applyAgentProcessLSOF(processes, string(out))
	return nil
}

func parseAgentProcessesPS(out string) []*AgentProcess {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	processes := make([]*AgentProcess, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		command := fields[1]
		args := ""
		if len(fields) > 2 {
			args = strings.Join(fields[2:], " ")
		}
		agent, source, ok := classifyAgentProcess(command, args)
		if !ok {
			continue
		}
		processes = append(processes, &AgentProcess{
			PID:     pid,
			Agent:   agent,
			Source:  source,
			Command: command,
			Args:    args,
		})
	}
	return processes
}

func classifyAgentProcess(command, args string) (models.AgentKind, string, bool) {
	base := filepath.Base(strings.TrimSpace(command))
	if strings.EqualFold(filepath.Ext(base), ".exe") {
		base = strings.TrimSuffix(base, filepath.Ext(base))
	}
	switch {
	case strings.Contains(args, "Claude.app") || base == "Claude":
		return models.AgentKindClaude, "desktop", true
	case isClaudeCLIProcess(base, args):
		return models.AgentKindClaude, "cli", true
	case isCodexCLIProcess(base, args):
		return models.AgentKindCodex, "cli", true
	case isCopilotCLIProcess(base, args):
		return models.AgentKindCopilot, "cli", true
	case strings.EqualFold(base, "pi"):
		return models.AgentKindPi, "cli", true
	default:
		return "", "", false
	}
}

func isCodexCLIProcess(commandBase, args string) bool {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(commandBase)))
	if base == "codex" {
		return true
	}

	tokens := splitCommandTokens(args)
	switch base {
	case "node", "bun":
		for _, token := range tokens {
			normalized := strings.ReplaceAll(strings.ToLower(token), `\`, "/")
			if strings.Contains(normalized, "/@openai/codex/") ||
				strings.EqualFold(filepath.Base(token), "codex") {
				return true
			}
		}
	case "npm", "npx", "pnpm", "yarn":
		for _, token := range tokens {
			if strings.EqualFold(token, "@openai/codex") {
				return true
			}
		}
	}
	return false
}

func isCopilotCLIProcess(commandBase, args string) bool {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(commandBase)))
	if base == "copilot" {
		return true
	}

	tokens := splitCommandTokens(args)
	switch base {
	case "node", "bun":
		for _, token := range tokens {
			normalized := strings.ReplaceAll(strings.ToLower(token), `\`, "/")
			if strings.Contains(normalized, "/@github/copilot/") ||
				strings.EqualFold(filepath.Base(token), "copilot") {
				return true
			}
		}
	case "npm", "npx", "pnpm", "yarn":
		for _, token := range tokens {
			if strings.EqualFold(token, "@github/copilot") {
				return true
			}
		}
	}
	return false
}

func isClaudeCLIProcess(commandBase, args string) bool {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(commandBase)))
	if base == "claude" || base == "claude-code" {
		return true
	}

	tokens := splitCommandTokens(args)
	switch base {
	case "node", "bun":
		return nodeWrapperInvokesClaude(tokens, base)
	case "npm", "npx", "pnpm", "yarn":
		return packageManagerInvokesClaude(tokens, base)
	case "sh", "bash", "zsh", "dash", "ksh", "fish":
		return shellInvokesClaude(tokens, base)
	default:
		return false
	}
}

func splitCommandTokens(args string) []string {
	raw := strings.Fields(strings.TrimSpace(args))
	tokens := make([]string, 0, len(raw))
	for _, token := range raw {
		token = strings.Trim(strings.TrimSpace(token), `"'`)
		if token != "" {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func nodeWrapperInvokesClaude(tokens []string, commandBase string) bool {
	for _, token := range trimLeadingCommand(tokens, commandBase) {
		if strings.HasPrefix(token, "-") {
			continue
		}
		return isClaudeCLIToken(token)
	}
	return false
}

func packageManagerInvokesClaude(tokens []string, commandBase string) bool {
	remaining := trimLeadingCommand(tokens, commandBase)
	for i, token := range remaining {
		if strings.HasPrefix(token, "-") {
			continue
		}
		switch strings.ToLower(token) {
		case "exec", "dlx", "x":
			for _, candidate := range remaining[i+1:] {
				if strings.HasPrefix(candidate, "-") {
					continue
				}
				return isClaudeCLIToken(candidate)
			}
			return false
		default:
			return isClaudeCLIToken(token)
		}
	}
	return false
}

func shellInvokesClaude(tokens []string, commandBase string) bool {
	remaining := trimLeadingCommand(tokens, commandBase)
	shellCommandExpected := false
	for _, token := range remaining {
		if token == "" {
			continue
		}
		if shellCommandExpected {
			if strings.EqualFold(token, "exec") {
				continue
			}
			return isClaudeCLIToken(token)
		}
		switch token {
		case "-c", "-lc", "-ic":
			shellCommandExpected = true
		}
	}
	return false
}

func trimLeadingCommand(tokens []string, commandBase string) []string {
	if len(tokens) == 0 {
		return nil
	}
	if strings.EqualFold(filepath.Base(tokens[0]), commandBase) {
		return tokens[1:]
	}
	return tokens
}

func isClaudeCLIToken(token string) bool {
	token = strings.Trim(strings.TrimSpace(token), `"'`)
	if token == "" {
		return false
	}

	lowerToken := strings.ToLower(token)
	switch strings.ToLower(filepath.Base(token)) {
	case "claude", "claude-code":
		return true
	}
	return strings.Contains(lowerToken, "@anthropic-ai/claude-code")
}

func applyAgentProcessLSOF(processes []*AgentProcess, out string) {
	byPID := make(map[int]*AgentProcess, len(processes))
	for _, process := range processes {
		if process != nil {
			byPID[process.PID] = process
		}
	}

	var current *AgentProcess
	currentFD := ""
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		switch line[0] {
		case 'p':
			pid, err := strconv.Atoi(line[1:])
			if err != nil {
				current = nil
				currentFD = ""
				continue
			}
			current = byPID[pid]
			currentFD = ""
		case 'f':
			currentFD = line[1:]
		case 'n':
			if current == nil {
				continue
			}
			name := filepath.Clean(strings.TrimSpace(line[1:]))
			if !filepath.IsAbs(name) {
				continue
			}
			if currentFD == "cwd" {
				current.CWD = name
				continue
			}
			current.OpenFiles = append(current.OpenFiles, name)
		}
	}
}

func cloneAgentProcesses(in []*AgentProcess) []*AgentProcess {
	if len(in) == 0 {
		return nil
	}
	out := make([]*AgentProcess, 0, len(in))
	for _, process := range in {
		if process == nil {
			continue
		}
		copied := *process
		if len(process.OpenFiles) > 0 {
			copied.OpenFiles = append([]string(nil), process.OpenFiles...)
		}
		out = append(out, &copied)
	}
	return out
}
