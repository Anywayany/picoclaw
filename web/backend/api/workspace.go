package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sipeed/picoclaw/pkg/config"
)

const maxWorkspaceDirectoryEntries = 5000

type workspaceEntry struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Size     int64  `json:"size,omitempty"`
	Modified string `json:"modified,omitempty"`
}

type workspaceDirectoryResponse struct {
	Name    string           `json:"name"`
	Path    string           `json:"path"`
	Entries []workspaceEntry `json:"entries"`
}

func (h *Handler) registerWorkspaceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/workspace", h.handleListWorkspaceDirectory)
}

func resolveWorkspaceDirectory(root, relativePath string) (string, string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", err
	}

	relativePath = strings.TrimSpace(relativePath)
	if relativePath == "" || relativePath == "." {
		return root, "", nil
	}

	relativePath = filepath.Clean(filepath.FromSlash(relativePath))
	if filepath.IsAbs(relativePath) || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("path must stay inside the workspace")
	}

	target, err := filepath.EvalSymlinks(filepath.Join(root, relativePath))
	if err != nil {
		return "", "", err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("path must stay inside the workspace")
	}

	return target, filepath.ToSlash(rel), nil
}

func (h *Handler) handleListWorkspaceDirectory(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	directory, relativePath, err := resolveWorkspaceDirectory(cfg.WorkspacePath(), r.URL.Query().Get("path"))
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Workspace directory not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Invalid workspace path: %v", err), http.StatusBadRequest)
		return
	}

	info, err := os.Stat(directory)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Workspace directory not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to inspect workspace directory", http.StatusInternalServerError)
		return
	}
	if !info.IsDir() {
		http.Error(w, "Workspace path is not a directory", http.StatusBadRequest)
		return
	}

	directoryEntries, err := os.ReadDir(directory)
	if err != nil {
		http.Error(w, "Failed to read workspace directory", http.StatusInternalServerError)
		return
	}
	if len(directoryEntries) > maxWorkspaceDirectoryEntries {
		http.Error(w, "Workspace directory contains too many entries", http.StatusRequestEntityTooLarge)
		return
	}

	entries := make([]workspaceEntry, 0, len(directoryEntries))
	for _, entry := range directoryEntries {
		entryType := "file"
		if entry.IsDir() {
			entryType = "directory"
		} else if entry.Type()&os.ModeSymlink != 0 {
			entryType = "symlink"
		}

		item := workspaceEntry{
			Name: entry.Name(),
			Path: filepath.ToSlash(filepath.Join(relativePath, entry.Name())),
			Type: entryType,
		}
		if entryInfo, infoErr := entry.Info(); infoErr == nil {
			if entryType == "file" {
				item.Size = entryInfo.Size()
			}
			item.Modified = entryInfo.ModTime().UTC().Format("2006-01-02T15:04:05Z")
		}
		entries = append(entries, item)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type == "directory" && entries[j].Type != "directory" {
			return true
		}
		if entries[i].Type != "directory" && entries[j].Type == "directory" {
			return false
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	name := filepath.Base(cfg.WorkspacePath())
	if relativePath != "" {
		name = filepath.Base(directory)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(workspaceDirectoryResponse{
		Name:    name,
		Path:    relativePath,
		Entries: entries,
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
