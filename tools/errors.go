package tools

import "fmt"

// Error represents an error that occurred during tool execution,
// optionally carrying an error code for the transport layer.
type Error struct {
	Code    int
	Message string
	Data    interface{}
	Cause   error // The underlying error, if any
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s (code: %d): %v", e.Message, e.Code, e.Cause)
	}
	return fmt.Sprintf("%s (code: %d)", e.Message, e.Code)
}

// Unwrap returns the underlying error
func (e *Error) Unwrap() error {
	return e.Cause
}

// NewError creates a new tool error
func NewError(code int, message string) *Error {
	return &Error{Code: code, Message: message}
}

// NewErrorWithCause creates a new tool error wrapping an underlying error
func NewErrorWithCause(code int, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

// NewInvalidParamsError creates a new error indicating invalid parameters.
// This corresponds to JSON-RPC error code -32602.
func NewInvalidParamsError(message string) *Error {
	return &Error{
		Code:    CodeInvalidParams,
		Message: message,
	}
}

// Common error codes that tools might want to use.
// These match standard JSON-RPC 2.0 error codes.
const (
	CodeInvalidParams = -32602
	CodeInternalError = -32603
)
