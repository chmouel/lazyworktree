//go:build windows

package services

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/chmouel/lazyworktree/internal/models"
	"golang.org/x/sys/windows"
)

func agentHookProcessPID(agent models.AgentKind) int {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(snapshot)

	processes := map[int]agentHookProcessInfo{}
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return 0
	}
	for {
		command := windows.UTF16ToString(entry.ExeFile[:])
		processes[int(entry.ProcessID)] = agentHookProcessInfo{
			ParentPID: int(entry.ParentProcessID),
			Command:   command,
			Args:      filepath.Base(command),
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}

	return findAgentAncestorPID(agent, os.Getppid(), func(pid int) (agentHookProcessInfo, bool) {
		process, ok := processes[pid]
		if !ok {
			return agentHookProcessInfo{}, false
		}
		process.Args = agentHookProcessArgs(pid, process.Command, windowsProcessCommandLine)
		return process, true
	})
}

func windowsProcessCommandLine(pid int) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	filter := "ProcessId = " + strconv.Itoa(pid)
	out, err := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		"(Get-CimInstance -ClassName Win32_Process -Filter '"+filter+"').CommandLine").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// agentHookPIDAlive reports whether the hook-reported process still exists.
func agentHookPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return err == windows.ERROR_ACCESS_DENIED
	}
	_ = windows.CloseHandle(handle)
	return true
}
