package ast

import "github.com/kolkov/uawk/internal/token"

// -----------------------------------------------------------------------------
// Literals
// -----------------------------------------------------------------------------

// NumLit represents a numeric literal (integer or float).
// Examples: 42, 3.14, 1e10, 0x1F
type NumLit struct {
	BaseExpr
	Value float64 // Parsed numeric value
	Raw   string  // Original source text (for exact representation)
}

// StrLit represents a string literal.
// Examples: "hello", "world\n"
type StrLit struct {
	BaseExpr
	Value string // Unescaped string value
}

// RegexLit represents a regex literal.
// Examples: /pattern/, /[a-z]+/i
type RegexLit struct {
	BaseExpr
	Pattern string // Regex pattern without delimiters
	Flags   string // Optional flags (e.g., "i" for case-insensitive)
}

// -----------------------------------------------------------------------------
// References
// -----------------------------------------------------------------------------

// Ident represents an identifier (variable name).
// Examples: x, NF, FILENAME
type Ident struct {
	BaseExpr
	Name string // Identifier name
}

// FieldExpr represents a field reference.
// Examples: $0, $1, $NF, $(i+1)
type FieldExpr struct {
	BaseExpr
	Index Expr // Field index expression (nil means $0)
}

// IndexExpr represents an array subscript expression.
// Examples: arr[key], arr[i,j], ARGV[0]
type IndexExpr struct {
	BaseExpr
	Array Expr   // Array expression (usually *Ident)
	Index []Expr // Subscript expressions (multiple for multi-dimensional)
}

// -----------------------------------------------------------------------------
// Operations
// -----------------------------------------------------------------------------

// BinaryExpr represents a binary operation.
// Examples: a + b, x == y, str ~ /re/
type BinaryExpr struct {
	BaseExpr
	Left  Expr        // Left operand
	Op    token.Token // Operator token
	Right Expr        // Right operand
}

// UnaryExpr represents a unary operation.
// Examples: -x, !flag, ++i, i++
type UnaryExpr struct {
	BaseExpr
	Op   token.Token // Operator token (SUB, NOT, INCR, DECR)
	Expr Expr        // Operand
	Post bool        // true for postfix (i++), false for prefix (++i)
}

// TernaryExpr represents a conditional expression.
// Example: cond ? true_val : false_val
type TernaryExpr struct {
	BaseExpr
	Cond Expr // Condition expression
	Then Expr // Value if condition is true
	Else Expr // Value if condition is false
}

// AssignExpr represents an assignment expression.
// Examples: x = 1, arr[k] = v, $1 = "new"
type AssignExpr struct {
	BaseExpr
	Left  Expr        // Target (must be lvalue: Ident, IndexExpr, or FieldExpr)
	Op    token.Token // Assignment operator (ASSIGN, ADD_ASSIGN, etc.)
	Right Expr        // Value expression
}

// ConcatExpr represents implicit string concatenation.
// Example: a b c (three adjacent expressions concatenate)
type ConcatExpr struct {
	BaseExpr
	Exprs []Expr // Expressions to concatenate (at least 2)
}

// GroupExpr represents a parenthesized expression.
// Used to preserve explicit grouping in the source.
// Example: (a + b)
type GroupExpr struct {
	BaseExpr
	Expr Expr // Inner expression
}

// -----------------------------------------------------------------------------
// Calls
// -----------------------------------------------------------------------------

// CallExpr represents a user-defined function call.
// Example: my_func(a, b, c)
type CallExpr struct {
	BaseExpr
	Name string // Function name
	Args []Expr // Arguments (may be empty)
}

// BuiltinExpr represents a built-in function call.
// Examples: length($0), substr(s, 1, 5), split(s, arr, ":")
type BuiltinExpr struct {
	BaseExpr
	Func token.Token // Built-in function token (F_LENGTH, F_SUBSTR, etc.)
	Args []Expr      // Arguments (may be empty for some like length())
}

// GetlineExpr represents a getline expression.
// Forms:
//   - getline              -> read next line into $0
//   - getline var          -> read next line into var
//   - getline < file       -> read from file into $0
//   - getline var < file   -> read from file into var
//   - cmd | getline        -> read from command into $0
//   - cmd | getline var    -> read from command into var
type GetlineExpr struct {
	BaseExpr
	Target  Expr // Variable to read into (nil means $0)
	File    Expr // File to read from (nil means stdin/current input)
	Command Expr // Command to pipe from (nil if not piped)
}

// -----------------------------------------------------------------------------
// Special expressions
// -----------------------------------------------------------------------------

// InExpr represents an array membership test.
// Examples: key in arr, (i,j) in arr
type InExpr struct {
	BaseExpr
	Index []Expr // Key expression(s)
	Array Expr   // Array expression (usually *Ident)
}

// MatchExpr represents a regex match expression.
// Examples: str ~ /re/, str !~ /pattern/
type MatchExpr struct {
	BaseExpr
	Expr    Expr        // String expression to match
	Op      token.Token // MATCH (~) or NOT_MATCH (!~)
	Pattern Expr        // Regex pattern (RegexLit or dynamic expression)
}

// CommaExpr represents a comma expression (rare in AWK).
// Used primarily in pattern ranges: /start/,/end/
type CommaExpr struct {
	BaseExpr
	Left  Expr
	Right Expr
}

// -----------------------------------------------------------------------------
// Compile-time checks
// -----------------------------------------------------------------------------

// Ensure all expression types implement Expr interface.
var (
	_ Expr = (*NumLit)(nil)
	_ Expr = (*StrLit)(nil)
	_ Expr = (*RegexLit)(nil)
	_ Expr = (*Ident)(nil)
	_ Expr = (*FieldExpr)(nil)
	_ Expr = (*IndexExpr)(nil)
	_ Expr = (*BinaryExpr)(nil)
	_ Expr = (*UnaryExpr)(nil)
	_ Expr = (*TernaryExpr)(nil)
	_ Expr = (*AssignExpr)(nil)
	_ Expr = (*ConcatExpr)(nil)
	_ Expr = (*GroupExpr)(nil)
	_ Expr = (*CallExpr)(nil)
	_ Expr = (*BuiltinExpr)(nil)
	_ Expr = (*GetlineExpr)(nil)
	_ Expr = (*InExpr)(nil)
	_ Expr = (*MatchExpr)(nil)
	_ Expr = (*CommaExpr)(nil)
)
