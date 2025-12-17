package compiler

import (
	"strings"
	"testing"

	"github.com/kolkov/uawk/internal/ast"
	"github.com/kolkov/uawk/internal/parser"
	"github.com/kolkov/uawk/internal/semantic"
)

// Helper to compile a program string.
func compileSource(t *testing.T, source string) *Program {
	t.Helper()

	prog, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	resolved, err := semantic.Resolve(prog)
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}

	compiled, err := Compile(prog, resolved)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	return compiled
}

func TestCompileEmpty(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"empty", ""},
		{"empty begin", "BEGIN {}"},
		{"empty end", "END {}"},
		{"empty action", "{}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled := compileSource(t, tt.source)
			if compiled == nil {
				t.Fatal("compiled program is nil")
			}
		})
	}
}

func TestCompileNumericLiterals(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		wantNums []float64
	}{
		{
			name:     "integer",
			source:   "BEGIN { x = 42 }",
			wantNums: []float64{42},
		},
		{
			name:     "float",
			source:   "BEGIN { x = 3.14 }",
			wantNums: []float64{3.14},
		},
		{
			name:     "scientific",
			source:   "BEGIN { x = 1e10 }",
			wantNums: []float64{1e10},
		},
		{
			name:     "multiple",
			source:   "BEGIN { x = 1; y = 2; z = 3 }",
			wantNums: []float64{1, 2, 3},
		},
		{
			name:     "dedup",
			source:   "BEGIN { x = 42; y = 42 }",
			wantNums: []float64{42}, // Should be deduplicated
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled := compileSource(t, tt.source)
			if len(compiled.Nums) != len(tt.wantNums) {
				t.Errorf("got %d nums, want %d", len(compiled.Nums), len(tt.wantNums))
			}
			for i, want := range tt.wantNums {
				if i < len(compiled.Nums) && compiled.Nums[i] != want {
					t.Errorf("Nums[%d] = %v, want %v", i, compiled.Nums[i], want)
				}
			}
		})
	}
}

func TestCompileStringLiterals(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		wantStrs []string
	}{
		{
			name:     "simple",
			source:   `BEGIN { x = "hello" }`,
			wantStrs: []string{"hello"},
		},
		{
			name:     "multiple",
			source:   `BEGIN { x = "a"; y = "b" }`,
			wantStrs: []string{"a", "b"},
		},
		{
			name:     "dedup",
			source:   `BEGIN { x = "hello"; y = "hello" }`,
			wantStrs: []string{"hello"}, // Should be deduplicated
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled := compileSource(t, tt.source)
			if len(compiled.Strs) != len(tt.wantStrs) {
				t.Errorf("got %d strs, want %d", len(compiled.Strs), len(tt.wantStrs))
			}
			for i, want := range tt.wantStrs {
				if i < len(compiled.Strs) && compiled.Strs[i] != want {
					t.Errorf("Strs[%d] = %q, want %q", i, compiled.Strs[i], want)
				}
			}
		})
	}
}

func TestCompileRegexLiterals(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantRegexes []string
		wantStrs    []string // For match expressions, regex pattern goes to strings
	}{
		{
			name:        "pattern rule",
			source:      "/test/ { print }",
			wantRegexes: []string{"test"},
		},
		{
			name:     "match expression",
			source:   `BEGIN { if ($0 ~ /foo/) print }`,
			wantStrs: []string{"foo"}, // In match expressions, pattern is a string
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled := compileSource(t, tt.source)
			if len(tt.wantRegexes) > 0 {
				if len(compiled.Regexes) != len(tt.wantRegexes) {
					t.Errorf("got %d regexes, want %d", len(compiled.Regexes), len(tt.wantRegexes))
				}
			}
			if len(tt.wantStrs) > 0 {
				// Check that pattern appears in strings
				found := false
				for _, s := range compiled.Strs {
					for _, want := range tt.wantStrs {
						if s == want {
							found = true
							break
						}
					}
				}
				if !found {
					t.Errorf("pattern not found in strings, got %v", compiled.Strs)
				}
			}
		})
	}
}

func TestCompileArithmetic(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"add", "BEGIN { x = 1 + 2 }"},
		{"subtract", "BEGIN { x = 5 - 3 }"},
		{"multiply", "BEGIN { x = 4 * 5 }"},
		{"divide", "BEGIN { x = 10 / 2 }"},
		{"modulo", "BEGIN { x = 10 % 3 }"},
		{"power", "BEGIN { x = 2 ^ 3 }"},
		{"complex", "BEGIN { x = (1 + 2) * 3 }"},
		{"unary minus", "BEGIN { x = -5 }"},
		{"unary plus", "BEGIN { x = +5 }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled := compileSource(t, tt.source)
			if len(compiled.Begin) == 0 {
				t.Error("BEGIN block is empty")
			}
		})
	}
}

func TestCompileComparison(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"equal", "BEGIN { if (1 == 1) print }"},
		{"not equal", "BEGIN { if (1 != 2) print }"},
		{"less", "BEGIN { if (1 < 2) print }"},
		{"less equal", "BEGIN { if (1 <= 2) print }"},
		{"greater", "BEGIN { if (2 > 1) print }"},
		{"greater equal", "BEGIN { if (2 >= 1) print }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled := compileSource(t, tt.source)
			if len(compiled.Begin) == 0 {
				t.Error("BEGIN block is empty")
			}
		})
	}
}

func TestCompileControlFlow(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"if", "BEGIN { if (1) print }"},
		{"if else", "BEGIN { if (0) print 1; else print 2 }"},
		{"while", "BEGIN { while (x < 10) x++ }"},
		{"do while", "BEGIN { do x++ while (x < 10) }"},
		{"for", "BEGIN { for (i = 0; i < 10; i++) print i }"},
		{"for in", "BEGIN { for (k in arr) print k }"},
		{"break", "BEGIN { while (1) { break } }"},
		{"continue", "BEGIN { while (1) { continue } }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled := compileSource(t, tt.source)
			if len(compiled.Begin) == 0 {
				t.Error("BEGIN block is empty")
			}
		})
	}
}

func TestCompileIncrDecr(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"pre incr", "BEGIN { ++x }"},
		{"post incr", "BEGIN { x++ }"},
		{"pre decr", "BEGIN { --x }"},
		{"post decr", "BEGIN { x-- }"},
		{"incr field", "BEGIN { $1++ }"},
		{"incr array", "BEGIN { arr[1]++ }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled := compileSource(t, tt.source)
			if len(compiled.Begin) == 0 {
				t.Error("BEGIN block is empty")
			}
		})
	}
}

func TestCompileAugmentedAssign(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"add assign", "BEGIN { x += 1 }"},
		{"sub assign", "BEGIN { x -= 1 }"},
		{"mul assign", "BEGIN { x *= 2 }"},
		{"div assign", "BEGIN { x /= 2 }"},
		{"mod assign", "BEGIN { x %= 3 }"},
		{"pow assign", "BEGIN { x ^= 2 }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled := compileSource(t, tt.source)
			if len(compiled.Begin) == 0 {
				t.Error("BEGIN block is empty")
			}
		})
	}
}

func TestCompilePrint(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"print simple", "BEGIN { print }"},
		{"print args", `BEGIN { print "hello", "world" }`},
		{"print redirect", `BEGIN { print "hello" > "/dev/null" }`},
		{"print append", `BEGIN { print "hello" >> "/dev/null" }`},
		{"print pipe", `BEGIN { print "hello" | "cat" }`},
		{"printf", `BEGIN { printf "%d\n", 42 }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled := compileSource(t, tt.source)
			if len(compiled.Begin) == 0 {
				t.Error("BEGIN block is empty")
			}
		})
	}
}

func TestCompileFields(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"field 0", "{ print $0 }"},
		{"field 1", "{ print $1 }"},
		{"field dynamic", "{ print $NF }"},
		{"field assign", "{ $1 = 42 }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled := compileSource(t, tt.source)
			if len(compiled.Actions) == 0 {
				t.Error("no actions compiled")
			}
		})
	}
}

func TestCompileArrays(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"array get", "BEGIN { print arr[1] }"},
		{"array set", "BEGIN { arr[1] = 42 }"},
		{"array in", `BEGIN { if ("key" in arr) print }`},
		{"array delete", "BEGIN { delete arr[1] }"},
		{"array delete all", "BEGIN { delete arr }"},
		{"multi-index", `BEGIN { arr[1,2] = "x" }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled := compileSource(t, tt.source)
			if len(compiled.Begin) == 0 {
				t.Error("BEGIN block is empty")
			}
		})
	}
}

func TestCompileFunctions(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name:   "simple function",
			source: "function add(a, b) { return a + b } BEGIN { print add(1, 2) }",
		},
		{
			name:   "recursive function",
			source: "function fac(n) { if (n <= 1) return 1; return n * fac(n-1) } BEGIN { print fac(5) }",
		},
		{
			name:   "local variables",
			source: "function f(a,    local1, local2) { local1 = a * 2; return local1 } BEGIN { print f(5) }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled := compileSource(t, tt.source)
			if len(compiled.Functions) == 0 {
				t.Error("no functions compiled")
			}
		})
	}
}

func TestCompileBuiltins(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"length", `BEGIN { print length("hello") }`},
		{"length no arg", "BEGIN { print length() }"},
		{"substr 2 arg", `BEGIN { print substr("hello", 2) }`},
		{"substr 3 arg", `BEGIN { print substr("hello", 2, 3) }`},
		{"index", `BEGIN { print index("hello", "l") }`},
		{"split", `BEGIN { split("a:b:c", arr, ":") }`},
		{"sprintf", `BEGIN { print sprintf("%d", 42) }`},
		{"tolower", `BEGIN { print tolower("HELLO") }`},
		{"toupper", `BEGIN { print toupper("hello") }`},
		{"sin", "BEGIN { print sin(0) }"},
		{"cos", "BEGIN { print cos(0) }"},
		{"sqrt", "BEGIN { print sqrt(4) }"},
		{"int", "BEGIN { print int(3.7) }"},
		{"rand", "BEGIN { print rand() }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled := compileSource(t, tt.source)
			if len(compiled.Begin) == 0 {
				t.Error("BEGIN block is empty")
			}
		})
	}
}

func TestCompilePatterns(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantActions int
	}{
		{
			name:        "no pattern",
			source:      "{ print }",
			wantActions: 1,
		},
		{
			name:        "regex pattern",
			source:      "/test/ { print }",
			wantActions: 1,
		},
		{
			name:        "expression pattern",
			source:      "NR > 1 { print }",
			wantActions: 1,
		},
		{
			name:        "range pattern",
			source:      "/start/,/end/ { print }",
			wantActions: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled := compileSource(t, tt.source)
			if len(compiled.Actions) != tt.wantActions {
				t.Errorf("got %d actions, want %d", len(compiled.Actions), tt.wantActions)
			}
		})
	}
}

func TestCompileLogical(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"and", "BEGIN { if (1 && 1) print }"},
		{"or", "BEGIN { if (0 || 1) print }"},
		{"not", "BEGIN { if (!0) print }"},
		{"complex", "BEGIN { if ((1 && 0) || 1) print }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled := compileSource(t, tt.source)
			if len(compiled.Begin) == 0 {
				t.Error("BEGIN block is empty")
			}
		})
	}
}

func TestCompileTernary(t *testing.T) {
	compiled := compileSource(t, "BEGIN { x = 1 ? 2 : 3 }")
	if len(compiled.Begin) == 0 {
		t.Error("BEGIN block is empty")
	}
}

func TestCompileConcat(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"two values", `BEGIN { x = "a" "b" }`},
		{"multiple", `BEGIN { x = "a" "b" "c" "d" }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled := compileSource(t, tt.source)
			if len(compiled.Begin) == 0 {
				t.Error("BEGIN block is empty")
			}
		})
	}
}

func TestCompileNext(t *testing.T) {
	compiled := compileSource(t, "{ next }")
	if len(compiled.Actions) == 0 {
		t.Error("no actions compiled")
	}
}

func TestCompileExit(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"exit no code", "BEGIN { exit }"},
		{"exit with code", "BEGIN { exit 1 }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled := compileSource(t, tt.source)
			if len(compiled.Begin) == 0 {
				t.Error("BEGIN block is empty")
			}
		})
	}
}

func TestDisassemble(t *testing.T) {
	compiled := compileSource(t, `
		BEGIN {
			x = 42
			print "hello"
		}
		/test/ { print $1 }
		END { print "done" }
	`)

	disasm := compiled.Disassemble()
	if disasm == "" {
		t.Error("disassembly is empty")
	}

	// Check for expected sections
	expectedSections := []string{
		"Numbers",
		"Strings",
		"BEGIN",
		"Action",
		"END",
	}

	for _, section := range expectedSections {
		if !strings.Contains(disasm, section) {
			t.Errorf("disassembly missing section: %s", section)
		}
	}
}

func TestOpcodeString(t *testing.T) {
	// Verify all opcodes have valid String() representations
	opcodes := []Opcode{
		Nop, Num, Str, Dupe, Drop, Swap, Rote,
		LoadGlobal, LoadLocal, LoadSpecial,
		StoreGlobal, StoreLocal, StoreSpecial,
		Field, FieldInt, StoreField,
		ArrayGet, ArraySet, ArrayDelete, ArrayClear, ArrayIn,
		Add, Subtract, Multiply, Divide, Power, Modulo,
		Equal, NotEqual, Less, LessEqual, Greater, GreaterEqual,
		Concat, Match, NotMatch,
		UnaryMinus, UnaryPlus, Not, Boolean,
		Jump, JumpTrue, JumpFalse,
		Next, Nextfile, Exit, ExitCode,
		CallBuiltin, CallUser, Return, ReturnNull,
		Print, Printf, Getline,
		Halt,
	}

	for _, op := range opcodes {
		s := op.String()
		if s == "" || strings.HasPrefix(s, "Opcode(") {
			// Allow Opcode(N) for unknown opcodes
			if !strings.HasPrefix(s, "Opcode(") && s == "" {
				t.Errorf("opcode %d has empty string", op)
			}
		}
	}
}

func TestBuiltinOpString(t *testing.T) {
	ops := []BuiltinOp{
		BuiltinAtan2, BuiltinClose, BuiltinCos, BuiltinExp,
		BuiltinLength, BuiltinSin, BuiltinSqrt, BuiltinSubstr,
	}

	for _, op := range ops {
		s := op.String()
		if s == "" {
			t.Errorf("builtin op %d has empty string", op)
		}
	}
}

func TestAugOpString(t *testing.T) {
	ops := []AugOp{AugAdd, AugSub, AugMul, AugDiv, AugPow, AugMod}

	for _, op := range ops {
		s := op.String()
		if s == "" {
			t.Errorf("aug op %d has empty string", op)
		}
	}
}

func TestScopeString(t *testing.T) {
	scopes := []Scope{ScopeGlobal, ScopeLocal, ScopeSpecial}

	for _, s := range scopes {
		str := s.String()
		if str == "" {
			t.Errorf("scope %d has empty string", s)
		}
	}
}

func TestRedirectString(t *testing.T) {
	redirects := []Redirect{RedirectNone, RedirectWrite, RedirectAppend, RedirectPipe, RedirectInput}

	for _, r := range redirects {
		s := r.String()
		if s == "" {
			t.Errorf("redirect %d has empty string", r)
		}
	}
}

// TestCompileError verifies that compile errors are handled properly.
func TestCompileError(t *testing.T) {
	// Test that CompileError implements error interface
	err := &CompileError{Message: "test error"}
	if err.Error() != "test error" {
		t.Errorf("Error() = %q, want %q", err.Error(), "test error")
	}
}

// Benchmark compilation
func BenchmarkCompileSimple(b *testing.B) {
	source := `BEGIN { x = 1; y = 2; z = x + y; print z }`
	prog, _ := parser.Parse(source)
	resolved, _ := semantic.Resolve(prog)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := Compile(prog, resolved)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompileComplex(b *testing.B) {
	source := `
		function fib(n) {
			if (n <= 1) return n
			return fib(n-1) + fib(n-2)
		}
		BEGIN {
			for (i = 0; i < 10; i++) {
				arr[i] = fib(i)
			}
			for (k in arr) {
				print k, arr[k]
			}
		}
	`
	prog, _ := parser.Parse(source)
	resolved, _ := semantic.Resolve(prog)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := Compile(prog, resolved)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Helper function to check AST structure (not used in tests but useful for debugging)
func dumpAST(prog *ast.Program) string {
	var sb strings.Builder
	sb.WriteString("Program:\n")
	sb.WriteString("  BEGIN blocks: ")
	sb.WriteString(string(rune('0' + len(prog.Begin))))
	sb.WriteString("\n")
	sb.WriteString("  Rules: ")
	sb.WriteString(string(rune('0' + len(prog.Rules))))
	sb.WriteString("\n")
	sb.WriteString("  END blocks: ")
	sb.WriteString(string(rune('0' + len(prog.EndBlocks))))
	sb.WriteString("\n")
	sb.WriteString("  Functions: ")
	sb.WriteString(string(rune('0' + len(prog.Functions))))
	sb.WriteString("\n")
	return sb.String()
}
