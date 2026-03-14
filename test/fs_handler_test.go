package codeye_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	fstool "github.com/one710/codeye/internal/tools/fs"
)

func TestNormalizeRejectsSiblingPrefixEscape(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	h := fstool.New(root)
	bad := root + "2" + string(filepath.Separator) + "file.txt"
	_, err := h.ReadTextFile(bad)
	if err == nil {
		t.Fatalf("expected sibling-prefix path to be rejected")
	}
}

func TestReadWriteWithinRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	h := fstool.New(root)
	path := filepath.Join(root, "a.txt")
	if err := h.WriteTextFile(path, "hello"); err != nil {
		t.Fatalf("WriteTextFile: %v", err)
	}
	got, err := h.ReadTextFile(path)
	if err != nil {
		t.Fatalf("ReadTextFile: %v", err)
	}
	if got != "hello" {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestRejectRelativePath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	os.MkdirAll(root, 0o755)
	h := fstool.New(root)
	path := filepath.Join(root, "relative", "path.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := h.ReadTextFile("relative/path.txt")
	if err != nil {
		t.Fatalf("expected relative path to resolve in root, got %v", err)
	}
	if got != "ok" {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestRejectTraversalPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	os.MkdirAll(root, 0o755)
	h := fstool.New(root)
	bad := filepath.Join(root, "..", "outside.txt")
	_, err := h.ReadTextFile(bad)
	if err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("expected outside workspace error, got %v", err)
	}
}

func TestWriteRejectTraversal(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	os.MkdirAll(root, 0o755)
	h := fstool.New(root)
	bad := filepath.Join(root, "..", "escape.txt")
	err := h.WriteTextFile(bad, "pwned")
	if err == nil {
		t.Fatal("expected error for traversal write")
	}
}

func TestWriteRejectRelativePath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	os.MkdirAll(root, 0o755)
	h := fstool.New(root)
	err := h.WriteTextFile("relative.txt", "data")
	if err != nil {
		t.Fatalf("expected relative write to succeed, got %v", err)
	}
	b, readErr := os.ReadFile(filepath.Join(root, "relative.txt"))
	if readErr != nil {
		t.Fatalf("expected file written in root: %v", readErr)
	}
	if string(b) != "data" {
		t.Fatalf("unexpected file content: %q", string(b))
	}
}

func TestReadNonexistentFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	os.MkdirAll(root, 0o755)
	h := fstool.New(root)
	_, err := h.ReadTextFile(filepath.Join(root, "nofile.txt"))
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
