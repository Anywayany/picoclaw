package gateway

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/media"
)

func TestStartRuntimeMediaStoreSweepsExpiredAttachments(t *testing.T) {
	workspace := t.TempDir()
	dir := filepath.Join(workspace, "tmp", "picoclaw-attachments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	oldPath := filepath.Join(dir, "expired.png")
	freshPath := filepath.Join(dir, "fresh.png")
	for _, path := range []string{oldPath, freshPath} {
		if err := os.WriteFile(path, []byte("image"), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", path, err)
		}
	}
	oldTime := time.Now().Add(-11 * time.Minute)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = workspace
	cfg.Tools.MediaCleanup.Enabled = true
	cfg.Tools.MediaCleanup.MaxAge = 10
	cfg.Tools.MediaCleanup.Interval = 60

	store := startRuntimeMediaStore(cfg)
	t.Cleanup(func() {
		if fileStore, ok := store.(*media.FileMediaStore); ok {
			fileStore.Stop()
		}
	})

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("expired attachment still exists; Stat() error = %v", err)
	}
	if _, err := os.Stat(freshPath); err != nil {
		t.Fatalf("fresh attachment should remain; Stat() error = %v", err)
	}
}

func TestStartRuntimeMediaStoreDoesNotSweepWhenDisabled(t *testing.T) {
	workspace := t.TempDir()
	dir := filepath.Join(workspace, "tmp", "picoclaw-attachments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	oldPath := filepath.Join(dir, "expired.png")
	if err := os.WriteFile(oldPath, []byte("image"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	oldTime := time.Now().Add(-11 * time.Minute)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = workspace
	cfg.Tools.MediaCleanup.Enabled = false
	cfg.Tools.MediaCleanup.MaxAge = 10
	cfg.Tools.MediaCleanup.Interval = 60

	store := startRuntimeMediaStore(cfg)
	t.Cleanup(func() {
		if fileStore, ok := store.(*media.FileMediaStore); ok {
			fileStore.Stop()
		}
	})

	if _, err := os.Stat(oldPath); err != nil {
		t.Fatalf("disabled cleanup should preserve attachment; Stat() error = %v", err)
	}
}
