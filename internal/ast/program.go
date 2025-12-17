package ast

import "github.com/kolkov/uawk/internal/token"

// Program represents a complete AWK program.
// An AWK program consists of:
//   - Optional BEGIN blocks (executed before processing input)
//   - Pattern-action rules (executed for each input record)
//   - Optional END blocks (executed after all input is processed)
//   - User-defined functions
type Program struct {
	// Source file name (for error messages)
	Filename string

	// BEGIN blocks, executed in order before any input processing.
	Begin []*BlockStmt

	// Pattern-action rules, executed for each input record.
	// Rules are executed in order for each record.
	Rules []*Rule

	// EndBlocks are executed in order after all input is processed.
	// Named EndBlocks to avoid conflict with End() method.
	EndBlocks []*BlockStmt

	// User-defined function declarations.
	Functions []*FuncDecl

	// Position information for the entire program.
	StartPos token.Position
	EndPos   token.Position
}

// Pos returns the position of the first token in the program.
func (p *Program) Pos() token.Position { return p.StartPos }

// End returns the position after the last token in the program.
func (p *Program) End() token.Position { return p.EndPos }

// Rule represents a pattern-action rule.
// Examples:
//   - { print }                    -> Pattern is nil (matches all records)
//   - /regex/ { print }            -> Pattern is *RegexLit
//   - $1 > 100 { print $2 }        -> Pattern is *BinaryExpr
//   - NR == 1, NR == 10 { print }  -> Range pattern (*CommaExpr)
//   - /start/,/end/ { print }      -> Range pattern (*CommaExpr with RegexLit)
type Rule struct {
	// Pattern expression that determines if action is executed.
	// nil means the rule matches all records (always executes).
	// For range patterns, this is a *CommaExpr.
	Pattern Expr

	// Action to execute when pattern matches.
	// nil means default action: { print $0 }
	Action *BlockStmt

	// Position information
	StartPos token.Position
	EndPos   token.Position
}

// Pos returns the position of the first token in the rule.
func (r *Rule) Pos() token.Position { return r.StartPos }

// End returns the position after the last token in the rule.
func (r *Rule) End() token.Position { return r.EndPos }

// FuncDecl represents a user-defined function declaration.
// Example: function add(a, b) { return a + b }
//
// AWK functions have these characteristics:
//   - All parameters are passed by value (scalars) or reference (arrays)
//   - Local variables are declared by adding extra parameters
//   - Functions can access and modify global variables
type FuncDecl struct {
	BaseDecl

	// Function name
	Name string

	// Parameter names (includes local variables by AWK convention)
	// In AWK, local variables are declared as extra parameters:
	// function foo(a, b,    local1, local2)
	Params []string

	// Number of actual parameters (rest are local variables)
	// This is determined by the parser based on spacing conventions
	// or can be set to len(Params) if not distinguishable.
	NumParams int

	// Function body
	Body *BlockStmt

	// Name position for error messages
	NamePos token.Position
}

// LocalVars returns the names of local variables (parameters beyond NumParams).
func (f *FuncDecl) LocalVars() []string {
	if f.NumParams >= len(f.Params) {
		return nil
	}
	return f.Params[f.NumParams:]
}

// ActualParams returns the actual parameter names.
func (f *FuncDecl) ActualParams() []string {
	if f.NumParams > len(f.Params) {
		return f.Params
	}
	return f.Params[:f.NumParams]
}

// -----------------------------------------------------------------------------
// Compile-time checks
// -----------------------------------------------------------------------------

var (
	_ Node = (*Program)(nil)
	_ Node = (*Rule)(nil)
	_ Decl = (*FuncDecl)(nil)
)
