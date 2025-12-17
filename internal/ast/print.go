package ast

import (
	"fmt"
	"io"
	"strings"

	"github.com/kolkov/uawk/internal/token"
)

// Printer provides pretty-printing for AST nodes.
// It outputs a human-readable representation suitable for debugging.
type Printer struct {
	w      io.Writer
	indent int
	err    error
}

// NewPrinter creates a new Printer that writes to w.
func NewPrinter(w io.Writer) *Printer {
	return &Printer{w: w}
}

// Print writes a pretty-printed representation of the node to the writer.
func (p *Printer) Print(node Node) error {
	p.printNode(node)
	return p.err
}

func (p *Printer) printf(format string, args ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintf(p.w, format, args...)
}

func (p *Printer) writeIndent() {
	if p.err != nil {
		return
	}
	for i := 0; i < p.indent; i++ {
		_, p.err = io.WriteString(p.w, "    ")
	}
}

func (p *Printer) printNode(node Node) {
	if node == nil {
		p.printf("<nil>")
		return
	}

	switch n := node.(type) {
	case *Program:
		p.printProgram(n)
	case *Rule:
		p.printRule(n)
	case *FuncDecl:
		p.printFuncDecl(n)
	case Expr:
		p.printExpr(n)
	case Stmt:
		p.printStmt(n)
	default:
		p.printf("<%T>", node)
	}
}

func (p *Printer) printProgram(prog *Program) {
	for _, b := range prog.Begin {
		p.printf("BEGIN ")
		p.printStmt(b)
		p.printf("\n\n")
	}

	for _, r := range prog.Rules {
		p.printRule(r)
		p.printf("\n\n")
	}

	for _, b := range prog.EndBlocks {
		p.printf("END ")
		p.printStmt(b)
		p.printf("\n\n")
	}

	for _, f := range prog.Functions {
		p.printFuncDecl(f)
		p.printf("\n\n")
	}
}

func (p *Printer) printRule(r *Rule) {
	if r.Pattern != nil {
		p.printExpr(r.Pattern)
		if r.Action != nil {
			p.printf(" ")
		}
	}
	if r.Action != nil {
		p.printStmt(r.Action)
	}
}

func (p *Printer) printFuncDecl(f *FuncDecl) {
	p.printf("function %s(", f.Name)
	for i, param := range f.Params {
		if i > 0 {
			if i == f.NumParams {
				p.printf(",    ") // AWK convention: space before locals
			} else {
				p.printf(", ")
			}
		}
		p.printf("%s", param)
	}
	p.printf(") ")
	p.printStmt(f.Body)
}

func (p *Printer) printExpr(e Expr) {
	if e == nil {
		p.printf("<nil>")
		return
	}

	switch n := e.(type) {
	case *NumLit:
		if n.Raw != "" {
			p.printf("%s", n.Raw)
		} else {
			p.printf("%g", n.Value)
		}

	case *StrLit:
		p.printf("%q", n.Value)

	case *RegexLit:
		p.printf("/%s/", escapeRegex(n.Pattern))
		if n.Flags != "" {
			p.printf("%s", n.Flags)
		}

	case *Ident:
		p.printf("%s", n.Name)

	case *FieldExpr:
		p.printf("$")
		if n.Index != nil {
			needParen := needsParens(n.Index)
			if needParen {
				p.printf("(")
			}
			p.printExpr(n.Index)
			if needParen {
				p.printf(")")
			}
		} else {
			p.printf("0")
		}

	case *IndexExpr:
		p.printExpr(n.Array)
		p.printf("[")
		for i, idx := range n.Index {
			if i > 0 {
				p.printf(", ")
			}
			p.printExpr(idx)
		}
		p.printf("]")

	case *BinaryExpr:
		p.printBinaryExpr(n)

	case *UnaryExpr:
		p.printUnaryExpr(n)

	case *TernaryExpr:
		p.printExpr(n.Cond)
		p.printf(" ? ")
		p.printExpr(n.Then)
		p.printf(" : ")
		p.printExpr(n.Else)

	case *AssignExpr:
		p.printExpr(n.Left)
		p.printf(" %s ", assignOpString(n.Op))
		p.printExpr(n.Right)

	case *ConcatExpr:
		for i, expr := range n.Exprs {
			if i > 0 {
				p.printf(" ")
			}
			p.printExpr(expr)
		}

	case *GroupExpr:
		p.printf("(")
		p.printExpr(n.Expr)
		p.printf(")")

	case *CallExpr:
		p.printf("%s(", n.Name)
		p.printArgs(n.Args)
		p.printf(")")

	case *BuiltinExpr:
		p.printf("%s(", builtinName(n.Func))
		p.printArgs(n.Args)
		p.printf(")")

	case *GetlineExpr:
		if n.Command != nil {
			p.printExpr(n.Command)
			p.printf(" | ")
		}
		p.printf("getline")
		if n.Target != nil {
			p.printf(" ")
			p.printExpr(n.Target)
		}
		if n.File != nil {
			p.printf(" < ")
			p.printExpr(n.File)
		}

	case *InExpr:
		if len(n.Index) == 1 {
			p.printExpr(n.Index[0])
		} else {
			p.printf("(")
			for i, idx := range n.Index {
				if i > 0 {
					p.printf(", ")
				}
				p.printExpr(idx)
			}
			p.printf(")")
		}
		p.printf(" in ")
		p.printExpr(n.Array)

	case *MatchExpr:
		p.printExpr(n.Expr)
		if n.Op == token.MATCH {
			p.printf(" ~ ")
		} else {
			p.printf(" !~ ")
		}
		p.printExpr(n.Pattern)

	case *CommaExpr:
		p.printExpr(n.Left)
		p.printf(", ")
		p.printExpr(n.Right)

	default:
		p.printf("<%T>", e)
	}
}

func (p *Printer) printBinaryExpr(n *BinaryExpr) {
	leftNeedsParen := needsParens(n.Left)
	rightNeedsParen := needsParens(n.Right)

	if leftNeedsParen {
		p.printf("(")
	}
	p.printExpr(n.Left)
	if leftNeedsParen {
		p.printf(")")
	}

	// Special case: CONCAT uses space
	if n.Op == token.CONCAT {
		p.printf(" ")
	} else {
		p.printf(" %s ", tokenString(n.Op))
	}

	if rightNeedsParen {
		p.printf("(")
	}
	p.printExpr(n.Right)
	if rightNeedsParen {
		p.printf(")")
	}
}

func (p *Printer) printUnaryExpr(n *UnaryExpr) {
	if n.Post {
		p.printExpr(n.Expr)
		p.printf("%s", tokenString(n.Op))
	} else {
		p.printf("%s", tokenString(n.Op))
		p.printExpr(n.Expr)
	}
}

func (p *Printer) printArgs(args []Expr) {
	for i, arg := range args {
		if i > 0 {
			p.printf(", ")
		}
		p.printExpr(arg)
	}
}

func (p *Printer) printStmt(s Stmt) {
	if s == nil {
		p.printf("<nil>")
		return
	}

	switch n := s.(type) {
	case *ExprStmt:
		p.printExpr(n.Expr)

	case *PrintStmt:
		if n.Printf {
			p.printf("printf ")
		} else {
			p.printf("print ")
		}
		p.printArgs(n.Args)
		if n.Dest != nil {
			p.printf(" %s ", tokenString(n.Redirect))
			p.printExpr(n.Dest)
		}

	case *BlockStmt:
		p.printf("{\n")
		p.indent++
		for _, stmt := range n.Stmts {
			p.writeIndent()
			p.printStmt(stmt)
			p.printf("\n")
		}
		p.indent--
		p.writeIndent()
		p.printf("}")

	case *IfStmt:
		p.printf("if (")
		p.printExpr(n.Cond)
		p.printf(") ")
		p.printStmt(n.Then)
		if n.Else != nil {
			p.printf(" else ")
			p.printStmt(n.Else)
		}

	case *WhileStmt:
		p.printf("while (")
		p.printExpr(n.Cond)
		p.printf(") ")
		p.printStmt(n.Body)

	case *DoWhileStmt:
		p.printf("do ")
		p.printStmt(n.Body)
		p.printf(" while (")
		p.printExpr(n.Cond)
		p.printf(")")

	case *ForStmt:
		p.printf("for (")
		if n.Init != nil {
			p.printStmt(n.Init)
		}
		p.printf("; ")
		if n.Cond != nil {
			p.printExpr(n.Cond)
		}
		p.printf("; ")
		if n.Post != nil {
			p.printStmt(n.Post)
		}
		p.printf(") ")
		p.printStmt(n.Body)

	case *ForInStmt:
		p.printf("for (")
		p.printExpr(n.Var)
		p.printf(" in ")
		p.printExpr(n.Array)
		p.printf(") ")
		p.printStmt(n.Body)

	case *BreakStmt:
		p.printf("break")

	case *ContinueStmt:
		p.printf("continue")

	case *NextStmt:
		p.printf("next")

	case *NextFileStmt:
		p.printf("nextfile")

	case *ReturnStmt:
		p.printf("return")
		if n.Value != nil {
			p.printf(" ")
			p.printExpr(n.Value)
		}

	case *ExitStmt:
		p.printf("exit")
		if n.Code != nil {
			p.printf(" ")
			p.printExpr(n.Code)
		}

	case *DeleteStmt:
		p.printf("delete ")
		p.printExpr(n.Array)
		if len(n.Index) > 0 {
			p.printf("[")
			for i, idx := range n.Index {
				if i > 0 {
					p.printf(", ")
				}
				p.printExpr(idx)
			}
			p.printf("]")
		}

	default:
		p.printf("<%T>", s)
	}
}

// String returns a string representation of the node.
func String(node Node) string {
	var sb strings.Builder
	p := NewPrinter(&sb)
	p.Print(node)
	return sb.String()
}

// Helper functions

// tokenString returns a string representation of a token.
// Since token.Token doesn't have a String() method, we provide one here.
func tokenString(t token.Token) string {
	switch t {
	// Operators
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
	case token.ASSIGN:
		return "="
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
	case token.CONCAT:
		return " " // Space for concatenation
	default:
		return "?"
	}
}

func needsParens(e Expr) bool {
	switch e.(type) {
	case *BinaryExpr, *TernaryExpr, *AssignExpr, *ConcatExpr:
		return true
	default:
		return false
	}
}

func escapeRegex(s string) string {
	return strings.ReplaceAll(s, "/", `\/`)
}

func assignOpString(op token.Token) string {
	switch op {
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
	default:
		return tokenString(op)
	}
}

func builtinName(t token.Token) string {
	switch t {
	case token.F_ATAN2:
		return "atan2"
	case token.F_CLOSE:
		return "close"
	case token.F_COS:
		return "cos"
	case token.F_EXP:
		return "exp"
	case token.F_FFLUSH:
		return "fflush"
	case token.F_GSUB:
		return "gsub"
	case token.F_INDEX:
		return "index"
	case token.F_INT:
		return "int"
	case token.F_LENGTH:
		return "length"
	case token.F_LOG:
		return "log"
	case token.F_MATCH:
		return "match"
	case token.F_RAND:
		return "rand"
	case token.F_SIN:
		return "sin"
	case token.F_SPLIT:
		return "split"
	case token.F_SPRINTF:
		return "sprintf"
	case token.F_SQRT:
		return "sqrt"
	case token.F_SRAND:
		return "srand"
	case token.F_SUB:
		return "sub"
	case token.F_SUBSTR:
		return "substr"
	case token.F_SYSTEM:
		return "system"
	case token.F_TOLOWER:
		return "tolower"
	case token.F_TOUPPER:
		return "toupper"
	default:
		return "unknown"
	}
}
