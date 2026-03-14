package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type AgentConfig struct {
	Command           string   `json:"command"`
	Args              []string `json:"args,omitempty"`
	InitializeTimeout int      `json:"initializeTimeout,omitempty"` // seconds, 0 = use built-in default
	NewSessionTimeout int      `json:"newSessionTimeout,omitempty"` // seconds, 0 = use built-in default
}

type Config struct {
	DefaultAgent        string                 `json:"defaultAgent"`
	Format              string                 `json:"format"`
	PermissionMode      string                 `json:"permissionMode"`
	NonInteractivePerms string                 `json:"nonInteractivePermissions"`
	QueueTTLSeconds     int                    `json:"queueTTLSeconds"`
	QueueMaxDepth       int                    `json:"queueMaxDepth"`
	AuthPolicy          string                 `json:"authPolicy"`
	Auth                map[string]string      `json:"auth"`
	Agents              map[string]AgentConfig `json:"agents"`
}

func Default() Config {
	return Config{
		DefaultAgent:        "cursor",
		Format:              "text",
		PermissionMode:      "approve-reads",
		NonInteractivePerms: "deny",
		QueueTTLSeconds:     60,
		QueueMaxDepth:       32,
		AuthPolicy:          "skip",
		Auth:                map[string]string{},
		Agents: map[string]AgentConfig{
			"cursor": {Command: "agent", Args: []string{"acp"}},
		},
	}
}

func Load(cwd string) (Config, error) {
	cfg := Default()
	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, err
	}
	global := filepath.Join(home, ".codeye", "config.json")
	project := filepath.Join(cwd, ".codeye.json")
	if err := mergeFile(global, &cfg); err != nil {
		return cfg, err
	}
	if err := mergeFile(project, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func mergeFile(path string, cfg *Config) error {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var incoming Config
	if err := json.Unmarshal(b, &incoming); err != nil {
		return err
	}
	if incoming.DefaultAgent != "" {
		cfg.DefaultAgent = incoming.DefaultAgent
	}
	if incoming.Format != "" {
		cfg.Format = incoming.Format
	}
	if incoming.PermissionMode != "" {
		cfg.PermissionMode = incoming.PermissionMode
	}
	if incoming.NonInteractivePerms != "" {
		cfg.NonInteractivePerms = incoming.NonInteractivePerms
	}
	if incoming.QueueTTLSeconds > 0 {
		cfg.QueueTTLSeconds = incoming.QueueTTLSeconds
	}
	if incoming.QueueMaxDepth > 0 {
		cfg.QueueMaxDepth = incoming.QueueMaxDepth
	}
	if incoming.AuthPolicy != "" {
		cfg.AuthPolicy = incoming.AuthPolicy
	}
	if len(incoming.Auth) > 0 {
		if cfg.Auth == nil {
			cfg.Auth = map[string]string{}
		}
		for k, v := range incoming.Auth {
			cfg.Auth[k] = v
		}
	}
	if len(incoming.Agents) > 0 {
		for k, v := range incoming.Agents {
			cfg.Agents[k] = v
		}
	}
	return nil
}
