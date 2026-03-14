package errors

import "fmt"

const (
	CodeUsage            = "USAGE"
	CodeRuntime          = "RUNTIME"
	CodePermissionDenied = "PERMISSION_DENIED"
	CodeTimeout          = "TIMEOUT"
	CodeAgentUnavailable = "AGENT_UNAVAILABLE"
)

type OutputError struct {
	Code    string
	Message string
	Err     error
}

func (e *OutputError) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func Wrap(code, msg string, err error) error {
	return &OutputError{Code: code, Message: msg, Err: err}
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var oe *OutputError
	if As(err, &oe) {
		switch oe.Code {
		case CodeUsage:
			return 2
		case CodePermissionDenied:
			return 4
		case CodeTimeout:
			return 124
		case CodeAgentUnavailable:
			return 10
		default:
			return 1
		}
	}
	return 1
}

func JSONRPCCode(err error) int {
	if err == nil {
		return -32603
	}
	var oe *OutputError
	if As(err, &oe) {
		switch oe.Code {
		case CodeUsage:
			return -32602
		case CodePermissionDenied:
			return -32000
		case CodeTimeout:
			return -32603
		case CodeAgentUnavailable:
			return -32603
		default:
			return -32603
		}
	}
	return -32603
}

func ErrorCode(err error) string {
	if err == nil {
		return CodeRuntime
	}
	var oe *OutputError
	if As(err, &oe) {
		return oe.Code
	}
	return CodeRuntime
}

// As bridges stdlib errors.As without importing package name collisions in callers.
func As(err error, target interface{}) bool {
	switch t := target.(type) {
	case **OutputError:
		current := err
		for current != nil {
			if cast, ok := current.(*OutputError); ok {
				*t = cast
				return true
			}
			type unwrapper interface{ Unwrap() error }
			u, ok := current.(unwrapper)
			if !ok {
				break
			}
			current = u.Unwrap()
		}
	}
	return false
}
