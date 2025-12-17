// Package lexer provides AWK source code tokenization.
package lexer

import (
	"unicode/utf8"

	"github.com/kolkov/uawk/internal/token"
)

// Lexer tokenizes AWK source code.
type Lexer struct {
	src     []byte         // Source code
	ch      byte           // Current character (0 at EOF)
	offset  int            // Current byte offset
	pos     token.Position // Current position
	nextPos token.Position // Position of next character

	hadSpace bool        // Was there whitespace before current token?
	lastTok  token.Token // Previous token (for regex detection)
}

// New creates a new Lexer for the given source code.
func New(src []byte) *Lexer {
	l := &Lexer{
		src: src,
		nextPos: token.Position{
			Line:   1,
			Column: 1,
		},
	}
	l.next() // Initialize first character
	return l
}

// NewFromString creates a new Lexer from a string.
func NewFromString(src string) *Lexer {
	return New([]byte(src))
}

// Token represents a scanned token with its position and value.
type Token struct {
	Type  token.Token
	Pos   token.Position
	Value string
}

// Scan scans and returns the next token.
func (l *Lexer) Scan() Token {
	tok := l.scan()
	l.lastTok = tok.Type
	return tok
}

// HadSpace returns true if there was whitespace before the current token.
// Used by parser for function call detection (no space between name and paren).
func (l *Lexer) HadSpace() bool {
	return l.hadSpace
}

func (l *Lexer) scan() Token {
	l.skipWhitespace()

	// Skip comments
	if l.ch == '#' {
		l.skipComment()
	}

	// Record position
	pos := l.pos

	// EOF
	if l.ch == 0 {
		return Token{Type: token.EOF, Pos: pos}
	}

	// Single character tokens and operators
	switch l.ch {
	case '\n':
		l.next()
		return Token{Type: token.NEWLINE, Pos: pos}

	case '+':
		l.next()
		if l.ch == '+' {
			l.next()
			return Token{Type: token.INCR, Pos: pos, Value: "++"}
		}
		if l.ch == '=' {
			l.next()
			return Token{Type: token.ADD_ASSIGN, Pos: pos, Value: "+="}
		}
		return Token{Type: token.ADD, Pos: pos, Value: "+"}

	case '-':
		l.next()
		if l.ch == '-' {
			l.next()
			return Token{Type: token.DECR, Pos: pos, Value: "--"}
		}
		if l.ch == '=' {
			l.next()
			return Token{Type: token.SUB_ASSIGN, Pos: pos, Value: "-="}
		}
		return Token{Type: token.SUB, Pos: pos, Value: "-"}

	case '*':
		l.next()
		if l.ch == '=' {
			l.next()
			return Token{Type: token.MUL_ASSIGN, Pos: pos, Value: "*="}
		}
		return Token{Type: token.MUL, Pos: pos, Value: "*"}

	case '/':
		// Could be division or regex
		if l.canBeRegex() {
			return l.scanRegex(pos)
		}
		l.next()
		if l.ch == '=' {
			l.next()
			return Token{Type: token.DIV_ASSIGN, Pos: pos, Value: "/="}
		}
		return Token{Type: token.DIV, Pos: pos, Value: "/"}

	case '%':
		l.next()
		if l.ch == '=' {
			l.next()
			return Token{Type: token.MOD_ASSIGN, Pos: pos, Value: "%="}
		}
		return Token{Type: token.MOD, Pos: pos, Value: "%"}

	case '^':
		l.next()
		if l.ch == '=' {
			l.next()
			return Token{Type: token.POW_ASSIGN, Pos: pos, Value: "^="}
		}
		return Token{Type: token.POW, Pos: pos, Value: "^"}

	case '=':
		l.next()
		if l.ch == '=' {
			l.next()
			return Token{Type: token.EQUALS, Pos: pos, Value: "=="}
		}
		return Token{Type: token.ASSIGN, Pos: pos, Value: "="}

	case '!':
		l.next()
		if l.ch == '=' {
			l.next()
			return Token{Type: token.NOT_EQUALS, Pos: pos, Value: "!="}
		}
		if l.ch == '~' {
			l.next()
			return Token{Type: token.NOT_MATCH, Pos: pos, Value: "!~"}
		}
		return Token{Type: token.NOT, Pos: pos, Value: "!"}

	case '<':
		l.next()
		if l.ch == '=' {
			l.next()
			return Token{Type: token.LTE, Pos: pos, Value: "<="}
		}
		return Token{Type: token.LESS, Pos: pos, Value: "<"}

	case '>':
		l.next()
		if l.ch == '=' {
			l.next()
			return Token{Type: token.GTE, Pos: pos, Value: ">="}
		}
		if l.ch == '>' {
			l.next()
			return Token{Type: token.APPEND, Pos: pos, Value: ">>"}
		}
		return Token{Type: token.GREATER, Pos: pos, Value: ">"}

	case '&':
		l.next()
		if l.ch == '&' {
			l.next()
			return Token{Type: token.AND, Pos: pos, Value: "&&"}
		}
		return Token{Type: token.ILLEGAL, Pos: pos, Value: "unexpected '&'"}

	case '|':
		l.next()
		if l.ch == '|' {
			l.next()
			return Token{Type: token.OR, Pos: pos, Value: "||"}
		}
		return Token{Type: token.PIPE, Pos: pos, Value: "|"}

	case '~':
		l.next()
		return Token{Type: token.MATCH, Pos: pos, Value: "~"}

	case '(':
		l.next()
		return Token{Type: token.LPAREN, Pos: pos, Value: "("}
	case ')':
		l.next()
		return Token{Type: token.RPAREN, Pos: pos, Value: ")"}
	case '{':
		l.next()
		return Token{Type: token.LBRACE, Pos: pos, Value: "{"}
	case '}':
		l.next()
		return Token{Type: token.RBRACE, Pos: pos, Value: "}"}
	case '[':
		l.next()
		return Token{Type: token.LBRACKET, Pos: pos, Value: "["}
	case ']':
		l.next()
		return Token{Type: token.RBRACKET, Pos: pos, Value: "]"}
	case ',':
		l.next()
		return Token{Type: token.COMMA, Pos: pos, Value: ","}
	case ';':
		l.next()
		return Token{Type: token.SEMICOLON, Pos: pos, Value: ";"}
	case ':':
		l.next()
		return Token{Type: token.COLON, Pos: pos, Value: ":"}
	case '?':
		l.next()
		return Token{Type: token.QUESTION, Pos: pos, Value: "?"}
	case '$':
		l.next()
		return Token{Type: token.DOLLAR, Pos: pos, Value: "$"}
	case '@':
		l.next()
		return Token{Type: token.AT, Pos: pos, Value: "@"}

	case '"', '\'':
		return l.scanString(pos)

	default:
		if isDigit(l.ch) || (l.ch == '.' && l.offset < len(l.src) && isDigit(l.src[l.offset])) {
			return l.scanNumber(pos)
		}
		if isIdentStart(l.ch) {
			return l.scanIdent(pos)
		}
		ch := l.ch
		l.next()
		return Token{Type: token.ILLEGAL, Pos: pos, Value: string(ch)}
	}
}

// ScanRegex scans a regex token. Called by parser when expecting regex.
func (l *Lexer) ScanRegex() Token {
	l.skipWhitespace()
	pos := l.pos
	if l.ch == '/' {
		return l.scanRegex(pos)
	}
	return Token{Type: token.ILLEGAL, Pos: pos, Value: "expected regex"}
}

func (l *Lexer) scanRegex(pos token.Position) Token {
	l.next()              // consume opening /
	start := l.pos.Offset // Position of first regex character

	for l.ch != 0 && l.ch != '/' && l.ch != '\n' {
		if l.ch == '\\' {
			l.next() // skip escape
			if l.ch != 0 && l.ch != '\n' {
				l.next()
			}
		} else {
			l.next()
		}
	}

	if l.ch != '/' {
		return Token{Type: token.ILLEGAL, Pos: pos, Value: "unterminated regex"}
	}

	value := string(l.src[start:l.pos.Offset]) // End at closing /
	l.next()                                   // consume closing /
	return Token{Type: token.REGEX, Pos: pos, Value: value}
}

func (l *Lexer) scanString(pos token.Position) Token {
	quote := l.ch
	l.next() // consume opening quote

	var sb []byte
	for l.ch != 0 && l.ch != quote && l.ch != '\n' {
		if l.ch == '\\' {
			l.next()
			switch l.ch {
			case 'n':
				sb = append(sb, '\n')
			case 't':
				sb = append(sb, '\t')
			case 'r':
				sb = append(sb, '\r')
			case 'b':
				sb = append(sb, '\b')
			case 'f':
				sb = append(sb, '\f')
			case 'a':
				sb = append(sb, '\a')
			case 'v':
				sb = append(sb, '\v')
			case '\\':
				sb = append(sb, '\\')
			case '"':
				sb = append(sb, '"')
			case '\'':
				sb = append(sb, '\'')
			case '/':
				sb = append(sb, '/')
			case '0', '1', '2', '3', '4', '5', '6', '7':
				// Octal escape
				n := int(l.ch - '0')
				l.next()
				for i := 0; i < 2 && l.ch >= '0' && l.ch <= '7'; i++ {
					n = n*8 + int(l.ch-'0')
					l.next()
				}
				sb = append(sb, byte(n))
				continue
			case 'x':
				// Hex escape
				l.next()
				if isHexDigit(l.ch) {
					n := hexValue(l.ch)
					l.next()
					if isHexDigit(l.ch) {
						n = n*16 + hexValue(l.ch)
						l.next()
					}
					sb = append(sb, byte(n))
					continue
				}
				sb = append(sb, 'x')
				continue
			default:
				sb = append(sb, l.ch)
			}
			l.next()
		} else {
			sb = append(sb, l.ch)
			l.next()
		}
	}

	if l.ch != quote {
		return Token{Type: token.ILLEGAL, Pos: pos, Value: "unterminated string"}
	}
	l.next() // consume closing quote

	return Token{Type: token.STRING, Pos: pos, Value: string(sb)}
}

func (l *Lexer) scanNumber(pos token.Position) Token {
	start := pos.Offset // Use position offset to include first character

	// Check for hex
	if l.ch == '0' && l.offset < len(l.src) && (l.src[l.offset] == 'x' || l.src[l.offset] == 'X') {
		l.next() // 0
		l.next() // x
		for isHexDigit(l.ch) {
			l.next()
		}
		// Optional hex fraction
		if l.ch == '.' {
			l.next()
			for isHexDigit(l.ch) {
				l.next()
			}
		}
		// Optional binary exponent
		if l.ch == 'p' || l.ch == 'P' {
			l.next()
			if l.ch == '+' || l.ch == '-' {
				l.next()
			}
			for isDigit(l.ch) {
				l.next()
			}
		}
		return Token{Type: token.NUMBER, Pos: pos, Value: string(l.src[start:l.endOffset()])}
	}

	// Decimal number
	for isDigit(l.ch) {
		l.next()
	}
	if l.ch == '.' {
		l.next()
		for isDigit(l.ch) {
			l.next()
		}
	}
	// Check for exponent: only consume e/E if followed by digit or +/- then digit
	// This ensures 1e+a is parsed as 1, e, +, a (not as invalid number 1e+)
	if l.ch == 'e' || l.ch == 'E' {
		if l.hasValidExponent() {
			l.next() // consume e/E
			if l.ch == '+' || l.ch == '-' {
				l.next()
			}
			for isDigit(l.ch) {
				l.next()
			}
		}
	}

	return Token{Type: token.NUMBER, Pos: pos, Value: string(l.src[start:l.endOffset()])}
}

func (l *Lexer) scanIdent(pos token.Position) Token {
	start := pos.Offset // Use position offset to include first character
	for isIdentContinue(l.ch) {
		l.next()
	}
	name := string(l.src[start:l.endOffset()])
	return Token{Type: token.LookupIdent(name), Pos: pos, Value: name}
}

// endOffset returns the correct end offset for slicing l.src.
// At EOF, l.pos is not updated, so we use len(l.src); otherwise l.pos.Offset.
func (l *Lexer) endOffset() int {
	if l.ch == 0 {
		return len(l.src)
	}
	return l.pos.Offset
}

// hasValidExponent checks if current e/E is followed by a valid exponent.
// Returns true if next char is digit, or +/- followed by digit.
func (l *Lexer) hasValidExponent() bool {
	// We're at 'e' or 'E', look ahead without consuming
	idx := l.offset // Next char position (after e/E)
	if idx >= len(l.src) {
		return false
	}

	ch := l.src[idx]
	if isDigit(ch) {
		return true
	}
	if ch == '+' || ch == '-' {
		idx++
		if idx < len(l.src) && isDigit(l.src[idx]) {
			return true
		}
	}
	return false
}

func (l *Lexer) skipWhitespace() {
	l.hadSpace = false
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\r' || l.ch == '\\' {
		l.hadSpace = true
		if l.ch == '\\' {
			// Line continuation
			l.next()
			if l.ch == '\r' {
				l.next()
			}
			if l.ch != '\n' {
				return // Will produce error on next scan
			}
		}
		l.next()
	}
}

func (l *Lexer) skipComment() {
	for l.ch != 0 && l.ch != '\n' {
		l.next()
	}
}

func (l *Lexer) next() {
	if l.offset >= len(l.src) {
		l.ch = 0
		return
	}

	l.pos = l.nextPos

	// Handle UTF-8
	if l.src[l.offset] >= utf8.RuneSelf {
		r, size := utf8.DecodeRune(l.src[l.offset:])
		l.offset += size
		l.nextPos.Column += size
		l.nextPos.Offset = l.offset
		if r == '\n' {
			l.nextPos.Line++
			l.nextPos.Column = 1
		}
		l.ch = byte(r) // Note: multi-byte runes become single byte (simplified)
		return
	}

	l.ch = l.src[l.offset]
	l.offset++
	l.nextPos.Column++
	l.nextPos.Offset = l.offset

	if l.ch == '\n' {
		l.nextPos.Line++
		l.nextPos.Column = 1
	}
}

// canBeRegex returns true if the next / should start a regex.
func (l *Lexer) canBeRegex() bool {
	switch l.lastTok {
	case token.ILLEGAL, token.EOF, token.NEWLINE,
		token.LPAREN, token.LBRACE, token.LBRACKET,
		token.COMMA, token.SEMICOLON, token.COLON, token.QUESTION,
		token.AND, token.OR, token.NOT, token.MATCH, token.NOT_MATCH,
		token.ADD, token.SUB, token.MUL, token.DIV, token.MOD, token.POW,
		token.ASSIGN, token.ADD_ASSIGN, token.SUB_ASSIGN, token.MUL_ASSIGN,
		token.DIV_ASSIGN, token.MOD_ASSIGN, token.POW_ASSIGN,
		token.EQUALS, token.NOT_EQUALS, token.LESS, token.LTE, token.GREATER, token.GTE,
		token.PRINT, token.PRINTF, token.IF, token.WHILE, token.FOR, token.DO,
		token.RETURN, token.GETLINE, token.IN:
		return true
	default:
		return false
	}
}

// Helper functions

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isHexDigit(ch byte) bool {
	return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

func hexValue(ch byte) int {
	if ch >= '0' && ch <= '9' {
		return int(ch - '0')
	}
	if ch >= 'a' && ch <= 'f' {
		return int(ch - 'a' + 10)
	}
	return int(ch - 'A' + 10)
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentContinue(ch byte) bool {
	return isIdentStart(ch) || isDigit(ch)
}
