package persistence

type Record struct {
	Schema     string   `json:"schema"`
	RecordID   string   `json:"recordId"`
	ACPSession string   `json:"acpSessionId"`
	Agent      string   `json:"agent"`
	Cwd        string   `json:"cwd"`
	Name       string   `json:"name,omitempty"`
	CreatedAt  string   `json:"createdAt"`
	UpdatedAt  string   `json:"updatedAt"`
	LastPrompt string   `json:"lastPromptAt,omitempty"`
	Closed     bool     `json:"closed"`
	Messages   []string `json:"messages,omitempty"`
}

type Index struct {
	Schema  string            `json:"schema"`
	ByScope map[string]string `json:"byScope"`
}
