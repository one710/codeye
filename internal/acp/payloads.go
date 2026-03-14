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

type PromptTextPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type SessionPromptRequest struct {
	SessionID string           `json:"sessionId"`
	Prompt    []PromptTextPart `json:"prompt"`
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
