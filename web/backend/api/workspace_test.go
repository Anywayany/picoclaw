package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sipeed/picoclaw/pkg/config"
)

func setupWorkspaceHandler(t *testing.T) (*http.ServeMux, string) {
	t.Helper()
	configPath, cleanup := setupOAuthTestEnv(t)
	t.Cleanup(cleanup)

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	workspace := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	cfg.Agents.Defaults.Workspace = workspace
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return mux, workspace
}

func TestHandleListWorkspaceDirectory(t *testing.T) {
	mux, workspace := setupWorkspaceHandler(t)
	if err := os.MkdirAll(filepath.Join(workspace, "docs"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "docs", "guide.md"), []byte("guide"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/workspace", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var root workspaceDirectoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &root); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if root.Name != "workspace" || root.Path != "" {
		t.Fatalf("root = %#v, want workspace root", root)
	}
	if len(root.Entries) != 2 || root.Entries[0].Name != "docs" || root.Entries[0].Type != "directory" {
		t.Fatalf("entries = %#v, want directory first", root.Entries)
	}
	if root.Entries[1].Name != "README.md" || root.Entries[1].Size != 5 {
		t.Fatalf("file entry = %#v, want README.md", root.Entries[1])
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/workspace?path=docs", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("nested status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var nested workspaceDirectoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &nested); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if nested.Path != "docs" || len(nested.Entries) != 1 || nested.Entries[0].Path != "docs/guide.md" {
		t.Fatalf("nested = %#v, want docs/guide.md", nested)
	}
}

func TestHandleListWorkspaceDirectoryRejectsEscapes(t *testing.T) {
	mux, workspace := setupWorkspaceHandler(t)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/workspace?path=../", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("traversal status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if runtime.GOOS == "windows" {
		return
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(workspace, "outside")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/workspace?path=outside", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("symlink escape status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
