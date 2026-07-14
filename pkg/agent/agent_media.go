// PicoClaw - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package agent

import (
	"bytes"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/h2non/filetype"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/media"
	"github.com/sipeed/picoclaw/pkg/providers"
)

// genericPlaceholderRegex matches generic media placeholders emitted by various
// channels: [image], [image: photo], [image: filename.jpg] — but NOT path tags
// like [image:/path/to/file] (path tags have no space after the colon).
var (
	imagePlaceholderRegex = regexp.MustCompile(`\[image(:\s+[^\]]*)?\]`)
	audioPlaceholderRegex = regexp.MustCompile(`\[audio(:\s+[^\]]*)?\]`)
	videoPlaceholderRegex = regexp.MustCompile(`\[video(:\s+[^\]]*)?\]`)
	filePlaceholderRegex  = regexp.MustCompile(`\[file(:\s+[^\]]*)?\]`)
)

func normalizeCurrentTurnStart(messages []providers.Message, currentTurnStart int) int {
	if currentTurnStart < 0 {
		return 0
	}
	if currentTurnStart > len(messages) {
		return len(messages)
	}
	return currentTurnStart
}

func mediaAttachmentSystemPromptRule() string {
	return "**Media attachments** - Paths in [image:/path], [audio:/path], [video:/path], and [file:/path] tags are managed temporary files. Decide from the user's actual request whether to inspect, process, or preserve them. If the user asks to save, copy, move, or export an attachment, use an available tool to create the requested durable file and only report success after the tool succeeds. Do not treat the temporary attachment path itself as a completed save."
}

func currentTurnMessages(messages []providers.Message, currentTurnStart int) []providers.Message {
	currentTurnStart = normalizeCurrentTurnStart(messages, currentTurnStart)
	return messages[currentTurnStart:]
}

// resolveMediaRefs resolves media:// refs in messages.
// For user messages: images get path tags only ([image:/path]) so the LLM
// can decide whether to view them via load_image or operate on the file.
// For tool messages: images are base64-encoded and appended as a synthetic
// user message only after the contiguous tool-message block ends, so we don't
// break the tool-results-must-immediately-follow-assistant constraint that
// LLM APIs enforce.
// Only tool messages from the current turn may emit the synthetic user
// follow-up; historical tool results stay as plain path-tagged history.
// Non-image files always get path tags regardless of role.
// Returns a new slice; original messages are not mutated.
func resolveMediaRefs(
	messages []providers.Message,
	store media.MediaStore,
	maxSize int,
	currentTurnStart int,
	workspaceDir string,
) []providers.Message {
	currentTurnStart = normalizeCurrentTurnStart(messages, currentTurnStart)

	result := make([]providers.Message, 0, len(messages))
	var pendingToolImages []string

	for idx, m := range messages {
		// When leaving a tool-message block, flush any accumulated images
		// as a synthetic user message.
		if m.Role != "tool" && len(pendingToolImages) > 0 {
			result = append(result, toolImageFollowUpPromptMessage(pendingToolImages))
			pendingToolImages = nil
		}

		if len(m.Media) == 0 {
			result = append(result, m)
			if idx == len(messages)-1 && len(pendingToolImages) > 0 {
				result = append(result, toolImageFollowUpPromptMessage(pendingToolImages))
				pendingToolImages = nil
			}
			continue
		}

		msg := m
		resolved := make([]string, 0, len(m.Media))
		var pathTags []string

		for _, ref := range m.Media {
			// Convert current-turn inline data: URLs to temp files with path
			// tags, consistent with how channel media:// refs are handled.
			// This prevents sending image_url blocks to non-vision models
			// (e.g. deepseek-v4-flash) while still letting the model access
			// the file via load_image.
			if strings.HasPrefix(ref, "data:image/") && m.Role == "user" && idx >= currentTurnStart {
				localPath, mime, err := saveDataImageRef(ref, workspaceDir, maxSize)
				if err != nil {
					logger.WarnCF("agent", "Failed to save inline image for current turn", map[string]any{
						"error": err.Error(),
					})
					resolved = append(resolved, ref)
					continue
				}
				registerInlineImageForCleanup(store, localPath, mime)
				pathTags = append(pathTags, buildPathTag(mime, localPath))
				continue
			}

			if !strings.HasPrefix(ref, "media://") {
				// Drop inline data: URLs from historical messages. These are
				// base64-encoded payloads that were already processed in the
				// turn they were sent; replaying them on subsequent turns
				// causes errors with non-vision models (e.g. deepseek-v4-flash).
				if idx < currentTurnStart && strings.HasPrefix(ref, "data:") {
					continue
				}
				resolved = append(resolved, ref)
				continue
			}
			if store == nil {
				resolved = append(resolved, ref)
				continue
			}

			localPath, meta, err := store.ResolveWithMeta(ref)
			if err != nil {
				logger.WarnCF("agent", "Failed to resolve media ref", map[string]any{
					"ref":   ref,
					"error": err.Error(),
				})
				continue
			}

			info, err := os.Stat(localPath)
			if err != nil {
				logger.WarnCF("agent", "Failed to stat media file", map[string]any{
					"path":  localPath,
					"error": err.Error(),
				})
				continue
			}

			mime := detectMIME(localPath, meta)
			pathTags = append(pathTags, buildPathTag(mime, localPath))

			if m.Role == "tool" && idx >= currentTurnStart && strings.HasPrefix(mime, "image/") {
				dataURL := encodeImageToDataURL(localPath, mime, info, maxSize)
				if dataURL != "" {
					pendingToolImages = append(pendingToolImages, dataURL)
				}
			}
		}

		msg.Media = resolved
		if len(pathTags) > 0 {
			msg.Content = injectPathTags(msg.Content, pathTags)
		}
		result = append(result, msg)

		// If this is the last message and we have pending images, flush them.
		if idx == len(messages)-1 && len(pendingToolImages) > 0 {
			result = append(result, toolImageFollowUpPromptMessage(pendingToolImages))
			pendingToolImages = nil
		}
	}

	return result
}

func saveDataImageRef(ref, defaultDir string, maxSize int) (string, string, error) {
	comma := strings.IndexByte(ref, ',')
	if comma < 0 {
		return "", "", os.ErrInvalid
	}
	metadata := ref[:comma]
	payload := ref[comma+1:]
	if !strings.HasPrefix(metadata, "data:image/") || !strings.Contains(metadata, ";base64") {
		return "", "", os.ErrInvalid
	}
	if maxSize > 0 && base64.StdEncoding.DecodedLen(len(payload)) > maxSize {
		return "", "", os.ErrInvalid
	}

	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return "", "", err
	}
	if maxSize > 0 && len(data) > maxSize {
		return "", "", os.ErrInvalid
	}

	mime := strings.TrimPrefix(metadata, "data:")
	if semicolon := strings.IndexByte(mime, ';'); semicolon >= 0 {
		mime = mime[:semicolon]
	}
	if kind, err := filetype.Match(data); err == nil && kind != filetype.Unknown {
		mime = kind.MIME.Value
	}
	if !strings.HasPrefix(mime, "image/") {
		return "", "", os.ErrInvalid
	}

	var cleanTarget string
	if defaultDir != "" {
		cleanTarget, err = createAttachmentPathInDir(defaultDir, mime)
	} else {
		cleanTarget, err = createDefaultInlineAttachmentPath(mime)
	}
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(cleanTarget), 0o700); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(cleanTarget, data, 0o600); err != nil {
		return "", "", err
	}
	return cleanTarget, mime, nil
}

// registerInlineImageForCleanup hands ownership of a decoded inline image to
// the shared MediaStore. The returned media ref is intentionally not exposed:
// the current model turn consumes the local path, while the store keeps the
// file alive until its configured TTL expires and then removes it.
func registerInlineImageForCleanup(store media.MediaStore, localPath, mime string) {
	if store == nil || localPath == "" {
		return
	}

	_, err := store.Store(localPath, media.MediaMeta{
		Filename:      filepath.Base(localPath),
		ContentType:   mime,
		Source:        "agent:inline-image",
		CleanupPolicy: media.CleanupPolicyDeleteOnCleanup,
	}, "agent:inline-image:"+localPath)
	if err != nil {
		logger.WarnCF("agent", "Failed to register inline image for cleanup", map[string]any{
			"path":  localPath,
			"error": err.Error(),
		})
	}
}

func createAttachmentPathInDir(baseDir, mime string) (string, error) {
	dir := filepath.Join(baseDir, "tmp", "picoclaw-attachments")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	pattern := "attachment-*"
	switch mime {
	case "image/jpeg":
		pattern += ".jpg"
	case "image/webp":
		pattern += ".webp"
	case "image/gif":
		pattern += ".gif"
	case "image/bmp":
		pattern += ".bmp"
	default:
		pattern += ".png"
	}
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	path := f.Name()
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return path, nil
}

func createDefaultInlineAttachmentPath(mime string) (string, error) {
	dir := filepath.Join(os.TempDir(), "picoclaw-attachments")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	pattern := "attachment-*"
	switch mime {
	case "image/jpeg":
		pattern += ".jpg"
	case "image/webp":
		pattern += ".webp"
	case "image/gif":
		pattern += ".gif"
	case "image/bmp":
		pattern += ".bmp"
	default:
		pattern += ".png"
	}
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	path := f.Name()
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return path, nil
}

// encodeImageToDataURL base64-encodes an image file into a data URL.
// Returns empty string if the file exceeds maxSize or encoding fails.
func encodeImageToDataURL(localPath, mime string, info os.FileInfo, maxSize int) string {
	if info.Size() > int64(maxSize) {
		logger.WarnCF("agent", "Media file too large, skipping", map[string]any{
			"path":     localPath,
			"size":     info.Size(),
			"max_size": maxSize,
		})
		return ""
	}

	f, err := os.Open(localPath)
	if err != nil {
		logger.WarnCF("agent", "Failed to open media file", map[string]any{
			"path":  localPath,
			"error": err.Error(),
		})
		return ""
	}
	defer f.Close()

	prefix := "data:" + mime + ";base64,"
	encodedLen := base64.StdEncoding.EncodedLen(int(info.Size()))
	var buf bytes.Buffer
	buf.Grow(len(prefix) + encodedLen)
	buf.WriteString(prefix)

	encoder := base64.NewEncoder(base64.StdEncoding, &buf)
	_, copyErr := io.Copy(encoder, f)
	closeErr := encoder.Close()
	if copyErr != nil {
		logger.WarnCF("agent", "Failed to encode media file", map[string]any{
			"path":  localPath,
			"error": copyErr.Error(),
		})
		return ""
	}
	if closeErr != nil {
		logger.WarnCF("agent", "Failed to close base64 encoder", map[string]any{
			"path":  localPath,
			"error": closeErr.Error(),
		})
		return ""
	}

	return buf.String()
}

func buildArtifactTags(store media.MediaStore, refs []string) []string {
	if store == nil || len(refs) == 0 {
		return nil
	}

	tags := make([]string, 0, len(refs))
	for _, ref := range refs {
		localPath, meta, err := store.ResolveWithMeta(ref)
		if err != nil {
			continue
		}
		mime := detectMIME(localPath, meta)
		tags = append(tags, buildPathTag(mime, localPath))
	}

	return tags
}

func buildProviderAttachments(store media.MediaStore, refs []string) []providers.Attachment {
	if store == nil || len(refs) == 0 {
		return nil
	}

	attachments := make([]providers.Attachment, 0, len(refs))
	for _, ref := range refs {
		attachment := providers.Attachment{Ref: ref}
		if _, meta, err := store.ResolveWithMeta(ref); err == nil {
			attachment.Filename = meta.Filename
			attachment.ContentType = meta.ContentType
			attachment.Type = inferMediaType(meta.Filename, meta.ContentType)
		}
		attachments = append(attachments, attachment)
	}

	return attachments
}

// detectMIME determines the MIME type from metadata or magic-bytes detection.
// Returns empty string if detection fails.
func detectMIME(localPath string, meta media.MediaMeta) string {
	if meta.ContentType != "" {
		return meta.ContentType
	}
	kind, err := filetype.MatchFile(localPath)
	if err != nil || kind == filetype.Unknown {
		return ""
	}
	return kind.MIME.Value
}

// buildPathTag creates a structured tag exposing the local file path.
// Tag type is derived from MIME: [image:/path], [audio:/path], [video:/path], or [file:/path].
func buildPathTag(mime, localPath string) string {
	switch {
	case strings.HasPrefix(mime, "image/"):
		return "[image:" + localPath + "]"
	case strings.HasPrefix(mime, "audio/"):
		return "[audio:" + localPath + "]"
	case strings.HasPrefix(mime, "video/"):
		return "[video:" + localPath + "]"
	default:
		return "[file:" + localPath + "]"
	}
}

// injectPathTags replaces generic media tags in content with path-bearing versions,
// or appends if no matching generic tag is found. Channels emit a few different
// placeholder formats — [image], [image: photo], [image: filename.jpg] — so we
// match all of them via regex while leaving path tags ([image:/path]) untouched.
//
// When content is structured data (e.g., JSON from Feishu interactive cards or
// post messages), tags are only injected via placeholder replacement — never
// appended — to avoid corrupting the payload.
func injectPathTags(content string, tags []string) string {
	isStructured := looksLikeJSON(content)
	for _, tag := range tags {
		var pattern *regexp.Regexp
		switch {
		case strings.HasPrefix(tag, "[image:"):
			pattern = imagePlaceholderRegex
		case strings.HasPrefix(tag, "[audio:"):
			pattern = audioPlaceholderRegex
		case strings.HasPrefix(tag, "[video:"):
			pattern = videoPlaceholderRegex
		case strings.HasPrefix(tag, "[file:"):
			pattern = filePlaceholderRegex
		}

		if pattern != nil {
			if loc := pattern.FindStringIndex(content); loc != nil {
				content = content[:loc[0]] + tag + content[loc[1]:]
				continue
			}
		}

		if isStructured {
			content = tag + "\n" + content
			continue
		}

		if content == "" {
			content = tag
		} else {
			content += " " + tag
		}
	}
	return content
}

func looksLikeJSON(s string) bool {
	s = strings.TrimSpace(s)
	return len(s) > 1 && s[0] == '{'
}
