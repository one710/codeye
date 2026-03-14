package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type msg struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	Params  interface{} `json:"params,omitempty"`
}

func main() {
	caps := parseCapabilitiesEnv(os.Getenv("MOCK_CAPS"))
	requireAuth := strings.EqualFold(strings.TrimSpace(os.Getenv("MOCK_REQUIRE_AUTH")), "1") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("MOCK_REQUIRE_AUTH")), "true")
	authMethod := strings.TrimSpace(os.Getenv("MOCK_AUTH_METHOD"))
	if authMethod == "" {
		authMethod = "api_key"
	}
	expectedCredential := strings.TrimSpace(os.Getenv("MOCK_AUTH_CREDENTIAL"))
	if expectedCredential == "" {
		expectedCredential = "secret"
	}
	authed := !requireAuth
	toolDir := strings.TrimSpace(os.Getenv("MOCK_TOOL_DIR"))

	r := bufio.NewScanner(os.Stdin)
	r.Buffer(make([]byte, 1024*1024), 1024*1024)

	for r.Scan() {
		var m msg
		if err := json.Unmarshal(r.Bytes(), &m); err != nil {
			continue
		}
		if m.Method == "" && m.ID != nil {
			continue
		}
		if m.Method == "session/cancel" {
			continue
		}
		switch m.Method {
		case "initialize":
			resp := map[string]interface{}{
				"protocolVersion":   1,
				"agentCapabilities": caps,
			}
			if requireAuth {
				resp["authMethods"] = []map[string]string{{"id": authMethod}}
			}
			writeResp(m.ID, resp)
		case "authenticate":
			methodID := strField(m.Params, "methodId")
			if methodID != authMethod {
				writeErr(m.ID, -32602, "unsupported auth method")
				continue
			}
			if !hasExpectedCredential(authMethod, expectedCredential) {
				writeErr(m.ID, -32000, "invalid credentials")
				continue
			}
			authed = true
			writeResp(m.ID, map[string]interface{}{})
		case "session/new":
			if !authed {
				writeErr(m.ID, -32000, "authentication required")
				continue
			}
			writeResp(m.ID, map[string]interface{}{"sessionId": "mock-session"})
		case "session/list":
			if !authed {
				writeErr(m.ID, -32000, "authentication required")
				continue
			}
			writeResp(m.ID, map[string]interface{}{
				"sessions": []map[string]interface{}{{"sessionId": "mock-session"}},
			})
		case "session/load":
			if !authed {
				writeErr(m.ID, -32000, "authentication required")
				continue
			}
			writeNote("session/update", map[string]interface{}{"event": "replay_chunk", "text": "old1"})
			writeNote("session/update", map[string]interface{}{"event": "replay_chunk", "text": "old2"})
			writeResp(m.ID, map[string]interface{}{})
		case "session/prompt":
			if !authed {
				writeErr(m.ID, -32000, "authentication required")
				continue
			}
			promptText := extractPromptText(m.Params)
			if promptText == "test-tools" && toolDir != "" {
				exerciseTools(r, toolDir)
			}
			writeNote("session/update", map[string]interface{}{"event": "assistant_message_chunk", "text": "hello"})
			stopReason := "end_turn"
			if promptText == "no-stop-reason" {
				stopReason = ""
			}
			writeResp(m.ID, map[string]interface{}{"stopReason": stopReason})
		case "session/set_mode":
			if !authed {
				writeErr(m.ID, -32000, "authentication required")
				continue
			}
			writeResp(m.ID, map[string]interface{}{})
		case "session/set_config_option":
			if !authed {
				writeErr(m.ID, -32000, "authentication required")
				continue
			}
			writeResp(m.ID, map[string]interface{}{"configOptions": []string{}})
		default:
			writeErr(m.ID, -32601, "method not found")
		}
	}
}

func exerciseTools(scanner *bufio.Scanner, toolDir string) {
	readPath := toolDir + "/readable.txt"
	writePath := toolDir + "/written.txt"

	sendAndRead(scanner, "t-1", "fs/read_text_file", map[string]interface{}{"path": readPath})
	sendAndRead(scanner, "t-2", "fs/write_text_file", map[string]interface{}{"path": writePath, "content": "agent-wrote"})
	sendAndRead(scanner, "t-3", "session/request_permission", map[string]interface{}{
		"toolCall": map[string]interface{}{"method": "fs/read_text_file"},
	})
	resp := sendAndRead(scanner, "t-4", "terminal/create", map[string]interface{}{"command": "echo", "args": []interface{}{"tool-test"}})
	termID := floatField(resp, "id")
	sendAndRead(scanner, "t-5", "terminal/output", map[string]interface{}{"id": termID})
	sendAndRead(scanner, "t-6", "terminal/wait_for_exit", map[string]interface{}{"id": termID})
	sendAndRead(scanner, "t-7", "terminal/release", map[string]interface{}{"id": termID})

	resp2 := sendAndRead(scanner, "t-8", "terminal/create", map[string]interface{}{"command": "sleep", "args": []interface{}{"60"}})
	termID2 := floatField(resp2, "id")
	sendAndRead(scanner, "t-9", "terminal/kill", map[string]interface{}{"id": termID2})
	sendAndRead(scanner, "t-10", "terminal/wait_for_exit", map[string]interface{}{"id": termID2})

	sendAndRead(scanner, "t-99", "unknown/method", nil)
}

func sendAndRead(scanner *bufio.Scanner, id, method string, params interface{}) map[string]interface{} {
	b, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	fmt.Println(string(b))
	if !scanner.Scan() {
		return nil
	}
	var resp msg
	json.Unmarshal(scanner.Bytes(), &resp)
	if m, ok := resp.Result.(map[string]interface{}); ok {
		return m
	}
	return nil
}

func extractPromptText(params interface{}) string {
	m, ok := params.(map[string]interface{})
	if !ok {
		return ""
	}
	parts, ok := m["prompt"].([]interface{})
	if !ok || len(parts) == 0 {
		return ""
	}
	part, ok := parts[0].(map[string]interface{})
	if !ok {
		return ""
	}
	text, _ := part["text"].(string)
	return text
}

func strField(params interface{}, key string) string {
	m, ok := params.(map[string]interface{})
	if !ok {
		return ""
	}
	v, _ := m[key].(string)
	return strings.TrimSpace(v)
}

func floatField(m map[string]interface{}, key string) float64 {
	if m == nil {
		return 0
	}
	v, _ := m[key].(float64)
	return v
}

func hasExpectedCredential(methodID, expected string) bool {
	if expected == "" {
		return true
	}
	if strings.TrimSpace(os.Getenv(methodID)) == expected {
		return true
	}
	token := normalizeMethodToken(methodID)
	if token != "" {
		if strings.TrimSpace(os.Getenv(token)) == expected {
			return true
		}
		if strings.TrimSpace(os.Getenv("CODEYE_AUTH_"+token)) == expected {
			return true
		}
	}
	return false
}

func normalizeMethodToken(methodID string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(methodID)) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

func parseCapabilitiesEnv(raw string) map[string]interface{} {
	enabled := map[string]bool{
		"listSessions": true, "loadSession": true,
		"setMode": true, "setConfigOption": true,
	}
	if strings.TrimSpace(raw) == "" {
		return toAnyMap(enabled)
	}
	enabled = map[string]bool{}
	for _, token := range strings.Split(raw, ",") {
		switch strings.TrimSpace(token) {
		case "list":
			enabled["listSessions"] = true
		case "load":
			enabled["loadSession"] = true
		case "set-mode":
			enabled["setMode"] = true
		case "set-config":
			enabled["setConfigOption"] = true
		}
	}
	return toAnyMap(enabled)
}

func toAnyMap(v map[string]bool) map[string]interface{} {
	out := map[string]interface{}{}
	for k, val := range v {
		out[k] = val
	}
	return out
}

func writeResp(id interface{}, result interface{}) {
	b, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": id, "result": result})
	fmt.Println(string(b))
}

func writeErr(id interface{}, code int, message string) {
	b, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "id": id,
		"error": map[string]interface{}{"code": code, "message": message},
	})
	fmt.Println(string(b))
}

func writeNote(method string, params interface{}) {
	b, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "method": method, "params": params})
	fmt.Println(string(b))
}
