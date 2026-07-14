package agent

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/media"
	"github.com/sipeed/picoclaw/pkg/providers"
)

func TestResolveMediaRefs_InlineImageIsRemovedByMediaTTL(t *testing.T) {
	workspace := t.TempDir()
	store := media.NewFileMediaStoreWithCleanup(media.MediaCleanerConfig{
		Enabled: true,
		MaxAge:  time.Nanosecond,
	})
	dataURL := testInlinePNGDataURL()

	result := resolveMediaRefs([]providers.Message{{
		Role:    "user",
		Content: "describe this image",
		Media:   []string{dataURL},
	}}, store, config.DefaultMaxMediaSize, 0, workspace)

	paths, err := filepath.Glob(filepath.Join(workspace, "tmp", "picoclaw-attachments", "attachment-*.png"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("decoded inline image paths = %v, want exactly one", paths)
	}
	if !strings.Contains(result[0].Content, "[image:"+paths[0]+"]") {
		t.Fatalf("resolved content = %q, want temporary image path", result[0].Content)
	}
	if _, err := os.Stat(paths[0]); err != nil {
		t.Fatalf("temporary image must exist before cleanup: %v", err)
	}

	if cleaned := store.CleanExpired(); cleaned != 1 {
		t.Fatalf("CleanExpired() = %d, want 1", cleaned)
	}
	if _, err := os.Stat(paths[0]); !os.IsNotExist(err) {
		t.Fatalf("temporary image still exists after TTL cleanup: %v", err)
	}
}

func TestResolveMediaRefs_SaveRequestRemainsAgentDecisionAndUsesManagedTemporaryFile(t *testing.T) {
	workspace := t.TempDir()
	store := media.NewFileMediaStoreWithCleanup(media.MediaCleanerConfig{
		Enabled: true,
		MaxAge:  time.Nanosecond,
	})

	result := resolveMediaRefs([]providers.Message{{
		Role:    "user",
		Content: "保存这张图片",
		Media:   []string{testInlinePNGDataURL()},
	}}, store, config.DefaultMaxMediaSize, 0, workspace)

	paths, err := filepath.Glob(filepath.Join(workspace, "tmp", "picoclaw-attachments", "attachment-*.png"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("saved inline image paths = %v, want exactly one", paths)
	}
	if !strings.HasPrefix(result[0].Content, "保存这张图片 ") ||
		!strings.Contains(result[0].Content, "[image:"+paths[0]+"]") {
		t.Fatalf("resolved content = %q, want original request plus temporary image path", result[0].Content)
	}

	if cleaned := store.CleanExpired(); cleaned != 1 {
		t.Fatalf("CleanExpired() = %d, want 1", cleaned)
	}
	if _, err := os.Stat(paths[0]); !os.IsNotExist(err) {
		t.Fatalf("managed temporary image still exists after cleanup: %v", err)
	}
}

func TestResolveMediaRefs_NegatedSaveRequestIsPreservedForAgent(t *testing.T) {
	workspace := t.TempDir()
	content := "不要保存这张图片，只告诉我它是什么"
	result := resolveMediaRefs([]providers.Message{{
		Role:    "user",
		Content: content,
		Media:   []string{testInlinePNGDataURL()},
	}}, media.NewFileMediaStore(), config.DefaultMaxMediaSize, 0, workspace)

	if !strings.HasPrefix(result[0].Content, content+" ") {
		t.Fatalf("resolved content = %q, want negated request preserved", result[0].Content)
	}
	if !strings.Contains(result[0].Content, "[image:") {
		t.Fatalf("resolved content = %q, want temporary image path", result[0].Content)
	}
}

func TestSystemPromptExplainsAgentOwnedAttachmentDecision(t *testing.T) {
	prompt := NewContextBuilder(t.TempDir()).BuildSystemPrompt()
	rule := mediaAttachmentSystemPromptRule()
	if !strings.Contains(prompt, rule) {
		t.Fatalf("system prompt does not contain media attachment rule %q", rule)
	}
}

func testInlinePNGDataURL() string {
	pngBytes := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x02,
		0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE,
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)
}
