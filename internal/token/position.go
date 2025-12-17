package token

import "fmt"

// Position represents a position in source code.
type Position struct {
	// Filename is the name of the source file (optional).
	Filename string
	// Line number (1-indexed).
	Line int
	// Column is the byte offset on the line (1-indexed).
	Column int
	// Offset is the byte offset from the start of source (0-indexed).
	Offset int
}

// String returns a string representation of the position.
// Format: "filename:line:column" or "line:column" if filename is empty.
func (p Position) String() string {
	if p.Filename != "" {
		return fmt.Sprintf("%s:%d:%d", p.Filename, p.Line, p.Column)
	}
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// IsValid returns true if the position is valid (line > 0).
func (p Position) IsValid() bool {
	return p.Line > 0
}

// Before returns true if p is before other in the source.
func (p Position) Before(other Position) bool {
	if p.Line != other.Line {
		return p.Line < other.Line
	}
	return p.Column < other.Column
}

// After returns true if p is after other in the source.
func (p Position) After(other Position) bool {
	if p.Line != other.Line {
		return p.Line > other.Line
	}
	return p.Column > other.Column
}

// Span represents a range in source code from Start to End.
type Span struct {
	Start Position
	End   Position
}

// String returns a string representation of the span.
func (s Span) String() string {
	if s.Start.Line == s.End.Line {
		return fmt.Sprintf("%s-%d", s.Start.String(), s.End.Column)
	}
	return fmt.Sprintf("%s-%s", s.Start.String(), s.End.String())
}

// Contains returns true if the span contains the given position.
func (s Span) Contains(p Position) bool {
	return !p.Before(s.Start) && !p.After(s.End)
}

// NoPos is a zero Position used when position is unknown.
var NoPos = Position{}
