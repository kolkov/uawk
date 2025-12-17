package parser_test

import (
	"testing"

	"github.com/kolkov/uawk/internal/parser"
)

// FuzzParser tests the parser with random inputs to find crashes.
func FuzzParser(f *testing.F) {
	// Add seed corpus with valid AWK programs
	seeds := []string{
		// Empty and minimal
		"",
		"{}",
		"{ print }",

		// BEGIN/END blocks
		"BEGIN { print }",
		"END { print }",
		"BEGIN { x = 0 } { x++ } END { print x }",

		// Patterns
		"/foo/",
		"/foo/ { print }",
		"$1 > 0",
		"$1 > 0 { print $2 }",
		"/start/,/end/ { print }",
		"NR == 1",
		"NR == 1, NR == 10",

		// Functions
		"function f() { }",
		"function f(a) { return a }",
		"function f(a, b) { return a + b }",
		"function max(a, b) { if (a > b) return a; return b }",

		// Expressions
		"{ print 42 }",
		"{ print 3.14 }",
		"{ print 1e10 }",
		`{ print "hello" }`,
		"{ print $1 }",
		"{ print $NF }",
		"{ print $(1+2) }",
		"{ print a + b }",
		"{ print a - b }",
		"{ print a * b }",
		"{ print a / b }",
		"{ print a % b }",
		"{ print a ^ b }",
		"{ print a == b }",
		"{ print a != b }",
		"{ print a < b }",
		"{ print a <= b }",
		"{ print a > b }",
		"{ print a >= b }",
		"{ print a && b }",
		"{ print a || b }",
		"{ print !a }",
		"{ print -a }",
		"{ print ++a }",
		"{ print a++ }",
		"{ print --a }",
		"{ print a-- }",
		"{ print a ? b : c }",
		"{ x = 1 }",
		"{ x += 1 }",
		"{ x -= 1 }",
		"{ x *= 2 }",
		"{ x /= 2 }",
		"{ x %= 2 }",
		"{ x ^= 2 }",
		"{ print arr[i] }",
		"{ print arr[i, j] }",
		"{ print i in arr }",
		`{ print x ~ "pat" }`,
		`{ print x !~ "pat" }`,
		"{ print func(a, b) }",
		"{ print a b c }",
		"{ print (a + b) }",

		// Statements
		"{ if (x) print }",
		"{ if (x) print; else print y }",
		"{ if (x) { print } else { print y } }",
		"{ while (x) x-- }",
		"{ while (x > 0) { x--; print x } }",
		"{ do x++ while (x < 10) }",
		"{ for (i = 0; i < 10; i++) print i }",
		"{ for (;;) print }",
		"{ for (k in arr) print k }",
		"{ while (1) { break } }",
		"{ while (1) { continue } }",
		"{ next }",
		"{ nextfile }",
		"{ exit }",
		"{ exit 1 }",
		"{ delete arr[k] }",
		"{ delete arr }",
		`{ print "x" > "file" }`,
		`{ print "x" >> "file" }`,
		`{ print "x" | "cmd" }`,
		`{ printf "%d\n", x }`,

		// Builtins
		"{ print length }",
		"{ print length($0) }",
		"{ print substr(s, 1) }",
		"{ print substr(s, 1, 5) }",
		"{ print index(s, t) }",
		`{ print split(s, a, ":") }`,
		`{ print sprintf("%d", x) }`,
		"{ sub(/re/, r) }",
		"{ gsub(/re/, r) }",
		"{ print match(s, /re/) }",
		"{ print tolower(s) }",
		"{ print toupper(s) }",
		"{ print int(x) }",
		"{ print sqrt(x) }",
		"{ print exp(x) }",
		"{ print log(x) }",
		"{ print sin(x) }",
		"{ print cos(x) }",
		"{ print atan2(y, x) }",
		"{ print rand() }",
		"{ srand() }",
		"{ print system(cmd) }",
		"{ close(f) }",

		// Getline
		"{ getline }",
		"{ getline x }",
		`{ getline < "file" }`,
		`{ getline x < "file" }`,
		`{ "cmd" | getline }`,
		`{ "cmd" | getline x }`,

		// Complex programs
		"BEGIN { FS = \":\" } { print $1 } END { print NR }",
		"{ sum += $1 } END { print sum }",
		"{ arr[$1]++ } END { for (k in arr) print k, arr[k] }",
		"/error/ { errors++ } END { print errors }",

		// Edge cases
		"{ print 1 + 2 * 3 }",
		"{ print 2 ^ 3 ^ 4 }",
		"{ print a = b = c }",
		"{ print a || b && c }",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	// Add some invalid inputs to ensure graceful error handling
	invalid := []string{
		"{",                   // Unclosed brace
		"{ print(",            // Unclosed paren
		"{ if () print }",     // Empty condition
		"BEGIN { break }",     // Break outside loop
		"BEGIN { continue }",  // Continue outside loop
		"BEGIN { return 1 }",  // Return outside function
		"BEGIN { next }",      // Next in BEGIN
		"function f(a, a) {}", // Duplicate param
	}

	for _, inv := range invalid {
		f.Add(inv)
	}

	// Fuzz function
	f.Fuzz(func(t *testing.T, src string) {
		// Limit input size to prevent timeouts
		const maxLen = 10000
		if len(src) > maxLen {
			return
		}

		// Parser should not panic on any input
		_, _ = parser.Parse(src)

		// ParseExpr should also not panic
		_, _ = parser.ParseExpr(src)
	})
}

// FuzzParseExpr specifically tests expression parsing.
func FuzzParseExpr(f *testing.F) {
	// Seed with valid expressions
	exprs := []string{
		"42",
		"3.14",
		"1e10",
		`"hello"`,
		"x",
		"$1",
		"$NF",
		"a + b",
		"a - b",
		"a * b",
		"a / b",
		"a % b",
		"a ^ b",
		"a == b",
		"a != b",
		"a < b",
		"a <= b",
		"a > b",
		"a >= b",
		"a && b",
		"a || b",
		"!a",
		"-a",
		"+a",
		"++a",
		"a++",
		"--a",
		"a--",
		"a ? b : c",
		"a = 1",
		"a += 1",
		"arr[i]",
		"arr[i, j]",
		"i in arr",
		`x ~ "pat"`,
		`x !~ "pat"`,
		"func(a, b)",
		"a b c",
		"(a + b)",
		"length($0)",
		"substr(s, 1, 5)",
		"1 + 2 * 3",
		"2 ^ 3 ^ 4",
	}

	for _, expr := range exprs {
		f.Add(expr)
	}

	f.Fuzz(func(t *testing.T, src string) {
		const maxLen = 1000
		if len(src) > maxLen {
			return
		}
		_, _ = parser.ParseExpr(src)
	})
}
