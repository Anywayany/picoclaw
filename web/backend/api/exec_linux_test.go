//go:build linux

package api

import (
	"os/exec"
	"syscall"
	"testing"
)

func TestApplyLauncherProcAttrsSetsParentDeathSignal(t *testing.T) {
	cmd := exec.Command("true")
	applyLauncherProcAttrs(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("applyLauncherProcAttrs() did not initialize SysProcAttr")
	}
	if cmd.SysProcAttr.Pdeathsig != syscall.SIGTERM {
		t.Fatalf(
			"Pdeathsig = %v, want %v",
			cmd.SysProcAttr.Pdeathsig,
			syscall.SIGTERM,
		)
	}
}
