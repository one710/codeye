package codeye_test

import (
	"strings"
	"testing"

	"github.com/one710/codeye/internal/agentregistry"
	"github.com/one710/codeye/internal/config"
)

func agentConfigs(m map[string]string) map[string]config.AgentConfig {
	out := make(map[string]config.AgentConfig)
	for k, v := range m {
		out[k] = config.AgentConfig{Command: v}
	}
	return out
}

func TestResolveCursor(t *testing.T) {
	cmds := agentConfigs(map[string]string{"cursor": "cursor-agent-acp"})
	a, err := agentregistry.Resolve("cursor", "", cmds)
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "cursor" || a.Command != "cursor-agent-acp" {
		t.Fatalf("unexpected: %+v", a)
	}
	if a.InitializeTO != 20 || a.NewSessionTO != 60 {
		t.Fatalf("unexpected timeouts: init=%d new=%d", a.InitializeTO, a.NewSessionTO)
	}
}

func TestResolveCustomCommand(t *testing.T) {
	a, err := agentregistry.Resolve("anything", "my-agent --stdio --profile ci", map[string]config.AgentConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "custom" {
		t.Fatalf("expected name=custom, got %s", a.Name)
	}
	if a.Command != "my-agent" {
		t.Fatalf("expected command=my-agent, got %s", a.Command)
	}
	if len(a.Args) != 3 || a.Args[0] != "--stdio" {
		t.Fatalf("unexpected args: %v", a.Args)
	}
}

func TestResolveEmptyCustomCommand(t *testing.T) {
	_, err := agentregistry.Resolve("x", "  ", map[string]config.AgentConfig{})
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected error for empty --agent, got %v", err)
	}
}

func TestResolveUnknownAgent(t *testing.T) {
	_, err := agentregistry.Resolve("unknown", "", map[string]config.AgentConfig{})
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported agent error, got %v", err)
	}
}

func TestResolveCustomAgentFromConfig(t *testing.T) {
	cmds := agentConfigs(map[string]string{"myagent": "my-cmd"})
	a, err := agentregistry.Resolve("myagent", "", cmds)
	if err != nil {
		t.Fatalf("expected success for configured agent: %v", err)
	}
	if a.Name != "myagent" || a.Command != "my-cmd" {
		t.Fatalf("unexpected adapter: %+v", a)
	}
}

func TestValidateInstalledMissing(t *testing.T) {
	a := agentregistry.Adapter{Command: "nonexistent-binary-xyz-999"}
	err := agentregistry.ValidateInstalled(a)
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestValidateInstalledPresent(t *testing.T) {
	a := agentregistry.Adapter{Command: "go"}
	if err := agentregistry.ValidateInstalled(a); err != nil {
		t.Fatalf("go should be installed: %v", err)
	}
}
