package codeye_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/one710/codeye/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.Default()
	if cfg.DefaultAgent != "cursor" {
		t.Fatalf("expected default agent cursor, got %s", cfg.DefaultAgent)
	}
	if cfg.Format != "text" {
		t.Fatalf("expected format text, got %s", cfg.Format)
	}
	if cfg.PermissionMode != "approve-reads" {
		t.Fatalf("expected permission mode approve-reads, got %s", cfg.PermissionMode)
	}
	if cfg.QueueTTLSeconds != 60 {
		t.Fatalf("expected TTL 60, got %d", cfg.QueueTTLSeconds)
	}
	if cfg.QueueMaxDepth != 32 {
		t.Fatalf("expected max depth 32, got %d", cfg.QueueMaxDepth)
	}
	if cfg.AuthPolicy != "skip" {
		t.Fatalf("expected auth policy skip, got %s", cfg.AuthPolicy)
	}
	if _, ok := cfg.Agents["cursor"]; !ok {
		t.Fatal("expected cursor agent config")
	}
}

func TestLoadWithNoFiles(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DefaultAgent != "cursor" {
		t.Fatalf("expected defaults, got agent=%s", cfg.DefaultAgent)
	}
}

func TestLoadMergesGlobalConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	globalDir := filepath.Join(home, ".codeye")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	global := config.Config{
		DefaultAgent:    "cursor",
		Format:          "json",
		PermissionMode:  "deny-all",
		QueueTTLSeconds: 120,
		QueueMaxDepth:   8,
		AuthPolicy:      "fail",
		Auth:            map[string]string{"api_key": "test123"},
		Agents:          map[string]config.AgentConfig{"custom": {Command: "my-agent"}},
	}
	b, _ := json.Marshal(global)
	if err := os.WriteFile(filepath.Join(globalDir, "config.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DefaultAgent != "cursor" {
		t.Fatalf("expected cursor (from global), got %s", cfg.DefaultAgent)
	}
	if cfg.Format != "json" {
		t.Fatalf("expected json, got %s", cfg.Format)
	}
	if cfg.PermissionMode != "deny-all" {
		t.Fatalf("expected deny-all, got %s", cfg.PermissionMode)
	}
	if cfg.QueueTTLSeconds != 120 {
		t.Fatalf("expected 120, got %d", cfg.QueueTTLSeconds)
	}
	if cfg.QueueMaxDepth != 8 {
		t.Fatalf("expected 8, got %d", cfg.QueueMaxDepth)
	}
	if cfg.AuthPolicy != "fail" {
		t.Fatalf("expected fail, got %s", cfg.AuthPolicy)
	}
	if cfg.Auth["api_key"] != "test123" {
		t.Fatalf("expected auth credential, got %v", cfg.Auth)
	}
	if cfg.Agents["custom"].Command != "my-agent" {
		t.Fatalf("expected custom agent, got %v", cfg.Agents)
	}
	if cfg.Agents["cursor"].Command != "agent" {
		t.Fatal("expected default cursor agent preserved after merge")
	}
}

func TestLoadMergesProjectConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := t.TempDir()
	project := config.Config{
		DefaultAgent:        "cursor",
		NonInteractivePerms: "fail",
	}
	b, _ := json.Marshal(project)
	if err := os.WriteFile(filepath.Join(cwd, ".codeye.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cwd)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DefaultAgent != "cursor" {
		t.Fatalf("expected cursor, got %s", cfg.DefaultAgent)
	}
	if cfg.NonInteractivePerms != "fail" {
		t.Fatalf("expected fail, got %s", cfg.NonInteractivePerms)
	}
}

func TestLoadProjectOverridesGlobal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	globalDir := filepath.Join(home, ".codeye")
	os.MkdirAll(globalDir, 0o755)
	globalCfg := config.Config{DefaultAgent: "other", Format: "json"}
	b, _ := json.Marshal(globalCfg)
	os.WriteFile(filepath.Join(globalDir, "config.json"), b, 0o644)

	cwd := t.TempDir()
	projCfg := config.Config{DefaultAgent: "cursor"}
	b, _ = json.Marshal(projCfg)
	os.WriteFile(filepath.Join(cwd, ".codeye.json"), b, 0o644)

	cfg, err := config.Load(cwd)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DefaultAgent != "cursor" {
		t.Fatalf("project should override global: got %s", cfg.DefaultAgent)
	}
	if cfg.Format != "json" {
		t.Fatal("global format should persist if not overridden by project")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := t.TempDir()
	os.WriteFile(filepath.Join(cwd, ".codeye.json"), []byte("{invalid}"), 0o644)
	_, err := config.Load(cwd)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
