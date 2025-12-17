package semantic

import (
	"strings"
	"testing"

	"github.com/kolkov/uawk/internal/parser"
	"github.com/kolkov/uawk/internal/token"
)

// Helper to parse and resolve
func resolveCode(t *testing.T, code string) (*ResolveResult, error) {
	t.Helper()
	prog, err := parser.Parse(code)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return Resolve(prog)
}

// Helper to check for expected error
func expectError(t *testing.T, code string, errSubstr string) {
	t.Helper()
	prog, err := parser.Parse(code)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := Resolve(prog)
	if err == nil {
		// Try checker too
		errs := Check(prog, result)
		if len(errs) == 0 {
			t.Errorf("expected error containing %q, got no error", errSubstr)
			return
		}
		for _, e := range errs {
			if strings.Contains(e.Error(), errSubstr) {
				return // Found expected error
			}
		}
		t.Errorf("expected error containing %q, got: %v", errSubstr, errs)
		return
	}
	if !strings.Contains(err.Error(), errSubstr) {
		t.Errorf("expected error containing %q, got: %v", errSubstr, err)
	}
}

// Helper to check no errors
func expectNoError(t *testing.T, code string) *ResolveResult {
	t.Helper()
	prog, err := parser.Parse(code)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := Resolve(prog)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	return result
}

func TestResolveGlobals(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		vars   []string // Expected global variables
		arrays []string // Expected arrays
	}{
		{
			name:   "simple assignment",
			code:   `BEGIN { x = 1 }`,
			vars:   []string{"x"},
			arrays: nil,
		},
		{
			name:   "multiple globals",
			code:   `BEGIN { x = 1; y = 2; z = 3 }`,
			vars:   []string{"x", "y", "z"},
			arrays: nil,
		},
		{
			name:   "array access",
			code:   `BEGIN { a[1] = "one"; a[2] = "two" }`,
			vars:   nil,
			arrays: []string{"a"},
		},
		{
			name:   "mixed scalar and array",
			code:   `BEGIN { x = 1; a[1] = 2; y = 3 }`,
			vars:   []string{"x", "y"},
			arrays: []string{"a"},
		},
		{
			name:   "global in rule",
			code:   `{ x = $1 }`,
			vars:   []string{"x"},
			arrays: nil,
		},
		{
			name:   "auto-create on read",
			code:   `BEGIN { print x }`,
			vars:   []string{"x"},
			arrays: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expectNoError(t, tt.code)

			// Check scalars
			scalars := []string{}
			result.Globals.ForEach(func(name string, sym *Symbol) {
				if sym.Kind == SymbolGlobal && sym.Type == TypeScalar {
					scalars = append(scalars, name)
				}
			})

			if len(scalars) != len(tt.vars) {
				t.Errorf("expected %d scalars %v, got %d: %v", len(tt.vars), tt.vars, len(scalars), scalars)
			}

			// Check arrays
			arrays := []string{}
			result.Globals.ForEach(func(name string, sym *Symbol) {
				if sym.Kind == SymbolGlobal && sym.Type == TypeArray {
					arrays = append(arrays, name)
				}
			})

			if len(arrays) != len(tt.arrays) {
				t.Errorf("expected %d arrays %v, got %d: %v", len(tt.arrays), tt.arrays, len(arrays), arrays)
			}
		})
	}
}

func TestResolveSpecials(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		special string
	}{
		{"NR", `{ print NR }`, "NR"},
		{"NF", `{ print NF }`, "NF"},
		{"FS", `BEGIN { FS = ":" }`, "FS"},
		{"RS", `BEGIN { RS = "\n\n" }`, "RS"},
		{"OFS", `BEGIN { OFS = "," }`, "OFS"},
		{"ORS", `BEGIN { ORS = "\n" }`, "ORS"},
		{"FILENAME", `{ print FILENAME }`, "FILENAME"},
		{"FNR", `{ print FNR }`, "FNR"},
		{"RSTART", `{ if (match($0, /x/)) print RSTART }`, "RSTART"},
		{"RLENGTH", `{ if (match($0, /x/)) print RLENGTH }`, "RLENGTH"},
		{"SUBSEP", `BEGIN { print SUBSEP }`, "SUBSEP"},
		{"CONVFMT", `BEGIN { print CONVFMT }`, "CONVFMT"},
		{"OFMT", `BEGIN { print OFMT }`, "OFMT"},
		{"ARGC", `BEGIN { print ARGC }`, "ARGC"},
		{"ARGV array", `BEGIN { print ARGV[0] }`, "ARGV"},
		{"ENVIRON array", `BEGIN { print ENVIRON["PATH"] }`, "ENVIRON"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expectNoError(t, tt.code)

			sym, found := result.Globals.LookupLocal(tt.special)
			if !found {
				t.Errorf("special variable %s not found", tt.special)
				return
			}
			if sym.Kind != SymbolSpecial {
				t.Errorf("expected %s to be SymbolSpecial, got %v", tt.special, sym.Kind)
			}
		})
	}
}

func TestResolveLocals(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		funcName string
		params   []string
	}{
		{
			name:     "single param",
			code:     `function f(x) { return x }`,
			funcName: "f",
			params:   []string{"x"},
		},
		{
			name:     "multiple params",
			code:     `function add(a, b) { return a + b }`,
			funcName: "add",
			params:   []string{"a", "b"},
		},
		{
			name:     "with local vars",
			code:     `function f(a, b, local1, local2) { local1 = a; local2 = b; return local1 + local2 }`,
			funcName: "f",
			params:   []string{"a", "b", "local1", "local2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expectNoError(t, tt.code)

			funcInfo, ok := result.Functions[tt.funcName]
			if !ok {
				t.Fatalf("function %s not found", tt.funcName)
			}

			if len(funcInfo.Params) != len(tt.params) {
				t.Errorf("expected %d params, got %d", len(tt.params), len(funcInfo.Params))
			}

			for i, param := range tt.params {
				if i >= len(funcInfo.Params) {
					break
				}
				if funcInfo.Params[i] != param {
					t.Errorf("param %d: expected %s, got %s", i, param, funcInfo.Params[i])
				}

				sym, found := funcInfo.Symbols.LookupLocal(param)
				if !found {
					t.Errorf("param %s not in symbol table", param)
				} else if sym.Kind != SymbolParam && sym.Kind != SymbolLocal {
					t.Errorf("param %s has wrong kind: %v", param, sym.Kind)
				}
			}
		})
	}
}

func TestResolveFunctions(t *testing.T) {
	tests := []struct {
		name  string
		code  string
		funcs []string
	}{
		{
			name:  "single function",
			code:  `function f() { }`,
			funcs: []string{"f"},
		},
		{
			name:  "multiple functions",
			code:  `function f() { } function g() { } function h() { }`,
			funcs: []string{"f", "g", "h"},
		},
		{
			name:  "recursive function",
			code:  `function fac(n) { return n <= 1 ? 1 : n * fac(n-1) }`,
			funcs: []string{"fac"},
		},
		{
			name: "mutual recursion",
			code: `function even(n) { return n == 0 ? 1 : odd(n-1) }
			       function odd(n) { return n == 0 ? 0 : even(n-1) }`,
			funcs: []string{"even", "odd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expectNoError(t, tt.code)

			if len(result.Functions) != len(tt.funcs) {
				t.Errorf("expected %d functions, got %d", len(tt.funcs), len(result.Functions))
			}

			for _, name := range tt.funcs {
				if _, ok := result.Functions[name]; !ok {
					t.Errorf("function %s not found", name)
				}
			}
		})
	}
}

func TestCheckBreakOutsideLoop(t *testing.T) {
	// Note: Parser already checks break/continue outside loop
	// These tests verify valid cases work
	tests := []struct {
		name string
		code string
	}{
		{
			name: "break in while",
			code: `BEGIN { while (1) { break } }`,
		},
		{
			name: "break in for",
			code: `BEGIN { for (i=0; i<10; i++) { break } }`,
		},
		{
			name: "break in do-while",
			code: `BEGIN { do { break } while (0) }`,
		},
		{
			name: "break in nested loop",
			code: `BEGIN { for (i=0; i<10; i++) { for (j=0; j<10; j++) { break } } }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectNoError(t, tt.code)
		})
	}
}

func TestCheckContinueOutsideLoop(t *testing.T) {
	// Note: Parser already checks continue outside loop
	// These tests verify valid cases work
	tests := []struct {
		name string
		code string
	}{
		{
			name: "continue in while",
			code: `BEGIN { while (1) { continue } }`,
		},
		{
			name: "continue in for",
			code: `BEGIN { for (i=0; i<10; i++) { continue } }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectNoError(t, tt.code)
		})
	}
}

func TestCheckReturnOutsideFunction(t *testing.T) {
	// Note: Parser already checks return outside function
	// These tests verify valid cases work
	tests := []struct {
		name string
		code string
	}{
		{
			name: "return in function",
			code: `function f() { return 1 }`,
		},
		{
			name: "bare return in function",
			code: `function f() { return }`,
		},
		{
			name: "return in nested if",
			code: `function f(x) { if (x) return 1; else return 0 }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectNoError(t, tt.code)
		})
	}
}

func TestCheckUndefinedFunction(t *testing.T) {
	tests := []struct {
		name      string
		code      string
		shouldErr bool
	}{
		{
			name:      "call defined function",
			code:      `function f() { } BEGIN { f() }`,
			shouldErr: false,
		},
		{
			name:      "call undefined function",
			code:      `BEGIN { undefined_func() }`,
			shouldErr: true,
		},
		{
			name:      "forward reference",
			code:      `BEGIN { f() } function f() { }`,
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldErr {
				expectError(t, tt.code, "undefined function")
			} else {
				expectNoError(t, tt.code)
			}
		})
	}
}

func TestCheckDuplicateFunction(t *testing.T) {
	expectError(t, `function f() { } function f() { }`, "already defined")
}

func TestCheckDuplicateParam(t *testing.T) {
	// Note: Parser already checks duplicate parameters
	// This test verifies unique params work
	expectNoError(t, `function f(a, b, c) { return a + b + c }`)
}

func TestTypeInference(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		varName  string
		expected VarType
	}{
		{
			name:     "scalar from assignment",
			code:     `BEGIN { x = 1 }`,
			varName:  "x",
			expected: TypeScalar,
		},
		{
			name:     "array from index",
			code:     `BEGIN { a[1] = 1 }`,
			varName:  "a",
			expected: TypeArray,
		},
		{
			name:     "array from for-in",
			code:     `BEGIN { for (k in a) print k }`,
			varName:  "a",
			expected: TypeArray,
		},
		{
			name:     "array from in-expr",
			code:     `BEGIN { if (1 in a) print "yes" }`,
			varName:  "a",
			expected: TypeArray,
		},
		{
			name:     "array from delete",
			code:     `BEGIN { delete a[1] }`,
			varName:  "a",
			expected: TypeArray,
		},
		{
			name:     "array from split",
			code:     `BEGIN { split("a:b:c", arr, ":") }`,
			varName:  "arr",
			expected: TypeArray,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expectNoError(t, tt.code)

			sym, found := result.Globals.LookupLocal(tt.varName)
			if !found {
				t.Fatalf("variable %s not found", tt.varName)
			}
			if sym.Type != tt.expected {
				t.Errorf("expected %s to be %v, got %v", tt.varName, tt.expected, sym.Type)
			}
		})
	}
}

func TestFunctionArgumentTypeInference(t *testing.T) {
	// Test that array types are properly inferred
	// Note: type inference across function calls is complex
	code := `
		function process_array(arr) {
			for (k in arr) print arr[k]
		}
		BEGIN {
			a[1] = "one"
			a[2] = "two"
		}
	`

	result := expectNoError(t, code)

	// Check that 'a' is inferred as array
	sym, found := result.Globals.LookupLocal("a")
	if !found {
		t.Fatal("variable 'a' not found")
	}
	if sym.Type != TypeArray {
		t.Errorf("expected 'a' to be array, got %v", sym.Type)
	}

	// Check that parameter 'arr' is inferred as array (from for-in usage)
	funcInfo, ok := result.Functions["process_array"]
	if !ok {
		t.Fatal("function process_array not found")
	}
	paramSym, found := funcInfo.Symbols.LookupLocal("arr")
	if !found {
		t.Fatal("parameter 'arr' not found")
	}
	if paramSym.Type != TypeArray {
		t.Errorf("expected param 'arr' to be array, got %v", paramSym.Type)
	}
}

func TestSymbolIndices(t *testing.T) {
	code := `
		BEGIN {
			a = 1
			b = 2
			c[1] = 3
			d = 4
		}
	`

	result := expectNoError(t, code)

	// Scalars should have sequential indices
	scalars := map[string]int{}
	arrays := map[string]int{}

	result.Globals.ForEach(func(name string, sym *Symbol) {
		if sym.Kind == SymbolGlobal {
			if sym.Type == TypeScalar {
				scalars[name] = sym.Index
			} else if sym.Type == TypeArray {
				arrays[name] = sym.Index
			}
		}
	})

	// Check that indices are assigned
	for name, idx := range scalars {
		if idx < 0 {
			t.Errorf("scalar %s has invalid index %d", name, idx)
		}
	}
	for name, idx := range arrays {
		if idx < 0 {
			t.Errorf("array %s has invalid index %d", name, idx)
		}
	}

	// Check that indices are unique within category
	seenScalar := make(map[int]string)
	for name, idx := range scalars {
		if prev, exists := seenScalar[idx]; exists {
			t.Errorf("scalars %s and %s have same index %d", prev, name, idx)
		}
		seenScalar[idx] = name
	}

	seenArray := make(map[int]string)
	for name, idx := range arrays {
		if prev, exists := seenArray[idx]; exists {
			t.Errorf("arrays %s and %s have same index %d", prev, name, idx)
		}
		seenArray[idx] = name
	}
}

func TestNextInBeginEnd(t *testing.T) {
	// Note: Parser already checks next/nextfile in BEGIN/END
	// These tests verify valid cases work
	tests := []struct {
		name string
		code string
	}{
		{
			name: "next in rule",
			code: `{ next }`,
		},
		{
			name: "nextfile in rule",
			code: `{ nextfile }`,
		},
		{
			name: "next in pattern-action",
			code: `/pattern/ { next }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectNoError(t, tt.code)
		})
	}
}

func TestIsSpecialVar(t *testing.T) {
	specials := []string{
		"NR", "NF", "FS", "RS", "OFS", "ORS", "FILENAME", "FNR",
		"RSTART", "RLENGTH", "SUBSEP", "CONVFMT", "OFMT", "ARGC", "ARGV", "ENVIRON",
	}

	for _, name := range specials {
		if !IsSpecialVar(name) {
			t.Errorf("expected %s to be special variable", name)
		}
	}

	nonSpecials := []string{"x", "y", "foo", "bar", "NRX", "ANF"}
	for _, name := range nonSpecials {
		if IsSpecialVar(name) {
			t.Errorf("expected %s to NOT be special variable", name)
		}
	}
}

func TestSpecialVarIndex(t *testing.T) {
	// Check that all specials have positive indices
	for name := range specialVars {
		idx := SpecialVarIndex(name)
		if idx <= 0 {
			t.Errorf("special %s has non-positive index %d", name, idx)
		}
	}

	// Non-special should return -1
	if idx := SpecialVarIndex("x"); idx != -1 {
		t.Errorf("non-special 'x' should return -1, got %d", idx)
	}
}

func TestIsBuiltinFunc(t *testing.T) {
	builtins := []string{
		"length", "substr", "index", "split", "sub", "gsub", "match", "sprintf",
		"tolower", "toupper", "sin", "cos", "atan2", "exp", "log", "sqrt", "int",
		"rand", "srand", "close", "fflush", "system",
	}

	for _, name := range builtins {
		if !IsBuiltinFunc(name) {
			t.Errorf("expected %s to be builtin function", name)
		}
	}

	nonBuiltins := []string{"print", "printf", "myfunction", "foo"}
	for _, name := range nonBuiltins {
		if IsBuiltinFunc(name) {
			t.Errorf("expected %s to NOT be builtin function", name)
		}
	}
}

// Benchmark symbol table operations
func BenchmarkSymbolTableLookup(b *testing.B) {
	st := NewSymbolTable(nil, "global")
	for i := 0; i < 100; i++ {
		st.Define("var"+string(rune('a'+i%26))+string(rune('0'+i/26)), SymbolGlobal, token.Position{})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st.Lookup("varz3")
	}
}

func BenchmarkResolveSmallProgram(b *testing.B) {
	code := `BEGIN { x = 1; y = 2; print x + y }`
	prog, _ := parser.Parse(code)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Resolve(prog)
	}
}

func BenchmarkResolveLargeProgram(b *testing.B) {
	code := `
		function fib(n) { return n <= 1 ? n : fib(n-1) + fib(n-2) }
		function fac(n) { return n <= 1 ? 1 : n * fac(n-1) }
		function max(a, b) { return a > b ? a : b }
		function min(a, b) { return a < b ? a : b }
		BEGIN {
			for (i = 1; i <= 20; i++) {
				f = fib(i)
				g = fac(i)
				print i, f, g, max(f, g), min(f, g)
			}
		}
		{
			split($0, fields, ":")
			for (k in fields) {
				gsub(/[^a-z]/, "", fields[k])
				if (length(fields[k]) > 0) {
					words[fields[k]]++
				}
			}
		}
		END {
			for (w in words) {
				printf "%s: %d\n", w, words[w]
			}
		}
	`
	prog, _ := parser.Parse(code)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Resolve(prog)
	}
}
