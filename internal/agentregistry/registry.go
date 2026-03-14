package agentregistry

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/one710/codeye/internal/config"
)

type Adapter struct {
	Name            string
	Command         string
	Args            []string
	NewSessionTO    int
	InitializeTO    int
	ConfigKeyCompat map[string]string
}

func Resolve(agentName string, commandOverride string, configured map[string]config.AgentConfig) (Adapter, error) {
	if commandOverride != "" {
		parts := strings.Fields(commandOverride)
		if len(parts) == 0 {
			return Adapter{}, fmt.Errorf("empty --agent command")
		}
		return Adapter{Name: "custom", Command: parts[0], Args: parts[1:]}, nil
	}

	ac := configured[agentName]
	if ac.Command == "" {
		return Adapter{}, fmt.Errorf("unsupported agent: %s", agentName)
	}
	args := ac.Args
	if args == nil {
		args = []string{}
	}
	initTO, newTO := 20, 60
	switch agentName {
	case "cursor":
		newTO = 60
	default:
		newTO = 60 // custom agents from config
	}
	if ac.InitializeTimeout > 0 {
		initTO = ac.InitializeTimeout
	}
	if ac.NewSessionTimeout > 0 {
		newTO = ac.NewSessionTimeout
	}
	return Adapter{Name: agentName, Command: ac.Command, Args: args, InitializeTO: initTO, NewSessionTO: newTO}, nil
}

func ValidateInstalled(a Adapter) error {
	_, err := exec.LookPath(a.Command)
	if err != nil {
		return fmt.Errorf("%s not found in PATH: %w", a.Command, err)
	}
	return nil
}
