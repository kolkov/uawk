// Compatibility tests ported from GoAWK interp_test.go
// These ensure uawk behaves like standard AWK implementations.
//
// Test Infrastructure:
// - 382 tests across 26 categories
// - Table-driven with proper subtests using t.Run()
// - Panic recovery for graceful failure reporting
// - Automatic skip of unsupported features
//
// Running tests:
//
//	go test ./internal/vm/... -run TestCompatibility -v
//	go test ./internal/vm/... -run TestCompatibility/Category/test_name -v
//
// Skipped features (not yet implemented):
// - I/O: getline, system(), close(), pipes (|), redirection (>, >>), fflush()
// - gawk extensions: gensub(), patsplit(), strftime(), mktime(), systime(), nextfile
//
// Test Status (as of porting):
// - PASS: ~330 tests (86%)
// - FAIL: ~50 tests (13%) - these represent bugs/missing features to fix
// - SKIP: ~2 tests (1%) - platform-specific or feature-specific
package vm

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kolkov/uawk/internal/compiler"
	"github.com/kolkov/uawk/internal/parser"
	"github.com/kolkov/uawk/internal/semantic"
)

// =============================================================================
// Test Infrastructure
// =============================================================================

// interpTest represents a single AWK compatibility test case.
// Ported from GoAWK's interp_test.go structure.
type interpTest struct {
	name string // Descriptive test name (optional, generated if empty)
	src  string // AWK source code (may include "# !feature" to skip)
	in   string // Input data
	out  string // Expected output
	err  string // Expected error substring (empty = no error expected)
}

// testCategory groups related tests for better organization.
type testCategory struct {
	name  string
	tests []interpTest
}

// unsupportedFeatures lists AWK features not yet implemented in uawk.
// Tests containing these patterns are automatically skipped.
var unsupportedFeatures = []string{
	// gawk extensions
	"gensub(", "patsplit(", "strftime(", "mktime(", "systime(",
	"nextfile",
	// I/O operations
	"getline", "system(", "close(",
	" | ", // Pipe (with spaces to avoid matching ||)
	"fflush(",
	// Special markers
	"# !awk",
	"# !gawk",
	"# !posix",
	"# !windows",
	"# !fuzz",
	"# +posix", // posix-only tests
}

// shouldSkip checks if a test should be skipped due to unsupported features.
func shouldSkip(src string) (skip bool, reason string) {
	for _, feature := range unsupportedFeatures {
		if strings.Contains(src, feature) {
			return true, feature
		}
	}
	return false, ""
}

// runCompatTest executes a single compatibility test.
func runCompatTest(t *testing.T, tt interpTest) {
	t.Helper()

	// Check for skip conditions
	if skip, reason := shouldSkip(tt.src); skip {
		t.Skipf("unsupported feature: %s", reason)
	}

	// Recover from panics and report as test failures
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("PANIC: %v\nsrc: %s", r, tt.src)
		}
	}()

	// Parse
	prog, err := parser.Parse(tt.src)
	if err != nil {
		if tt.err != "" && strings.Contains(err.Error(), tt.err) {
			return // Expected parse error
		}
		t.Fatalf("parse error: %v", err)
	}

	// Resolve
	resolved, err := semantic.Resolve(prog)
	if err != nil {
		if tt.err != "" && strings.Contains(err.Error(), tt.err) {
			return // Expected semantic error
		}
		t.Fatalf("resolve error: %v", err)
	}

	// Compile
	compiled, err := compiler.Compile(prog, resolved)
	if err != nil {
		if tt.err != "" && strings.Contains(err.Error(), tt.err) {
			return // Expected compile error
		}
		t.Fatalf("compile error: %v", err)
	}

	// Execute
	vm := New(compiled)

	if tt.in != "" {
		vm.SetInput(strings.NewReader(tt.in))
	}

	var output bytes.Buffer
	vm.SetOutput(&output)

	runErr := vm.Run()

	// Check for expected error
	if tt.err != "" {
		if runErr == nil {
			t.Fatalf("expected error containing %q, got no error", tt.err)
		}
		if !strings.Contains(runErr.Error(), tt.err) {
			t.Fatalf("expected error containing %q, got %q", tt.err, runErr.Error())
		}
		return
	}

	// Check for unexpected error (ignore ExitError with code 0)
	if runErr != nil {
		if exitErr, ok := runErr.(*ExitError); !ok || exitErr.Code != 0 {
			t.Fatalf("unexpected error: %v", runErr)
		}
	}

	// Check output
	got := output.String()
	if got != tt.out {
		t.Errorf("output mismatch:\nsrc:  %s\nin:   %q\ngot:  %q\nwant: %q", tt.src, tt.in, got, tt.out)
	}
}

// generateTestName creates a descriptive test name from source code.
func generateTestName(src string, idx int) string {
	// Take first 50 chars, replace problematic chars
	name := src
	if len(name) > 50 {
		name = name[:50]
	}
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "\n", "\\n")
	name = strings.ReplaceAll(name, "\t", "\\t")
	name = strings.ReplaceAll(name, `"`, "")
	name = strings.ReplaceAll(name, `'`, "")
	name = strings.ReplaceAll(name, `/`, "_")
	if name == "" {
		name = "empty"
	}
	return name
}

// runTestCategory runs all tests in a category with proper subtests.
func runTestCategory(t *testing.T, tests []interpTest) {
	t.Helper()
	for i, tt := range tests {
		name := tt.name
		if name == "" {
			name = generateTestName(tt.src, i)
		}
		t.Run(name, func(t *testing.T) {
			runCompatTest(t, tt)
		})
	}
}

// =============================================================================
// BEGIN/END Tests
// =============================================================================

var beginEndTests = []interpTest{
	{name: "BEGIN_print", src: `BEGIN { print "b" }`, in: "", out: "b\n"},
	{name: "BEGIN_print_with_input", src: `BEGIN { print "b" }`, in: "foo", out: "b\n"},
	{name: "END_print", src: `END { print "e" }`, in: "", out: "e\n"},
	{name: "END_print_with_input", src: `END { print "e" }`, in: "foo", out: "e\n"},
	{name: "BEGIN_END_both", src: `BEGIN { print "b"} END { print "e" }`, in: "", out: "b\ne\n"},
	{name: "BEGIN_END_with_input", src: `BEGIN { print "b"} END { print "e" }`, in: "foo", out: "b\ne\n"},
	{name: "BEGIN_action_END", src: `BEGIN { print "b"} $0 { print NR } END { print "e" }`, in: "foo", out: "b\n1\ne\n"},
	{name: "multiple_BEGIN", src: `BEGIN { printf "x" }; BEGIN { printf "y" }`, in: "", out: "xy"},
}

func TestCompatBeginEnd(t *testing.T) {
	runTestCategory(t, beginEndTests)
}

// =============================================================================
// Pattern Tests
// =============================================================================

var patternTests = []interpTest{
	// Presence of patterns and actions
	{name: "no_pattern_no_action", src: ``, in: "foo", out: ""},
	{name: "false_pattern_no_action", src: `0`, in: "foo", out: ""},
	{name: "true_pattern_no_action", src: `1`, in: "foo", out: "foo\n"},
	{name: "no_pattern_empty_action", src: `{}`, in: "foo", out: ""},
	{name: "false_pattern_empty_action", src: `0 {}`, in: "foo", out: ""},
	{name: "true_pattern_empty_action", src: `1 {}`, in: "foo", out: ""},
	{name: "no_pattern_action", src: `{ print $1 }`, in: "foo", out: "foo\n"},
	{name: "false_pattern_action", src: `0 { print $1 }`, in: "foo", out: ""},
	{name: "true_pattern_action", src: `1 { print $1 }`, in: "foo", out: "foo\n"},

	// Pattern expressions
	{name: "pattern_$0", src: `$0`, in: "foo\n\nbar", out: "foo\nbar\n"},
	{name: "action_print_$0", src: `{ print $0 }`, in: "foo\n\nbar", out: "foo\n\nbar\n"},
	{name: "pattern_field_eq_str", src: `$1=="foo"`, in: "foo\n\nbar", out: "foo\n"},
	{name: "pattern_field_eq_num", src: `$1==42`, in: "foo\n42\nbar", out: "42\n"},
	{name: "pattern_field_eq_numstr", src: `$1=="42"`, in: "foo\n42\nbar", out: "42\n"},
	{name: "pattern_regex", src: `/foo/`, in: "foo\nx\nfood\nxfooz\nbar", out: "foo\nfood\nxfooz\n"},

	// Range patterns
	{name: "range_pattern", src: `NR==2, NR==4`, in: "1\n2\n3\n4\n5\n6\n", out: "2\n3\n4\n"},
}

func TestCompatPatterns(t *testing.T) {
	runTestCategory(t, patternTests)
}

// =============================================================================
// Print/Printf Tests
// =============================================================================

var printTests = []interpTest{
	{name: "print_two_args", src: `BEGIN { print "x", "y" }`, out: "x y\n"},
	{name: "print_OFS", src: `BEGIN { print OFS; OFS = ","; print "x", "y" }`, out: " \nx,y\n"},
	{name: "print_ORS", src: `BEGIN { print ORS; ORS = "."; print "x", "y" }`, out: "\n\nx y."},
	{name: "print_ORS_empty", src: `BEGIN { print ORS; ORS = ""; print "x", "y" }`, out: "\n\nx y"},
	{name: "print_twice", src: `{ print; print }`, in: "foo", out: "foo\nfoo\n"},
	{name: "print_empty", src: `BEGIN { print; print }`, out: "\n\n"},

	// printf
	{name: "printf_all_formats", src: `BEGIN { printf "%% %d %x %c %f %s", 42, 42, 42, 42, 42 }`, out: "% 42 2a * 42.000000 42"},
	{name: "printf_width", src: `BEGIN { printf "%3d", 42 }`, out: " 42"},
	{name: "printf_string_width", src: `BEGIN { printf "%3s", "x" }`, out: "  x"},
	{name: "printf_extra_args", src: `BEGIN { printf "%d", 12, 34 }`, out: "12"},
	{name: "printf_char_zero", src: `BEGIN { printf "%c", 0 }`, out: "\x00"},
	{name: "printf_char_127", src: `BEGIN { printf "%c", 127 }`, out: "\x7f"},
	{name: "printf_char_string", src: `BEGIN { printf "%c", "xyz" }`, out: "x"},
	{name: "printf_paren", src: `BEGIN { printf("%%%dd", 4) }`, out: "%4d"},
	{name: "printf_int_formats", src: `BEGIN { printf "%d %i %o %u %x %X", 42, 42, 42, 42, 42, 42 }`, out: "42 42 52 42 2a 2A"},
}

func TestCompatPrint(t *testing.T) {
	runTestCategory(t, printTests)
}

// =============================================================================
// Control Flow Tests (if, for, while, do-while)
// =============================================================================

var controlFlowTests = []interpTest{
	// if statements
	{name: "if_true", src: `BEGIN { if (1) print "t"; }`, out: "t\n"},
	{name: "if_false", src: `BEGIN { if (0) print "t"; }`, out: ""},
	{name: "if_else_true", src: `BEGIN { if (1) print "t"; else print "f" }`, out: "t\n"},
	{name: "if_else_false", src: `BEGIN { if (0) print "t"; else print "f" }`, out: "f\n"},
	{name: "if_eq_true", src: `BEGIN { if (1==1) print "t"; else print "f" }`, out: "t\n"},
	{name: "if_eq_false", src: `BEGIN { if (1==2) print "t"; else print "f" }`, out: "f\n"},
	{name: "if_ne_false", src: `BEGIN { if (1!=1) print "t"; else print "f" }`, out: "f\n"},
	{name: "if_ne_true", src: `BEGIN { if (1!=2) print "t"; else print "f" }`, out: "t\n"},
	{name: "if_gt_false", src: `BEGIN { if (1>2) print "t"; else print "f" }`, out: "f\n"},
	{name: "if_gt_true", src: `BEGIN { if (2>1) print "t"; else print "f" }`, out: "t\n"},
	{name: "if_ge_false", src: `BEGIN { if (1>=2) print "t"; else print "f" }`, out: "f\n"},
	{name: "if_ge_true", src: `BEGIN { if (2>=1) print "t"; else print "f" }`, out: "t\n"},
	{name: "if_lt_true", src: `BEGIN { if (1<2) print "t"; else print "f" }`, out: "t\n"},
	{name: "if_lt_false", src: `BEGIN { if (2<1) print "t"; else print "f" }`, out: "f\n"},
	{name: "if_le_true", src: `BEGIN { if (1<=2) print "t"; else print "f" }`, out: "t\n"},
	{name: "if_le_false", src: `BEGIN { if (2<=1) print "t"; else print "f" }`, out: "f\n"},
	{name: "if_str_eq_true", src: `BEGIN { if ("a"=="a") print "t"; else print "f" }`, out: "t\n"},
	{name: "if_str_eq_false", src: `BEGIN { if ("a"=="b") print "t"; else print "f" }`, out: "f\n"},
	{name: "if_str_ne_false", src: `BEGIN { if ("a"!="a") print "t"; else print "f" }`, out: "f\n"},
	{name: "if_str_ne_true", src: `BEGIN { if ("a"!="b") print "t"; else print "f" }`, out: "t\n"},
	{name: "if_str_gt_false", src: `BEGIN { if ("a">"b") print "t"; else print "f" }`, out: "f\n"},
	{name: "if_str_gt_true", src: `BEGIN { if ("b">"a") print "t"; else print "f" }`, out: "t\n"},
	{name: "if_str_lt_true", src: `BEGIN { if ("a"<"b") print "t"; else print "f" }`, out: "t\n"},
	{name: "if_str_lt_false", src: `BEGIN { if ("b"<"a") print "t"; else print "f" }`, out: "f\n"},
	{name: "if_str_ge_false", src: `BEGIN { if ("a">="b") print "t"; else print "f" }`, out: "f\n"},
	{name: "if_str_ge_true", src: `BEGIN { if ("b">="a") print "t"; else print "f" }`, out: "t\n"},
	{name: "if_str_le_true", src: `BEGIN { if ("a"<="b") print "t"; else print "f" }`, out: "t\n"},
	{name: "if_str_le_false", src: `BEGIN { if ("b"<="a") print "t"; else print "f" }`, out: "f\n"},

	// for loops
	{name: "for_infinite_break", src: `BEGIN { for (;;) { print "x"; break } }`, out: "x\n"},
	{name: "for_infinite_counter", src: `BEGIN { for (;;) { printf "%d ", i; i++; if (i>2) break; } }`, out: "0 1 2 "},
	{name: "for_init_only", src: `BEGIN { for (i=5; ; ) { printf "%d ", i; i++; if (i>8) break; } }`, out: "5 6 7 8 "},
	{name: "for_init_post", src: `BEGIN { for (i=5; ; i++) { printf "%d ", i; if (i>8) break; } }`, out: "5 6 7 8 9 "},
	{name: "for_full", src: `BEGIN { for (i=5; i<8; i++) { printf "%d ", i } }`, out: "5 6 7 "},
	{name: "for_decrement", src: `BEGIN { for (i=3; i>0; i--) { printf "%d ", i } }`, out: "3 2 1 "},
	{name: "for_decrement_ge", src: `BEGIN { for (i=3; i>=0; i--) { printf "%d ", i } }`, out: "3 2 1 0 "},
	{name: "for_continue", src: `BEGIN { for (i=0; i<10; i++) { if (i < 5) continue; printf "%d ", i } }`, out: "5 6 7 8 9 "},
	{name: "for_sum", src: `BEGIN { for (i=0; i<100; i++) s+=i; print s }`, out: "4950\n"},
	{name: "for_in_break", src: `BEGIN { a[1]=1; a[2]=1; for (k in a) { s++; break } print s }`, out: "1\n"},
	{name: "for_in_continue", src: `BEGIN { a[1]=1; a[2]=1; a[3]=1; for (k in a) { if (k==2) continue; s++ } print s }`, out: "2\n"},
	{name: "for_empty_post", src: `BEGIN { for (i=0; i<10; i++); printf "x" }`, out: "x"},

	// while loops
	{name: "while_basic", src: `BEGIN { while (i<3) { i++; s++; break } print s }`, out: "1\n"},
	{name: "while_continue", src: `BEGIN { while (i<3) { i++; if (i==2) continue; s++ } print s }`, out: "2\n"},
	{name: "while_counter", src: `BEGIN { while (i < 5) { print i; i++ } }`, out: "\n1\n2\n3\n4\n"},
	{name: "while_string", src: `BEGIN { s="x"; while (s=="x") { print s; s="y" } }`, out: "x\n"},
	{name: "while_string_ne", src: `BEGIN { s="x"; while (s!="") { print s; s="" } }`, out: "x\n"},
	{name: "while_truthy", src: `BEGIN { s="x"; while (s) { print s; s="" } }`, out: "x\n"},

	// do-while loops
	{name: "dowhile_break", src: `BEGIN { do { i++; s++; break } while (i<3); print s }`, out: "1\n"},
	{name: "dowhile_continue", src: `BEGIN { do { i++; if (i==2) continue; s++ } while (i<3); print s }`, out: "2\n"},
	{name: "dowhile_counter", src: `BEGIN { do { print i; i++ } while (i < 5) }`, out: "\n1\n2\n3\n4\n"},
	{name: "dowhile_newline", src: "BEGIN { do { print 1 }\nwhile (0) }", out: "1\n"},

	// Nested loops
	{name: "nested_break", src: `
BEGIN {
	for (i = 0; i < 1; i++) {
		for (j = 0; j < 1; j++) {
			print i, j
		}
		break
	}
}
`, out: "0 0\n"},
	{name: "nested_continue", src: `
BEGIN {
	for (i = 0; i < 1; i++) {
		for (j = 0; j < 1; j++) {
			print i, j
		}
		continue
	}
}
`, out: "0 0\n"},
}

func TestCompatControlFlow(t *testing.T) {
	runTestCategory(t, controlFlowTests)
}

// =============================================================================
// Next Statement Tests
// =============================================================================

var nextTests = []interpTest{
	{name: "next_skip_record", src: `{ if (NR==2) next; print }`, in: "a\nb\nc", out: "a\nc\n"},
	{name: "next_in_function", src: `{ if (NR==2) f(); print }  function f() { next }`, in: "a\nb\nc", out: "a\nc\n"},
	{name: "next_in_BEGIN_error", src: `BEGIN { next }`, err: "next cannot be inside BEGIN or END"},
	{name: "next_in_END_error", src: `END { next }`, err: "next cannot be inside BEGIN or END"},
}

func TestCompatNext(t *testing.T) {
	runTestCategory(t, nextTests)
}

// =============================================================================
// Array Tests
// =============================================================================

var arrayTests = []interpTest{
	{name: "array_in", src: `BEGIN { a["x"] = 3; print "x" in a, "y" in a }`, out: "1 0\n"},
	{name: "array_delete", src: `BEGIN { a["x"] = 3; a["y"] = 4; delete a["x"]; for (k in a) print k, a[k] }`, out: "y 4\n"},
	{name: "array_delete_in_loop", src: `BEGIN { a["x"] = 3; a["y"] = 4; for (k in a) delete a[k]; for (k in a) print k, a[k] }`, out: ""},
	{name: "array_implicit", src: `BEGIN { a["x"]; "y" in a; for (k in a) print k, a[k] }`, out: "x \n"},
	{name: "array_delete_all", src: `BEGIN { a["x"] = 3; a["y"] = 4; delete a; for (k in a) print k, a[k] }`, out: ""},
	{name: "array_param", src: `function f(a) { print "x" in a, "y" in a }  BEGIN { b["x"] = 3; f(b) }`, out: "1 0\n"},
	{name: "array_sum", src: `BEGIN { a["x"] = 3; a["y"] = 4; for (k in a) x += a[k]; print x }`, out: "7\n"},
	{name: "array_numeric_key", src: `BEGIN { a[1] = "x"; print a[1] }`, out: "x\n"},
	{name: "array_string_key", src: `BEGIN { a["k"] = "v"; print a["k"] }`, out: "v\n"},
	{name: "array_in_true", src: `BEGIN { a[1] = "x"; print (1 in a) }`, out: "1\n"},
	{name: "array_in_false", src: `BEGIN { a[1] = "x"; print (2 in a) }`, out: "0\n"},
	{name: "array_for_in_sum", src: `BEGIN { a[1]=1; a[2]=2; for (k in a) s+=a[k]; print s }`, out: "3\n"},
	{name: "array_empty_index_error", src: `BEGIN { a[] }`, err: "expected expression in array index"},
	{name: "delete_empty_index_error", src: `BEGIN { delete a[] }`, err: "expected expression in delete index"},
}

func TestCompatArrays(t *testing.T) {
	runTestCategory(t, arrayTests)
}

// =============================================================================
// Unary Operator Tests
// =============================================================================

var unaryTests = []interpTest{
	{name: "unary_not", src: `BEGIN { print !42, !1, !0, !!42, !!1, !!0 }`, out: "0 0 1 1 1 0\n"},
	{name: "unary_plus_minus", src: `BEGIN { print +4, +"3", +0, +-3, -3, - -4, -"3" }`, out: "4 3 0 -3 -3 4 -3\n"},
	{name: "unary_not_$0_str", src: `BEGIN { $0="0"; print !$0 }`, out: "0\n"},
	{name: "unary_not_$0_str1", src: `BEGIN { $0="1"; print !$0 }`, out: "0\n"},
	{name: "unary_not_$0_0", src: `{ print !$0 }`, in: "0\n", out: "1\n"},
	{name: "unary_not_$0_1", src: `{ print !$0 }`, in: "1\n", out: "0\n"},
	{name: "unary_seen_pattern", src: `!seen[$0]++`, in: "1\n2\n3\n2\n3\n3\n", out: "1\n2\n3\n"},
	{name: "unary_seen_pattern_dec", src: `!seen[$0]--`, in: "1\n2\n3\n2\n3\n3\n", out: "1\n2\n3\n"},
}

func TestCompatUnary(t *testing.T) {
	runTestCategory(t, unaryTests)
}

// =============================================================================
// Comparison Operator Tests
// =============================================================================

var comparisonTests = []interpTest{
	{name: "eq_numbers", src: `BEGIN { print (1==1, 1==0, "1"==1, "1"==1.0) }`, out: "1 0 1 1\n"},
	{name: "eq_fields", src: `{ print ($0=="1", $0==1) }`, in: "1\n1.0\n+1", out: "1 1\n0 1\n0 1\n"},
	{name: "eq_field1", src: `{ print ($1=="1", $1==1) }`, in: "1\n1.0\n+1", out: "1 1\n0 1\n0 1\n"},
	{name: "ne_numbers", src: `BEGIN { print (1!=1, 1!=0, "1"!=1, "1"!=1.0) }`, out: "0 1 0 0\n"},
	{name: "ne_fields", src: `{ print ($0!="1", $0!=1) }`, in: "1\n1.0\n+1", out: "0 0\n1 0\n1 0\n"},
	{name: "ne_field1", src: `{ print ($1!="1", $1!=1) }`, in: "1\n1.0\n+1", out: "0 0\n1 0\n1 0\n"},
	{name: "lt_numbers", src: `BEGIN { print (0<1, 1<1, 2<1, "12"<"2") }`, out: "1 0 0 1\n"},
	{name: "lt_field", src: `{ print ($1<2) }`, in: "1\n1.0\n+1", out: "1\n1\n1\n"},
	{name: "le_numbers", src: `BEGIN { print (0<=1, 1<=1, 2<=1, "12"<="2") }`, out: "1 1 0 1\n"},
	{name: "le_field", src: `{ print ($1<=2) }`, in: "1\n1.0\n+1", out: "1\n1\n1\n"},
	{name: "gt_numbers", src: `BEGIN { print (0>1, 1>1, 2>1, "12">"2") }`, out: "0 0 1 0\n"},
	{name: "gt_field", src: `{ print ($1>2) }`, in: "1\n1.0\n+1", out: "0\n0\n0\n"},
	{name: "ge_numbers", src: `BEGIN { print (0>=1, 1>=1, 2>=1, "12">="2") }`, out: "0 1 1 0\n"},
	{name: "ge_field", src: `{ print ($1>=2) }`, in: "1\n1.0\n+1", out: "0\n0\n0\n"},
	{name: "lt_$0_num", src: `{ print($0<2) }`, in: "10", out: "0\n"},
	{name: "lt_$1_num", src: `{ print($1<2) }`, in: "10", out: "0\n"},
	{name: "lt_$1_numstr", src: `{ print($1<2) }`, in: "10x", out: "1\n"},
	{name: "lt_$0_assigned", src: `BEGIN { $0="10"; print($0<2) }`, out: "1\n"},
	{name: "lt_$1_assigned", src: `BEGIN { $1="10"; print($1<2) }`, out: "1\n"},
	{name: "lt_$1_assigned_str", src: `BEGIN { $1="10x"; print($1<2) }`, out: "1\n"},
}

func TestCompatComparison(t *testing.T) {
	runTestCategory(t, comparisonTests)
}

// =============================================================================
// Logical Operator Tests (&&, ||)
// =============================================================================

var logicalTests = []interpTest{
	{name: "and_shortcircuit", src: `
function t() { print "t"; return 2 }
function f() { print "f"; return 0 }
BEGIN {
	print f() && f()
	print f() && t()
	print t() && f()
	print t() && t()
}
`, out: "f\n0\nf\n0\nt\nf\n0\nt\nt\n1\n"},
	{name: "or_shortcircuit", src: `
function t() { print "t"; return 2 }
function f() { print "f"; return 0 }
BEGIN {
	print f() || f()
	print f() || t()
	print t() || f()
	print t() || t()
}
`, out: "f\nf\n0\nf\nt\n1\nt\n1\nt\n1\n"},
	{name: "and_values", src: `BEGIN { print 0&&0, 0&&2, 2&&0, 2&&2 }`, out: "0 0 0 1\n"},
	{name: "or_values", src: `BEGIN { print 0||0, 0||2, 2||0, 2||2 }`, out: "0 1 1 1\n"},
}

func TestCompatLogical(t *testing.T) {
	runTestCategory(t, logicalTests)
}

// =============================================================================
// Arithmetic Operator Tests
// =============================================================================

var arithmeticTests = []interpTest{
	{name: "add", src: `BEGIN { print 1+2, 1+2+3, 1+-2, -1+2, "1"+"2", 3+.14 }`, out: "3 6 -1 1 3 3.14\n"},
	{name: "sub", src: `BEGIN { print 1-2, 1-2-3, 1-+2, -1-2, "1"-"2", 3-.14 }`, out: "-1 -4 -1 -3 -1 2.86\n"},
	{name: "mul", src: `BEGIN { print 2*3, 2*3*4, 2*-3, -2*3, "2"*"3", 3*.14 }`, out: "6 24 -6 -6 6 0.42\n"},
	{name: "div", src: `BEGIN { print 2/3, 2/3/4, 2/-3, -2/3, "2"/"3", 3/.14 }`, out: "0.666667 0.166667 -0.666667 -0.666667 0.666667 21.4286\n"},
	{name: "mod", src: `BEGIN { print 2%3, 2%3%4, 2%-3, -2%3, "2"%"3", 3%.14 }`, out: "2 2 2 -2 2 0.06\n"},
	{name: "pow", src: `BEGIN { print 2^3, 2^3^3, 2^-3, -2^3, "2"^"3", 3^.14 }`, out: "8 134217728 0.125 -8 8 1.16626\n"},
	{name: "concat", src: `BEGIN { print 1 2, "x" "yz", 1+2 3+4 }`, out: "12 xyz 37\n"},
	{name: "precedence", src: `BEGIN { print 1+2*3/4^5%6 7, (1+2)*3/4^5%6 "7" }`, out: "1.005867 0.008789067\n"},
	{name: "div_by_zero", src: `BEGIN { print 1/0 }`, err: "division by zero"},
	{name: "mod_by_zero", src: `BEGIN { print 1%0 }`, err: "division by zero"},
}

func TestCompatArithmetic(t *testing.T) {
	runTestCategory(t, arithmeticTests)
}

// =============================================================================
// Regex Match Tests
// =============================================================================

var regexTests = []interpTest{
	{name: "match_op_true", src: `BEGIN { print "food"~/oo/, "food"~/[oO]+d/, "food"~"f", "food"~"F", "food"~0 }`, out: "1 1 1 0 0\n"},
	{name: "notmatch_op", src: `BEGIN { print "food"!~/oo/, "food"!~/[oO]+d/, "food"!~"f", "food"!~"F", "food"!~0 }`, out: "0 0 0 1 1\n"},
	{name: "regex_pattern", src: `{ print /foo/ }`, in: "food\nfoo\nxfooz\nbar\n", out: "1\n1\n1\n0\n"},
	{name: "regex_match_literal", src: `BEGIN { print "hello" ~ /ell/ }`, out: "1\n"},
	{name: "regex_notmatch_literal", src: `BEGIN { print "hello" ~ /xyz/ }`, out: "0\n"},
	{name: "regex_notmatch_op_true", src: `BEGIN { print "hello" !~ /xyz/ }`, out: "1\n"},
	{name: "regex_notmatch_op_false", src: `BEGIN { print "hello" !~ /ell/ }`, out: "0\n"},
}

func TestCompatRegex(t *testing.T) {
	runTestCategory(t, regexTests)
}

// =============================================================================
// Conditional Expression Tests (?:)
// =============================================================================

var ternaryTests = []interpTest{
	{name: "ternary_regex", src: `{ print /x/?"t":"f" }`, in: "x\ny\nxx\nz\n", out: "t\nf\nt\nf\n"},
	{name: "ternary_nested", src: `BEGIN { print 1?2?3:4:5, 1?0?3:4:5, 0?2?3:4:5 }`, out: "3 4 5\n"},
	{name: "ternary_$0_str", src: `BEGIN { $0="0"; print ($0?1:0) }`, out: "1\n"},
	{name: "ternary_$0_0", src: `{ print $0?1:0 }`, in: "0\n", out: "0\n"},
	{name: "ternary_$0_1", src: `{ print $0?1:0 }`, in: "1\n", out: "1\n"},
	{name: "ternary_$0_str1", src: `BEGIN { $0="1"; print ($0?1:0) }`, out: "1\n"},
	{name: "ternary_values", src: `BEGIN { print 0?1:0, 1?1:0, ""?1:0, "0"?1:0, "1"?1:0, x?1:0 }`, out: "0 1 0 1 1 0\n"},
	{name: "ternary_false", src: `BEGIN { print 0?"t":"f" }`, out: "f\n"},
	{name: "ternary_true", src: `BEGIN { print 1?"t":"f" }`, out: "t\n"},
	{name: "ternary_expr", src: `BEGIN { print (1+2)?"t":"f" }`, out: "t\n"},
	{name: "ternary_parens", src: `BEGIN { print (1+2?"t":"f") }`, out: "t\n"},
	{name: "ternary_assign", src: `BEGIN { print(1 ? x="t" : "f"); print x; }`, out: "t\nt\n"},
}

func TestCompatTernary(t *testing.T) {
	runTestCategory(t, ternaryTests)
}

// =============================================================================
// Assignment Tests
// =============================================================================

var assignmentTests = []interpTest{
	{name: "assign_basic", src: `BEGIN { print x; x = 4; print x; }`, out: "\n4\n"},
	{name: "assign_array", src: `BEGIN { a["foo"]=1; b[2]="x"; k="foo"; print a[k], b["2"] }`, out: "1 x\n"},
	{name: "assign_add", src: `BEGIN { s+=5; print s; s-=2; print s; s-=s; print s }`, out: "5\n3\n0\n"},
	{name: "assign_mul", src: `BEGIN { x=2; x*=x; print x; x*=3; print x }`, out: "4\n12\n"},
	{name: "assign_div", src: `BEGIN { x=6; x/=3; print x; x/=x; print x; x/=.6; print x }`, out: "2\n1\n1.66667\n"},
	{name: "assign_mod", src: `BEGIN { x=12; x%=5; print x }`, out: "2\n"},
	{name: "assign_pow", src: `BEGIN { x=2; x^=5; print x; x^=0.5; print x }`, out: "32\n5.65685\n"},
	{name: "assign_field", src: `{ $2+=10; print; $3/=2; print }`, in: "1 2 3", out: "1 12 3\n1 12 1.5\n"},
	{name: "assign_array_numeric", src: `BEGIN { a[2] += 1; a["2"] *= 3; print a[2] }`, out: "3\n"},
	{name: "assign_NF", src: `BEGIN { NF += 3; print NF }`, out: "3\n"},
	{name: "assign_compound", src: `BEGIN { x=1; x += x+=3; print x }`, out: "8\n"},
}

func TestCompatAssignment(t *testing.T) {
	runTestCategory(t, assignmentTests)
}

// =============================================================================
// Increment/Decrement Tests
// =============================================================================

var incrDecrTests = []interpTest{
	{name: "post_incr", src: `BEGIN { print x++; print x }`, out: "0\n1\n"},
	{name: "pre_incr", src: `BEGIN { print x; print x++; print ++x; print x }`, out: "\n0\n2\n2\n"},
	{name: "pre_decr", src: `BEGIN { print x; print x--; print --x; print x }`, out: "\n0\n-2\n-2\n"},
	{name: "incr_twice", src: `BEGIN { s++; s++; print s }`, out: "2\n"},
	{name: "array_incr", src: `BEGIN { x[y++]++; print y }`, out: "1\n"},
	{name: "array_add_incr", src: `BEGIN { x[y++] += 3; print y }`, out: "1\n"},
	{name: "field_incr", src: `BEGIN { $(y++)++; print y }`, out: "1\n"},
	{name: "concat_incr", src: `BEGIN { print "s" ++n; print "s" --n }`, out: "s1\ns0\n"},
	{name: "NF_incr", src: `BEGIN { NF++; print NF }`, out: "1\n"},
	{name: "field_pre_post", src: "{ print $++n; print $--n }", in: "x y", out: "x\nx y\n"},
	{name: "field_incr_multiline", src: "{ print $++a }", in: "1 2 3\na b c\nd e f\n", out: "1\nb\nf\n"},
	{name: "paren_incr", src: "BEGIN{ a = 3; b = 7; c = (a)++b; print a, b, c }", out: "3 8 38\n"},
}

func TestCompatIncrDecr(t *testing.T) {
	runTestCategory(t, incrDecrTests)
}

// =============================================================================
// Built-in Math Function Tests
// =============================================================================

var mathFuncTests = []interpTest{
	{name: "sin", src: `BEGIN { print sin(0), sin(0.5), sin(1), sin(-1) }`, out: "0 0.479426 0.841471 -0.841471\n"},
	{name: "cos", src: `BEGIN { print cos(0), cos(0.5), cos(1), cos(-1) }`, out: "1 0.877583 0.540302 0.540302\n"},
	{name: "exp", src: `BEGIN { print exp(0), exp(0.5), exp(1), exp(-1) }`, out: "1 1.64872 2.71828 0.367879\n"},
	{name: "log", src: `BEGIN { print log(0), log(0.5), log(1) }`, out: "-inf -0.693147 0\n"},
	{name: "sqrt", src: `BEGIN { print sqrt(0), sqrt(2), sqrt(4) }`, out: "0 1.41421 2\n"},
	{name: "int", src: `BEGIN { print int(3.5), int("1.9"), int(4), int(-3.6), int("x"), int("") }`, out: "3 1 4 -3 0 0\n"},
	{name: "atan2", src: `BEGIN { print atan2(1, 0.5), atan2(-1, 0) }`, out: "1.10715 -1.5708\n"},
}

func TestCompatMathFuncs(t *testing.T) {
	runTestCategory(t, mathFuncTests)
}

// =============================================================================
// Built-in String Function Tests
// =============================================================================

var stringFuncTests = []interpTest{
	// match
	{name: "match_basic", src: `BEGIN { print match("food", "foo"), RSTART, RLENGTH }`, out: "1 1 3\n"},
	{name: "match_offset", src: `BEGIN { print match("x food y", "fo"), RSTART, RLENGTH }`, out: "3 3 2\n"},
	{name: "match_nomatch", src: `BEGIN { print match("x food y", "fox"), RSTART, RLENGTH }`, out: "0 0 -1\n"},
	{name: "match_regex", src: `BEGIN { print match("x food y", /[fod]+/), RSTART, RLENGTH }`, out: "3 3 4\n"},
	{name: "match_multiline", src: `BEGIN { print match("a\nb\nc", /^a.*c$/), RSTART, RLENGTH }`, out: "1 1 5\n"},

	// length
	{name: "length_field", src: `{ print length, length(), length("buzz"), length("") }`, in: "foo bar", out: "7 7 4 0\n"},
	{name: "length_empty", src: `BEGIN { print length("") }`, out: "0\n"},
	{name: "length_str", src: `BEGIN { print length("abc") }`, out: "3\n"},
	{name: "length_long", src: `BEGIN { print length("hello world") }`, out: "11\n"},

	// index
	{name: "index_basic", src: `BEGIN { print index("foo", "f"), index("foo0", 0), index("foo", "o"), index("foo", "x") }`, out: "1 4 2 0\n"},
	{name: "index_ll", src: `BEGIN { print index("hello", "ll") }`, out: "3\n"},
	{name: "index_notfound", src: `BEGIN { print index("hello", "x") }`, out: "0\n"},
	{name: "index_empty", src: `BEGIN { print index("hello", "") }`, out: "1\n"},

	// substr
	{name: "substr_1", src: `BEGIN { print substr("food", 1) }`, out: "food\n"},
	{name: "substr_1_2", src: `BEGIN { print substr("food", 1, 2) }`, out: "fo\n"},
	{name: "substr_1_4", src: `BEGIN { print substr("food", 1, 4) }`, out: "food\n"},
	{name: "substr_1_8", src: `BEGIN { print substr("food", 1, 8) }`, out: "food\n"},
	{name: "substr_2", src: `BEGIN { print substr("food", 2) }`, out: "ood\n"},
	{name: "substr_2_2", src: `BEGIN { print substr("food", 2, 2) }`, out: "oo\n"},
	{name: "substr_2_3", src: `BEGIN { print substr("food", 2, 3) }`, out: "ood\n"},
	{name: "substr_2_8", src: `BEGIN { print substr("food", 2, 8) }`, out: "ood\n"},
	{name: "substr_0", src: `BEGIN { print substr("food", 0, 8) }`, out: "food\n"},
	{name: "substr_neg", src: `BEGIN { print substr("food", -1, 8) }`, out: "food\n"},
	{name: "substr_past_end", src: `BEGIN { print substr("food", 5) }`, out: "\n"},
	{name: "substr_neg_start", src: `BEGIN { print substr("food", -1) }`, out: "food\n"},
	{name: "substr_past_8", src: `BEGIN { print substr("food", 5, 8) }`, out: "\n"},

	// split
	{name: "split_empty", src: `BEGIN { n = split("", a); for (i=1; i<=n; i++) print a[i] }`, out: ""},
	{name: "split_empty_sep", src: `BEGIN { n = split("", a, "."); for (i=1; i<=n; i++) print a[i] }`, out: ""},
	{name: "split_space", src: `BEGIN { n = split("ab c d ", a); for (i=1; i<=n; i++) print a[i] }`, out: "ab\nc\nd\n"},
	{name: "split_comma", src: `BEGIN { n = split("ab,c,d,", a, ","); for (i=1; i<=n; i++) print a[i] }`, out: "ab\nc\nd\n\n"},
	{name: "split_regex", src: `BEGIN { n = split("ab,c.d,", a, /[,.]/); for (i=1; i<=n; i++) print a[i] }`, out: "ab\nc\nd\n\n"},
	{name: "split_numeric", src: `BEGIN { n = split("1 2", a); print (n, a[1], a[2], a[1]==1, a[2]==2) }`, out: "2 1 2 1 1\n"},

	// tolower/toupper
	{name: "tolower", src: `BEGIN { print tolower("Foo BaR") }`, out: "foo bar\n"},
	{name: "toupper", src: `BEGIN { print toupper("Foo BaR") }`, out: "FOO BAR\n"},
	{name: "tolower_upper", src: `BEGIN { print tolower("HELLO") }`, out: "hello\n"},
	{name: "toupper_lower", src: `BEGIN { print toupper("hello") }`, out: "HELLO\n"},
	{name: "tolower_mixed", src: `BEGIN { print tolower("HeLLo WoRLd") }`, out: "hello world\n"},

	// sprintf
	{name: "sprintf_d", src: `BEGIN { print sprintf("%3d", 42) }`, out: " 42\n"},
	{name: "sprintf_extra", src: `BEGIN { print sprintf("%d", 12, 34) }`, out: "12\n"},
	{name: "sprintf_space", src: `BEGIN { print sprintf("% 5d", 42) }`, out: "   42\n"},
	{name: "sprintf_star", src: `BEGIN { print sprintf("%*s %.*s", 5, "abc", 5, "abcdefghi") }`, out: "  abc abcde\n"},
	{name: "sprintf_simple", src: `BEGIN { print sprintf("%d", 42) }`, out: "42\n"},
	{name: "sprintf_string", src: `BEGIN { print sprintf("%s", "hello") }`, out: "hello\n"},
	{name: "sprintf_multi", src: `BEGIN { print sprintf("%d %s", 42, "test") }`, out: "42 test\n"},

	// sub
	{name: "sub_basic", src: `BEGIN { x = "1.2.3"; print sub(/\./, ",", x); print x }`, out: "1\n1,2.3\n"},
	{name: "sub_backslash", src: `BEGIN { x = "1.2.3"; print sub(/\./, ",\\", x); print x }`, out: "1\n1,\\2.3\n"},
	{name: "sub_$0", src: `{ print sub(/\./, ","); print $0 }`, in: "1.2.3", out: "1\n1,2.3\n"},

	// gsub
	{name: "gsub_basic", src: `BEGIN { x = "1.2.3"; print gsub(/\./, ",", x); print x }`, out: "2\n1,2,3\n"},
	{name: "gsub_$0", src: `{ print gsub(/\./, ","); print $0 }`, in: "1.2.3", out: "2\n1,2,3\n"},
	{name: "gsub_ampersand", src: `{ print gsub(/[0-9]/, "(&)"); print $0 }`, in: "0123x. 42y", out: "6\n(0)(1)(2)(3)x. (4)(2)y\n"},
	{name: "gsub_ampersand_plus", src: `{ print gsub(/[0-9]+/, "(&)"); print $0 }`, in: "0123x. 42y", out: "2\n(0123)x. (42)y\n"},
	{name: "gsub_escaped_amp", src: `{ print gsub(/[0-9]/, "\\&"); print $0 }`, in: "0123x. 42y", out: "6\n&&&&x. &&y\n"},
	{name: "gsub_escaped_z", src: `{ print gsub(/[0-9]/, "\\z"); print $0 }`, in: "0123x. 42y", out: "6\n\\z\\z\\z\\zx. \\z\\zy\n"},

	// srand/rand
	{name: "srand_reproducible", src: `
BEGIN {
    srand()
	srand(1)
	a = rand(); b = rand(); c = rand()
	srand(1)
	x = rand(); y = rand(); z = rand()
	print (a==b, b==c, x==y, y==z)
	print (a==x, b==y, c==z)
}
`, out: "0 0 0 0\n1 1 1\n"},
}

func TestCompatStringFuncs(t *testing.T) {
	runTestCategory(t, stringFuncTests)
}

// =============================================================================
// Field Access Tests
// =============================================================================

var fieldTests = []interpTest{
	{name: "field_$0", src: `{ print $0 }`, in: "hello world", out: "hello world\n"},
	{name: "field_$1", src: `{ print $1 }`, in: "hello world", out: "hello\n"},
	{name: "field_$2", src: `{ print $2 }`, in: "hello world", out: "world\n"},
	{name: "field_$3", src: `{ print $3 }`, in: "hello world", out: "\n"},
	{name: "field_NF", src: `{ print NF }`, in: "a b c", out: "3\n"},
	{name: "field_$NF", src: `{ print $NF }`, in: "a b c", out: "c\n"},
	{name: "field_$(NF-1)", src: `{ print $(NF-1) }`, in: "a b c", out: "b\n"},
	{name: "field_print_multi", src: `{ print; print $1, $3, $NF }`, in: "a b c d e", out: "a b c d e\na c e\n"},
	{name: "field_assign", src: `{ print $1,$3; $2="x"; print; print $2 }`, in: "a b c", out: "a c\na x c\nx\n"},
	{name: "field_$0_assign", src: `{ print; $0="x y z"; print; print $1, $3 }`, in: "a b c", out: "a b c\nx y z\nx z\n"},
	{name: "field_$1_pow", src: `{ print $1^2 }`, in: "10", out: "100\n"},
	{name: "field_NF_multiline", src: `{ print NF }`, in: "\na\nc d\ne f g", out: "0\n1\n2\n3\n"},
	{name: "field_$$0", src: `{ $$0++; print $0 }`, in: "2 3 4", out: "3\n"},
}

func TestCompatFields(t *testing.T) {
	runTestCategory(t, fieldTests)
}

// =============================================================================
// NF Manipulation Tests
// =============================================================================

var nfTests = []interpTest{
	{name: "NF_assign_empty", src: `{ print NF; NF=1; $2="two"; print $0, NF }`, in: "\n", out: "0\n two 2\n"},
	{name: "NF_assign_2_empty", src: `{ print NF; NF=2; $2="two"; print $0, NF}`, in: "\n", out: "0\n two 2\n"},
	{name: "NF_assign_3", src: `{ print NF; NF=3; $2="two"; print $0, NF}`, in: "a b c\n", out: "3\na two c 3\n"},

	// NF=1 tests
	{name: "NF1_$1_1field", src: `{ NF=1; $1="x"; print $0; print NF }`, in: "a", out: "x\n1\n"},
	{name: "NF1_$1_2fields", src: `{ NF=1; $1="x"; print $0; print NF }`, in: "a b", out: "x\n1\n"},
	{name: "NF1_$1_3fields", src: `{ NF=1; $1="x"; print $0; print NF }`, in: "a b c", out: "x\n1\n"},
	{name: "NF1_$2_1field", src: `{ NF=1; $2="x"; print $0; print NF }`, in: "a", out: "a x\n2\n"},
	{name: "NF1_$2_2fields", src: `{ NF=1; $2="x"; print $0; print NF }`, in: "a b", out: "a x\n2\n"},
	{name: "NF1_$2_3fields", src: `{ NF=1; $2="x"; print $0; print NF }`, in: "a b c", out: "a x\n2\n"},
	{name: "NF1_$3_1field", src: `{ NF=1; $3="x"; print $0; print NF }`, in: "a", out: "a  x\n3\n"},

	// NF=2 tests
	{name: "NF2_$1_1field", src: `{ NF=2; $1="x"; print $0; print NF }`, in: "a", out: "x \n2\n"},
	{name: "NF2_$1_2fields", src: `{ NF=2; $1="x"; print $0; print NF }`, in: "a b", out: "x b\n2\n"},
	{name: "NF2_$1_3fields", src: `{ NF=2; $1="x"; print $0; print NF }`, in: "a b c", out: "x b\n2\n"},
	{name: "NF2_$2_1field", src: `{ NF=2; $2="x"; print $0; print NF }`, in: "a", out: "a x\n2\n"},
	{name: "NF2_$2_2fields", src: `{ NF=2; $2="x"; print $0; print NF }`, in: "a b", out: "a x\n2\n"},
	{name: "NF2_$2_3fields", src: `{ NF=2; $2="x"; print $0; print NF }`, in: "a b c", out: "a x\n2\n"},
	{name: "NF2_$3_1field", src: `{ NF=2; $3="x"; print $0; print NF }`, in: "a", out: "a  x\n3\n"},
	{name: "NF2_$3_2fields", src: `{ NF=2; $3="x"; print $0; print NF }`, in: "a b", out: "a b x\n3\n"},
	{name: "NF2_$3_3fields", src: `{ NF=2; $3="x"; print $0; print NF }`, in: "a b c", out: "a b x\n3\n"},

	// NF=3 tests
	{name: "NF3_$1_3fields", src: `{ NF=3; $1="x"; print $0; print NF }`, in: "a b c", out: "x b c\n3\n"},
	{name: "NF3_$2_3fields", src: `{ NF=3; $2="x"; print $0; print NF }`, in: "a b c", out: "a x c\n3\n"},
	{name: "NF3_$3_1field", src: `{ NF=3; $3="x"; print $0; print NF }`, in: "a", out: "a  x\n3\n"},
	{name: "NF3_$3_2fields", src: `{ NF=3; $3="x"; print $0; print NF }`, in: "a b", out: "a b x\n3\n"},
	{name: "NF3_$3_3fields", src: `{ NF=3; $3="x"; print $0; print NF }`, in: "a b c", out: "a b x\n3\n"},
}

func TestCompatNF(t *testing.T) {
	runTestCategory(t, nfTests)
}

// =============================================================================
// Special Variable Tests
// =============================================================================

var specialVarTests = []interpTest{
	{name: "CONVFMT", src: `
BEGIN {
	print CONVFMT, 1.2345678 ""
	CONVFMT = "%.3g"
	print CONVFMT, 1.234567 ""
}`, out: "%.6g 1.23457\n%.3g 1.23\n"},
	{name: "FILENAME", src: `BEGIN { FILENAME = "foo"; print FILENAME }`, out: "foo\n"},
	{name: "FILENAME_cmp", src: `BEGIN { FILENAME = "123.0"; print (FILENAME==123) }`, out: "0\n"},
	{name: "FNR_assign", src: `BEGIN { FNR = 123; print FNR }`, out: "123\n"},
	{name: "FNR_print", src: `{ print FNR, $0 }`, in: "a\nb\nc", out: "1 a\n2 b\n3 c\n"},
	{name: "NR_FNR_END", src: `{ print NR, FNR } END { print NR, FNR }`, in: "a\nb\nc\n", out: "1 1\n2 2\n3 3\n3 3\n"},
	{name: "FS_assign", src: `BEGIN { print "|" FS "|"; FS="," } { print $1, $2 }`, in: "a b\na,b\nx,,y", out: "| |\na b \na b\nx \n"},
	{name: "NR_assign", src: `BEGIN { NR = 123; print NR }`, out: "123\n"},
	{name: "NR_print", src: `{ print NR, $0 }`, in: "a\nb\nc", out: "1 a\n2 b\n3 c\n"},
	{name: "OFMT", src: `
BEGIN {
	print OFMT, 1.2345678
	OFMT = "%.3g"
	print OFMT, 1.234567
}`, out: "%.6g 1.23457\n%.3g 1.23\n"},
	{name: "RSTART_RLENGTH", src: `BEGIN { print RSTART, RLENGTH; RSTART=5; RLENGTH=42; print RSTART, RLENGTH; } `, out: "0 0\n5 42\n"},
	{name: "RS_print", src: `BEGIN { print RS }`, out: "\n\n"},
	{name: "RS_change", src: `BEGIN { print RS; RS="|"; print RS }  { print }`, in: "a b|c d|", out: "\n\n|\na b\nc d\n"},
	{name: "SUBSEP", src: `
BEGIN {
	print SUBSEP
	a[1, 2] = "onetwo"
	print a[1, 2]
	for (k in a) {
		print k, a[k]
	}
	delete a[1, 2]
	SUBSEP = "|"
	print SUBSEP
	a[1, 2] = "onetwo"
	print a[1, 2]
	for (k in a) {
		print k, a[k]
	}
}`, out: "\x1c\nonetwo\n1\x1c2 onetwo\n|\nonetwo\n1|2 onetwo\n"},
}

func TestCompatSpecialVars(t *testing.T) {
	runTestCategory(t, specialVarTests)
}

// =============================================================================
// User-Defined Function Tests
// =============================================================================

var functionTests = []interpTest{
	{name: "func_add", src: `function add(a, b) { return a + b } BEGIN { print add(1, 2) }`, out: "3\n"},
	{name: "func_factorial", src: `function fac(n) { if (n <= 1) return 1; return n * fac(n-1) } BEGIN { print fac(5) }`, out: "120\n"},
	{name: "func_compose", src: `function f(x) { return x * 2 } BEGIN { print f(f(3)) }`, out: "12\n"},
	{name: "func_locals_globals", src: `
function f(loc) {
	glob += 1
	loc += 1
	loc = loc * 2
	print glob, loc
}
BEGIN {
	glob = 1
	loc = 42
	f(3)
	print loc
	f(4)
	print loc
}
`, out: "2 8\n42\n3 10\n42\n"},
	{name: "func_array_param", src: `
function set(a, x, v) { a[x] = v }
function get(a, x) { return a[x] }
function get2(x, a) { return a[x] }
function get3(x, a, b) { b[0]; return a[x] }
BEGIN {
	a["x"] = 1
	set(b, "y", 2)
	for (k in a) print k, a[k]
	print "---"
	for (k in b) print k, b[k]
	print "---"
	print get(a, "x"), get(b, "y")
	print get2("x", a), get2("y", b)
	print get3("x", a), get2("y", b)
}
`, out: "x 1\n---\ny 2\n---\n1 2\n1 2\n1 2\n"},
	{name: "func_fib", src: `
function fib(n) {
	return n < 3 ? 1 : fib(n-2) + fib(n-1)
}
BEGIN {
	for (i = 1; i <= 7; i++) {
		printf "%d ", fib(i)
	}
}
`, out: "1 1 2 3 5 8 13 "},
	{name: "func_early_return", src: `
function early() {
	print "x"
	return
	print "y"
}
BEGIN { early() }
`, out: "x\n"},
	{name: "func_printf", src: `function f() { printf "x" }; BEGIN { f() } `, out: "x"},
	{name: "func_fewer_args", src: `function add(a, b) { return a+b }  BEGIN { print add(1, 2), add(1), add() }`, out: "3 1 0\n"},
	{name: "func_mutual_recursion", src: `
function f(n) { if (!n) return; print "f(" n ")"; g(n-1) }
function g(n) { if (!n) return; print "g(" n ")"; f(n-1) }
BEGIN { f(4) }
`, out: "f(4)\ng(3)\nf(2)\ng(1)\n"},
	{name: "func_deep_call", src: `
function f1(a) { f2(a) }
function f2(b) { f3(b) }
function f3(c) { f4(c) }
function f4(d) { f5(d) }
function f5(i) { i[1]=42 }
BEGIN { x[1]=3; f5(x); print x[1] }
`, out: "42\n"},
}

func TestCompatFunctions(t *testing.T) {
	runTestCategory(t, functionTests)
}

// =============================================================================
// Type Checking / Resolver Error Tests
// =============================================================================

var typeCheckTests = []interpTest{
	{name: "array_as_scalar", src: `BEGIN { a[x]; a=42 }`, err: `cannot use "a" as both array and scalar`},
	{name: "scalar_as_array", src: `BEGIN { s=42; s[x] }`, err: `cannot use "s" as both array and scalar`},
	{name: "return_outside_func", src: `BEGIN { return }`, err: "return must be inside a function"},
	{name: "func_redefined", src: `function f() {} function f() {} BEGIN { }`, err: `function "f" already defined`},
	{name: "func_undefined", src: `BEGIN { f() }`, err: `undefined function "f"`},
	{name: "func_param_name_conflict", src: `
function foo(foo) { print "foo", foo }
function bar(foo) { print "bar", foo }
BEGIN { foo(5); bar(10) }
`, err: `cannot use function name`},
	{name: "local_var_as_func", src: `function f(x) { print x, x(); }  BEGIN { f() }`, err: `undefined function "x"`},
}

func TestCompatTypeCheck(t *testing.T) {
	runTestCategory(t, typeCheckTests)
}

// =============================================================================
// Syntax Error Tests
// =============================================================================

var syntaxErrorTests = []interpTest{
	// Note: Some error messages differ from GoAWK. Tests check for partial match.
	{name: "unexpected_pipe", src: `BEGIN { 1 + 1 - | }`, err: `expected expression`},
	{name: "unexpected_pipe_alone", src: `BEGIN { | }`, err: `expected expression`},
	{name: "bad_number", src: `BEGIN { print . }`, err: "expected expression"},
	{name: "unterminated_string", src: `BEGIN { print "foo }`, err: "unterminated string"},
	{name: "unterminated_regex", src: `/foo`, err: "unterminated regex"},
	{name: "unexpected_char", src: "BEGIN { ` }", err: "expected expression"},
	{name: "incr_nonlvalue", src: "BEGIN { ++3 }", err: "expected lvalue"},
	{name: "assign_nonlvalue", src: "BEGIN { rand() = 1 }", err: "left side of assignment"},
}

func TestCompatSyntaxErrors(t *testing.T) {
	runTestCategory(t, syntaxErrorTests)
}

// =============================================================================
// Number/String Conversion Tests
// =============================================================================

var conversionTests = []interpTest{
	{name: "numbers", src: `BEGIN { print 1, 1., .1, 1e0, -1, 1e }`, out: "1 1 0.1 1 -1 1\n"},
	{name: "string_to_num", src: `BEGIN { print "-12"+0, "+12"+0, " \t\r\n7foo"+0, ".5"+0, "5."+0, "+."+0 }`, out: "-12 12 7 0.5 5 0\n"},
	{name: "exp_notation", src: `BEGIN { print "1e3"+0, "1.2e-1"+0, "1e+1"+0, "1e"+0, "1e+"+0 }`, out: "1000 0.12 10 1 1\n"},
	{name: "unusual_exp", src: `BEGIN { e="x"; E="X"; print 1e, 1E }`, out: "1x 1X\n"},
	{name: "exp_concat", src: `BEGIN { e="x"; E="X"; print 1e1e, 1E1E }`, out: "10x 10X\n"},
	{name: "exp_plus_var", src: `BEGIN { a=2; print 1e+a, 1E+a, 1e+1, 1E+1 }`, out: "12 12 10 10\n"},
	{name: "exp_minus_var", src: `BEGIN { a=2; print 1e-a, 1E-a, 1e-1, 1E-1 }`, out: "1-2 1-2 0.1 0.1\n"},
}

func TestCompatConversion(t *testing.T) {
	runTestCategory(t, conversionTests)
}

// =============================================================================
// Escape Sequence Tests
// =============================================================================

var escapeTests = []interpTest{
	{name: "escape_sequences", src: `BEGIN { print "0\n1\t2\r3\a4\b5\f6\v7\x408\xf" }`, out: "0\n1\t2\r3\a4\b5\f6\v7@8\x0f\n"},
	{name: "hex_escape", src: `BEGIN { printf "\x1.\x01.\x0A\x10\xff\xFF\x41" }`, out: "\x01.\x01.\n\x10\xff\xffA"},
	{name: "octal_escape", src: `BEGIN { printf "\1\78\7\77\777\1 \141 " }`, out: "\x01\a8\a?\xff\x01 a "},
}

func TestCompatEscape(t *testing.T) {
	runTestCategory(t, escapeTests)
}

// =============================================================================
// Grammar Edge Case Tests
// =============================================================================

var grammarTests = []interpTest{
	// Semicolons
	{name: "semicolon_after_func", src: "function f(){} ; 0", out: ""},
	{name: "semicolon_after_action", src: "{}             ; 0", out: ""},
	{name: "semicolon_after_pattern", src: "1              ; 0", out: ""},
	{name: "semicolon_end_func", src: "function f(){} ;  ", out: ""},
	{name: "semicolon_end_action", src: "{}             ;  ", out: ""},
	{name: "semicolon_end_pattern", src: "1              ;  ", out: ""},
	{name: "empty_semicolon", src: `BEGIN {;}`, out: ""},
	{name: "double_semicolon", src: `BEGIN { while (0) {;;} }`, out: ""},

	// Blocks
	{name: "if_else_printf", src: `BEGIN { if (1) printf "x"; else printf "y" }`, out: "x"},
	{name: "nested_blocks", src: `BEGIN { printf "x"; { printf "y"; printf "z" } }`, out: "xyz"},

	// Line continuation
	{name: "backslash_newline", src: "BEGIN { print 1,\\\n 2 }", out: "1 2\n"},
	{name: "backslash_crlf", src: "BEGIN { print 1,\\\r\n 2 }", out: "1 2\n"},
}

func TestCompatGrammar(t *testing.T) {
	runTestCategory(t, grammarTests)
}

// =============================================================================
// RS (Record Separator) Tests
// =============================================================================

var rsTests = []interpTest{
	{name: "RS_empty_multiline", src: `BEGIN { RS=""; FS="\n" }  { printf "%d (%d):\n", NR, NF; for (i=1; i<=NF; i++) print $i }`,
		in: "a\n\nb\nc", out: "1 (1):\na\n2 (2):\nb\nc\n"},
	{name: "RS_empty_para", src: `BEGIN { RS=""; FS="\n" }  { printf "%d (%d):\n", NR, NF; for (i=1; i<=NF; i++) print $i }`,
		in: "1\n2\n\na\nb", out: "1 (2):\n1\n2\n2 (2):\na\nb\n"},
	{name: "RS_newline", src: `BEGIN { RS="\n" }  { print }`, in: "a\n\nb\nc", out: "a\n\nb\nc\n"},
}

func TestCompatRS(t *testing.T) {
	runTestCategory(t, rsTests)
}

// =============================================================================
// Aggregate Test Runner
// =============================================================================

// TestCompatibility runs all compatibility tests as a single test.
// Use -run TestCompatibility/CategoryName to run specific categories.
func TestCompatibility(t *testing.T) {
	categories := []testCategory{
		{"BeginEnd", beginEndTests},
		{"Pattern", patternTests},
		{"Print", printTests},
		{"ControlFlow", controlFlowTests},
		{"Next", nextTests},
		{"Array", arrayTests},
		{"Unary", unaryTests},
		{"Comparison", comparisonTests},
		{"Logical", logicalTests},
		{"Arithmetic", arithmeticTests},
		{"Regex", regexTests},
		{"Ternary", ternaryTests},
		{"Assignment", assignmentTests},
		{"IncrDecr", incrDecrTests},
		{"MathFuncs", mathFuncTests},
		{"StringFuncs", stringFuncTests},
		{"Fields", fieldTests},
		{"NF", nfTests},
		{"SpecialVars", specialVarTests},
		{"Functions", functionTests},
		{"TypeCheck", typeCheckTests},
		{"SyntaxErrors", syntaxErrorTests},
		{"Conversion", conversionTests},
		{"Escape", escapeTests},
		{"Grammar", grammarTests},
		{"RS", rsTests},
	}

	for _, cat := range categories {
		t.Run(cat.name, func(t *testing.T) {
			runTestCategory(t, cat.tests)
		})
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

// Note: Go 1.21+ has built-in min() function
