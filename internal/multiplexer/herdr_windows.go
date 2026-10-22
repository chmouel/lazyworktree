//go:build windows

package multiplexer

import (
	"os/exec"
	"syscall"
)

const detachedProcess = 0x00000008

func configureDetachedHerdrCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess}
}
