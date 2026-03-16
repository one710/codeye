package cli

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/one710/codeye/internal/acp"
)

// BuildPromptParts returns ACP prompt parts: one text block (if text is non-empty), then image blocks, then audio blocks.
// Image and audio files are read and base64-encoded; mime types are inferred from extension.
func BuildPromptParts(text string, imagePaths, audioPaths []string) ([]acp.PromptPart, error) {
	var parts []acp.PromptPart
	text = strings.TrimSpace(text)
	if text != "" {
		parts = append(parts, acp.PromptPart{Type: "text", Text: text})
	}
	for _, p := range imagePaths {
		mime, data, err := readFileAsBase64(p, imageMimeTypes)
		if err != nil {
			return nil, fmt.Errorf("image %s: %w", p, err)
		}
		parts = append(parts, acp.PromptPart{Type: "image", MimeType: mime, Data: data})
	}
	for _, p := range audioPaths {
		mime, data, err := readFileAsBase64(p, audioMimeTypes)
		if err != nil {
			return nil, fmt.Errorf("audio %s: %w", p, err)
		}
		parts = append(parts, acp.PromptPart{Type: "audio", MimeType: mime, Data: data})
	}
	if len(parts) == 0 {
		parts = []acp.PromptPart{{Type: "text", Text: ""}}
	}
	return parts, nil
}

func readFileAsBase64(path string, mimeMap map[string]string) (mimeType, b64 string, err error) {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	mimeType = mimeMap[ext]
	if mimeType == "" {
		return "", "", fmt.Errorf("unsupported extension .%s", ext)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	return mimeType, base64.StdEncoding.EncodeToString(raw), nil
}

var imageMimeTypes = map[string]string{
	"png":  "image/png",
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"gif":  "image/gif",
	"webp": "image/webp",
}

var audioMimeTypes = map[string]string{
	"wav":  "audio/wav",
	"mp3":  "audio/mpeg",
	"mpeg": "audio/mpeg",
	"ogg":  "audio/ogg",
	"flac": "audio/flac",
	"m4a":  "audio/mp4",
}
