package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kolkov/uawk/internal/ast"
	"github.com/kolkov/uawk/internal/lexer"
	"github.com/kolkov/uawk/internal/token"
)

// tokenName returns a human-readable name for a token type.
func tokenName(t token.Token) string {
	switch t {
	case token.ILLEGAL:
		return "illegal"
	case token.EOF:
		return "end of file"
	case token.NEWLINE:
		return "newline"
	case token.ADD:
		return "+"
	case token.SUB:
		return "-"
	case token.MUL:
		return "*"
	case token.DIV:
		return "/"
	case token.MOD:
		return "%"
	case token.POW:
		return "^"
	case token.ASSIGN:
		return "="
	case token.ADD_ASSIGN:
		return "+="
	case token.SUB_ASSIGN:
		return "-="
	case token.MUL_ASSIGN:
		return "*="
	case token.DIV_ASSIGN:
		return "/="
	case token.MOD_ASSIGN:
		return "%="
	case token.POW_ASSIGN:
		return "^="
	case token.EQUALS:
		return "=="
	case token.NOT_EQUALS:
		return "!="
	case token.LESS:
		return "<"
	case token.LTE:
		return "<="
	case token.GREATER:
		return ">"
	case token.GTE:
		return ">="
	case token.AND:
		return "&&"
	case token.OR:
		return "||"
	case token.NOT:
		return "!"
	case token.MATCH:
		return "~"
	case token.NOT_MATCH:
		return "!~"
	case token.INCR:
		return "++"
	case token.DECR:
		return "--"
	case token.APPEND:
		return ">>"
	case token.PIPE:
		return "|"
	case token.LPAREN:
		return "("
	case token.RPAREN:
		return ")"
	case token.LBRACE:
		return "{"
	case token.RBRACE:
		return "}"
	case token.LBRACKET:
		return "["
	case token.RBRACKET:
		return "]"
	case token.COMMA:
		return ","
	case token.SEMICOLON:
		return ";"
	case token.COLON:
		return ":"
	case token.QUESTION:
		return "?"
	case token.DOLLAR:
		return "$"
	case token.BEGIN:
		return "BEGIN"
	case token.END:
		return "END"
	case token.IF:
		return "if"
	case token.ELSE:
		return "else"
	case token.WHILE:
		return "while"
	case token.FOR:
		return "for"
	case token.DO:
		return "do"
	case token.BREAK:
		return "break"
	case token.CONTINUE:
		return "continue"
	case token.FUNCTION:
		return "function"
	case token.RETURN:
		return "return"
	case token.DELETE:
		return "delete"
	case token.EXIT:
		return "exit"
	case token.NEXT:
		return "next"
	case token.NEXTFILE:
		return "nextfile"
	case token.GETLINE:
		return "getline"
	case token.PRINT:
		return "print"
	case token.PRINTF:
		return "printf"
	case token.IN:
		return "in"
	case token.NAME:
		return "name"
	case token.NUMBER:
		return "number"
	case token.STRING:
		return "string"
	case token.REGEX:
		return "regex"
	default:
		return fmt.Sprintf("token(%d)", t)
	}
}

// Parser is a recursive descent parser for AWK programs.
type Parser struct {
	lexer   *lexer.Lexer // Lexer instance
	tok     lexer.Token  // Current token
	prevTok lexer.Token  // Previous token (for newline handling)
	errors  ErrorList    // Accumulated errors

	// Parsing state
	inAction  bool   // true if parsing pattern-action (not BEGIN/END)
	funcName  string // current function name, empty if not in function
	loopDepth int    // nesting depth of loops (for break/continue validation)
}

// Parse parses an AWK program from source code.
// Returns the AST and any parse errors encountered.
func Parse(src string) (*ast.Program, error) {
	return ParseBytes([]byte(src))
}

// ParseBytes parses an AWK program from byte slice.
func ParseBytes(src []byte) (*ast.Program, error) {
	p := &Parser{
		lexer: lexer.New(src),
	}
	p.next() // Initialize first token

	prog := p.parseProgram()

	if err := p.errors.Err(); err != nil {
		return nil, err
	}
	return prog, nil
}

// ParseExpr parses a single expression (useful for testing).
func ParseExpr(src string) (ast.Expr, error) {
	p := &Parser{
		lexer: lexer.New([]byte(src)),
	}
	p.next()

	expr := p.parseExpr()

	if err := p.errors.Err(); err != nil {
		return nil, err
	}
	return expr, nil
}

// -----------------------------------------------------------------------------
// Token handling
// -----------------------------------------------------------------------------

// next advances to the next token.
func (p *Parser) next() {
	p.prevTok = p.tok
	p.tok = p.lexer.Scan()
}

// expect checks that the current token is tok and advances.
// If not, it records an error.
func (p *Parser) expect(tok token.Token) bool {
	if p.tok.Type != tok {
		p.error(expectedError(p.tok.Pos, tokenName(tok), p.tokenDesc()))
		return false
	}
	p.next()
	return true
}

// expectName expects a NAME token and returns its value and position.
func (p *Parser) expectName() (string, token.Position) {
	name := p.tok.Value
	pos := p.tok.Pos
	if !p.expect(token.NAME) {
		return "", pos
	}
	return name, pos
}

// match returns true if current token matches any of the given types.
func (p *Parser) match(types ...token.Token) bool {
	for _, t := range types {
		if p.tok.Type == t {
			return true
		}
	}
	return false
}

// tokenDesc returns a description of the current token for error messages.
func (p *Parser) tokenDesc() string {
	switch p.tok.Type {
	case token.NAME, token.NUMBER, token.STRING:
		return p.tok.Value
	case token.ILLEGAL:
		// ILLEGAL token's Value contains the actual error message
		return p.tok.Value
	case token.NEWLINE:
		return "newline"
	case token.EOF:
		return "end of file"
	default:
		return tokenName(p.tok.Type)
	}
}

// error records a parse error.
func (p *Parser) error(err *ParseError) {
	p.errors = append(p.errors, err)
}

// errorf records a formatted parse error at current position.
func (p *Parser) errorf(format string, args ...any) {
	p.error(errorf(p.tok.Pos, format, args...))
}

// -----------------------------------------------------------------------------
// Newline and terminator handling
// -----------------------------------------------------------------------------

// optionalNewlines skips any number of newline tokens.
func (p *Parser) optionalNewlines() {
	for p.tok.Type == token.NEWLINE {
		p.next()
	}
}

// commaNewlines expects a comma followed by optional newlines.
func (p *Parser) commaNewlines() {
	p.expect(token.COMMA)
	p.optionalNewlines()
}

// isTerminator returns true if current token is a statement terminator.
func (p *Parser) isTerminator() bool {
	return p.match(token.NEWLINE, token.SEMICOLON, token.RBRACE, token.EOF)
}

// skipTerminators skips newlines and semicolons.
func (p *Parser) skipTerminators() {
	for p.match(token.NEWLINE, token.SEMICOLON) {
		p.next()
	}
}

// -----------------------------------------------------------------------------
// Program parsing
// -----------------------------------------------------------------------------

// parseProgram parses a complete AWK program.
func (p *Parser) parseProgram() *ast.Program {
	startPos := p.tok.Pos
	prog := &ast.Program{
		StartPos: startPos,
	}

	// Terminator required after items, except:
	// 1. After last item
	// 2. After item ending with }
	needsTerminator := false

	for p.tok.Type != token.EOF {
		if needsTerminator {
			if !p.match(token.NEWLINE, token.SEMICOLON) {
				p.errorf("expected ; or newline between items")
				return prog
			}
			p.next()
			needsTerminator = false
		}
		p.optionalNewlines()

		switch p.tok.Type {
		case token.EOF:
			// End of file

		case token.BEGIN:
			p.next()
			block := p.parseBlock()
			if block != nil {
				prog.Begin = append(prog.Begin, block)
			}

		case token.END:
			p.next()
			block := p.parseBlock()
			if block != nil {
				prog.EndBlocks = append(prog.EndBlocks, block)
			}

		case token.FUNCTION:
			fn := p.parseFunction()
			if fn != nil {
				prog.Functions = append(prog.Functions, fn)
			}

		default:
			// Pattern-action rule
			p.inAction = true
			rule := p.parseRule()
			if rule != nil {
				prog.Rules = append(prog.Rules, rule)
				if rule.Action == nil {
					needsTerminator = true
				}
			}
			p.inAction = false
		}
	}

	prog.EndPos = p.tok.Pos
	return prog
}

// parseRule parses a pattern-action rule.
func (p *Parser) parseRule() *ast.Rule {
	startPos := p.tok.Pos
	rule := &ast.Rule{StartPos: startPos}

	// Parse pattern (optional)
	if !p.match(token.LBRACE, token.EOF) {
		pattern := p.parseExpr()

		// Check for range pattern: pattern, pattern
		if p.tok.Type == token.COMMA && !p.match(token.LBRACE, token.EOF, token.NEWLINE, token.SEMICOLON) {
			p.next()
			p.optionalNewlines()
			pattern2 := p.parseExpr()
			pattern = &ast.CommaExpr{
				BaseExpr: ast.MakeBaseExpr(pattern.Pos(), pattern2.End()),
				Left:     pattern,
				Right:    pattern2,
			}
		}
		rule.Pattern = pattern
	}

	// Parse action (optional)
	if p.tok.Type == token.LBRACE {
		rule.Action = p.parseBlock()
	}

	rule.EndPos = p.tok.Pos
	return rule
}

// parseFunction parses a function declaration.
func (p *Parser) parseFunction() *ast.FuncDecl {
	startPos := p.tok.Pos
	p.expect(token.FUNCTION) // consume 'function'

	name, namePos := p.expectName()
	if name == "" {
		return nil
	}

	p.expect(token.LPAREN)

	// Parse parameters
	var params []string
	numParams := 0
	seen := make(map[string]bool)
	first := true

	for p.tok.Type != token.RPAREN && p.tok.Type != token.EOF {
		if !first {
			p.commaNewlines()
		}
		first = false

		paramName := p.tok.Value
		if paramName == name {
			p.errorf("cannot use function name %q as parameter", name)
		}
		if seen[paramName] {
			p.errorf("duplicate parameter %q", paramName)
		}
		seen[paramName] = true
		p.expect(token.NAME)
		params = append(params, paramName)
		numParams++
	}

	p.expect(token.RPAREN)
	p.optionalNewlines()

	// Parse body
	p.funcName = name
	body := p.parseBlock()
	p.funcName = ""

	return &ast.FuncDecl{
		BaseDecl:  ast.MakeBaseDecl(startPos, p.tok.Pos),
		Name:      name,
		Params:    params,
		NumParams: numParams,
		Body:      body,
		NamePos:   namePos,
	}
}

// parseBlock parses a block statement { ... }.
func (p *Parser) parseBlock() *ast.BlockStmt {
	startPos := p.tok.Pos
	if !p.expect(token.LBRACE) {
		return nil
	}
	p.optionalNewlines()

	var stmts []ast.Stmt
	for p.tok.Type != token.RBRACE && p.tok.Type != token.EOF {
		if p.match(token.SEMICOLON, token.NEWLINE) {
			p.next()
			continue
		}
		stmt := p.parseStmt()
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
	}

	endPos := p.tok.Pos
	p.expect(token.RBRACE)

	// Skip trailing semicolon
	if p.tok.Type == token.SEMICOLON {
		p.next()
	}

	return &ast.BlockStmt{
		BaseStmt: ast.MakeBaseStmt(startPos, endPos),
		Stmts:    stmts,
	}
}

// -----------------------------------------------------------------------------
// Statement parsing
// -----------------------------------------------------------------------------

// parseStmt parses any statement.
func (p *Parser) parseStmt() ast.Stmt {
	startPos := p.tok.Pos

	var stmt ast.Stmt

	switch p.tok.Type {
	case token.IF:
		stmt = p.parseIfStmt()

	case token.WHILE:
		stmt = p.parseWhileStmt()

	case token.FOR:
		stmt = p.parseForStmt()

	case token.DO:
		stmt = p.parseDoWhileStmt()

	case token.BREAK:
		if p.loopDepth == 0 {
			p.errorf("break must be inside a loop")
		}
		p.next()
		stmt = &ast.BreakStmt{BaseStmt: ast.MakeBaseStmt(startPos, p.tok.Pos)}

	case token.CONTINUE:
		if p.loopDepth == 0 {
			p.errorf("continue must be inside a loop")
		}
		p.next()
		stmt = &ast.ContinueStmt{BaseStmt: ast.MakeBaseStmt(startPos, p.tok.Pos)}

	case token.NEXT:
		if !p.inAction && p.funcName == "" {
			p.errorf("next cannot be inside BEGIN or END")
		}
		p.next()
		stmt = &ast.NextStmt{BaseStmt: ast.MakeBaseStmt(startPos, p.tok.Pos)}

	case token.NEXTFILE:
		if !p.inAction && p.funcName == "" {
			p.errorf("nextfile cannot be inside BEGIN or END")
		}
		p.next()
		stmt = &ast.NextFileStmt{BaseStmt: ast.MakeBaseStmt(startPos, p.tok.Pos)}

	case token.EXIT:
		p.next()
		var code ast.Expr
		if !p.isTerminator() {
			code = p.parseExpr()
		}
		stmt = &ast.ExitStmt{
			BaseStmt: ast.MakeBaseStmt(startPos, p.tok.Pos),
			Code:     code,
		}

	case token.RETURN:
		if p.funcName == "" {
			p.errorf("return must be inside a function")
		}
		p.next()
		var value ast.Expr
		if !p.isTerminator() {
			value = p.parseExpr()
		}
		stmt = &ast.ReturnStmt{
			BaseStmt: ast.MakeBaseStmt(startPos, p.tok.Pos),
			Value:    value,
		}

	case token.DELETE:
		stmt = p.parseDeleteStmt()

	case token.PRINT, token.PRINTF:
		stmt = p.parsePrintStmt()

	case token.LBRACE:
		stmt = p.parseBlock()

	default:
		// Expression statement
		stmt = p.parseSimpleStmt()
	}

	return stmt
}

// parseSimpleStmt parses a simple statement (expression or print).
func (p *Parser) parseSimpleStmt() ast.Stmt {
	startPos := p.tok.Pos

	switch p.tok.Type {
	case token.PRINT, token.PRINTF:
		return p.parsePrintStmt()

	case token.DELETE:
		return p.parseDeleteStmt()

	default:
		expr := p.parseExpr()
		return &ast.ExprStmt{
			BaseStmt: ast.MakeBaseStmt(startPos, p.tok.Pos),
			Expr:     expr,
		}
	}
}

// parseIfStmt parses an if statement.
func (p *Parser) parseIfStmt() *ast.IfStmt {
	startPos := p.tok.Pos
	p.next() // consume 'if'

	p.expect(token.LPAREN)
	cond := p.parseExpr()
	p.expect(token.RPAREN)
	p.optionalNewlines()

	then := p.parseStmts()
	p.skipTerminators() // Skip ; or newlines between then and else

	var elseStmt ast.Stmt
	if p.tok.Type == token.ELSE {
		p.next()
		p.optionalNewlines()
		elseStmt = p.parseStmts()
	}

	return &ast.IfStmt{
		BaseStmt: ast.MakeBaseStmt(startPos, p.tok.Pos),
		Cond:     cond,
		Then:     then,
		Else:     elseStmt,
	}
}

// parseWhileStmt parses a while statement.
func (p *Parser) parseWhileStmt() *ast.WhileStmt {
	startPos := p.tok.Pos
	p.next() // consume 'while'

	p.expect(token.LPAREN)
	cond := p.parseExpr()
	p.expect(token.RPAREN)
	p.optionalNewlines()

	body := p.parseLoopBody()

	return &ast.WhileStmt{
		BaseStmt: ast.MakeBaseStmt(startPos, p.tok.Pos),
		Cond:     cond,
		Body:     body,
	}
}

// parseDoWhileStmt parses a do-while statement.
func (p *Parser) parseDoWhileStmt() *ast.DoWhileStmt {
	startPos := p.tok.Pos
	p.next() // consume 'do'
	p.optionalNewlines()

	body := p.parseLoopBody()
	p.optionalNewlines()

	p.expect(token.WHILE)
	p.expect(token.LPAREN)
	cond := p.parseExpr()
	p.expect(token.RPAREN)

	return &ast.DoWhileStmt{
		BaseStmt: ast.MakeBaseStmt(startPos, p.tok.Pos),
		Body:     body,
		Cond:     cond,
	}
}

// parseForStmt parses a for or for-in statement.
func (p *Parser) parseForStmt() ast.Stmt {
	startPos := p.tok.Pos
	p.next() // consume 'for'
	p.expect(token.LPAREN)

	// Try to parse first part
	var pre ast.Stmt
	if p.tok.Type != token.SEMICOLON {
		pre = p.parseSimpleStmt()
	}

	// Check for for-in: for (var in array)
	if pre != nil && p.tok.Type == token.RPAREN {
		p.next() // consume ')'
		p.optionalNewlines()

		// Must be: for (var in array)
		exprStmt, ok := pre.(*ast.ExprStmt)
		if !ok {
			p.errorf("expected 'for (var in array)'")
			return nil
		}
		inExpr, ok := exprStmt.Expr.(*ast.InExpr)
		if !ok {
			p.errorf("expected 'for (var in array)'")
			return nil
		}
		if len(inExpr.Index) != 1 {
			p.errorf("expected single variable in for-in")
			return nil
		}
		varExpr, ok := inExpr.Index[0].(*ast.Ident)
		if !ok {
			p.errorf("expected variable name in for-in")
			return nil
		}

		body := p.parseLoopBody()

		return &ast.ForInStmt{
			BaseStmt: ast.MakeBaseStmt(startPos, p.tok.Pos),
			Var:      varExpr,
			Array:    inExpr.Array,
			Body:     body,
		}
	}

	// C-style for loop: for (init; cond; post)
	p.expect(token.SEMICOLON)
	p.optionalNewlines()

	var cond ast.Expr
	if p.tok.Type != token.SEMICOLON {
		cond = p.parseExpr()
	}
	p.expect(token.SEMICOLON)
	p.optionalNewlines()

	var post ast.Stmt
	if p.tok.Type != token.RPAREN {
		post = p.parseSimpleStmt()
	}
	p.expect(token.RPAREN)
	p.optionalNewlines()

	body := p.parseLoopBody()

	return &ast.ForStmt{
		BaseStmt: ast.MakeBaseStmt(startPos, p.tok.Pos),
		Init:     pre,
		Cond:     cond,
		Post:     post,
		Body:     body,
	}
}

// parseLoopBody parses a loop body with proper loop depth tracking.
func (p *Parser) parseLoopBody() ast.Stmt {
	p.loopDepth++
	stmt := p.parseStmts()
	p.loopDepth--
	return stmt
}

// parseStmts parses one or more statements (block or single).
func (p *Parser) parseStmts() ast.Stmt {
	switch p.tok.Type {
	case token.SEMICOLON:
		p.next()
		return nil
	case token.LBRACE:
		return p.parseBlock()
	default:
		return p.parseStmt()
	}
}

// parseDeleteStmt parses a delete statement.
func (p *Parser) parseDeleteStmt() *ast.DeleteStmt {
	startPos := p.tok.Pos
	p.next() // consume 'delete'

	name, _ := p.expectName()
	arrayExpr := &ast.Ident{
		BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
		Name:     name,
	}

	var index []ast.Expr
	if p.tok.Type == token.LBRACKET {
		p.next()
		index = p.parseExprList(p.parseExpr)
		if len(index) == 0 {
			p.errorf("expected expression in delete index")
		}
		p.expect(token.RBRACKET)
	}

	return &ast.DeleteStmt{
		BaseStmt: ast.MakeBaseStmt(startPos, p.tok.Pos),
		Array:    arrayExpr,
		Index:    index,
	}
}

// parsePrintStmt parses a print or printf statement.
func (p *Parser) parsePrintStmt() *ast.PrintStmt {
	startPos := p.tok.Pos
	isPrintf := p.tok.Type == token.PRINTF
	p.next()

	// Parse arguments
	args := p.parseExprList(p.parsePrintExpr)

	// Handle parenthesized arguments
	if len(args) == 1 {
		if group, ok := args[0].(*ast.GroupExpr); ok {
			// print(a, b) -> treat inner comma as separate args
			if concat, ok := group.Expr.(*ast.ConcatExpr); ok {
				args = concat.Exprs
			}
		}
	}

	// Parse redirect
	redirect := token.ILLEGAL
	var dest ast.Expr
	if p.match(token.GREATER, token.APPEND, token.PIPE) {
		redirect = p.tok.Type
		p.next()
		dest = p.parseExpr()
	}

	if isPrintf && len(args) == 0 {
		p.errorf("printf requires at least one argument")
	}

	return &ast.PrintStmt{
		BaseStmt: ast.MakeBaseStmt(startPos, p.tok.Pos),
		Printf:   isPrintf,
		Args:     args,
		Redirect: redirect,
		Dest:     dest,
	}
}

// -----------------------------------------------------------------------------
// Expression parsing
// -----------------------------------------------------------------------------

// parseExpr parses a full expression.
func (p *Parser) parseExpr() ast.Expr {
	return p.parseAssign(p.parseGetline)
}

// parsePrintExpr parses an expression in print context (no > comparison).
func (p *Parser) parsePrintExpr() ast.Expr {
	return p.parseAssign(p.parsePrintCond)
}

// parseGetline handles the special "expr | getline [var]" case.
func (p *Parser) parseGetline() ast.Expr {
	expr := p.parseCond()

	// Check for: expr | getline [var]
	if p.tok.Type == token.PIPE {
		p.next()
		if p.tok.Type == token.GETLINE {
			p.next()
			target := p.parseOptionalLValue()
			return &ast.GetlineExpr{
				BaseExpr: ast.MakeBaseExpr(expr.Pos(), p.tok.Pos),
				Command:  expr,
				Target:   target,
			}
		}
		// Not getline, continue as binary OR... but PIPE is not OR
		// This is an error case
		p.errorf("expected getline after |")
	}

	return expr
}

// parseAssign parses assignment expressions.
func (p *Parser) parseAssign(higher func() ast.Expr) ast.Expr {
	startPos := p.tok.Pos
	expr := higher()
	if expr == nil {
		return nil
	}

	if p.match(token.ASSIGN, token.ADD_ASSIGN, token.SUB_ASSIGN,
		token.MUL_ASSIGN, token.DIV_ASSIGN, token.MOD_ASSIGN, token.POW_ASSIGN) {

		op := p.tok.Type
		p.next()
		right := p.parseAssign(higher)
		if right == nil {
			return expr
		}

		if !ast.IsLValue(expr) {
			// Try to handle weird cases like "1 && x=1"
			if binary, ok := expr.(*ast.BinaryExpr); ok && ast.IsLValue(binary.Right) {
				switch binary.Op {
				case token.AND, token.OR, token.MATCH, token.NOT_MATCH,
					token.EQUALS, token.NOT_EQUALS, token.LESS, token.LTE, token.GTE, token.GREATER:
					assign := p.makeAssign(binary.Right, op, right)
					return &ast.BinaryExpr{
						BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
						Left:     binary.Left,
						Op:       binary.Op,
						Right:    assign,
					}
				}
			}
			p.errorf("left side of assignment must be a variable, field, or array element")
			return expr
		}
		return p.makeAssign(expr, op, right)
	}
	return expr
}

// makeAssign creates an assignment expression.
func (p *Parser) makeAssign(left ast.Expr, op token.Token, right ast.Expr) ast.Expr {
	return &ast.AssignExpr{
		BaseExpr: ast.MakeBaseExpr(left.Pos(), right.End()),
		Left:     left,
		Op:       op,
		Right:    right,
	}
}

// parseCond parses a ternary conditional expression.
func (p *Parser) parseCond() ast.Expr {
	return p._parseCond(p.parseOr)
}

func (p *Parser) parsePrintCond() ast.Expr {
	return p._parseCond(p.parsePrintOr)
}

func (p *Parser) _parseCond(higher func() ast.Expr) ast.Expr {
	expr := higher()
	if expr == nil {
		return nil
	}

	if p.tok.Type == token.QUESTION {
		p.next()
		p.optionalNewlines()
		then := p.parseExpr()
		p.expect(token.COLON)
		p.optionalNewlines()
		els := p.parseExpr()
		if then == nil || els == nil {
			return expr
		}
		return &ast.TernaryExpr{
			BaseExpr: ast.MakeBaseExpr(expr.Pos(), els.End()),
			Cond:     expr,
			Then:     then,
			Else:     els,
		}
	}
	return expr
}

// parseOr parses || expressions.
func (p *Parser) parseOr() ast.Expr {
	return p.parseBinaryLeft(p.parseAnd, true, token.OR)
}

func (p *Parser) parsePrintOr() ast.Expr {
	return p.parseBinaryLeft(p.parsePrintAnd, true, token.OR)
}

// parseAnd parses && expressions.
func (p *Parser) parseAnd() ast.Expr {
	return p.parseBinaryLeft(p.parseIn, true, token.AND)
}

func (p *Parser) parsePrintAnd() ast.Expr {
	return p.parseBinaryLeft(p.parsePrintIn, true, token.AND)
}

// parseIn parses "in" expressions.
func (p *Parser) parseIn() ast.Expr {
	return p._parseIn(p.parseMatch)
}

func (p *Parser) parsePrintIn() ast.Expr {
	return p._parseIn(p.parsePrintMatch)
}

func (p *Parser) _parseIn(higher func() ast.Expr) ast.Expr {
	expr := higher()
	if expr == nil {
		return nil
	}

	for p.tok.Type == token.IN {
		p.next()
		name, namePos := p.expectName()
		arrayExpr := &ast.Ident{
			BaseExpr: ast.MakeBaseExpr(namePos, p.tok.Pos),
			Name:     name,
		}
		expr = &ast.InExpr{
			BaseExpr: ast.MakeBaseExpr(expr.Pos(), p.tok.Pos),
			Index:    []ast.Expr{expr},
			Array:    arrayExpr,
		}
	}
	return expr
}

// parseMatch parses ~ and !~ expressions.
func (p *Parser) parseMatch() ast.Expr {
	return p._parseMatch(p.parseCompare)
}

func (p *Parser) parsePrintMatch() ast.Expr {
	return p._parseMatch(p.parsePrintCompare)
}

func (p *Parser) _parseMatch(higher func() ast.Expr) ast.Expr {
	expr := higher()
	if expr == nil {
		return nil
	}

	if p.match(token.MATCH, token.NOT_MATCH) {
		op := p.tok.Type
		p.next()
		right := p.parseRegexOrExpr(higher)
		if right == nil {
			return expr
		}
		return &ast.MatchExpr{
			BaseExpr: ast.MakeBaseExpr(expr.Pos(), right.End()),
			Expr:     expr,
			Op:       op,
			Pattern:  right,
		}
	}
	return expr
}

// parseCompare parses comparison expressions.
func (p *Parser) parseCompare() ast.Expr {
	return p._parseCompare(token.EQUALS, token.NOT_EQUALS, token.LESS, token.LTE, token.GTE, token.GREATER)
}

func (p *Parser) parsePrintCompare() ast.Expr {
	// In print context, > is redirect, not comparison
	return p._parseCompare(token.EQUALS, token.NOT_EQUALS, token.LESS, token.LTE, token.GTE)
}

func (p *Parser) _parseCompare(ops ...token.Token) ast.Expr {
	expr := p.parseConcat()
	if expr == nil {
		return nil
	}

	if p.match(ops...) {
		op := p.tok.Type
		p.next()
		right := p.parseConcat() // Not associative
		if right == nil {
			return expr
		}
		return &ast.BinaryExpr{
			BaseExpr: ast.MakeBaseExpr(expr.Pos(), right.End()),
			Left:     expr,
			Op:       op,
			Right:    right,
		}
	}
	return expr
}

// parseConcat parses implicit concatenation.
func (p *Parser) parseConcat() ast.Expr {
	expr := p.parseAdd()
	if expr == nil {
		return nil
	}

	// Concatenation: adjacent expressions without operator
	if p.canStartPrimary() {
		exprs := []ast.Expr{expr}
		for p.canStartPrimary() {
			next := p.parseAdd()
			if next == nil {
				break
			}
			exprs = append(exprs, next)
		}
		if len(exprs) > 1 {
			return &ast.ConcatExpr{
				BaseExpr: ast.MakeBaseExpr(exprs[0].Pos(), exprs[len(exprs)-1].End()),
				Exprs:    exprs,
			}
		}
	}
	return expr
}

// canStartPrimary returns true if current token can start a primary expression.
func (p *Parser) canStartPrimary() bool {
	switch p.tok.Type {
	case token.DOLLAR, token.AT, token.NOT, token.NAME, token.NUMBER, token.STRING,
		token.LPAREN, token.INCR, token.DECR, token.GETLINE:
		return true
	default:
		return p.tok.Type.IsBuiltin()
	}
}

// parseAdd parses + and - expressions.
func (p *Parser) parseAdd() ast.Expr {
	return p.parseBinaryLeft(p.parseMul, false, token.ADD, token.SUB)
}

// parseMul parses *, /, and % expressions.
func (p *Parser) parseMul() ast.Expr {
	return p.parseBinaryLeft(p.parsePow, false, token.MUL, token.DIV, token.MOD)
}

// parsePow parses ^ expressions (right-associative).
func (p *Parser) parsePow() ast.Expr {
	expr := p.parsePostIncr()
	if expr == nil {
		return nil
	}

	if p.tok.Type == token.POW {
		p.next()
		right := p.parsePow() // Right-associative
		if right == nil {
			return expr
		}
		return &ast.BinaryExpr{
			BaseExpr: ast.MakeBaseExpr(expr.Pos(), right.End()),
			Left:     expr,
			Op:       token.POW,
			Right:    right,
		}
	}
	return expr
}

// parsePostIncr parses postfix ++ and -- expressions.
func (p *Parser) parsePostIncr() ast.Expr {
	expr := p.parsePrimary()
	if expr == nil {
		return nil
	}

	if p.match(token.INCR, token.DECR) && ast.IsLValue(expr) {
		op := p.tok.Type
		p.next()
		return &ast.UnaryExpr{
			BaseExpr: ast.MakeBaseExpr(expr.Pos(), p.tok.Pos),
			Op:       op,
			Expr:     expr,
			Post:     true,
		}
	}
	return expr
}

// parsePrimary parses primary expressions.
func (p *Parser) parsePrimary() ast.Expr {
	startPos := p.tok.Pos

	switch p.tok.Type {
	case token.NUMBER:
		s := strings.TrimRight(p.tok.Value, "eE")
		n, _ := strconv.ParseFloat(s, 64)
		p.next()
		return &ast.NumLit{
			BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
			Value:    n,
			Raw:      p.prevTok.Value,
		}

	case token.STRING:
		s := p.tok.Value
		p.next()
		return &ast.StrLit{
			BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
			Value:    s,
		}

	case token.REGEX:
		pattern := p.tok.Value
		p.next()
		return &ast.RegexLit{
			BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
			Pattern:  pattern,
		}

	case token.DIV, token.DIV_ASSIGN:
		// Regex in expression position
		tok := p.lexer.ScanRegex()
		if tok.Type == token.ILLEGAL {
			p.errorf("%s", tok.Value)
			return nil
		}
		pattern := tok.Value
		p.next()
		return &ast.RegexLit{
			BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
			Pattern:  pattern,
		}

	case token.DOLLAR:
		p.next()
		index := p.parsePrimary()
		if index == nil {
			return nil
		}
		expr := &ast.FieldExpr{
			BaseExpr: ast.MakeBaseExpr(startPos, index.End()),
			Index:    index,
		}
		// Field expressions can have post-increment
		if p.match(token.INCR, token.DECR) {
			op := p.tok.Type
			p.next()
			return &ast.UnaryExpr{
				BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
				Op:       op,
				Expr:     expr,
				Post:     true,
			}
		}
		return expr

	case token.AT:
		p.next()
		return p.parsePrimary() // Named field - simplified

	case token.NOT:
		p.next()
		expr := p.parsePow()
		if expr == nil {
			return nil
		}
		return &ast.UnaryExpr{
			BaseExpr: ast.MakeBaseExpr(startPos, expr.End()),
			Op:       token.NOT,
			Expr:     expr,
		}

	case token.ADD, token.SUB:
		op := p.tok.Type
		p.next()
		expr := p.parsePow()
		if expr == nil {
			return nil
		}
		return &ast.UnaryExpr{
			BaseExpr: ast.MakeBaseExpr(startPos, expr.End()),
			Op:       op,
			Expr:     expr,
		}

	case token.INCR, token.DECR:
		op := p.tok.Type
		p.next()
		expr := p.parseOptionalLValue()
		if expr == nil {
			p.errorf("expected lvalue after %s", tokenName(op))
			return nil
		}
		return &ast.UnaryExpr{
			BaseExpr: ast.MakeBaseExpr(startPos, expr.End()),
			Op:       op,
			Expr:     expr,
			Post:     false,
		}

	case token.NAME:
		name, namePos := p.expectName()

		// Array index: name[expr]
		if p.tok.Type == token.LBRACKET {
			p.next()
			index := p.parseExprList(p.parseExpr)
			if len(index) == 0 {
				p.errorf("expected expression in array index")
			}
			p.expect(token.RBRACKET)
			return &ast.IndexExpr{
				BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
				Array:    &ast.Ident{BaseExpr: ast.MakeBaseExpr(namePos, p.tok.Pos), Name: name},
				Index:    index,
			}
		}

		// Function call: name(args) - no space between name and (
		if p.tok.Type == token.LPAREN && !p.lexer.HadSpace() {
			return p.parseUserCall(name, namePos)
		}

		// Simple variable
		return &ast.Ident{
			BaseExpr: ast.MakeBaseExpr(namePos, p.tok.Pos),
			Name:     name,
		}

	case token.LPAREN:
		p.next()
		exprs := p.parseExprList(p.parseExpr)

		switch len(exprs) {
		case 0:
			p.errorf("expected expression, not %s", p.tokenDesc())
			p.expect(token.RPAREN)
			return nil
		case 1:
			p.expect(token.RPAREN)
			return &ast.GroupExpr{
				BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
				Expr:     exprs[0],
			}
		default:
			// Multi-dimensional array "in" check
			p.expect(token.RPAREN)
			if p.tok.Type == token.IN {
				p.next()
				name, namePos := p.expectName()
				return &ast.InExpr{
					BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
					Index:    exprs,
					Array:    &ast.Ident{BaseExpr: ast.MakeBaseExpr(namePos, p.tok.Pos), Name: name},
				}
			}
			// Treat as grouped concatenation
			return &ast.GroupExpr{
				BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
				Expr: &ast.ConcatExpr{
					BaseExpr: ast.MakeBaseExpr(exprs[0].Pos(), exprs[len(exprs)-1].End()),
					Exprs:    exprs,
				},
			}
		}

	case token.GETLINE:
		return p.parseGetlineExpr()

	default:
		// Built-in function calls
		if p.tok.Type.IsBuiltin() {
			return p.parseBuiltinCall()
		}

		p.errorf("expected expression, got %s", p.tokenDesc())
		p.next()
		return nil
	}
}

// parseGetlineExpr parses standalone getline expression.
func (p *Parser) parseGetlineExpr() ast.Expr {
	startPos := p.tok.Pos
	p.next() // consume 'getline'

	target := p.parseOptionalLValue()

	var file ast.Expr
	if p.tok.Type == token.LESS {
		p.next()
		file = p.parsePrimary()
	}

	return &ast.GetlineExpr{
		BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
		Target:   target,
		File:     file,
	}
}

// parseOptionalLValue parses an optional lvalue (for getline target, ++/--, etc).
func (p *Parser) parseOptionalLValue() ast.Expr {
	switch p.tok.Type {
	case token.NAME:
		// Check if it's a function call
		if p.lexer.HadSpace() || p.tok.Type != token.LPAREN {
			name, namePos := p.expectName()
			if p.tok.Type == token.LBRACKET {
				p.next()
				index := p.parseExprList(p.parseExpr)
				if len(index) == 0 {
					p.errorf("expected expression in array index")
				}
				p.expect(token.RBRACKET)
				return &ast.IndexExpr{
					BaseExpr: ast.MakeBaseExpr(namePos, p.tok.Pos),
					Array:    &ast.Ident{BaseExpr: ast.MakeBaseExpr(namePos, p.tok.Pos), Name: name},
					Index:    index,
				}
			}
			return &ast.Ident{
				BaseExpr: ast.MakeBaseExpr(namePos, p.tok.Pos),
				Name:     name,
			}
		}
		return nil

	case token.DOLLAR:
		startPos := p.tok.Pos
		p.next()
		index := p.parsePrimary()
		if index == nil {
			return nil
		}
		return &ast.FieldExpr{
			BaseExpr: ast.MakeBaseExpr(startPos, index.End()),
			Index:    index,
		}

	default:
		return nil
	}
}

// parseUserCall parses a user-defined function call.
func (p *Parser) parseUserCall(name string, namePos token.Position) *ast.CallExpr {
	p.expect(token.LPAREN)

	var args []ast.Expr
	first := true
	for !p.match(token.NEWLINE, token.RPAREN, token.EOF) {
		if !first {
			p.commaNewlines()
		}
		first = false
		args = append(args, p.parseExpr())
	}
	p.expect(token.RPAREN)

	return &ast.CallExpr{
		BaseExpr: ast.MakeBaseExpr(namePos, p.tok.Pos),
		Name:     name,
		Args:     args,
	}
}

// parseBuiltinCall parses a built-in function call.
func (p *Parser) parseBuiltinCall() ast.Expr {
	startPos := p.tok.Pos
	fn := p.tok.Type
	p.next()

	switch fn {
	case token.F_LENGTH:
		// length can be called without parens
		var args []ast.Expr
		if p.tok.Type == token.LPAREN {
			p.next()
			if p.tok.Type != token.RPAREN {
				args = append(args, p.parseExpr())
			}
			p.expect(token.RPAREN)
		}
		return &ast.BuiltinExpr{
			BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
			Func:     fn,
			Args:     args,
		}

	case token.F_RAND:
		p.expect(token.LPAREN)
		p.expect(token.RPAREN)
		return &ast.BuiltinExpr{
			BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
			Func:     fn,
			Args:     nil,
		}

	case token.F_SRAND, token.F_FFLUSH:
		p.expect(token.LPAREN)
		var args []ast.Expr
		if p.tok.Type != token.RPAREN {
			args = append(args, p.parseExpr())
		}
		p.expect(token.RPAREN)
		return &ast.BuiltinExpr{
			BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
			Func:     fn,
			Args:     args,
		}

	case token.F_COS, token.F_SIN, token.F_EXP, token.F_LOG, token.F_SQRT,
		token.F_INT, token.F_TOLOWER, token.F_TOUPPER, token.F_SYSTEM, token.F_CLOSE:
		// 1-argument functions
		p.expect(token.LPAREN)
		arg := p.parseExpr()
		p.expect(token.RPAREN)
		return &ast.BuiltinExpr{
			BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
			Func:     fn,
			Args:     []ast.Expr{arg},
		}

	case token.F_ATAN2, token.F_INDEX:
		// 2-argument functions
		p.expect(token.LPAREN)
		arg1 := p.parseExpr()
		p.commaNewlines()
		arg2 := p.parseExpr()
		p.expect(token.RPAREN)
		return &ast.BuiltinExpr{
			BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
			Func:     fn,
			Args:     []ast.Expr{arg1, arg2},
		}

	case token.F_SUBSTR:
		p.expect(token.LPAREN)
		str := p.parseExpr()
		p.commaNewlines()
		start := p.parseExpr()
		args := []ast.Expr{str, start}
		if p.tok.Type == token.COMMA {
			p.commaNewlines()
			args = append(args, p.parseExpr())
		}
		p.expect(token.RPAREN)
		return &ast.BuiltinExpr{
			BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
			Func:     fn,
			Args:     args,
		}

	case token.F_SPRINTF:
		p.expect(token.LPAREN)
		args := []ast.Expr{p.parseExpr()}
		for p.tok.Type == token.COMMA {
			p.commaNewlines()
			args = append(args, p.parseExpr())
		}
		p.expect(token.RPAREN)
		return &ast.BuiltinExpr{
			BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
			Func:     fn,
			Args:     args,
		}

	case token.F_SPLIT:
		p.expect(token.LPAREN)
		str := p.parseExpr()
		p.commaNewlines()
		arrayName, arrayPos := p.expectName()
		args := []ast.Expr{
			str,
			&ast.Ident{BaseExpr: ast.MakeBaseExpr(arrayPos, p.tok.Pos), Name: arrayName},
		}
		if p.tok.Type == token.COMMA {
			p.commaNewlines()
			args = append(args, p.parseRegexOrExpr(p.parseExpr))
		}
		p.expect(token.RPAREN)
		return &ast.BuiltinExpr{
			BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
			Func:     fn,
			Args:     args,
		}

	case token.F_SUB, token.F_GSUB:
		p.expect(token.LPAREN)
		regex := p.parseRegexOrExpr(p.parseExpr)
		p.commaNewlines()
		repl := p.parseExpr()
		args := []ast.Expr{regex, repl}
		if p.tok.Type == token.COMMA {
			p.commaNewlines()
			target := p.parseExpr()
			if !ast.IsLValue(target) {
				p.errorf("third argument to %s must be an lvalue", tokenName(fn))
			}
			args = append(args, target)
		}
		p.expect(token.RPAREN)
		return &ast.BuiltinExpr{
			BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
			Func:     fn,
			Args:     args,
		}

	case token.F_MATCH:
		p.expect(token.LPAREN)
		str := p.parseExpr()
		p.commaNewlines()
		regex := p.parseRegexOrExpr(p.parseExpr)
		p.expect(token.RPAREN)
		return &ast.BuiltinExpr{
			BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
			Func:     fn,
			Args:     []ast.Expr{str, regex},
		}

	default:
		p.errorf("unknown builtin function")
		return nil
	}
}

// parseRegexOrExpr parses a regex literal or falls back to expression.
func (p *Parser) parseRegexOrExpr(fallback func() ast.Expr) ast.Expr {
	if p.match(token.DIV, token.DIV_ASSIGN) {
		startPos := p.tok.Pos
		tok := p.lexer.ScanRegex()
		if tok.Type == token.ILLEGAL {
			p.errorf("%s", tok.Value)
			return nil
		}
		pattern := tok.Value
		p.next()
		return &ast.RegexLit{
			BaseExpr: ast.MakeBaseExpr(startPos, p.tok.Pos),
			Pattern:  pattern,
		}
	}
	return fallback()
}

// -----------------------------------------------------------------------------
// Helper functions
// -----------------------------------------------------------------------------

// parseBinaryLeft parses left-associative binary operators.
func (p *Parser) parseBinaryLeft(higher func() ast.Expr, allowNewline bool, ops ...token.Token) ast.Expr {
	expr := higher()
	if expr == nil {
		return nil
	}

	for p.match(ops...) {
		op := p.tok.Type
		p.next()
		if allowNewline {
			p.optionalNewlines()
		}
		right := higher()
		if right == nil {
			break
		}
		expr = &ast.BinaryExpr{
			BaseExpr: ast.MakeBaseExpr(expr.Pos(), right.End()),
			Left:     expr,
			Op:       op,
			Right:    right,
		}
	}
	return expr
}

// parseExprList parses comma-separated expressions.
func (p *Parser) parseExprList(parse func() ast.Expr) []ast.Expr {
	var exprs []ast.Expr
	first := true

	for !p.match(token.NEWLINE, token.SEMICOLON, token.RBRACE, token.RBRACKET,
		token.RPAREN, token.GREATER, token.PIPE, token.APPEND, token.EOF) {
		if !first {
			p.commaNewlines()
		}
		first = false
		exprs = append(exprs, parse())
	}
	return exprs
}
