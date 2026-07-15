//go:build linux

package api

import (
	"os/exec"
	"syscall"

	"github.com/sipeed/picoclaw/web/backend/utils"
)

func launcherExecCommand(name string, args ...string) *exec.Cmd {
	return utils.LauncherExecCommand(name, args...)
}

func applyLauncherProcAttrs(cmd *exec.Cmd) {
	utils.ApplyLauncherProcAttrs(cmd)
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	// Ask the kernel to terminate a launcher-owned gateway if the launcher
	// disappears without getting a chance to run its normal shutdown path.
	cmd.SysProcAttr.Pdeathsig = syscall.SIGTERM
}

func attachLauncherProcessLifetime(_ *exec.Cmd) (func(), error) {
	return nil, nil
}
