//go:build !windows

package services

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/chmouel/lazyworktree/internal/models"
)

func agentHookProcessPID(agent models.AgentKind) int {
	return findAgentAncestorPID(agent, os.Getppid(), func(pid int) (agentHookProcessInfo, bool) {
		out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "ppid=,comm=,args=").Output() //nolint:gosec // PID is an integer read from the OS process tree.
		if err != nil {
			return agentHookProcessInfo{}, false
		}
		fields := strings.Fields(string(out))
		if len(fields) < 2 {
			return agentHookProcessInfo{}, false
		}
		parentPID, err := strconv.Atoi(fields[0])
		if err != nil {
			return agentHookProcessInfo{}, false
		}
		args := ""
		if len(fields) > 2 {
			args = strings.Join(fields[2:], " ")
		}
		return agentHookProcessInfo{ParentPID: parentPID, Command: fields[1], Args: args}, true
	})
}

// agentHookPIDAlive reports whether a process with the given PID exists.
func agentHookPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
