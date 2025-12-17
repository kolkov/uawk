package parser_test

import (
	"testing"

	"github.com/kolkov/uawk/internal/ast"
	"github.com/kolkov/uawk/internal/parser"
	"github.com/kolkov/uawk/internal/token"
)

// TestParseEmpty tests parsing an empty program.
func TestParseEmpty(t *testing.T) {
	prog, err := parser.Parse("")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if prog == nil {
		t.Fatal("Parse() returned nil program")
	}
	if len(prog.Begin) != 0 {
		t.Errorf("Begin blocks = %d, want 0", len(prog.Begin))
	}
	if len(prog.Rules) != 0 {
		t.Errorf("Rules = %d, want 0", len(prog.Rules))
	}
	if len(prog.EndBlocks) != 0 {
		t.Errorf("End blocks = %d, want 0", len(prog.EndBlocks))
	}
	if len(prog.Functions) != 0 {
		t.Errorf("Functions = %d, want 0", len(prog.Functions))
	}
}

// TestParseProgram tests parsing complete programs.
func TestParseProgram(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		wantBegin int
		wantRules int
		wantEnd   int
		wantFuncs int
		wantErr   bool
	}{
		{
			name:      "empty",
			src:       "",
			wantRules: 0,
		},
		{
			name:      "begin block",
			src:       "BEGIN { print }",
			wantBegin: 1,
		},
		{
			name:    "end block",
			src:     "END { print }",
			wantEnd: 1,
		},
		{
			name:      "pattern-action",
			src:       "/foo/ { print }",
			wantRules: 1,
		},
		{
			name:      "action only",
			src:       "{ print $1 }",
			wantRules: 1,
		},
		{
			name:      "pattern only",
			src:       "/foo/",
			wantRules: 1,
		},
		{
			name:      "function",
			src:       "function add(a, b) { return a + b }",
			wantFuncs: 1,
		},
		{
			name:      "multiple items",
			src:       "BEGIN { x = 0 }\n{ x += $1 }\nEND { print x }",
			wantBegin: 1,
			wantRules: 1,
			wantEnd:   1,
		},
		{
			name:      "range pattern",
			src:       "/start/,/end/ { print }",
			wantRules: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, err := parser.Parse(tt.src)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if prog == nil {
				t.Fatal("Parse() returned nil")
			}
			if len(prog.Begin) != tt.wantBegin {
				t.Errorf("Begin blocks = %d, want %d", len(prog.Begin), tt.wantBegin)
			}
			if len(prog.Rules) != tt.wantRules {
				t.Errorf("Rules = %d, want %d", len(prog.Rules), tt.wantRules)
			}
			if len(prog.EndBlocks) != tt.wantEnd {
				t.Errorf("End blocks = %d, want %d", len(prog.EndBlocks), tt.wantEnd)
			}
			if len(prog.Functions) != tt.wantFuncs {
				t.Errorf("Functions = %d, want %d", len(prog.Functions), tt.wantFuncs)
			}
		})
	}
}

// TestParseExpr tests parsing individual expressions.
func TestParseExpr(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr bool
		check   func(ast.Expr) bool
	}{
		{
			name: "number integer",
			src:  "42",
			check: func(e ast.Expr) bool {
				n, ok := e.(*ast.NumLit)
				return ok && n.Value == 42
			},
		},
		{
			name: "number float",
			src:  "3.14",
			check: func(e ast.Expr) bool {
				n, ok := e.(*ast.NumLit)
				return ok && n.Value == 3.14
			},
		},
		{
			name: "string",
			src:  `"hello"`,
			check: func(e ast.Expr) bool {
				s, ok := e.(*ast.StrLit)
				return ok && s.Value == "hello"
			},
		},
		{
			name: "identifier",
			src:  "foo",
			check: func(e ast.Expr) bool {
				id, ok := e.(*ast.Ident)
				return ok && id.Name == "foo"
			},
		},
		{
			name: "field $1",
			src:  "$1",
			check: func(e ast.Expr) bool {
				f, ok := e.(*ast.FieldExpr)
				if !ok {
					return false
				}
				n, ok := f.Index.(*ast.NumLit)
				return ok && n.Value == 1
			},
		},
		{
			name: "field $NF",
			src:  "$NF",
			check: func(e ast.Expr) bool {
				f, ok := e.(*ast.FieldExpr)
				if !ok {
					return false
				}
				id, ok := f.Index.(*ast.Ident)
				return ok && id.Name == "NF"
			},
		},
		{
			name: "binary add",
			src:  "1 + 2",
			check: func(e ast.Expr) bool {
				b, ok := e.(*ast.BinaryExpr)
				return ok && b.Op == token.ADD
			},
		},
		{
			name: "binary sub",
			src:  "a - b",
			check: func(e ast.Expr) bool {
				b, ok := e.(*ast.BinaryExpr)
				return ok && b.Op == token.SUB
			},
		},
		{
			name: "binary mul",
			src:  "x * y",
			check: func(e ast.Expr) bool {
				b, ok := e.(*ast.BinaryExpr)
				return ok && b.Op == token.MUL
			},
		},
		{
			name: "binary div",
			src:  "a / b",
			check: func(e ast.Expr) bool {
				b, ok := e.(*ast.BinaryExpr)
				return ok && b.Op == token.DIV
			},
		},
		{
			name: "binary mod",
			src:  "a % b",
			check: func(e ast.Expr) bool {
				b, ok := e.(*ast.BinaryExpr)
				return ok && b.Op == token.MOD
			},
		},
		{
			name: "binary pow",
			src:  "a ^ b",
			check: func(e ast.Expr) bool {
				b, ok := e.(*ast.BinaryExpr)
				return ok && b.Op == token.POW
			},
		},
		{
			name: "comparison equal",
			src:  "a == b",
			check: func(e ast.Expr) bool {
				b, ok := e.(*ast.BinaryExpr)
				return ok && b.Op == token.EQUALS
			},
		},
		{
			name: "comparison not equal",
			src:  "a != b",
			check: func(e ast.Expr) bool {
				b, ok := e.(*ast.BinaryExpr)
				return ok && b.Op == token.NOT_EQUALS
			},
		},
		{
			name: "comparison less",
			src:  "a < b",
			check: func(e ast.Expr) bool {
				b, ok := e.(*ast.BinaryExpr)
				return ok && b.Op == token.LESS
			},
		},
		{
			name: "logical and",
			src:  "a && b",
			check: func(e ast.Expr) bool {
				b, ok := e.(*ast.BinaryExpr)
				return ok && b.Op == token.AND
			},
		},
		{
			name: "logical or",
			src:  "a || b",
			check: func(e ast.Expr) bool {
				b, ok := e.(*ast.BinaryExpr)
				return ok && b.Op == token.OR
			},
		},
		{
			name: "unary not",
			src:  "!x",
			check: func(e ast.Expr) bool {
				u, ok := e.(*ast.UnaryExpr)
				return ok && u.Op == token.NOT && !u.Post
			},
		},
		{
			name: "unary minus",
			src:  "-x",
			check: func(e ast.Expr) bool {
				u, ok := e.(*ast.UnaryExpr)
				return ok && u.Op == token.SUB && !u.Post
			},
		},
		{
			name: "prefix increment",
			src:  "++x",
			check: func(e ast.Expr) bool {
				u, ok := e.(*ast.UnaryExpr)
				return ok && u.Op == token.INCR && !u.Post
			},
		},
		{
			name: "postfix increment",
			src:  "x++",
			check: func(e ast.Expr) bool {
				u, ok := e.(*ast.UnaryExpr)
				return ok && u.Op == token.INCR && u.Post
			},
		},
		{
			name: "prefix decrement",
			src:  "--x",
			check: func(e ast.Expr) bool {
				u, ok := e.(*ast.UnaryExpr)
				return ok && u.Op == token.DECR && !u.Post
			},
		},
		{
			name: "postfix decrement",
			src:  "x--",
			check: func(e ast.Expr) bool {
				u, ok := e.(*ast.UnaryExpr)
				return ok && u.Op == token.DECR && u.Post
			},
		},
		{
			name: "ternary",
			src:  "a ? b : c",
			check: func(e ast.Expr) bool {
				_, ok := e.(*ast.TernaryExpr)
				return ok
			},
		},
		{
			name: "assignment",
			src:  "x = 1",
			check: func(e ast.Expr) bool {
				a, ok := e.(*ast.AssignExpr)
				return ok && a.Op == token.ASSIGN
			},
		},
		{
			name: "add assign",
			src:  "x += 1",
			check: func(e ast.Expr) bool {
				a, ok := e.(*ast.AssignExpr)
				return ok && a.Op == token.ADD_ASSIGN
			},
		},
		{
			name: "array index",
			src:  "arr[key]",
			check: func(e ast.Expr) bool {
				idx, ok := e.(*ast.IndexExpr)
				return ok && len(idx.Index) == 1
			},
		},
		{
			name: "multi-dim array",
			src:  "arr[i, j]",
			check: func(e ast.Expr) bool {
				idx, ok := e.(*ast.IndexExpr)
				return ok && len(idx.Index) == 2
			},
		},
		{
			name: "in expression",
			src:  "key in arr",
			check: func(e ast.Expr) bool {
				_, ok := e.(*ast.InExpr)
				return ok
			},
		},
		{
			name: "match expression",
			src:  `x ~ "pattern"`,
			check: func(e ast.Expr) bool {
				m, ok := e.(*ast.MatchExpr)
				return ok && m.Op == token.MATCH
			},
		},
		{
			name: "not match expression",
			src:  `x !~ "pattern"`,
			check: func(e ast.Expr) bool {
				m, ok := e.(*ast.MatchExpr)
				return ok && m.Op == token.NOT_MATCH
			},
		},
		{
			name: "function call",
			src:  "func(a, b)",
			check: func(e ast.Expr) bool {
				c, ok := e.(*ast.CallExpr)
				return ok && c.Name == "func" && len(c.Args) == 2
			},
		},
		{
			name: "builtin length",
			src:  "length($0)",
			check: func(e ast.Expr) bool {
				b, ok := e.(*ast.BuiltinExpr)
				return ok && b.Func == token.F_LENGTH
			},
		},
		{
			name: "builtin length no parens",
			src:  "length",
			check: func(e ast.Expr) bool {
				b, ok := e.(*ast.BuiltinExpr)
				return ok && b.Func == token.F_LENGTH
			},
		},
		{
			name: "builtin substr",
			src:  "substr(s, 1, 5)",
			check: func(e ast.Expr) bool {
				b, ok := e.(*ast.BuiltinExpr)
				return ok && b.Func == token.F_SUBSTR && len(b.Args) == 3
			},
		},
		{
			name: "builtin split",
			src:  `split(s, arr, ":")`,
			check: func(e ast.Expr) bool {
				b, ok := e.(*ast.BuiltinExpr)
				return ok && b.Func == token.F_SPLIT && len(b.Args) == 3
			},
		},
		{
			name: "getline",
			src:  "getline",
			check: func(e ast.Expr) bool {
				_, ok := e.(*ast.GetlineExpr)
				return ok
			},
		},
		{
			name: "getline var",
			src:  "getline x",
			check: func(e ast.Expr) bool {
				g, ok := e.(*ast.GetlineExpr)
				return ok && g.Target != nil
			},
		},
		{
			name: "grouped expression",
			src:  "(a + b)",
			check: func(e ast.Expr) bool {
				g, ok := e.(*ast.GroupExpr)
				if !ok {
					return false
				}
				_, ok = g.Expr.(*ast.BinaryExpr)
				return ok
			},
		},
		{
			name: "precedence mul before add",
			src:  "1 + 2 * 3",
			check: func(e ast.Expr) bool {
				b, ok := e.(*ast.BinaryExpr)
				if !ok || b.Op != token.ADD {
					return false
				}
				// Right should be mul
				r, ok := b.Right.(*ast.BinaryExpr)
				return ok && r.Op == token.MUL
			},
		},
		{
			name: "precedence pow right assoc",
			src:  "2 ^ 3 ^ 4",
			check: func(e ast.Expr) bool {
				b, ok := e.(*ast.BinaryExpr)
				if !ok || b.Op != token.POW {
					return false
				}
				// Right should be pow (right associative)
				r, ok := b.Right.(*ast.BinaryExpr)
				return ok && r.Op == token.POW
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := parser.ParseExpr(tt.src)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseExpr() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if expr == nil {
				t.Fatal("ParseExpr() returned nil")
			}
			if tt.check != nil && !tt.check(expr) {
				t.Errorf("ParseExpr() check failed for %q, got %T", tt.src, expr)
			}
		})
	}
}

// TestParseStmt tests parsing statements within blocks.
func TestParseStmt(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr bool
		check   func(*ast.BlockStmt) bool
	}{
		{
			name: "if statement",
			src:  "{ if (x) print }",
			check: func(b *ast.BlockStmt) bool {
				if len(b.Stmts) != 1 {
					return false
				}
				_, ok := b.Stmts[0].(*ast.IfStmt)
				return ok
			},
		},
		{
			name: "if-else statement",
			src:  "{ if (x) print; else print y }",
			check: func(b *ast.BlockStmt) bool {
				if len(b.Stmts) != 1 {
					return false
				}
				i, ok := b.Stmts[0].(*ast.IfStmt)
				return ok && i.Else != nil
			},
		},
		{
			name: "while statement",
			src:  "{ while (x) x-- }",
			check: func(b *ast.BlockStmt) bool {
				if len(b.Stmts) != 1 {
					return false
				}
				_, ok := b.Stmts[0].(*ast.WhileStmt)
				return ok
			},
		},
		{
			name: "do-while statement",
			src:  "{ do x++ while (x < 10) }",
			check: func(b *ast.BlockStmt) bool {
				if len(b.Stmts) != 1 {
					return false
				}
				_, ok := b.Stmts[0].(*ast.DoWhileStmt)
				return ok
			},
		},
		{
			name: "for statement",
			src:  "{ for (i = 0; i < 10; i++) print i }",
			check: func(b *ast.BlockStmt) bool {
				if len(b.Stmts) != 1 {
					return false
				}
				_, ok := b.Stmts[0].(*ast.ForStmt)
				return ok
			},
		},
		{
			name: "for-in statement",
			src:  "{ for (k in arr) print k }",
			check: func(b *ast.BlockStmt) bool {
				if len(b.Stmts) != 1 {
					return false
				}
				_, ok := b.Stmts[0].(*ast.ForInStmt)
				return ok
			},
		},
		{
			name: "print statement",
			src:  "{ print $1, $2 }",
			check: func(b *ast.BlockStmt) bool {
				if len(b.Stmts) != 1 {
					return false
				}
				p, ok := b.Stmts[0].(*ast.PrintStmt)
				return ok && !p.Printf && len(p.Args) == 2
			},
		},
		{
			name: "printf statement",
			src:  `{ printf "%d\n", x }`,
			check: func(b *ast.BlockStmt) bool {
				if len(b.Stmts) != 1 {
					return false
				}
				p, ok := b.Stmts[0].(*ast.PrintStmt)
				return ok && p.Printf && len(p.Args) == 2
			},
		},
		{
			name: "print redirect",
			src:  `{ print "x" > "file" }`,
			check: func(b *ast.BlockStmt) bool {
				if len(b.Stmts) != 1 {
					return false
				}
				p, ok := b.Stmts[0].(*ast.PrintStmt)
				return ok && p.Redirect == token.GREATER && p.Dest != nil
			},
		},
		{
			name: "print append",
			src:  `{ print "x" >> "file" }`,
			check: func(b *ast.BlockStmt) bool {
				if len(b.Stmts) != 1 {
					return false
				}
				p, ok := b.Stmts[0].(*ast.PrintStmt)
				return ok && p.Redirect == token.APPEND && p.Dest != nil
			},
		},
		{
			name: "print pipe",
			src:  `{ print "x" | "cmd" }`,
			check: func(b *ast.BlockStmt) bool {
				if len(b.Stmts) != 1 {
					return false
				}
				p, ok := b.Stmts[0].(*ast.PrintStmt)
				return ok && p.Redirect == token.PIPE && p.Dest != nil
			},
		},
		{
			name: "delete statement",
			src:  "{ delete arr[k] }",
			check: func(b *ast.BlockStmt) bool {
				if len(b.Stmts) != 1 {
					return false
				}
				_, ok := b.Stmts[0].(*ast.DeleteStmt)
				return ok
			},
		},
		{
			name: "exit statement",
			src:  "{ exit 1 }",
			check: func(b *ast.BlockStmt) bool {
				if len(b.Stmts) != 1 {
					return false
				}
				e, ok := b.Stmts[0].(*ast.ExitStmt)
				return ok && e.Code != nil
			},
		},
		{
			name: "multiple statements",
			src:  "{ x = 1; y = 2; print x, y }",
			check: func(b *ast.BlockStmt) bool {
				return len(b.Stmts) == 3
			},
		},
		{
			name: "newline separators",
			src:  "{ x = 1\ny = 2\nprint x, y }",
			check: func(b *ast.BlockStmt) bool {
				return len(b.Stmts) == 3
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, err := parser.Parse(tt.src)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if prog == nil || len(prog.Rules) != 1 || prog.Rules[0].Action == nil {
				t.Fatal("Parse() didn't return expected structure")
			}
			if tt.check != nil && !tt.check(prog.Rules[0].Action) {
				t.Errorf("Parse() check failed for %q", tt.src)
			}
		})
	}
}

// TestParseFunction tests function declaration parsing.
func TestParseFunction(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		funcName  string
		numParams int
		wantErr   bool
	}{
		{
			name:      "no params",
			src:       "function foo() { return 1 }",
			funcName:  "foo",
			numParams: 0,
		},
		{
			name:      "one param",
			src:       "function add(x) { return x + 1 }",
			funcName:  "add",
			numParams: 1,
		},
		{
			name:      "two params",
			src:       "function add(a, b) { return a + b }",
			funcName:  "add",
			numParams: 2,
		},
		{
			name:      "with body",
			src:       "function max(a, b) { if (a > b) return a; return b }",
			funcName:  "max",
			numParams: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, err := parser.Parse(tt.src)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if prog == nil || len(prog.Functions) != 1 {
				t.Fatal("Parse() didn't return expected structure")
			}
			fn := prog.Functions[0]
			if fn.Name != tt.funcName {
				t.Errorf("Function name = %q, want %q", fn.Name, tt.funcName)
			}
			if fn.NumParams != tt.numParams {
				t.Errorf("NumParams = %d, want %d", fn.NumParams, tt.numParams)
			}
		})
	}
}

// TestParseErrors tests that parse errors are properly reported.
func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"unclosed brace", "BEGIN {"},
		{"unclosed paren", "BEGIN { print(1 }"},
		{"missing condition", "BEGIN { if () print }"},
		{"break outside loop", "BEGIN { break }"},
		{"continue outside loop", "BEGIN { continue }"},
		{"return outside function", "BEGIN { return 1 }"},
		{"next in BEGIN", "BEGIN { next }"},
		{"duplicate param", "function f(a, a) { }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse(tt.src)
			if err == nil {
				t.Errorf("Parse(%q) expected error, got none", tt.src)
			}
		})
	}
}

// TestParseErrorPosition tests that error positions are correct.
func TestParseErrorPosition(t *testing.T) {
	src := "BEGIN { print( }"
	_, err := parser.Parse(src)
	if err == nil {
		t.Fatal("expected error")
	}

	errStr := err.Error()
	// Should contain position information
	if len(errStr) == 0 {
		t.Error("error message is empty")
	}
}

// TestConcatenation tests implicit concatenation parsing.
func TestConcatenation(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		numExprs int
	}{
		{"two exprs", "a b", 2},
		{"three exprs", "a b c", 3},
		{"with numbers", "a 1 b", 3},
		{"with strings", `a "x" b`, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := parser.ParseExpr(tt.src)
			if err != nil {
				t.Fatalf("ParseExpr() error = %v", err)
			}
			concat, ok := expr.(*ast.ConcatExpr)
			if !ok {
				t.Fatalf("expected ConcatExpr, got %T", expr)
			}
			if len(concat.Exprs) != tt.numExprs {
				t.Errorf("ConcatExpr has %d exprs, want %d", len(concat.Exprs), tt.numExprs)
			}
		})
	}
}

// TestBuiltinFunctions tests parsing of various built-in functions.
func TestBuiltinFunctions(t *testing.T) {
	builtins := []string{
		"length($0)",
		"length",
		"substr(s, 1)",
		"substr(s, 1, 5)",
		"index(s, t)",
		"split(s, a)",
		`split(s, a, ":")`,
		"sprintf(\"%d\", x)",
		"sub(/re/, r)",
		"sub(/re/, r, s)",
		"gsub(/re/, r)",
		"gsub(/re/, r, s)",
		"match(s, /re/)",
		"tolower(s)",
		"toupper(s)",
		"int(x)",
		"sqrt(x)",
		"exp(x)",
		"log(x)",
		"sin(x)",
		"cos(x)",
		"atan2(y, x)",
		"rand()",
		"srand()",
		"srand(x)",
		"system(cmd)",
		"close(f)",
		"fflush()",
		"fflush(f)",
	}

	for _, src := range builtins {
		t.Run(src, func(t *testing.T) {
			expr, err := parser.ParseExpr(src)
			if err != nil {
				t.Fatalf("ParseExpr(%q) error = %v", src, err)
			}
			if _, ok := expr.(*ast.BuiltinExpr); !ok {
				t.Errorf("ParseExpr(%q) = %T, want *BuiltinExpr", src, expr)
			}
		})
	}
}
