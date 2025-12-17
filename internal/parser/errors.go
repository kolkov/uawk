// Package parser provides an AWK recursive descent parser.
package parser

import (
	"fmt"

	"github.com/kolkov/uawk/internal/token"
)

// ParseError represents a syntax error encountered during parsing.
// It implements the error interface and includes source position information.
type ParseError struct {
	Pos     token.Position // Position where the error occurred
	Message string         // Human-readable error message
	Got     string         // Token/value that was found (optional)
	Want    string         // Token/value that was expected (optional)
}

// Error returns a formatted error message with position information.
func (e *ParseError) Error() string {
	if e.Pos.IsValid() {
		return fmt.Sprintf("%s: %s", e.Pos, e.Message)
	}
	return e.Message
}

// Unwrap returns nil as ParseError doesn't wrap other errors.
func (e *ParseError) Unwrap() error {
	return nil
}

// ErrorList is a list of parse errors.
type ErrorList []*ParseError

// Error returns a combined error message for all errors.
func (el ErrorList) Error() string {
	switch len(el) {
	case 0:
		return "no errors"
	case 1:
		return el[0].Error()
	default:
		return fmt.Sprintf("%s (and %d more errors)", el[0].Error(), len(el)-1)
	}
}

// Add appends an error to the list.
func (el *ErrorList) Add(pos token.Position, msg string) {
	*el = append(*el, &ParseError{Pos: pos, Message: msg})
}

// Err returns an error if there are any errors, nil otherwise.
func (el ErrorList) Err() error {
	if len(el) == 0 {
		return nil
	}
	return el
}

// errorf creates a ParseError at the given position with formatted message.
func errorf(pos token.Position, format string, args ...any) *ParseError {
	return &ParseError{
		Pos:     pos,
		Message: fmt.Sprintf(format, args...),
	}
}

// expectedError creates a ParseError for unexpected token.
func expectedError(pos token.Position, want string, got string) *ParseError {
	return &ParseError{
		Pos:     pos,
		Message: fmt.Sprintf("expected %s, got %s", want, got),
		Want:    want,
		Got:     got,
	}
}
