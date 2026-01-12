package compiler

import (
	"testing"

	"github.com/kolkov/uawk/internal/ast"
	"github.com/kolkov/uawk/internal/parser"
	"github.com/kolkov/uawk/internal/semantic"
)

// parseAndInfer parses AWK source and runs type inference.
func parseAndInfer(t *testing.T, src string) (*ast.Program, *TypeInfo) {
	t.Helper()

	prog, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	resolved, err := semantic.Resolve(prog)
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}

	typeInfo := InferTypes(prog, resolved)
	return prog, typeInfo
}

func TestTypeInference_Literals(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantType InferredType
	}{
		{
			name:     "numeric literal",
			src:      `BEGIN { x = 42 }`,
			wantType: TypeInferNum,
		},
		{
			name:     "string literal",
			src:      `BEGIN { x = "hello" }`,
			wantType: TypeInferStr,
		},
		{
			name:     "float literal",
			src:      `BEGIN { x = 3.14 }`,
			wantType: TypeInferNum,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, typeInfo := parseAndInfer(t, tt.src)

			// Find the assignment expression
			block := prog.Begin[0]
			stmt := block.Stmts[0].(*ast.ExprStmt)
			assign := stmt.Expr.(*ast.AssignExpr)

			gotType := typeInfo.GetExprType(assign.Right)
			if gotType != tt.wantType {
				t.Errorf("got type %v, want %v", gotType, tt.wantType)
			}
		})
	}
}

func TestTypeInference_Arithmetic(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantType InferredType
	}{
		{
			name:     "addition",
			src:      `BEGIN { x = 1 + 2 }`,
			wantType: TypeInferNum,
		},
		{
			name:     "subtraction",
			src:      `BEGIN { x = 10 - 5 }`,
			wantType: TypeInferNum,
		},
		{
			name:     "multiplication",
			src:      `BEGIN { x = 3 * 4 }`,
			wantType: TypeInferNum,
		},
		{
			name:     "division",
			src:      `BEGIN { x = 10 / 2 }`,
			wantType: TypeInferNum,
		},
		{
			name:     "modulo",
			src:      `BEGIN { x = 10 % 3 }`,
			wantType: TypeInferNum,
		},
		{
			name:     "power",
			src:      `BEGIN { x = 2 ^ 10 }`,
			wantType: TypeInferNum,
		},
		{
			name:     "complex expression",
			src:      `BEGIN { x = (1 + 2) * (3 - 4) / 5 }`,
			wantType: TypeInferNum,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, typeInfo := parseAndInfer(t, tt.src)

			block := prog.Begin[0]
			stmt := block.Stmts[0].(*ast.ExprStmt)
			assign := stmt.Expr.(*ast.AssignExpr)

			gotType := typeInfo.GetExprType(assign.Right)
			if gotType != tt.wantType {
				t.Errorf("got type %v, want %v", gotType, tt.wantType)
			}
		})
	}
}

func TestTypeInference_Comparison(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantType InferredType
	}{
		{
			name:     "less than",
			src:      `BEGIN { x = 1 < 2 }`,
			wantType: TypeInferNum,
		},
		{
			name:     "greater than",
			src:      `BEGIN { x = 5 > 3 }`,
			wantType: TypeInferNum,
		},
		{
			name:     "equal",
			src:      `BEGIN { x = 1 == 1 }`,
			wantType: TypeInferNum,
		},
		{
			name:     "not equal",
			src:      `BEGIN { x = 1 != 2 }`,
			wantType: TypeInferNum,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, typeInfo := parseAndInfer(t, tt.src)

			block := prog.Begin[0]
			stmt := block.Stmts[0].(*ast.ExprStmt)
			assign := stmt.Expr.(*ast.AssignExpr)

			gotType := typeInfo.GetExprType(assign.Right)
			if gotType != tt.wantType {
				t.Errorf("got type %v, want %v", gotType, tt.wantType)
			}
		})
	}
}

func TestTypeInference_Variables(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		varName  string
		wantType InferredType
	}{
		{
			name:     "numeric only assignments",
			src:      `BEGIN { x = 1; x = 2; x = 3 }`,
			varName:  "x",
			wantType: TypeInferNum,
		},
		{
			name:     "string only assignments",
			src:      `BEGIN { s = "a"; s = "b" }`,
			varName:  "s",
			wantType: TypeInferStr,
		},
		{
			name:     "mixed assignments",
			src:      `BEGIN { x = 1; x = "str" }`,
			varName:  "x",
			wantType: TypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, typeInfo := parseAndInfer(t, tt.src)

			gotType := typeInfo.GetVarType("", tt.varName)
			if gotType != tt.wantType {
				t.Errorf("var %s: got type %v, want %v", tt.varName, gotType, tt.wantType)
			}
		})
	}
}

func TestTypeInference_ForLoop(t *testing.T) {
	tests := []struct {
		name        string
		src         string
		varName     string
		wantNumeric bool
	}{
		{
			name:        "classic for loop",
			src:         `BEGIN { for (i = 0; i < 10; i++) print i }`,
			varName:     "i",
			wantNumeric: true,
		},
		{
			name:        "for loop with += increment",
			src:         `BEGIN { for (j = 0; j < 100; j += 1) print j }`,
			varName:     "j",
			wantNumeric: true,
		},
		{
			name:        "for loop with decrement",
			src:         `BEGIN { for (k = 10; k > 0; k--) print k }`,
			varName:     "k",
			wantNumeric: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, typeInfo := parseAndInfer(t, tt.src)

			gotNumeric := typeInfo.IsNumericLoopVar("", tt.varName)
			if gotNumeric != tt.wantNumeric {
				t.Errorf("var %s: got numeric=%v, want %v", tt.varName, gotNumeric, tt.wantNumeric)
			}

			gotType := typeInfo.GetVarType("", tt.varName)
			if tt.wantNumeric && gotType != TypeInferNum {
				t.Errorf("var %s: got type %v, want TypeInferNum", tt.varName, gotType)
			}
		})
	}
}

func TestTypeInference_FieldAccess(t *testing.T) {
	// Field access should always be unknown (comes from input)
	_, typeInfo := parseAndInfer(t, `{ sum += $1 }`)

	// Find the field expression
	if typeInfo == nil {
		t.Fatal("typeInfo is nil")
	}

	// Sum gets assigned from field, so it should be unknown
	// (unless the += forces it to numeric)
	gotType := typeInfo.GetVarType("", "sum")
	// += makes it numeric because the result is always numeric
	if gotType != TypeInferNum {
		t.Errorf("sum: got type %v, want TypeInferNum (due to +=)", gotType)
	}
}

func TestTypeInference_Builtins(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantType InferredType
	}{
		{
			name:     "length returns number",
			src:      `BEGIN { x = length("test") }`,
			wantType: TypeInferNum,
		},
		{
			name:     "substr returns string",
			src:      `BEGIN { x = substr("hello", 1, 3) }`,
			wantType: TypeInferStr,
		},
		{
			name:     "tolower returns string",
			src:      `BEGIN { x = tolower("HELLO") }`,
			wantType: TypeInferStr,
		},
		{
			name:     "sin returns number",
			src:      `BEGIN { x = sin(0) }`,
			wantType: TypeInferNum,
		},
		{
			name:     "sqrt returns number",
			src:      `BEGIN { x = sqrt(4) }`,
			wantType: TypeInferNum,
		},
		{
			name:     "sprintf returns string",
			src:      `BEGIN { x = sprintf("%d", 42) }`,
			wantType: TypeInferStr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, typeInfo := parseAndInfer(t, tt.src)

			block := prog.Begin[0]
			stmt := block.Stmts[0].(*ast.ExprStmt)
			assign := stmt.Expr.(*ast.AssignExpr)

			gotType := typeInfo.GetExprType(assign.Right)
			if gotType != tt.wantType {
				t.Errorf("got type %v, want %v", gotType, tt.wantType)
			}
		})
	}
}

func TestTypeInference_Concatenation(t *testing.T) {
	_, typeInfo := parseAndInfer(t, `BEGIN { x = "a" "b" "c" }`)

	// Concatenation produces string type
	gotType := typeInfo.GetVarType("", "x")
	if gotType != TypeInferStr {
		t.Errorf("got type %v, want TypeInferStr", gotType)
	}
}

func TestTypeInference_Ternary(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantType InferredType
	}{
		{
			name:     "both numeric",
			src:      `BEGIN { x = (1 > 0) ? 10 : 20 }`,
			wantType: TypeInferNum,
		},
		{
			name:     "both string",
			src:      `BEGIN { x = (1 > 0) ? "yes" : "no" }`,
			wantType: TypeInferStr,
		},
		{
			name:     "mixed types",
			src:      `BEGIN { x = (1 > 0) ? 10 : "no" }`,
			wantType: TypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, typeInfo := parseAndInfer(t, tt.src)

			block := prog.Begin[0]
			stmt := block.Stmts[0].(*ast.ExprStmt)
			assign := stmt.Expr.(*ast.AssignExpr)

			gotType := typeInfo.GetExprType(assign.Right)
			if gotType != tt.wantType {
				t.Errorf("got type %v, want %v", gotType, tt.wantType)
			}
		})
	}
}

func TestTypeInference_UnaryOps(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantType InferredType
	}{
		{
			name:     "negation",
			src:      `BEGIN { x = -5 }`,
			wantType: TypeInferNum,
		},
		{
			name:     "unary plus",
			src:      `BEGIN { x = +5 }`,
			wantType: TypeInferNum,
		},
		{
			name:     "logical not",
			src:      `BEGIN { x = !0 }`,
			wantType: TypeInferNum,
		},
		{
			name:     "pre-increment",
			src:      `BEGIN { i = 0; x = ++i }`,
			wantType: TypeInferNum,
		},
		{
			name:     "post-increment",
			src:      `BEGIN { i = 0; x = i++ }`,
			wantType: TypeInferNum,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, typeInfo := parseAndInfer(t, tt.src)

			block := prog.Begin[0]
			// Get last statement (the assignment)
			lastStmt := block.Stmts[len(block.Stmts)-1].(*ast.ExprStmt)
			assign := lastStmt.Expr.(*ast.AssignExpr)

			gotType := typeInfo.GetExprType(assign.Right)
			if gotType != tt.wantType {
				t.Errorf("got type %v, want %v", gotType, tt.wantType)
			}
		})
	}
}

func TestTypeInference_AugmentedAssign(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		varName  string
		wantType InferredType
	}{
		{
			name:     "+= makes variable numeric",
			src:      `BEGIN { sum += 1 }`,
			varName:  "sum",
			wantType: TypeInferNum,
		},
		{
			name:     "-= makes variable numeric",
			src:      `BEGIN { count -= 1 }`,
			varName:  "count",
			wantType: TypeInferNum,
		},
		{
			name:     "*= makes variable numeric",
			src:      `BEGIN { product *= 2 }`,
			varName:  "product",
			wantType: TypeInferNum,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, typeInfo := parseAndInfer(t, tt.src)

			gotType := typeInfo.GetVarType("", tt.varName)
			if gotType != tt.wantType {
				t.Errorf("var %s: got type %v, want %v", tt.varName, gotType, tt.wantType)
			}
		})
	}
}

func TestTypeInference_BothNumeric(t *testing.T) {
	prog, typeInfo := parseAndInfer(t, `BEGIN { x = 1 + 2 }`)

	block := prog.Begin[0]
	stmt := block.Stmts[0].(*ast.ExprStmt)
	assign := stmt.Expr.(*ast.AssignExpr)
	binary := assign.Right.(*ast.BinaryExpr)

	if !typeInfo.BothNumeric(binary.Left, binary.Right) {
		t.Error("expected both operands to be numeric")
	}
}

func TestTypeInference_IsNumericExpr(t *testing.T) {
	prog, typeInfo := parseAndInfer(t, `BEGIN { x = 42 }`)

	block := prog.Begin[0]
	stmt := block.Stmts[0].(*ast.ExprStmt)
	assign := stmt.Expr.(*ast.AssignExpr)

	if !typeInfo.IsNumericExpr(assign.Right) {
		t.Error("expected numeric literal to be numeric")
	}
}

func TestInferredTypeString(t *testing.T) {
	tests := []struct {
		t    InferredType
		want string
	}{
		{TypeUnknown, "unknown"},
		{TypeInferNum, "num"},
		{TypeInferStr, "str"},
	}

	for _, tt := range tests {
		got := tt.t.String()
		if got != tt.want {
			t.Errorf("%v.String() = %q, want %q", tt.t, got, tt.want)
		}
	}
}
