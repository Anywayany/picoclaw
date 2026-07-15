//go:build windows

package api

import (
	"fmt"
	"os/exec"
	"sync"
	"unsafe"

	"github.com/sipeed/picoclaw/web/backend/utils"
	"golang.org/x/sys/windows"
)

func launcherExecCommand(name string, args ...string) *exec.Cmd {
	return utils.LauncherExecCommand(name, args...)
}

func applyLauncherProcAttrs(cmd *exec.Cmd) {
	utils.ApplyLauncherProcAttrs(cmd)
}

func attachLauncherProcessLifetime(cmd *exec.Cmd) (func(), error) {
	if cmd == nil || cmd.Process == nil {
		return nil, fmt.Errorf("gateway process is not started")
	}

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create gateway job object: %w", err)
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		windows.CloseHandle(job)
		return nil, fmt.Errorf("configure gateway job object: %w", err)
	}

	process, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(cmd.Process.Pid),
	)
	if err != nil {
		windows.CloseHandle(job)
		return nil, fmt.Errorf("open gateway process: %w", err)
	}
	defer windows.CloseHandle(process)

	if err = windows.AssignProcessToJobObject(job, process); err != nil {
		windows.CloseHandle(job)
		return nil, fmt.Errorf("assign gateway process to job object: %w", err)
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			_ = windows.CloseHandle(job)
		})
	}, nil
}
