package output

import (
	"encoding/json"
	"fmt"
	"io"

	cerr "github.com/one710/codeye/internal/errors"
)

type Format string

const (
	Text       Format = "text"
	JSON       Format = "json"
	JSONStrict Format = "json-strict"
	Quiet      Format = "quiet"
)

type Emitter struct {
	Format Format
	Out    io.Writer
	Err    io.Writer
}

// PrintSuccess emits success. For JSON/JSONStrict it writes action+payload; for Quiet, sessionId if present.
// For Text format, if textMsg is provided the first string is printed; otherwise the action is printed.
func (e Emitter) PrintSuccess(action string, payload map[string]interface{}, textMsg ...string) {
	switch e.Format {
	case JSON, JSONStrict:
		if payload == nil {
			payload = map[string]interface{}{}
		}
		payload["action"] = action
		b, _ := json.Marshal(payload)
		_, _ = e.Out.Write(append(b, '\n'))
	case Quiet:
		if id, ok := payload["sessionId"].(string); ok {
			fmt.Fprintln(e.Out, id)
		}
	default:
		if len(textMsg) > 0 {
			if textMsg[0] != "" {
				fmt.Fprintln(e.Out, textMsg[0])
			}
		} else {
			fmt.Fprintf(e.Out, "%s\n", action)
		}
	}
}

func (e Emitter) PrintError(message string) {
	e.PrintRPCError(-32603, message, nil)
}

func (e Emitter) PrintErrorWithCause(err error, fallback string) {
	if err == nil {
		e.PrintError(fallback)
		return
	}
	e.PrintRPCError(cerr.JSONRPCCode(err), fallback, map[string]interface{}{
		"errorCode": cerr.ErrorCode(err),
	})
}

func (e Emitter) PrintRPCError(code int, message string, data map[string]interface{}) {
	switch e.Format {
	case JSON, JSONStrict:
		errObj := map[string]interface{}{
			"code":    code,
			"message": message,
		}
		if len(data) > 0 {
			errObj["data"] = data
		}
		b, _ := json.Marshal(map[string]interface{}{
			"jsonrpc": "2.0",
			"error":   errObj,
		})
		_, _ = e.Out.Write(append(b, '\n'))
	default:
		fmt.Fprintln(e.Err, message)
	}
}
