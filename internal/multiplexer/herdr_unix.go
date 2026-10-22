//go:build !windows

package multiplexer

import (
	"os/exec"
	"syscall"
)

func configureDetachedHerdrCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
