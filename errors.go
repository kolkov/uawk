package uawk

import (
	"fmt"
)

// ParseError represents a syntax error in AWK source code.
type ParseError struct {
	Line    int    // 1-based line number
	Column  int    // 1-based column number
	Message string // Error description
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error at %d:%d: %s", e.Line, e.Column, e.Message)
}

// CompileError represents a semantic error during compilation.
type CompileError struct {
	Message string // Error description
}

func (e *CompileError) Error() string {
	return fmt.Sprintf("compile error: %s", e.Message)
}

// RuntimeError represents an error during AWK execution.
type RuntimeError struct {
	Message string // Error description
}

func (e *RuntimeError) Error() string {
	return fmt.Sprintf("runtime error: %s", e.Message)
}

// ExitError represents a normal exit with a status code.
// This is not an error condition; it indicates the AWK program
// called exit with the given status.
type ExitError struct {
	Code int // Exit status code (0 = success)
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit %d", e.Code)
}

// IsExitError reports whether err is an ExitError and returns the exit code.
// Returns (code, true) if err is an ExitError, or (0, false) otherwise.
func IsExitError(err error) (int, bool) {
	if e, ok := err.(*ExitError); ok {
		return e.Code, true
	}
	return 0, false
}
