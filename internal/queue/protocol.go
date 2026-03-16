package queue

import "github.com/one710/codeye/internal/acp"

type Command string

const (
	CmdPrompt          Command = "prompt"
	CmdCancel          Command = "cancel"
	CmdSetMode         Command = "set_mode"
	CmdSetConfigOption Command = "set_config_option"
	CmdHealth          Command = "health"
)

type Request struct {
	RequestID   string           `json:"requestId,omitempty"`
	Command     Command          `json:"command"`
	SessionID   string           `json:"sessionId"`
	Prompt      string           `json:"prompt,omitempty"`      // legacy: single text part
	PromptParts []acp.PromptPart `json:"promptParts,omitempty"` // when set, used instead of Prompt
	Mode        string           `json:"mode,omitempty"`
	Key         string           `json:"key,omitempty"`
	Value       string           `json:"value,omitempty"`
}

type Response struct {
	RequestID            string `json:"requestId,omitempty"`
	OK                   bool   `json:"ok"`
	Message              string `json:"message,omitempty"`
	PID                  int    `json:"pid,omitempty"`
	StopReason           string `json:"stopReason,omitempty"`
	Text                 string `json:"text,omitempty"`
	Code                 string `json:"code,omitempty"`
	DetailCode           string `json:"detailCode,omitempty"`
	Origin               string `json:"origin,omitempty"`
	Retryable            bool   `json:"retryable,omitempty"`
	OutputAlreadyEmitted bool   `json:"outputAlreadyEmitted,omitempty"`
}

type PromptResult struct {
	StopReason string
	Text       string
}
