//go:build !windows

package api

import (
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

const ignoreTermHelperEnv = "PICOCLAW_TEST_IGNORE_TERM_HELPER"

func TestGatewayIgnoreTermHelperProcess(t *testing.T) {
	readyPath := os.Getenv(ignoreTermHelperEnv)
	if readyPath == "" {
		return
	}

	signal.Ignore(syscall.SIGTERM)
	if err := os.WriteFile(readyPath, []byte("ready"), 0o600); err != nil {
		os.Exit(2)
	}
	for {
		time.Sleep(time.Second)
	}
}

func TestStopGatewayForceKillsOwnedProcessAfterGracePeriod(t *testing.T) {
	resetGatewayTestState(t)

	readyPath := filepath.Join(t.TempDir(), "ready")
	cmd := exec.Command(os.Args[0], "-test.run=TestGatewayIgnoreTermHelperProcess")
	cmd.Env = append(os.Environ(), ignoreTermHelperEnv+"="+readyPath)
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(readyPath); err == nil {
			break
		} else if !os.IsNotExist(err) {
			t.Fatalf("Stat(%q) error = %v", readyPath, err)
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for SIGTERM-ignoring helper")
		}
		time.Sleep(10 * time.Millisecond)
	}

	gatewayRestartGracePeriod = 50 * time.Millisecond
	gatewayRestartForceKillWindow = time.Second
	gatewayRestartPollInterval = 10 * time.Millisecond

	gateway.mu.Lock()
	gateway.cmd = cmd
	gateway.owned = true
	setGatewayRuntimeStatusLocked("running")
	gateway.mu.Unlock()

	h := NewHandler(filepath.Join(t.TempDir(), "config.json"))
	h.StopGateway()

	select {
	case <-waitDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("StopGateway() did not force-kill the owned process")
	}

	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	if gateway.cmd != nil || gateway.owned || gateway.runtimeStatus != "stopped" {
		t.Fatalf(
			"gateway state after forced stop = {cmd:%v owned:%v status:%q}, want stopped and unowned",
			gateway.cmd,
			gateway.owned,
			gateway.runtimeStatus,
		)
	}
}
