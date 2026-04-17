// Spawnbot - Personal AI assistant
// License: MIT
//
// Copyright (c) 2026 Spawnbot contributors

package common

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/h2non/filetype"
)

// mediaTagRe matches [image:/path], [audio:/path], and [video:/path] tags that
// the agent loop (pkg/agent/loop_media.go) injects into message content to
// reference local media files. Providers are responsible for rewriting these
// tags into the native multimodal format expected by the upstream API.
var mediaTagRe = regexp.MustCompile(`\[(image|audio|video):([^\]]+)\]`)

// imageMaxInlineBytes caps per-image payload size. OpenAI/Ollama image_url
// data URLs become unreliable past ~20MB and quickly bloat request bodies.
const imageMaxInlineBytes = 20 * 1024 * 1024

// ExtractImageTags scans content for [image:/path] tags, reads each file,
// base64-encodes it, and returns the cleaned content plus data-URL entries
// suitable for appending to a Message's Media slice.
//
// Non-image media tags (audio, video) are left in place: audio is normally
// handled upstream via transcription, and the openai_compat wire format has
// no native video support.
//
// If a referenced file cannot be read, an inline "[image unavailable: path
// — reason]" marker replaces the tag so the failure surfaces to the model
// instead of being silently dropped.
func ExtractImageTags(content string) (cleaned string, media []string) {
	if !strings.Contains(content, "[image:") {
		return content, nil
	}
	matches := mediaTagRe.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return content, nil
	}

	var sb strings.Builder
	lastEnd := 0
	for _, loc := range matches {
		sb.WriteString(content[lastEnd:loc[0]])
		lastEnd = loc[1]

		mediaType := content[loc[2]:loc[3]]
		mediaPath := content[loc[4]:loc[5]]

		if mediaType != "image" {
			sb.WriteString(content[loc[0]:loc[1]])
			continue
		}

		dataURL, err := encodeImageAsDataURL(mediaPath)
		if err != nil {
			fmt.Fprintf(&sb, "[image unavailable: %s — %v]", mediaPath, err)
			continue
		}
		media = append(media, dataURL)
	}
	sb.WriteString(content[lastEnd:])

	cleaned = strings.TrimSpace(sb.String())
	return cleaned, media
}

func encodeImageAsDataURL(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat failed: %w", err)
	}
	if info.Size() > imageMaxInlineBytes {
		return "", fmt.Errorf("file exceeds %d byte inline limit (got %d)", imageMaxInlineBytes, info.Size())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read failed: %w", err)
	}
	mime := detectImageMIME(path, data)
	if mime == "" {
		return "", fmt.Errorf("unrecognized image format")
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func detectImageMIME(path string, data []byte) string {
	if len(data) > 0 {
		if kind, err := filetype.Match(data); err == nil && kind != filetype.Unknown {
			if strings.HasPrefix(kind.MIME.Value, "image/") {
				return kind.MIME.Value
			}
		}
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	}
	return ""
}
