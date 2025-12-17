// Package token defines lexical tokens for AWK.
package token

//go:generate stringer -type=Token -linecomment

// Token represents a lexical token type.
type Token uint8

const (
	// Special tokens
	ILLEGAL Token = iota // <illegal>
	EOF                  // EOF
	NEWLINE              // <newline>
	CONCAT               // <concat>

	// Operators and delimiters
	operatorStart
	ADD        // +
	ADD_ASSIGN // +=
	SUB        // -
	SUB_ASSIGN // -=
	MUL        // *
	MUL_ASSIGN // *=
	DIV        // /
	DIV_ASSIGN // /=
	MOD        // %
	MOD_ASSIGN // %=
	POW        // ^
	POW_ASSIGN // ^=

	ASSIGN     // =
	EQUALS     // ==
	NOT_EQUALS // !=
	LESS       // <
	LTE        // <=
	GREATER    // >
	GTE        // >=

	AND       // &&
	OR        // ||
	NOT       // !
	MATCH     // ~
	NOT_MATCH // !~

	INCR   // ++
	DECR   // --
	APPEND // >>
	PIPE   // |

	LPAREN    // (
	RPAREN    // )
	LBRACE    // {
	RBRACE    // }
	LBRACKET  // [
	RBRACKET  // ]
	COMMA     // ,
	SEMICOLON // ;
	COLON     // :
	QUESTION  // ?
	DOLLAR    // $
	AT        // @
	operatorEnd

	// Keywords
	keywordStart
	BEGIN    // BEGIN
	END      // END
	IF       // if
	ELSE     // else
	WHILE    // while
	FOR      // for
	DO       // do
	BREAK    // break
	CONTINUE // continue
	FUNCTION // function
	RETURN   // return
	DELETE   // delete
	EXIT     // exit
	NEXT     // next
	NEXTFILE // nextfile
	GETLINE  // getline
	PRINT    // print
	PRINTF   // printf
	IN       // in
	keywordEnd

	// Built-in functions
	builtinStart
	F_ATAN2   // atan2
	F_CLOSE   // close
	F_COS     // cos
	F_EXP     // exp
	F_FFLUSH  // fflush
	F_GSUB    // gsub
	F_INDEX   // index
	F_INT     // int
	F_LENGTH  // length
	F_LOG     // log
	F_MATCH   // match
	F_RAND    // rand
	F_SIN     // sin
	F_SPLIT   // split
	F_SPRINTF // sprintf
	F_SQRT    // sqrt
	F_SRAND   // srand
	F_SUB     // sub
	F_SUBSTR  // substr
	F_SYSTEM  // system
	F_TOLOWER // tolower
	F_TOUPPER // toupper
	builtinEnd

	// Literals
	NAME   // name
	NUMBER // number
	STRING // string
	REGEX  // regex
)

// IsOperator returns true if the token is an operator.
func (t Token) IsOperator() bool {
	return t > operatorStart && t < operatorEnd
}

// IsKeyword returns true if the token is a keyword.
func (t Token) IsKeyword() bool {
	return t > keywordStart && t < keywordEnd
}

// IsBuiltin returns true if the token is a built-in function.
func (t Token) IsBuiltin() bool {
	return t > builtinStart && t < builtinEnd
}

// IsLiteral returns true if the token is a literal (name, number, string, regex).
func (t Token) IsLiteral() bool {
	return t == NAME || t == NUMBER || t == STRING || t == REGEX
}

// keywords maps keyword strings to their token types.
var keywords = map[string]Token{
	"BEGIN":    BEGIN,
	"END":      END,
	"if":       IF,
	"else":     ELSE,
	"while":    WHILE,
	"for":      FOR,
	"do":       DO,
	"break":    BREAK,
	"continue": CONTINUE,
	"function": FUNCTION,
	"return":   RETURN,
	"delete":   DELETE,
	"exit":     EXIT,
	"next":     NEXT,
	"nextfile": NEXTFILE,
	"getline":  GETLINE,
	"print":    PRINT,
	"printf":   PRINTF,
	"in":       IN,
}

// builtins maps built-in function names to their token types.
var builtins = map[string]Token{
	"atan2":   F_ATAN2,
	"close":   F_CLOSE,
	"cos":     F_COS,
	"exp":     F_EXP,
	"fflush":  F_FFLUSH,
	"gsub":    F_GSUB,
	"index":   F_INDEX,
	"int":     F_INT,
	"length":  F_LENGTH,
	"log":     F_LOG,
	"match":   F_MATCH,
	"rand":    F_RAND,
	"sin":     F_SIN,
	"split":   F_SPLIT,
	"sprintf": F_SPRINTF,
	"sqrt":    F_SQRT,
	"srand":   F_SRAND,
	"sub":     F_SUB,
	"substr":  F_SUBSTR,
	"system":  F_SYSTEM,
	"tolower": F_TOLOWER,
	"toupper": F_TOUPPER,
}

// LookupIdent returns the token type for a given identifier.
// Returns a keyword or builtin token if found, otherwise NAME.
func LookupIdent(ident string) Token {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	if tok, ok := builtins[ident]; ok {
		return tok
	}
	return NAME
}

// LookupKeyword returns the token type for a keyword, or ILLEGAL if not found.
func LookupKeyword(name string) Token {
	if tok, ok := keywords[name]; ok {
		return tok
	}
	return ILLEGAL
}

// LookupBuiltin returns the token type for a builtin function, or ILLEGAL if not found.
func LookupBuiltin(name string) Token {
	if tok, ok := builtins[name]; ok {
		return tok
	}
	return ILLEGAL
}
