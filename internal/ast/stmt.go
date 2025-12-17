package ast

import "github.com/kolkov/uawk/internal/token"

// -----------------------------------------------------------------------------
// Basic statements
// -----------------------------------------------------------------------------

// ExprStmt represents an expression used as a statement.
// Examples: print_count++, func_call()
type ExprStmt struct {
	BaseStmt
	Expr Expr // Expression to evaluate
}

// PrintStmt represents a print or printf statement.
// Examples:
//   - print
//   - print $1, $2
//   - print "hello" > "file.txt"
//   - print "data" >> "log.txt"
//   - print "cmd" | "shell"
//   - printf "%d\n", count
type PrintStmt struct {
	BaseStmt
	Printf   bool        // true for printf, false for print
	Args     []Expr      // Arguments to print (may be empty for print)
	Redirect token.Token // Redirection operator (GREATER, APPEND, PIPE, or ILLEGAL if none)
	Dest     Expr        // Redirection destination (file or command)
}

// BlockStmt represents a block of statements.
// Example: { stmt1; stmt2; stmt3 }
type BlockStmt struct {
	BaseStmt
	Stmts []Stmt // Statements in the block (may be empty)
}

// -----------------------------------------------------------------------------
// Conditional statements
// -----------------------------------------------------------------------------

// IfStmt represents an if or if-else statement.
// Examples:
//   - if (cond) stmt
//   - if (cond) { stmts } else { stmts }
//   - if (cond) stmt else if (cond2) stmt2 else stmt3
type IfStmt struct {
	BaseStmt
	Cond Expr // Condition expression
	Then Stmt // Then branch (usually *BlockStmt)
	Else Stmt // Else branch (nil if no else, or another *IfStmt for else-if)
}

// -----------------------------------------------------------------------------
// Loop statements
// -----------------------------------------------------------------------------

// WhileStmt represents a while loop.
// Example: while (cond) { body }
type WhileStmt struct {
	BaseStmt
	Cond Expr // Loop condition
	Body Stmt // Loop body (usually *BlockStmt)
}

// DoWhileStmt represents a do-while loop.
// Example: do { body } while (cond)
type DoWhileStmt struct {
	BaseStmt
	Body Stmt // Loop body (usually *BlockStmt)
	Cond Expr // Loop condition (evaluated after each iteration)
}

// ForStmt represents a C-style for loop.
// Example: for (init; cond; post) { body }
type ForStmt struct {
	BaseStmt
	Init Stmt // Initialization statement (may be nil)
	Cond Expr // Condition expression (may be nil, means true)
	Post Stmt // Post-iteration statement (may be nil)
	Body Stmt // Loop body (usually *BlockStmt)
}

// ForInStmt represents a for-in loop over array keys.
// Example: for (key in array) { print key, array[key] }
type ForInStmt struct {
	BaseStmt
	Var   *Ident // Loop variable (receives each key)
	Array Expr   // Array to iterate over (usually *Ident)
	Body  Stmt   // Loop body (usually *BlockStmt)
}

// -----------------------------------------------------------------------------
// Control flow statements
// -----------------------------------------------------------------------------

// BreakStmt represents a break statement.
// Exits the innermost enclosing loop.
type BreakStmt struct {
	BaseStmt
}

// ContinueStmt represents a continue statement.
// Jumps to the next iteration of the innermost enclosing loop.
type ContinueStmt struct {
	BaseStmt
}

// NextStmt represents a next statement.
// Skips to the next input record, restarting pattern-action processing.
type NextStmt struct {
	BaseStmt
}

// NextFileStmt represents a nextfile statement.
// Skips to the next input file, restarting from the first record.
type NextFileStmt struct {
	BaseStmt
}

// ReturnStmt represents a return statement.
// Returns from the current function, optionally with a value.
// Example: return x + 1
type ReturnStmt struct {
	BaseStmt
	Value Expr // Return value (nil for bare return)
}

// ExitStmt represents an exit statement.
// Terminates AWK processing, optionally with an exit code.
// Example: exit 1
type ExitStmt struct {
	BaseStmt
	Code Expr // Exit code expression (nil defaults to 0)
}

// -----------------------------------------------------------------------------
// Other statements
// -----------------------------------------------------------------------------

// DeleteStmt represents a delete statement.
// Examples:
//   - delete arr[key]
//   - delete arr[i,j]
//   - delete arr (delete entire array - GNU extension)
type DeleteStmt struct {
	BaseStmt
	Array Expr   // Array expression (usually *Ident)
	Index []Expr // Key expression(s) (nil or empty to delete entire array)
}

// -----------------------------------------------------------------------------
// Compile-time checks
// -----------------------------------------------------------------------------

// Ensure all statement types implement Stmt interface.
var (
	_ Stmt = (*ExprStmt)(nil)
	_ Stmt = (*PrintStmt)(nil)
	_ Stmt = (*BlockStmt)(nil)
	_ Stmt = (*IfStmt)(nil)
	_ Stmt = (*WhileStmt)(nil)
	_ Stmt = (*DoWhileStmt)(nil)
	_ Stmt = (*ForStmt)(nil)
	_ Stmt = (*ForInStmt)(nil)
	_ Stmt = (*BreakStmt)(nil)
	_ Stmt = (*ContinueStmt)(nil)
	_ Stmt = (*NextStmt)(nil)
	_ Stmt = (*NextFileStmt)(nil)
	_ Stmt = (*ReturnStmt)(nil)
	_ Stmt = (*ExitStmt)(nil)
	_ Stmt = (*DeleteStmt)(nil)
)
