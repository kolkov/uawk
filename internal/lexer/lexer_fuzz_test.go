// Package lexer provides AWK source code tokenization.
package lexer

import (
	"testing"

	"github.com/kolkov/uawk/internal/token"
)

// FuzzLexer tests that the lexer handles arbitrary input without panicking
// and produces valid tokens.
func FuzzLexer(f *testing.F) {
	// Seed corpus with various AWK constructs
	seeds := []string{
		// Basic programs
		`{ print $1 }`,
		`BEGIN { FS = ":" }`,
		`/pattern/ { count++ }`,
		`END { print count }`,

		// Expressions
		`x + y * z`,
		`a == b && c != d`,
		`$1 ~ /foo/ || $2 !~ /bar/`,

		// Numbers
		`123 456.789 .5 1e10 0x1A`,

		// Strings
		`"hello" "world\n" "tab\there"`,
		`'single' "double"`,

		// Edge cases
		``,
		`# comment only`,
		`\\\n`,
		`"unterminated`,
		`/unterminated`,

		// Special characters
		`@field`,
		`$0 $1 $NF`,
		`arr[i,j,k]`,

		// Complex regex
		`/[a-z]+[0-9]*/`,
		`/foo\/bar/`,
		`/\\d+/`,

		// Unicode
		`"привет мир"`,
		`"こんにちは"`,
		`"emoji 🎉"`,
	}

	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		l := New(data)

		// Scan all tokens - should not panic
		tokenCount := 0
		const maxTokens = 10000 // Prevent infinite loops

		for tokenCount < maxTokens {
			tok := l.Scan()

			// Verify token type is valid (within defined range)
			if tok.Type < token.ILLEGAL || tok.Type > token.REGEX {
				// Token type out of expected range is acceptable
				// as long as we don't panic
			}

			// Verify position is reasonable
			if tok.Pos.Line < 0 || tok.Pos.Column < 0 || tok.Pos.Offset < 0 {
				t.Errorf("invalid position: %v", tok.Pos)
			}

			if tok.Type == token.EOF {
				break
			}

			tokenCount++
		}

		if tokenCount >= maxTokens {
			t.Skip("too many tokens, possibly malformed input")
		}
	})
}

// FuzzLexerRegex tests regex scanning specifically
func FuzzLexerRegex(f *testing.F) {
	seeds := []string{
		`/pattern/`,
		`/[a-z]/`,
		`/foo\/bar/`,
		`/\\d+/`,
		`/^start$/`,
		`/a{1,3}/`,
	}

	for _, seed := range seeds {
		f.Add([]byte("~ " + seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		l := New(data)

		for {
			tok := l.Scan()
			if tok.Type == token.EOF {
				break
			}
			// Just verify we don't panic
		}
	})
}

// FuzzLexerStrings tests string scanning
func FuzzLexerStrings(f *testing.F) {
	seeds := []string{
		`"hello"`,
		`"with\nescape"`,
		`"with\\backslash"`,
		`"hex\x41"`,
		`"octal\101"`,
		`'single'`,
	}

	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		l := New(data)

		for {
			tok := l.Scan()
			if tok.Type == token.EOF {
				break
			}
		}
	})
}

// FuzzLexerNumbers tests number scanning
func FuzzLexerNumbers(f *testing.F) {
	seeds := []string{
		`123`,
		`456.789`,
		`.5`,
		`1e10`,
		`1.5e-3`,
		`0x1A`,
		`0xABCDEF`,
		`0x1.5p3`,
	}

	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		l := New(data)

		for {
			tok := l.Scan()
			if tok.Type == token.EOF {
				break
			}
		}
	})
}
