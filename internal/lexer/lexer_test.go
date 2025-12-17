// Package lexer provides AWK source code tokenization.
package lexer

import (
	"testing"

	"github.com/kolkov/uawk/internal/token"
)

func TestScanBasicTokens(t *testing.T) {
	tests := []struct {
		input    string
		expected []token.Token
	}{
		{"+", []token.Token{token.ADD, token.EOF}},
		{"-", []token.Token{token.SUB, token.EOF}},
		{"*", []token.Token{token.MUL, token.EOF}},
		{"%", []token.Token{token.MOD, token.EOF}},
		{"^", []token.Token{token.POW, token.EOF}},
		{"++", []token.Token{token.INCR, token.EOF}},
		{"--", []token.Token{token.DECR, token.EOF}},
		{"+=", []token.Token{token.ADD_ASSIGN, token.EOF}},
		{"-=", []token.Token{token.SUB_ASSIGN, token.EOF}},
		{"*=", []token.Token{token.MUL_ASSIGN, token.EOF}},
		{"x /= 1", []token.Token{token.NAME, token.DIV_ASSIGN, token.NUMBER, token.EOF}},
		{"%=", []token.Token{token.MOD_ASSIGN, token.EOF}},
		{"^=", []token.Token{token.POW_ASSIGN, token.EOF}},
		{"=", []token.Token{token.ASSIGN, token.EOF}},
		{"==", []token.Token{token.EQUALS, token.EOF}},
		{"!=", []token.Token{token.NOT_EQUALS, token.EOF}},
		{"<", []token.Token{token.LESS, token.EOF}},
		{"<=", []token.Token{token.LTE, token.EOF}},
		{">", []token.Token{token.GREATER, token.EOF}},
		{">=", []token.Token{token.GTE, token.EOF}},
		{">>", []token.Token{token.APPEND, token.EOF}},
		{"~", []token.Token{token.MATCH, token.EOF}},
		{"!~", []token.Token{token.NOT_MATCH, token.EOF}},
		{"!", []token.Token{token.NOT, token.EOF}},
		{"&&", []token.Token{token.AND, token.EOF}},
		{"||", []token.Token{token.OR, token.EOF}},
		{"|", []token.Token{token.PIPE, token.EOF}},
		{"(", []token.Token{token.LPAREN, token.EOF}},
		{")", []token.Token{token.RPAREN, token.EOF}},
		{"{", []token.Token{token.LBRACE, token.EOF}},
		{"}", []token.Token{token.RBRACE, token.EOF}},
		{"[", []token.Token{token.LBRACKET, token.EOF}},
		{"]", []token.Token{token.RBRACKET, token.EOF}},
		{",", []token.Token{token.COMMA, token.EOF}},
		{";", []token.Token{token.SEMICOLON, token.EOF}},
		{":", []token.Token{token.COLON, token.EOF}},
		{"?", []token.Token{token.QUESTION, token.EOF}},
		{"$", []token.Token{token.DOLLAR, token.EOF}},
		{"@", []token.Token{token.AT, token.EOF}},
		{"\n", []token.Token{token.NEWLINE, token.EOF}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := NewFromString(tt.input)
			for i, exp := range tt.expected {
				tok := l.Scan()
				if tok.Type != exp {
					t.Errorf("token[%d]: expected %v, got %v", i, exp, tok.Type)
				}
			}
		})
	}
}

func TestScanKeywords(t *testing.T) {
	tests := []struct {
		input    string
		expected token.Token
	}{
		{"BEGIN", token.BEGIN},
		{"END", token.END},
		{"if", token.IF},
		{"else", token.ELSE},
		{"while", token.WHILE},
		{"for", token.FOR},
		{"do", token.DO},
		{"break", token.BREAK},
		{"continue", token.CONTINUE},
		{"function", token.FUNCTION},
		{"return", token.RETURN},
		{"delete", token.DELETE},
		{"exit", token.EXIT},
		{"next", token.NEXT},
		{"nextfile", token.NEXTFILE},
		{"print", token.PRINT},
		{"printf", token.PRINTF},
		{"getline", token.GETLINE},
		{"in", token.IN},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := NewFromString(tt.input)
			tok := l.Scan()
			if tok.Type != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, tok.Type)
			}
			if tok.Value != tt.input {
				t.Errorf("expected value %q, got %q", tt.input, tok.Value)
			}
		})
	}
}

func TestScanBuiltins(t *testing.T) {
	builtins := map[string]token.Token{
		"atan2":   token.F_ATAN2,
		"cos":     token.F_COS,
		"sin":     token.F_SIN,
		"exp":     token.F_EXP,
		"log":     token.F_LOG,
		"sqrt":    token.F_SQRT,
		"int":     token.F_INT,
		"rand":    token.F_RAND,
		"srand":   token.F_SRAND,
		"gsub":    token.F_GSUB,
		"index":   token.F_INDEX,
		"length":  token.F_LENGTH,
		"match":   token.F_MATCH,
		"split":   token.F_SPLIT,
		"sprintf": token.F_SPRINTF,
		"sub":     token.F_SUB,
		"substr":  token.F_SUBSTR,
		"tolower": token.F_TOLOWER,
		"toupper": token.F_TOUPPER,
		"close":   token.F_CLOSE,
		"fflush":  token.F_FFLUSH,
		"system":  token.F_SYSTEM,
	}

	for name, expected := range builtins {
		t.Run(name, func(t *testing.T) {
			l := NewFromString(name)
			tok := l.Scan()
			if tok.Type != expected {
				t.Errorf("expected %v for %q, got %v", expected, name, tok.Type)
			}
			if tok.Value != name {
				t.Errorf("expected value %q, got %q", name, tok.Value)
			}
		})
	}
}

func TestScanIdentifiers(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"x", "x"},
		{"foo", "foo"},
		{"_bar", "_bar"},
		{"x123", "x123"},
		{"CamelCase", "CamelCase"},
		{"snake_case", "snake_case"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := NewFromString(tt.input)
			tok := l.Scan()
			if tok.Type != token.NAME {
				t.Errorf("expected NAME, got %v", tok.Type)
			}
			if tok.Value != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tok.Value)
			}
		})
	}
}

func TestScanNumbers(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"0", "0"},
		{"123", "123"},
		{"3.14", "3.14"},
		{".5", ".5"},
		{"1.", "1."},
		{"1e10", "1e10"},
		{"1E10", "1E10"},
		{"1.5e-3", "1.5e-3"},
		{"1.5E+3", "1.5E+3"},
		{"0x1a", "0x1a"},
		{"0X1A", "0X1A"},
		{"0xABCDEF", "0xABCDEF"},
		{"0x1.5p3", "0x1.5p3"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := NewFromString(tt.input)
			tok := l.Scan()
			if tok.Type != token.NUMBER {
				t.Errorf("expected NUMBER for %q, got %v", tt.input, tok.Type)
			}
			if tok.Value != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tok.Value)
			}
		})
	}
}

func TestScanStrings(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"hello"`, "hello"},
		{`"hello world"`, "hello world"},
		{`""`, ""},
		{`"with\nnewline"`, "with\nnewline"},
		{`"with\ttab"`, "with\ttab"},
		{`"with\\backslash"`, "with\\backslash"},
		{`"with\"quote"`, "with\"quote"},
		{`"with\rcarriage"`, "with\rcarriage"},
		{`"octal\101"`, "octalA"},     // \101 = 'A'
		{`"hex\x41test"`, "hexAtest"}, // \x41 = 'A'
		{`'single quotes'`, "single quotes"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := NewFromString(tt.input)
			tok := l.Scan()
			if tok.Type != token.STRING {
				t.Errorf("expected STRING for %q, got %v (%s)", tt.input, tok.Type, tok.Value)
			}
			if tok.Value != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tok.Value)
			}
		})
	}
}

func TestScanUnterminatedString(t *testing.T) {
	l := NewFromString(`"unterminated`)
	tok := l.Scan()
	if tok.Type != token.ILLEGAL {
		t.Errorf("expected ILLEGAL for unterminated string, got %v", tok.Type)
	}
}

func TestScanRegex(t *testing.T) {
	tests := []struct {
		input    string
		tokens   []token.Token
		regexVal string
	}{
		// After operators that allow regex
		{"~ /foo/", []token.Token{token.MATCH, token.REGEX}, "foo"},
		{"!~ /bar/", []token.Token{token.NOT_MATCH, token.REGEX}, "bar"},
		{"if /test/", []token.Token{token.IF, token.REGEX}, "test"},
		// Division context - not regex
		{"x / y", []token.Token{token.NAME, token.DIV, token.NAME}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := NewFromString(tt.input)
			for i, exp := range tt.tokens {
				tok := l.Scan()
				if tok.Type != exp {
					t.Errorf("token[%d]: expected %v, got %v", i, exp, tok.Type)
				}
				if exp == token.REGEX && tok.Value != tt.regexVal {
					t.Errorf("regex value: expected %q, got %q", tt.regexVal, tok.Value)
				}
			}
		})
	}
}

func TestScanRegexEscapes(t *testing.T) {
	l := NewFromString(`~ /foo\/bar/`)
	l.Scan() // ~
	tok := l.Scan()
	if tok.Type != token.REGEX {
		t.Errorf("expected REGEX, got %v", tok.Type)
	}
	if tok.Value != `foo\/bar` {
		t.Errorf("expected %q, got %q", `foo\/bar`, tok.Value)
	}
}

func TestScanComments(t *testing.T) {
	tests := []struct {
		input    string
		expected []token.Token
	}{
		{"# comment\n", []token.Token{token.NEWLINE, token.EOF}},
		{"x # comment\n", []token.Token{token.NAME, token.NEWLINE, token.EOF}},
		{"x # comment", []token.Token{token.NAME, token.EOF}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := NewFromString(tt.input)
			for i, exp := range tt.expected {
				tok := l.Scan()
				if tok.Type != exp {
					t.Errorf("token[%d]: expected %v, got %v", i, exp, tok.Type)
				}
			}
		})
	}
}

func TestScanLineContinuation(t *testing.T) {
	input := "x +\\\n  y"
	l := NewFromString(input)

	tokens := []token.Token{token.NAME, token.ADD, token.NAME, token.EOF}
	for i, exp := range tokens {
		tok := l.Scan()
		if tok.Type != exp {
			t.Errorf("token[%d]: expected %v, got %v", i, exp, tok.Type)
		}
	}
}

func TestScanWhitespace(t *testing.T) {
	input := "  \t  x  \t  y  "
	l := NewFromString(input)

	tok1 := l.Scan()
	if tok1.Type != token.NAME || tok1.Value != "x" {
		t.Errorf("expected NAME 'x', got %v %q", tok1.Type, tok1.Value)
	}
	if !l.HadSpace() {
		t.Error("expected HadSpace() to be true for x")
	}

	tok2 := l.Scan()
	if tok2.Type != token.NAME || tok2.Value != "y" {
		t.Errorf("expected NAME 'y', got %v %q", tok2.Type, tok2.Value)
	}
	if !l.HadSpace() {
		t.Error("expected HadSpace() to be true for y")
	}
}

func TestScanHadSpaceForFunctionCalls(t *testing.T) {
	// In AWK, "f(x)" is a function call, "f (x)" is concat
	tests := []struct {
		input    string
		hasSpace bool
	}{
		{"f(", false},  // function call - no space
		{"f (", true},  // concat - has space
		{"f\t(", true}, // concat - has tab
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := NewFromString(tt.input)
			l.Scan() // f
			l.Scan() // (
			if l.HadSpace() != tt.hasSpace {
				t.Errorf("HadSpace() = %v, want %v", l.HadSpace(), tt.hasSpace)
			}
		})
	}
}

func TestScanPosition(t *testing.T) {
	input := "abc\ndef"
	l := NewFromString(input)

	tok1 := l.Scan() // abc
	if tok1.Pos.Line != 1 || tok1.Pos.Column != 1 {
		t.Errorf("abc: expected pos 1:1, got %d:%d", tok1.Pos.Line, tok1.Pos.Column)
	}

	l.Scan() // newline

	tok2 := l.Scan() // def
	if tok2.Pos.Line != 2 || tok2.Pos.Column != 1 {
		t.Errorf("def: expected pos 2:1, got %d:%d", tok2.Pos.Line, tok2.Pos.Column)
	}
}

func TestScanComplexExpression(t *testing.T) {
	input := `$1 == "hello" && $2 > 0 { print $0 }`
	expected := []struct {
		typ   token.Token
		value string
	}{
		{token.DOLLAR, "$"},
		{token.NUMBER, "1"},
		{token.EQUALS, "=="},
		{token.STRING, "hello"},
		{token.AND, "&&"},
		{token.DOLLAR, "$"},
		{token.NUMBER, "2"},
		{token.GREATER, ">"},
		{token.NUMBER, "0"},
		{token.LBRACE, "{"},
		{token.PRINT, "print"},
		{token.DOLLAR, "$"},
		{token.NUMBER, "0"},
		{token.RBRACE, "}"},
		{token.EOF, ""},
	}

	l := NewFromString(input)
	for i, exp := range expected {
		tok := l.Scan()
		if tok.Type != exp.typ {
			t.Errorf("token[%d]: expected %v, got %v", i, exp.typ, tok.Type)
		}
		if exp.value != "" && tok.Value != exp.value {
			t.Errorf("token[%d]: expected value %q, got %q", i, exp.value, tok.Value)
		}
	}
}

func TestScanAWKProgram(t *testing.T) {
	input := `
BEGIN {
	FS = ":"
	count = 0
}
/root/ {
	count++
	print "Found:", $1
}
END {
	print "Total:", count
}
`
	l := NewFromString(input)

	// Just verify no errors during scanning
	for {
		tok := l.Scan()
		if tok.Type == token.ILLEGAL {
			t.Errorf("unexpected ILLEGAL token: %s", tok.Value)
		}
		if tok.Type == token.EOF {
			break
		}
	}
}

func TestScanScanRegexMethod(t *testing.T) {
	input := " /pattern/"
	l := NewFromString(input)

	tok := l.ScanRegex()
	if tok.Type != token.REGEX {
		t.Errorf("expected REGEX, got %v", tok.Type)
	}
	if tok.Value != "pattern" {
		t.Errorf("expected 'pattern', got %q", tok.Value)
	}
}

func TestScanUnterminatedRegex(t *testing.T) {
	l := NewFromString("~ /unterminated")
	l.Scan() // ~
	tok := l.Scan()
	if tok.Type != token.ILLEGAL {
		t.Errorf("expected ILLEGAL for unterminated regex, got %v", tok.Type)
	}
}

// Benchmarks

func BenchmarkScanSimple(b *testing.B) {
	input := []byte(`{ print $1, $2, $3 }`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := New(input)
		for l.Scan().Type != token.EOF {
		}
	}
}

func BenchmarkScanComplex(b *testing.B) {
	input := []byte(`
BEGIN { FS = ":" }
$3 >= 1000 && $7 !~ /nologin/ {
	users[NR] = $1
	count++
}
END {
	for (i in users) print users[i]
	print "Total:", count
}
`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := New(input)
		for l.Scan().Type != token.EOF {
		}
	}
}

func BenchmarkScanNumbers(b *testing.B) {
	input := []byte(`123 456.789 0x1A 1e10 3.14159`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := New(input)
		for l.Scan().Type != token.EOF {
		}
	}
}

func BenchmarkScanStrings(b *testing.B) {
	input := []byte(`"hello" "world" "foo\nbar" "test\x41ing"`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := New(input)
		for l.Scan().Type != token.EOF {
		}
	}
}
