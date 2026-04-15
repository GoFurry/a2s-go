package a2s

import "fmt"

// ErrorCode classifies SDK failures.
type ErrorCode string

const (
	ErrorCodeAddress      ErrorCode = "address"
	ErrorCodeDial         ErrorCode = "dial"
	ErrorCodeWrite        ErrorCode = "write"
	ErrorCodeRead         ErrorCode = "read"
	ErrorCodeTimeout      ErrorCode = "timeout"
	ErrorCodePacketHeader ErrorCode = "packet_header"
	ErrorCodeChallenge    ErrorCode = "challenge"
	ErrorCodeMultiPacket  ErrorCode = "multi_packet"
	ErrorCodeDecode       ErrorCode = "decode"
	ErrorCodeUnsupported  ErrorCode = "unsupported"
)

// Error is the exported SDK error model.
type Error struct {
	Code    ErrorCode
	Op      string
	Addr    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	base := fmt.Sprintf("%s: %s", e.Code, e.Op)
	if e.Addr != "" {
		base = fmt.Sprintf("%s (%s)", base, e.Addr)
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

func newError(code ErrorCode, op string, addr string, message string, err error) *Error {
	return &Error{
		Code:    code,
		Op:      op,
		Addr:    addr,
		Message: message,
		Err:     err,
	}
}
