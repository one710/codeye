package acp

type InitializeRequest struct {
	ProtocolVersion int                    `json:"protocolVersion"`
	ClientInfo      map[string]interface{} `json:"clientInfo"`
	ClientCaps      map[string]interface{} `json:"clientCapabilities"`
}

type AuthMethod struct {
	ID string `json:"id"`
}

type InitializeResult struct {
	ProtocolVersion   int          `json:"protocolVersion"`
	AgentCapabilities interface{}  `json:"agentCapabilities,omitempty"`
	AuthMethods       []AuthMethod `json:"authMethods,omitempty"`
}

type AuthenticateRequest struct {
	MethodID string `json:"methodId"`
}

type SessionNewRequest struct {
	Cwd        string        `json:"cwd"`
	MCPServers []interface{} `json:"mcpServers"`
}

type SessionNewResponse struct {
	SessionID string `json:"sessionId"`
}

type SessionLoadRequest struct {
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
}

type SessionListRequest struct {
	Cwd string `json:"cwd"`
}

type SessionListEntry struct {
	SessionID string `json:"sessionId"`
}

type SessionListResponse struct {
	Sessions []SessionListEntry `json:"sessions"`
}

// PromptPart is one content block in a session/prompt request.
// For text: Type="text", Text set. For image/audio: Type="image"|"audio", MimeType and Data (base64) set.
type PromptPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"` // base64-encoded for image/audio
}

// TextPrompt returns a single text prompt part (convenience for callers that have only text).
func TextPrompt(text string) []PromptPart {
	return []PromptPart{{Type: "text", Text: text}}
}

type SessionPromptRequest struct {
	SessionID string       `json:"sessionId"`
	Prompt    []PromptPart `json:"prompt"`
}

type SessionPromptResponse struct {
	StopReason string `json:"stopReason,omitempty"`
}

type SessionSetModeRequest struct {
	SessionID string `json:"sessionId"`
	ModeID    string `json:"modeId"`
}

type SessionSetConfigOptionRequest struct {
	SessionID string `json:"sessionId"`
	ConfigID  string `json:"configId"`
	Value     string `json:"value"`
}

type FSReadTextFileRequest struct {
	Path string `json:"path"`
}

type FSReadTextFileResponse struct {
	Content string `json:"content"`
}

type FSWriteTextFileRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type TerminalCreateRequest struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

type TerminalCreateResponse struct {
	ID int `json:"id"`
}

type TerminalOutputRequest struct {
	ID int `json:"id"`
}

type TerminalOutputResponse struct {
	Output string `json:"output"`
}

type TerminalWaitForExitResponse struct {
	ExitCode int    `json:"exitCode"`
	Signal   string `json:"signal,omitempty"`
	Error    string `json:"error,omitempty"`
}
