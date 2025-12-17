// Package ast defines the abstract syntax tree for AWK programs.
//
// The AST is designed for:
//   - High-performance traversal (no interface{} allocations in hot paths)
//   - Type inference support (Type field in nodes)
//   - Source location tracking for error reporting
//   - Generic visitor pattern (Go 1.18+ generics)
//
// Node hierarchy:
//
//	Node (interface)
//	├── Expr (interface) - expressions that produce values
//	│   ├── NumLit, StrLit, RegexLit - literals
//	│   ├── Ident, FieldExpr, IndexExpr - references
//	│   ├── BinaryExpr, UnaryExpr, TernaryExpr - operations
//	│   ├── CallExpr, BuiltinExpr, GetlineExpr - calls
//	│   └── InExpr, MatchExpr, ConcatExpr, AssignExpr - special
//	├── Stmt (interface) - statements that perform actions
//	│   ├── ExprStmt, PrintStmt, IfStmt - basic
//	│   ├── WhileStmt, DoWhileStmt, ForStmt, ForInStmt - loops
//	│   ├── BreakStmt, ContinueStmt, NextStmt, NextFileStmt - control
//	│   ├── ReturnStmt, ExitStmt, DeleteStmt - other
//	│   └── BlockStmt - compound
//	└── Program, Rule, FuncDecl - top-level structures
package ast

import "github.com/kolkov/uawk/internal/token"

// Node is the interface implemented by all AST nodes.
// It provides source position information for error reporting
// and the basis for the visitor pattern.
type Node interface {
	// Pos returns the position of the first character belonging to this node.
	Pos() token.Position

	// End returns the position of the first character immediately after this node.
	End() token.Position
}

// Expr is the interface for all expression nodes.
// Expressions are AST nodes that evaluate to a value.
type Expr interface {
	Node
	exprNode() // marker method to prevent external implementations
}

// Stmt is the interface for all statement nodes.
// Statements are AST nodes that perform actions.
type Stmt interface {
	Node
	stmtNode() // marker method to prevent external implementations
}

// Decl is the interface for declarations (functions).
type Decl interface {
	Node
	declNode() // marker method to prevent external implementations
}

// BaseExpr provides common fields for all expression nodes.
// Embedded in concrete expression types for position tracking.
type BaseExpr struct {
	StartPos token.Position // Position of first token
	EndPos   token.Position // Position after last token
}

func (b *BaseExpr) Pos() token.Position { return b.StartPos }
func (b *BaseExpr) End() token.Position { return b.EndPos }
func (b *BaseExpr) exprNode()           {}

// BaseStmt provides common fields for all statement nodes.
// Embedded in concrete statement types for position tracking.
type BaseStmt struct {
	StartPos token.Position // Position of first token
	EndPos   token.Position // Position after last token
}

func (b *BaseStmt) Pos() token.Position { return b.StartPos }
func (b *BaseStmt) End() token.Position { return b.EndPos }
func (b *BaseStmt) stmtNode()           {}

// BaseDecl provides common fields for declaration nodes.
type BaseDecl struct {
	StartPos token.Position
	EndPos   token.Position
}

func (b *BaseDecl) Pos() token.Position { return b.StartPos }
func (b *BaseDecl) End() token.Position { return b.EndPos }
func (b *BaseDecl) declNode()           {}

// IsLValue returns true if the expression can be used as an lvalue
// (left-hand side of assignment, target of ++/--, third arg to sub/gsub).
func IsLValue(e Expr) bool {
	switch e.(type) {
	case *Ident, *FieldExpr, *IndexExpr:
		return true
	default:
		return false
	}
}

// -----------------------------------------------------------------------------
// Constructor helpers
// -----------------------------------------------------------------------------

// MakeBaseExpr creates a BaseExpr with the given positions.
func MakeBaseExpr(start, end token.Position) BaseExpr {
	return BaseExpr{StartPos: start, EndPos: end}
}

// MakeBaseStmt creates a BaseStmt with the given positions.
func MakeBaseStmt(start, end token.Position) BaseStmt {
	return BaseStmt{StartPos: start, EndPos: end}
}

// MakeBaseDecl creates a BaseDecl with the given positions.
func MakeBaseDecl(start, end token.Position) BaseDecl {
	return BaseDecl{StartPos: start, EndPos: end}
}
