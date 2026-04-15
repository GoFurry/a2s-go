package scanner

import (
	"fmt"

	"github.com/GoFurry/a2s-go/master"
)

// ErrorCode classifies scanner/probe failures.
type ErrorCode string

const (
	ErrorCodeInput       ErrorCode = "input"
	ErrorCodeConcurrency ErrorCode = "concurrency"
	ErrorCodeTimeout     ErrorCode = "timeout"
	ErrorCodePacketSize  ErrorCode = "packet_size"
	ErrorCodeDiscovery   ErrorCode = "discovery"
	ErrorCodeProbe       ErrorCode = "probe"
)

var zeroServer master.ServerAddr

// Error is the exported scanner error model.
type Error struct {
	Code    ErrorCode
	Op      string
	Server  master.ServerAddr
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	base := fmt.Sprintf("%s: %s", e.Code, e.Op)
	if hasServer(e.Server) {
		base = fmt.Sprintf("%s (%s)", base, e.Server.String())
	}
	if e.Message != "" {
		base = fmt.Sprintf("%s: %s", base, e.Message)
	}
	return base
}

// Unwrap exposes the wrapped error.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func newError(code ErrorCode, op string, server master.ServerAddr, message string, err error) *Error {
	return &Error{
		Code:    code,
		Op:      op,
		Server:  cloneServer(server),
		Message: message,
		Err:     err,
	}
}

func hasServer(server master.ServerAddr) bool {
	return server.Port != 0 || server.IP != nil
}
