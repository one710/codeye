package fs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Handler struct {
	Root string
}

func New(root string) *Handler {
	return &Handler{Root: filepath.Clean(root)}
}

func (h *Handler) normalize(path string) (string, error) {
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		// Be adapter-tolerant: treat relative paths as cwd-scoped.
		clean = filepath.Join(h.Root, clean)
	}
	rel, err := filepath.Rel(h.Root, clean)
	if err != nil {
		return "", errors.New("path outside workspace")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("path outside workspace")
	}
	return clean, nil
}

func (h *Handler) ReadTextFile(path string) (string, error) {
	p, err := h.normalize(path)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (h *Handler) WriteTextFile(path, content string) error {
	p, err := h.normalize(path)
	if err != nil {
		return err
	}
	return os.WriteFile(p, []byte(content), 0o644)
}
