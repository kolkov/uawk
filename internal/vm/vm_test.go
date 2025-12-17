package vm

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kolkov/uawk/internal/compiler"
	"github.com/kolkov/uawk/internal/parser"
	"github.com/kolkov/uawk/internal/semantic"
	"github.com/kolkov/uawk/internal/types"
)

// Helper to run an AWK program and return output.
func runAWK(t *testing.T, source, input string) string {
	t.Helper()

	prog, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	resolved, err := semantic.Resolve(prog)
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}

	compiled, err := compiler.Compile(prog, resolved)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	vm := New(compiled)

	if input != "" {
		vm.SetInput(strings.NewReader(input))
	}

	var output bytes.Buffer
	vm.SetOutput(&output)

	if err := vm.Run(); err != nil {
		if _, ok := err.(*ExitError); !ok {
			t.Fatalf("run error: %v", err)
		}
	}

	return output.String()
}

// TestInlineStackBasics tests the inline stack operations.
// These operations are now part of the VM struct for performance.
func TestInlineStackBasics(t *testing.T) {
	// Create a minimal compiled program for VM initialization
	prog := &compiler.Program{
		NumScalars: 0,
		NumArrays:  0,
	}
	vm := New(prog)

	// Test push/pop
	vm.push(types.Num(1))
	vm.push(types.Num(2))
	vm.push(types.Num(3))

	if vm.sp != 3 {
		t.Errorf("sp = %d, want 3", vm.sp)
	}

	v := vm.pop()
	if v.AsNum() != 3 {
		t.Errorf("pop() = %v, want 3", v.AsNum())
	}

	// Test peek
	v = vm.peek()
	if v.AsNum() != 2 {
		t.Errorf("peek() = %v, want 2", v.AsNum())
	}

	// Test dup
	vm.dup()
	if vm.sp != 3 {
		t.Errorf("sp after dup = %d, want 3", vm.sp)
	}

	// Test swap
	vm.push(types.Num(10))
	vm.swap()
	if vm.pop().AsNum() != 2 {
		t.Error("swap failed")
	}
}

// TestInlineStackRote tests the rote operation.
func TestInlineStackRote(t *testing.T) {
	prog := &compiler.Program{
		NumScalars: 0,
		NumArrays:  0,
	}
	vm := New(prog)

	vm.push(types.Num(1)) // a
	vm.push(types.Num(2)) // b
	vm.push(types.Num(3)) // c (top)

	vm.rote()

	// After rote: [b, c, a] where a is on top (GoAWK compatible)
	// [1, 2, 3] -> [2, 3, 1]
	if vm.pop().AsNum() != 1 {
		t.Error("rote: expected 1 on top")
	}
	if vm.pop().AsNum() != 3 {
		t.Error("rote: expected 3 second")
	}
	if vm.pop().AsNum() != 2 {
		t.Error("rote: expected 2 third")
	}
}

func TestVMBeginEnd(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "simple begin",
			source: `BEGIN { print "hello" }`,
			want:   "hello\n",
		},
		{
			name:   "begin with variable",
			source: `BEGIN { x = 42; print x }`,
			want:   "42\n",
		},
		{
			name:   "begin and end",
			source: `BEGIN { print "start" } END { print "end" }`,
			want:   "start\nend\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, "")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMArithmetic(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"add", "BEGIN { print 1 + 2 }", "3\n"},
		{"subtract", "BEGIN { print 5 - 3 }", "2\n"},
		{"multiply", "BEGIN { print 4 * 5 }", "20\n"},
		{"divide", "BEGIN { print 10 / 2 }", "5\n"},
		{"modulo", "BEGIN { print 10 % 3 }", "1\n"},
		{"power", "BEGIN { print 2 ^ 3 }", "8\n"},
		{"unary minus", "BEGIN { print -5 }", "-5\n"},
		{"complex", "BEGIN { print (1 + 2) * 3 }", "9\n"},
		{"precedence", "BEGIN { print 1 + 2 * 3 }", "7\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, "")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMComparison(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"equal true", "BEGIN { print 1 == 1 }", "1\n"},
		{"equal false", "BEGIN { print 1 == 2 }", "0\n"},
		{"not equal true", "BEGIN { print 1 != 2 }", "1\n"},
		{"less true", "BEGIN { print 1 < 2 }", "1\n"},
		{"less false", "BEGIN { print 2 < 1 }", "0\n"},
		{"greater true", "BEGIN { print (2 > 1) }", "1\n"},
		{"string equal", `BEGIN { print "a" == "a" }`, "1\n"},
		{"string less", `BEGIN { print "a" < "b" }`, "1\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, "")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMStrings(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"literal", `BEGIN { print "hello" }`, "hello\n"},
		{"concat", `BEGIN { print "hello" " " "world" }`, "hello world\n"},
		{"variable", `BEGIN { x = "test"; print x }`, "test\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, "")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMControlFlow(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "if true",
			source: `BEGIN { if (1) print "yes" }`,
			want:   "yes\n",
		},
		{
			name:   "if false",
			source: `BEGIN { if (0) print "yes" }`,
			want:   "",
		},
		{
			name:   "if else true",
			source: `BEGIN { if (1) print "yes"; else print "no" }`,
			want:   "yes\n",
		},
		{
			name:   "if else false",
			source: `BEGIN { if (0) print "yes"; else print "no" }`,
			want:   "no\n",
		},
		{
			name:   "while",
			source: `BEGIN { i = 0; while (i < 3) { print i; i++ } }`,
			want:   "0\n1\n2\n",
		},
		{
			name:   "for",
			source: `BEGIN { for (i = 0; i < 3; i++) print i }`,
			want:   "0\n1\n2\n",
		},
		{
			name:   "do while",
			source: `BEGIN { i = 0; do { print i; i++ } while (i < 3) }`,
			want:   "0\n1\n2\n",
		},
		{
			name:   "break",
			source: `BEGIN { for (i = 0; i < 10; i++) { if (i == 3) break; print i } }`,
			want:   "0\n1\n2\n",
		},
		{
			name:   "continue",
			source: `BEGIN { for (i = 0; i < 5; i++) { if (i == 2) continue; print i } }`,
			want:   "0\n1\n3\n4\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, "")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMIncrDecr(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"pre incr", "BEGIN { x = 5; print ++x }", "6\n"},
		{"post incr", "BEGIN { x = 5; print x++ }", "5\n"},
		{"pre decr", "BEGIN { x = 5; print --x }", "4\n"},
		{"post decr", "BEGIN { x = 5; print x-- }", "5\n"},
		{"incr stmt", "BEGIN { x = 5; x++; print x }", "6\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, "")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMAugmentedAssign(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"add assign", "BEGIN { x = 5; x += 3; print x }", "8\n"},
		{"sub assign", "BEGIN { x = 5; x -= 3; print x }", "2\n"},
		{"mul assign", "BEGIN { x = 5; x *= 3; print x }", "15\n"},
		{"div assign", "BEGIN { x = 6; x /= 2; print x }", "3\n"},
		{"mod assign", "BEGIN { x = 7; x %= 3; print x }", "1\n"},
		{"pow assign", "BEGIN { x = 2; x ^= 3; print x }", "8\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, "")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMArrays(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "set and get",
			source: `BEGIN { arr[1] = "a"; print arr[1] }`,
			want:   "a\n",
		},
		{
			name:   "string key",
			source: `BEGIN { arr["key"] = "value"; print arr["key"] }`,
			want:   "value\n",
		},
		{
			name:   "in operator true",
			source: `BEGIN { arr[1] = "x"; print (1 in arr) }`,
			want:   "1\n",
		},
		{
			name:   "in operator false",
			source: `BEGIN { arr[1] = "x"; print (2 in arr) }`,
			want:   "0\n",
		},
		{
			name:   "delete",
			source: `BEGIN { arr[1] = "x"; delete arr[1]; print (1 in arr) }`,
			want:   "0\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, "")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMFields(t *testing.T) {
	tests := []struct {
		name   string
		source string
		input  string
		want   string
	}{
		{
			name:   "field 0",
			source: "{ print $0 }",
			input:  "hello world\n",
			want:   "hello world\n",
		},
		{
			name:   "field 1",
			source: "{ print $1 }",
			input:  "hello world\n",
			want:   "hello\n",
		},
		{
			name:   "field 2",
			source: "{ print $2 }",
			input:  "hello world\n",
			want:   "world\n",
		},
		{
			name:   "NF",
			source: "{ print NF }",
			input:  "a b c\n",
			want:   "3\n",
		},
		{
			name:   "field NF",
			source: "{ print $NF }",
			input:  "a b c\n",
			want:   "c\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMPatterns(t *testing.T) {
	tests := []struct {
		name   string
		source string
		input  string
		want   string
	}{
		{
			name:   "no pattern",
			source: "{ print $1 }",
			input:  "a\nb\nc\n",
			want:   "a\nb\nc\n",
		},
		{
			name:   "NR pattern",
			source: "NR == 2 { print $0 }",
			input:  "a\nb\nc\n",
			want:   "b\n",
		},
		{
			name:   "regex pattern",
			source: "/b/ { print $0 }",
			input:  "a\nb\nc\n",
			want:   "b\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMFunctions(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "simple function",
			source: `function add(a, b) { return a + b } BEGIN { print add(1, 2) }`,
			want:   "3\n",
		},
		{
			name:   "recursive function",
			source: `function fac(n) { if (n <= 1) return 1; return n * fac(n-1) } BEGIN { print fac(5) }`,
			want:   "120\n",
		},
		{
			name:   "local variables",
			source: `function f(a,    local) { local = a * 2; return local } BEGIN { print f(5) }`,
			want:   "10\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, "")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMBuiltins(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"length string", `BEGIN { print length("hello") }`, "5\n"},
		{"length empty", `BEGIN { print length("") }`, "0\n"},
		{"substr 2 arg", `BEGIN { print substr("hello", 2) }`, "ello\n"},
		{"substr 3 arg", `BEGIN { print substr("hello", 2, 3) }`, "ell\n"},
		{"index found", `BEGIN { print index("hello", "ll") }`, "3\n"},
		{"index not found", `BEGIN { print index("hello", "x") }`, "0\n"},
		{"tolower", `BEGIN { print tolower("HELLO") }`, "hello\n"},
		{"toupper", `BEGIN { print toupper("hello") }`, "HELLO\n"},
		{"int", `BEGIN { print int(3.7) }`, "3\n"},
		{"int negative", `BEGIN { print int(-3.7) }`, "-3\n"},
		{"sqrt", `BEGIN { print sqrt(4) }`, "2\n"},
		{"sin 0", `BEGIN { print sin(0) }`, "0\n"},
		{"cos 0", `BEGIN { print cos(0) }`, "1\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, "")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMSplit(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "default separator",
			source: `BEGIN { n = split("a b c", arr); print n, arr[1], arr[2], arr[3] }`,
			want:   "3 a b c\n",
		},
		{
			name:   "custom separator",
			source: `BEGIN { n = split("a:b:c", arr, ":"); print n, arr[1], arr[2], arr[3] }`,
			want:   "3 a b c\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, "")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMSprintf(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"integer", `BEGIN { print sprintf("%d", 42) }`, "42\n"},
		{"float", `BEGIN { print sprintf("%.2f", 3.14159) }`, "3.14\n"},
		{"string", `BEGIN { print sprintf("%s", "hello") }`, "hello\n"},
		{"multiple", `BEGIN { print sprintf("%s=%d", "x", 42) }`, "x=42\n"},
		{"padding", `BEGIN { print sprintf("%5d", 42) }`, "   42\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, "")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMTernary(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"true", "BEGIN { print 1 ? 2 : 3 }", "2\n"},
		{"false", "BEGIN { print 0 ? 2 : 3 }", "3\n"},
		{"nested", "BEGIN { print 1 ? (0 ? 2 : 3) : 4 }", "3\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, "")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMLogical(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"and true", "BEGIN { print 1 && 1 }", "1\n"},
		{"and false", "BEGIN { print 1 && 0 }", "0\n"},
		{"or true", "BEGIN { print 0 || 1 }", "1\n"},
		{"or false", "BEGIN { print 0 || 0 }", "0\n"},
		{"not true", "BEGIN { print !0 }", "1\n"},
		{"not false", "BEGIN { print !1 }", "0\n"},
		{"short circuit and", "BEGIN { x = 0; 0 && (x = 1); print x }", "0\n"},
		{"short circuit or", "BEGIN { x = 0; 1 || (x = 1); print x }", "0\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, "")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMNext(t *testing.T) {
	got := runAWK(t, "{ if (NR == 2) next; print }", "a\nb\nc\n")
	want := "a\nc\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestVMExit(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "exit in begin",
			source: `BEGIN { print "a"; exit; print "b" }`,
			want:   "a\n",
		},
		{
			name:   "exit runs end",
			source: `BEGIN { exit } END { print "end" }`,
			want:   "end\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, "")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMSpecialVars(t *testing.T) {
	tests := []struct {
		name   string
		source string
		input  string
		want   string
	}{
		{
			name:   "NR",
			source: "{ print NR }",
			input:  "a\nb\nc\n",
			want:   "1\n2\n3\n",
		},
		{
			name:   "NF",
			source: "{ print NF }",
			input:  "a b\nc d e\n",
			want:   "2\n3\n",
		},
		{
			name:   "custom OFS",
			source: `BEGIN { OFS = "," } { print $1, $2 }`,
			input:  "a b\n",
			want:   "a,b\n",
		},
		{
			name:   "custom ORS",
			source: `BEGIN { ORS = ";" } { print $1 }`,
			input:  "a\nb\n",
			want:   "a;b;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMRegexMatch(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"match true", `BEGIN { print ("hello" ~ /ell/) }`, "1\n"},
		{"match false", `BEGIN { print ("hello" ~ /xyz/) }`, "0\n"},
		{"not match true", `BEGIN { print ("hello" !~ /xyz/) }`, "1\n"},
		{"not match false", `BEGIN { print ("hello" !~ /ell/) }`, "0\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, "")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestVMSubGsub tests sub/gsub functions
func TestVMSubGsub(t *testing.T) {
	tests := []struct {
		name   string
		source string
		input  string
		want   string
	}{
		// sub with explicit variable target
		{"sub_basic", `BEGIN { x = "hello"; sub(/l/, "L", x); print x }`, "", "heLlo\n"},
		{"sub_count", `BEGIN { x = "hello"; print sub(/l/, "L", x) }`, "", "1\n"},
		{"sub_no_match", `BEGIN { x = "hello"; print sub(/z/, "Z", x) }`, "", "0\n"},

		// gsub with explicit variable target
		{"gsub_basic", `BEGIN { x = "hello"; gsub(/l/, "L", x); print x }`, "", "heLLo\n"},
		{"gsub_count", `BEGIN { x = "hello"; print gsub(/l/, "L", x) }`, "", "2\n"},
		{"gsub_no_match", `BEGIN { x = "hello"; print gsub(/z/, "Z", x) }`, "", "0\n"},

		// sub/gsub with $0 (default target)
		{"sub_$0", `{ sub(/l/, "L"); print }`, "hello", "heLlo\n"},
		{"gsub_$0", `{ gsub(/l/, "L"); print }`, "hello", "heLLo\n"},

		// sub/gsub with & replacement
		{"sub_ampersand", `BEGIN { x = "hello"; sub(/e/, "[&]", x); print x }`, "", "h[e]llo\n"},
		{"gsub_ampersand", `{ gsub(/[0-9]/, "(&)"); print }`, "a1b2c3", "a(1)b(2)c(3)\n"},

		// sub/gsub with field target
		{"sub_field", `{ sub(/o/, "0", $1); print }`, "foo bar", "f0o bar\n"},
		{"gsub_field", `{ gsub(/o/, "0", $2); print }`, "foo boo", "foo b00\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, tt.input)
			if got != tt.want {
				t.Errorf("\nsource: %s\ninput: %q\ngot:  %q\nwant: %q", tt.source, tt.input, got, tt.want)
			}
		})
	}
}

func TestVMMatchFunction(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"match found", `BEGIN { print match("hello", /ell/) }`, "2\n"},
		{"match not found", `BEGIN { print match("hello", /xyz/) }`, "0\n"},
		{"match RSTART", `BEGIN { match("hello", /ell/); print RSTART }`, "2\n"},
		{"match RLENGTH", `BEGIN { match("hello", /ell/); print RLENGTH }`, "3\n"},
		{"match at start", `BEGIN { print match("hello", /^h/) }`, "1\n"},
		{"match at end", `BEGIN { match("hello", /o$/); print RSTART, RLENGTH }`, "5 1\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, "")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMMathBuiltins(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		// atan2
		{"atan2", `BEGIN { printf "%.4f\n", atan2(1, 1) }`, "0.7854\n"},
		{"atan2 zero", `BEGIN { printf "%.4f\n", atan2(0, 1) }`, "0.0000\n"},

		// exp and log
		{"exp 0", `BEGIN { print exp(0) }`, "1\n"},
		{"exp 1", `BEGIN { printf "%.4f\n", exp(1) }`, "2.7183\n"},
		{"log 1", `BEGIN { print log(1) }`, "0\n"},
		{"log e", `BEGIN { printf "%.4f\n", log(2.7182818) }`, "1.0000\n"},

		// sqrt (already tested but add more)
		{"sqrt 9", `BEGIN { print sqrt(9) }`, "3\n"},
		{"sqrt 2", `BEGIN { printf "%.4f\n", sqrt(2) }`, "1.4142\n"},

		// int
		{"int positive", `BEGIN { print int(3.9) }`, "3\n"},
		{"int negative", `BEGIN { print int(-3.9) }`, "-3\n"},
		{"int zero", `BEGIN { print int(0.5) }`, "0\n"},

		// sin and cos
		{"sin pi/2", `BEGIN { printf "%.4f\n", sin(3.14159265/2) }`, "1.0000\n"},
		{"cos pi", `BEGIN { printf "%.4f\n", cos(3.14159265) }`, "-1.0000\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runAWK(t, tt.source, "")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMRandSrand(t *testing.T) {
	// Test that srand with same seed gives same results
	source := `BEGIN { srand(42); r1 = rand(); srand(42); r2 = rand(); print (r1 == r2) }`
	got := runAWK(t, source, "")
	if got != "1\n" {
		t.Errorf("srand reproducibility: got %q, want %q", got, "1\n")
	}

	// Test that rand returns values in [0, 1)
	source = `BEGIN { srand(); for (i=0; i<10; i++) { r = rand(); if (r < 0 || r >= 1) print "BAD" } print "OK" }`
	got = runAWK(t, source, "")
	if got != "OK\n" {
		t.Errorf("rand range: got %q, want %q", got, "OK\n")
	}
}

// Benchmark VM execution
func BenchmarkVMSimple(b *testing.B) {
	source := `BEGIN { x = 0; for (i = 0; i < 1000; i++) x += i; print x }`
	prog, _ := parser.Parse(source)
	resolved, _ := semantic.Resolve(prog)
	compiled, _ := compiler.Compile(prog, resolved)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		vm := New(compiled)
		var buf bytes.Buffer
		vm.SetOutput(&buf)
		vm.Run()
	}
}

func BenchmarkVMFunctionCall(b *testing.B) {
	source := `
		function fib(n) { if (n <= 1) return n; return fib(n-1) + fib(n-2) }
		BEGIN { print fib(20) }
	`
	prog, _ := parser.Parse(source)
	resolved, _ := semantic.Resolve(prog)
	compiled, _ := compiler.Compile(prog, resolved)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		vm := New(compiled)
		var buf bytes.Buffer
		vm.SetOutput(&buf)
		vm.Run()
	}
}

func BenchmarkVMFieldAccess(b *testing.B) {
	source := `{ sum += $1 } END { print sum }`
	prog, _ := parser.Parse(source)
	resolved, _ := semantic.Resolve(prog)
	compiled, _ := compiler.Compile(prog, resolved)

	// Create input with 1000 lines
	var input strings.Builder
	for i := 0; i < 1000; i++ {
		input.WriteString("1\n")
	}
	inputStr := input.String()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		vm := New(compiled)
		vm.SetInput(strings.NewReader(inputStr))
		var buf bytes.Buffer
		vm.SetOutput(&buf)
		vm.Run()
	}
}

func BenchmarkVMSelectFields(b *testing.B) {
	// Pattern from select.awk: { print $1, $3, $5 }
	source := `{ print $1, $3, $5 }`
	prog, _ := parser.Parse(source)
	resolved, _ := semantic.Resolve(prog)
	compiled, _ := compiler.Compile(prog, resolved)

	// Create input with 10000 lines, 10 fields each
	var input strings.Builder
	for i := 0; i < 10000; i++ {
		input.WriteString("f1 f2 f3 f4 f5 f6 f7 f8 f9 f10\n")
	}
	inputStr := input.String()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		vm := New(compiled)
		vm.SetInput(strings.NewReader(inputStr))
		var buf bytes.Buffer
		vm.SetOutput(&buf)
		vm.Run()
	}
}

func BenchmarkVMNoFieldAccess(b *testing.B) {
	// Pattern that doesn't access fields - shows benefit of lazy splitting
	source := `{ n++ } END { print n }`
	prog, _ := parser.Parse(source)
	resolved, _ := semantic.Resolve(prog)
	compiled, _ := compiler.Compile(prog, resolved)

	// Create input with 10000 lines, 10 fields each
	var input strings.Builder
	for i := 0; i < 10000; i++ {
		input.WriteString("f1 f2 f3 f4 f5 f6 f7 f8 f9 f10\n")
	}
	inputStr := input.String()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		vm := New(compiled)
		vm.SetInput(strings.NewReader(inputStr))
		var buf bytes.Buffer
		vm.SetOutput(&buf)
		vm.Run()
	}
}
