package media

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSweepExpiredFilesRemovesOnlyExpiredRegularFiles(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	oldPath := createTempFile(t, dir, "old.png")
	freshPath := createTempFile(t, dir, "fresh.png")
	nestedDir := filepath.Join(dir, "nested")
	if err := os.Mkdir(nestedDir, 0o700); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	if err := os.Chtimes(oldPath, now.Add(-20*time.Minute), now.Add(-20*time.Minute)); err != nil {
		t.Fatalf("Chtimes(old) error = %v", err)
	}
	if err := os.Chtimes(freshPath, now, now); err != nil {
		t.Fatalf("Chtimes(fresh) error = %v", err)
	}

	removed, err := SweepExpiredFiles(dir, 10*time.Minute)
	if err != nil {
		t.Fatalf("SweepExpiredFiles() error = %v", err)
	}
	if removed != 1 {
		t.Fatalf("SweepExpiredFiles() removed = %d, want 1", removed)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("expired file still exists: %v", err)
	}
	if _, err := os.Stat(freshPath); err != nil {
		t.Fatalf("fresh file was removed: %v", err)
	}
	if _, err := os.Stat(nestedDir); err != nil {
		t.Fatalf("nested directory was removed: %v", err)
	}
}

func TestSweepExpiredFilesSkipsSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := createTempFile(t, t.TempDir(), "outside.png")
	link := filepath.Join(dir, "old-link.png")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("Symlink() unavailable: %v", err)
	}

	removed, err := SweepExpiredFiles(dir, time.Nanosecond)
	if err != nil {
		t.Fatalf("SweepExpiredFiles() error = %v", err)
	}
	if removed != 0 {
		t.Fatalf("SweepExpiredFiles() removed = %d, want 0", removed)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatalf("symlink was removed: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("symlink target was removed: %v", err)
	}
}

func TestSweepExpiredFilesMissingDirAndDisabledAgeAreNoOps(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if removed, err := SweepExpiredFiles(missing, time.Minute); err != nil || removed != 0 {
		t.Fatalf("missing dir: removed=%d err=%v, want 0 nil", removed, err)
	}

	dir := t.TempDir()
	path := createTempFile(t, dir, "old.png")
	if removed, err := SweepExpiredFiles(dir, 0); err != nil || removed != 0 {
		t.Fatalf("disabled age: removed=%d err=%v, want 0 nil", removed, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("disabled sweep removed file: %v", err)
	}
}
