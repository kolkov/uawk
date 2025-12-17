package ast_test

import (
	"strings"
	"testing"

	"github.com/kolkov/uawk/internal/ast"
	"github.com/kolkov/uawk/internal/token"
)

// TestNodeInterface verifies all node types implement Node interface correctly.
func TestNodeInterface(t *testing.T) {
	pos := token.Position{Line: 1, Column: 1, Offset: 0}
	endPos := token.Position{Line: 1, Column: 10, Offset: 9}

	tests := []struct {
		name string
		node ast.Node
	}{
		// Literals
		{"NumLit", &ast.NumLit{}},
		{"StrLit", &ast.StrLit{}},
		{"RegexLit", &ast.RegexLit{}},

		// References
		{"Ident", &ast.Ident{Name: "x"}},
		{"FieldExpr", &ast.FieldExpr{}},
		{"IndexExpr", &ast.IndexExpr{}},

		// Operations
		{"BinaryExpr", &ast.BinaryExpr{}},
		{"UnaryExpr", &ast.UnaryExpr{}},
		{"TernaryExpr", &ast.TernaryExpr{}},
		{"AssignExpr", &ast.AssignExpr{}},
		{"ConcatExpr", &ast.ConcatExpr{}},
		{"GroupExpr", &ast.GroupExpr{}},

		// Calls
		{"CallExpr", &ast.CallExpr{}},
		{"BuiltinExpr", &ast.BuiltinExpr{}},
		{"GetlineExpr", &ast.GetlineExpr{}},

		// Special expressions
		{"InExpr", &ast.InExpr{}},
		{"MatchExpr", &ast.MatchExpr{}},
		{"CommaExpr", &ast.CommaExpr{}},

		// Statements
		{"ExprStmt", &ast.ExprStmt{}},
		{"PrintStmt", &ast.PrintStmt{}},
		{"BlockStmt", &ast.BlockStmt{}},
		{"IfStmt", &ast.IfStmt{}},
		{"WhileStmt", &ast.WhileStmt{}},
		{"DoWhileStmt", &ast.DoWhileStmt{}},
		{"ForStmt", &ast.ForStmt{}},
		{"ForInStmt", &ast.ForInStmt{}},
		{"BreakStmt", &ast.BreakStmt{}},
		{"ContinueStmt", &ast.ContinueStmt{}},
		{"NextStmt", &ast.NextStmt{}},
		{"NextFileStmt", &ast.NextFileStmt{}},
		{"ReturnStmt", &ast.ReturnStmt{}},
		{"ExitStmt", &ast.ExitStmt{}},
		{"DeleteStmt", &ast.DeleteStmt{}},

		// Program-level
		{"Program", &ast.Program{StartPos: pos, EndPos: endPos}},
		{"Rule", &ast.Rule{StartPos: pos, EndPos: endPos}},
		{"FuncDecl", &ast.FuncDecl{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify the node implements Node interface
			_ = tt.node.Pos()
			_ = tt.node.End()
		})
	}
}

// TestIsLValue verifies lvalue detection works correctly.
func TestIsLValue(t *testing.T) {
	tests := []struct {
		name   string
		expr   ast.Expr
		expect bool
	}{
		{"Ident", &ast.Ident{Name: "x"}, true},
		{"FieldExpr", &ast.FieldExpr{}, true},
		{"IndexExpr", &ast.IndexExpr{}, true},
		{"NumLit", &ast.NumLit{Value: 42}, false},
		{"StrLit", &ast.StrLit{Value: "hello"}, false},
		{"BinaryExpr", &ast.BinaryExpr{}, false},
		{"CallExpr", &ast.CallExpr{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ast.IsLValue(tt.expr)
			if got != tt.expect {
				t.Errorf("IsLValue(%s) = %v, want %v", tt.name, got, tt.expect)
			}
		})
	}
}

// TestWalk verifies AST walking works correctly.
func TestWalk(t *testing.T) {
	// Build a simple AST: x + y
	prog := &ast.Program{
		Rules: []*ast.Rule{
			{
				Action: &ast.BlockStmt{
					Stmts: []ast.Stmt{
						&ast.ExprStmt{
							Expr: &ast.BinaryExpr{
								Left:  &ast.Ident{Name: "x"},
								Op:    token.ADD,
								Right: &ast.Ident{Name: "y"},
							},
						},
					},
				},
			},
		},
	}

	// Count nodes by type
	var identCount, binaryCount, totalCount int

	ast.Walk(prog, func(n ast.Node) bool {
		totalCount++
		switch n.(type) {
		case *ast.Ident:
			identCount++
		case *ast.BinaryExpr:
			binaryCount++
		}
		return true
	})

	if identCount != 2 {
		t.Errorf("identCount = %d, want 2", identCount)
	}
	if binaryCount != 1 {
		t.Errorf("binaryCount = %d, want 1", binaryCount)
	}
	if totalCount < 5 {
		t.Errorf("totalCount = %d, expected at least 5", totalCount)
	}
}

// TestInspectWithParent verifies parent tracking in Inspect.
func TestInspectWithParent(t *testing.T) {
	// Build: $1
	fieldExpr := &ast.FieldExpr{
		Index: &ast.NumLit{Value: 1},
	}
	prog := &ast.Program{
		Rules: []*ast.Rule{
			{
				Action: &ast.BlockStmt{
					Stmts: []ast.Stmt{
						&ast.ExprStmt{Expr: fieldExpr},
					},
				},
			},
		},
	}

	var numLitParent ast.Node

	ast.Inspect(prog, func(n, parent ast.Node) bool {
		if _, ok := n.(*ast.NumLit); ok {
			numLitParent = parent
		}
		return true
	})

	if numLitParent != fieldExpr {
		t.Errorf("NumLit parent = %T, want *FieldExpr", numLitParent)
	}
}

// TestPrinter verifies AST pretty-printing.
func TestPrinter(t *testing.T) {
	tests := []struct {
		name   string
		node   ast.Node
		expect string
	}{
		{
			name:   "NumLit integer",
			node:   &ast.NumLit{Value: 42, Raw: "42"},
			expect: "42",
		},
		{
			name:   "NumLit float",
			node:   &ast.NumLit{Value: 3.14, Raw: "3.14"},
			expect: "3.14",
		},
		{
			name:   "StrLit",
			node:   &ast.StrLit{Value: "hello"},
			expect: `"hello"`,
		},
		{
			name:   "RegexLit",
			node:   &ast.RegexLit{Pattern: "[a-z]+"},
			expect: "/[a-z]+/",
		},
		{
			name:   "Ident",
			node:   &ast.Ident{Name: "NF"},
			expect: "NF",
		},
		{
			name:   "FieldExpr $0",
			node:   &ast.FieldExpr{Index: nil},
			expect: "$0",
		},
		{
			name: "FieldExpr $1",
			node: &ast.FieldExpr{
				Index: &ast.NumLit{Value: 1, Raw: "1"},
			},
			expect: "$1",
		},
		{
			name: "BinaryExpr add",
			node: &ast.BinaryExpr{
				Left:  &ast.Ident{Name: "x"},
				Op:    token.ADD,
				Right: &ast.Ident{Name: "y"},
			},
			expect: "x + y",
		},
		{
			name: "UnaryExpr prefix",
			node: &ast.UnaryExpr{
				Op:   token.NOT,
				Expr: &ast.Ident{Name: "flag"},
				Post: false,
			},
			expect: "!flag",
		},
		{
			name: "UnaryExpr postfix",
			node: &ast.UnaryExpr{
				Op:   token.INCR,
				Expr: &ast.Ident{Name: "i"},
				Post: true,
			},
			expect: "i++",
		},
		{
			name: "CallExpr user function",
			node: &ast.CallExpr{
				Name: "my_func",
				Args: []ast.Expr{
					&ast.Ident{Name: "a"},
					&ast.Ident{Name: "b"},
				},
			},
			expect: "my_func(a, b)",
		},
		{
			name: "InExpr",
			node: &ast.InExpr{
				Index: []ast.Expr{&ast.Ident{Name: "key"}},
				Array: &ast.Ident{Name: "arr"},
			},
			expect: "key in arr",
		},
		{
			name:   "BreakStmt",
			node:   &ast.BreakStmt{},
			expect: "break",
		},
		{
			name:   "ContinueStmt",
			node:   &ast.ContinueStmt{},
			expect: "continue",
		},
		{
			name:   "NextStmt",
			node:   &ast.NextStmt{},
			expect: "next",
		},
		{
			name:   "NextFileStmt",
			node:   &ast.NextFileStmt{},
			expect: "nextfile",
		},
		{
			name: "ReturnStmt with value",
			node: &ast.ReturnStmt{
				Value: &ast.Ident{Name: "result"},
			},
			expect: "return result",
		},
		{
			name:   "ReturnStmt bare",
			node:   &ast.ReturnStmt{},
			expect: "return",
		},
		{
			name: "ExitStmt with code",
			node: &ast.ExitStmt{
				Code: &ast.NumLit{Value: 1, Raw: "1"},
			},
			expect: "exit 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ast.String(tt.node)
			if strings.TrimSpace(got) != tt.expect {
				t.Errorf("String() = %q, want %q", got, tt.expect)
			}
		})
	}
}

// TestFuncDeclHelpers tests FuncDecl helper methods.
func TestFuncDeclHelpers(t *testing.T) {
	tests := []struct {
		name       string
		params     []string
		numParams  int
		wantActual []string
		wantLocal  []string
	}{
		{
			name:       "no params",
			params:     nil,
			numParams:  0,
			wantActual: nil,
			wantLocal:  nil,
		},
		{
			name:       "all params",
			params:     []string{"a", "b"},
			numParams:  2,
			wantActual: []string{"a", "b"},
			wantLocal:  nil,
		},
		{
			name:       "with locals",
			params:     []string{"a", "b", "local1", "local2"},
			numParams:  2,
			wantActual: []string{"a", "b"},
			wantLocal:  []string{"local1", "local2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &ast.FuncDecl{
				Name:      "test",
				Params:    tt.params,
				NumParams: tt.numParams,
			}

			actual := f.ActualParams()
			locals := f.LocalVars()

			if len(actual) != len(tt.wantActual) {
				t.Errorf("ActualParams() len = %d, want %d", len(actual), len(tt.wantActual))
			}
			for i := range actual {
				if actual[i] != tt.wantActual[i] {
					t.Errorf("ActualParams()[%d] = %q, want %q", i, actual[i], tt.wantActual[i])
				}
			}

			if len(locals) != len(tt.wantLocal) {
				t.Errorf("LocalVars() len = %d, want %d", len(locals), len(tt.wantLocal))
			}
			for i := range locals {
				if locals[i] != tt.wantLocal[i] {
					t.Errorf("LocalVars()[%d] = %q, want %q", i, locals[i], tt.wantLocal[i])
				}
			}
		})
	}
}

// TestProgramPrint tests full program printing.
func TestProgramPrint(t *testing.T) {
	prog := &ast.Program{
		Begin: []*ast.BlockStmt{
			{
				Stmts: []ast.Stmt{
					&ast.ExprStmt{
						Expr: &ast.AssignExpr{
							Left:  &ast.Ident{Name: "sum"},
							Op:    token.ASSIGN,
							Right: &ast.NumLit{Value: 0, Raw: "0"},
						},
					},
				},
			},
		},
		Rules: []*ast.Rule{
			{
				Action: &ast.BlockStmt{
					Stmts: []ast.Stmt{
						&ast.ExprStmt{
							Expr: &ast.AssignExpr{
								Left: &ast.Ident{Name: "sum"},
								Op:   token.ADD_ASSIGN,
								Right: &ast.FieldExpr{
									Index: &ast.NumLit{Value: 1, Raw: "1"},
								},
							},
						},
					},
				},
			},
		},
		EndBlocks: []*ast.BlockStmt{
			{
				Stmts: []ast.Stmt{
					&ast.PrintStmt{
						Args: []ast.Expr{&ast.Ident{Name: "sum"}},
					},
				},
			},
		},
	}

	result := ast.String(prog)

	// Verify key parts are present
	if !strings.Contains(result, "BEGIN") {
		t.Error("missing BEGIN block")
	}
	if !strings.Contains(result, "END") {
		t.Error("missing END block")
	}
	if !strings.Contains(result, "sum") {
		t.Error("missing variable sum")
	}
}

// TestVisitorInterface ensures the Visitor interface compiles.
func TestVisitorInterface(t *testing.T) {
	// This is a compile-time check - if it compiles, the interface is valid
	var _ ast.Visitor[int] = (*testVisitor)(nil)
}

// testVisitor is a minimal implementation of Visitor[int] for compile testing.
type testVisitor struct{}

func (v *testVisitor) VisitProgram(*ast.Program) int           { return 0 }
func (v *testVisitor) VisitRule(*ast.Rule) int                 { return 0 }
func (v *testVisitor) VisitFuncDecl(*ast.FuncDecl) int         { return 0 }
func (v *testVisitor) VisitNumLit(*ast.NumLit) int             { return 0 }
func (v *testVisitor) VisitStrLit(*ast.StrLit) int             { return 0 }
func (v *testVisitor) VisitRegexLit(*ast.RegexLit) int         { return 0 }
func (v *testVisitor) VisitIdent(*ast.Ident) int               { return 0 }
func (v *testVisitor) VisitFieldExpr(*ast.FieldExpr) int       { return 0 }
func (v *testVisitor) VisitIndexExpr(*ast.IndexExpr) int       { return 0 }
func (v *testVisitor) VisitBinaryExpr(*ast.BinaryExpr) int     { return 0 }
func (v *testVisitor) VisitUnaryExpr(*ast.UnaryExpr) int       { return 0 }
func (v *testVisitor) VisitTernaryExpr(*ast.TernaryExpr) int   { return 0 }
func (v *testVisitor) VisitAssignExpr(*ast.AssignExpr) int     { return 0 }
func (v *testVisitor) VisitConcatExpr(*ast.ConcatExpr) int     { return 0 }
func (v *testVisitor) VisitGroupExpr(*ast.GroupExpr) int       { return 0 }
func (v *testVisitor) VisitCallExpr(*ast.CallExpr) int         { return 0 }
func (v *testVisitor) VisitBuiltinExpr(*ast.BuiltinExpr) int   { return 0 }
func (v *testVisitor) VisitGetlineExpr(*ast.GetlineExpr) int   { return 0 }
func (v *testVisitor) VisitInExpr(*ast.InExpr) int             { return 0 }
func (v *testVisitor) VisitMatchExpr(*ast.MatchExpr) int       { return 0 }
func (v *testVisitor) VisitCommaExpr(*ast.CommaExpr) int       { return 0 }
func (v *testVisitor) VisitExprStmt(*ast.ExprStmt) int         { return 0 }
func (v *testVisitor) VisitPrintStmt(*ast.PrintStmt) int       { return 0 }
func (v *testVisitor) VisitBlockStmt(*ast.BlockStmt) int       { return 0 }
func (v *testVisitor) VisitIfStmt(*ast.IfStmt) int             { return 0 }
func (v *testVisitor) VisitWhileStmt(*ast.WhileStmt) int       { return 0 }
func (v *testVisitor) VisitDoWhileStmt(*ast.DoWhileStmt) int   { return 0 }
func (v *testVisitor) VisitForStmt(*ast.ForStmt) int           { return 0 }
func (v *testVisitor) VisitForInStmt(*ast.ForInStmt) int       { return 0 }
func (v *testVisitor) VisitBreakStmt(*ast.BreakStmt) int       { return 0 }
func (v *testVisitor) VisitContinueStmt(*ast.ContinueStmt) int { return 0 }
func (v *testVisitor) VisitNextStmt(*ast.NextStmt) int         { return 0 }
func (v *testVisitor) VisitNextFileStmt(*ast.NextFileStmt) int { return 0 }
func (v *testVisitor) VisitReturnStmt(*ast.ReturnStmt) int     { return 0 }
func (v *testVisitor) VisitExitStmt(*ast.ExitStmt) int         { return 0 }
func (v *testVisitor) VisitDeleteStmt(*ast.DeleteStmt) int     { return 0 }

// TestAccept verifies the Accept generic function works.
func TestAccept(t *testing.T) {
	visitor := &testVisitor{}

	nodes := []ast.Node{
		&ast.NumLit{Value: 42},
		&ast.StrLit{Value: "test"},
		&ast.Ident{Name: "x"},
		&ast.BreakStmt{},
	}

	for _, n := range nodes {
		result := ast.Accept[int](n, visitor)
		if result != 0 {
			t.Errorf("Accept returned %d, want 0", result)
		}
	}
}
