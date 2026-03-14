package acp

import "encoding/json"

type ID = any

type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      ID          `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      ID          `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

func NewRequest(id ID, method string, params interface{}) Request {
	return Request{JSONRPC: "2.0", ID: id, Method: method, Params: params}
}

func NewNotification(method string, params interface{}) Request {
	return Request{JSONRPC: "2.0", Method: method, Params: params}
}

func DecodeMessage(line []byte) (Request, Response, bool, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(line, &probe); err != nil {
		return Request{}, Response{}, false, err
	}
	_, hasMethod := probe["method"]
	if hasMethod {
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			return Request{}, Response{}, false, err
		}
		return req, Response{}, true, nil
	}
	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return Request{}, Response{}, false, err
	}
	return Request{}, resp, false, nil
}
