// Package semantic provides semantic analysis for AWK programs.
//
// The semantic analyzer performs:
//   - Name resolution: binding identifiers to their declarations
//   - Scope analysis: tracking variable and function scopes
//   - Type inference: determining scalar vs array types
//   - Semantic validation: checking for errors like break outside loop
//
// AWK has unique semantics:
//   - Variables are automatically created on first use (global by default)
//   - Function parameters create local scope
//   - Special variables (NR, NF, etc.) are pre-defined
//   - Arrays and scalars are distinguished by usage context
package semantic

import (
	"fmt"
	"strings"

	"github.com/kolkov/uawk/internal/token"
)

// Error represents a semantic analysis error with source location.
type Error struct {
	Pos     token.Position
	Message string
}

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Pos, e.Message)
}

// Warning represents a semantic warning (non-fatal issue).
type Warning struct {
	Pos     token.Position
	Message string
}

// String returns the warning as a formatted string.
func (w *Warning) String() string {
	return fmt.Sprintf("%s: warning: %s", w.Pos, w.Message)
}

// ErrorList is a collection of semantic errors.
type ErrorList []*Error

// Add appends an error to the list.
func (el *ErrorList) Add(pos token.Position, format string, args ...any) {
	*el = append(*el, &Error{
		Pos:     pos,
		Message: fmt.Sprintf(format, args...),
	})
}

// Err returns an error if the list is non-empty, nil otherwise.
func (el ErrorList) Err() error {
	if len(el) == 0 {
		return nil
	}
	return el
}

// Error implements the error interface for ErrorList.
func (el ErrorList) Error() string {
	switch len(el) {
	case 0:
		return "no errors"
	case 1:
		return el[0].Error()
	default:
		var sb strings.Builder
		sb.WriteString(el[0].Error())
		for _, e := range el[1:] {
			sb.WriteByte('\n')
			sb.WriteString(e.Error())
		}
		return sb.String()
	}
}

// WarningList is a collection of semantic warnings.
type WarningList []*Warning

// Add appends a warning to the list.
func (wl *WarningList) Add(pos token.Position, format string, args ...any) {
	*wl = append(*wl, &Warning{
		Pos:     pos,
		Message: fmt.Sprintf(format, args...),
	})
}

// errorf creates a new semantic error.
func errorf(pos token.Position, format string, args ...any) *Error {
	return &Error{
		Pos:     pos,
		Message: fmt.Sprintf(format, args...),
	}
}

// warnf creates a new semantic warning.
func warnf(pos token.Position, format string, args ...any) *Warning {
	return &Warning{
		Pos:     pos,
		Message: fmt.Sprintf(format, args...),
	}
}

// Common error messages as constants for consistency.
const (
	errBreakOutsideLoop    = "break statement must be inside a loop"
	errContinueOutsideLoop = "continue statement must be inside a loop"
	errReturnOutsideFunc   = "return statement must be inside a function"
	errUndefinedFunc       = "undefined function %q"
	errDuplicateFunc       = "function %q already defined"
	errDuplicateParam      = "duplicate parameter %q in function %q"
	errParamShadowsFunc    = "parameter %q shadows function name"
	errTooManyArgs         = "too many arguments in call to %q"
	errNotEnoughArgs       = "not enough arguments in call to %q"
	errNotArray            = "cannot use %q as array (it is a scalar)"
	errNotScalar           = "cannot use %q as scalar (it is an array)"
	errDeleteNonArray      = "cannot delete from non-array %q"
	errAssignToNonLValue   = "cannot assign to non-lvalue"
	errNextInBeginEnd      = "next/nextfile cannot be used in BEGIN or END"
	errVarShadowsFunc      = "variable %q shadows function name"
	errArrayScalarConflict = "cannot use %q as both array and scalar"
)

// Common warning messages.
const (
	warnUnusedVar   = "variable %q is declared but never used"
	warnUnusedFunc  = "function %q is declared but never called"
	warnUnusedParam = "parameter %q is never used"
)
