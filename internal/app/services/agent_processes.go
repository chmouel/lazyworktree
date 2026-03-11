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

// AgentProcess describes a live Claude or pi process that may map to a session.
type AgentProcess struct {
	PID       int
	Agent     models.AgentKind
	Source    string
	Command   string
	Args      string
	CWD       string
	OpenFiles []string
}

// AgentProcessService snapshots live Claude/pi processes.
type AgentProcessService struct {
	mu      sync.RWMutex
	process []*AgentProcess
	runCmd  agentProcessRunner
	logf    func(string, ...any)
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

// Refresh discovers live Claude and pi processes.
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
	if len(processes) > 0 {
		if err := s.populateProcessDetails(processes); err != nil && s.logf != nil {
			s.logf("agent processes: detail lookup failed: %v", err)
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
	s.mu.Unlock()
	return cloneAgentProcesses(processes), nil
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
	switch {
	case strings.Contains(args, "Claude.app") || base == "Claude":
		return models.AgentKindClaude, "desktop", true
	case strings.EqualFold(base, "claude"):
		return models.AgentKindClaude, "cli", true
	case strings.EqualFold(base, "pi"):
		return models.AgentKindPi, "cli", true
	default:
		return "", "", false
	}
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
