package semantic

import (
	"github.com/kolkov/uawk/internal/ast"
	"github.com/kolkov/uawk/internal/token"
)

// Checker performs additional semantic validation after resolution.
// It checks for errors that require knowing the full program context.
type Checker struct {
	result *ResolveResult
	errors ErrorList

	// Context tracking
	inLoop   int
	inFunc   bool
	funcName string
	inBegin  bool
	inEnd    bool
}

// Check performs semantic validation on a resolved program.
// This is called after Resolve() to catch additional errors.
func Check(prog *ast.Program, result *ResolveResult) []error {
	c := &Checker{
		result: result,
	}

	c.checkProgram(prog)

	if len(c.errors) == 0 {
		return nil
	}

	errs := make([]error, len(c.errors))
	for i, e := range c.errors {
		errs[i] = e
	}
	return errs
}

func (c *Checker) checkProgram(prog *ast.Program) {
	// Check BEGIN blocks
	for _, block := range prog.Begin {
		c.inBegin = true
		c.checkBlock(block)
		c.inBegin = false
	}

	// Check rules
	for _, rule := range prog.Rules {
		c.checkRule(rule)
	}

	// Check END blocks
	for _, block := range prog.EndBlocks {
		c.inEnd = true
		c.checkBlock(block)
		c.inEnd = false
	}

	// Check functions
	for _, fn := range prog.Functions {
		c.checkFunction(fn)
	}
}

func (c *Checker) checkFunction(fn *ast.FuncDecl) {
	c.inFunc = true
	c.funcName = fn.Name
	if fn.Body != nil {
		c.checkBlock(fn.Body)
	}
	c.inFunc = false
	c.funcName = ""
}

func (c *Checker) checkRule(rule *ast.Rule) {
	if rule.Pattern != nil {
		c.checkExpr(rule.Pattern)
	}
	if rule.Action != nil {
		c.checkBlock(rule.Action)
	}
}

func (c *Checker) checkBlock(block *ast.BlockStmt) {
	if block == nil {
		return
	}
	for _, stmt := range block.Stmts {
		c.checkStmt(stmt)
	}
}

func (c *Checker) checkStmt(stmt ast.Stmt) {
	if stmt == nil {
		return
	}

	switch s := stmt.(type) {
	case *ast.ExprStmt:
		c.checkExpr(s.Expr)

	case *ast.PrintStmt:
		for _, arg := range s.Args {
			c.checkExpr(arg)
		}
		if s.Dest != nil {
			c.checkExpr(s.Dest)
		}

	case *ast.BlockStmt:
		c.checkBlock(s)

	case *ast.IfStmt:
		c.checkExpr(s.Cond)
		c.checkStmt(s.Then)
		if s.Else != nil {
			c.checkStmt(s.Else)
		}

	case *ast.WhileStmt:
		c.checkExpr(s.Cond)
		c.inLoop++
		c.checkStmt(s.Body)
		c.inLoop--

	case *ast.DoWhileStmt:
		c.inLoop++
		c.checkStmt(s.Body)
		c.inLoop--
		c.checkExpr(s.Cond)

	case *ast.ForStmt:
		if s.Init != nil {
			c.checkStmt(s.Init)
		}
		if s.Cond != nil {
			c.checkExpr(s.Cond)
		}
		if s.Post != nil {
			c.checkStmt(s.Post)
		}
		c.inLoop++
		c.checkStmt(s.Body)
		c.inLoop--

	case *ast.ForInStmt:
		c.checkExpr(s.Var)
		c.checkExpr(s.Array)
		c.inLoop++
		c.checkStmt(s.Body)
		c.inLoop--

	case *ast.BreakStmt:
		if c.inLoop == 0 {
			c.errors.Add(s.Pos(), errBreakOutsideLoop)
		}

	case *ast.ContinueStmt:
		if c.inLoop == 0 {
			c.errors.Add(s.Pos(), errContinueOutsideLoop)
		}

	case *ast.NextStmt:
		if c.inBegin || c.inEnd {
			c.errors.Add(s.Pos(), errNextInBeginEnd)
		}

	case *ast.NextFileStmt:
		if c.inBegin || c.inEnd {
			c.errors.Add(s.Pos(), errNextInBeginEnd)
		}

	case *ast.ReturnStmt:
		if !c.inFunc {
			c.errors.Add(s.Pos(), errReturnOutsideFunc)
		}
		if s.Value != nil {
			c.checkExpr(s.Value)
		}

	case *ast.ExitStmt:
		if s.Code != nil {
			c.checkExpr(s.Code)
		}

	case *ast.DeleteStmt:
		c.checkDeleteStmt(s)
	}
}

func (c *Checker) checkDeleteStmt(s *ast.DeleteStmt) {
	// Check that delete target is an array
	if ident, ok := s.Array.(*ast.Ident); ok {
		sym, _, found := c.result.LookupVar(c.funcName, ident.Name)
		if found && sym.Type == TypeScalar {
			c.errors.Add(s.Pos(), errDeleteNonArray, ident.Name)
		}
	}

	for _, idx := range s.Index {
		c.checkExpr(idx)
	}
}

func (c *Checker) checkExpr(expr ast.Expr) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *ast.NumLit, *ast.StrLit, *ast.RegexLit, *ast.Ident:
		// No additional checks needed

	case *ast.FieldExpr:
		c.checkExpr(e.Index)

	case *ast.IndexExpr:
		c.checkExpr(e.Array)
		for _, idx := range e.Index {
			c.checkExpr(idx)
		}

	case *ast.BinaryExpr:
		c.checkExpr(e.Left)
		c.checkExpr(e.Right)

	case *ast.UnaryExpr:
		c.checkExpr(e.Expr)
		// Check that increment/decrement target is lvalue
		if e.Op == token.INCR || e.Op == token.DECR {
			if !ast.IsLValue(e.Expr) {
				c.errors.Add(e.Pos(), errAssignToNonLValue)
			}
		}

	case *ast.TernaryExpr:
		c.checkExpr(e.Cond)
		c.checkExpr(e.Then)
		c.checkExpr(e.Else)

	case *ast.AssignExpr:
		c.checkAssignExpr(e)

	case *ast.ConcatExpr:
		for _, sub := range e.Exprs {
			c.checkExpr(sub)
		}

	case *ast.GroupExpr:
		c.checkExpr(e.Expr)

	case *ast.CallExpr:
		c.checkCallExpr(e)

	case *ast.BuiltinExpr:
		c.checkBuiltinExpr(e)

	case *ast.GetlineExpr:
		if e.Target != nil {
			c.checkExpr(e.Target)
		}
		if e.File != nil {
			c.checkExpr(e.File)
		}
		if e.Command != nil {
			c.checkExpr(e.Command)
		}

	case *ast.InExpr:
		for _, idx := range e.Index {
			c.checkExpr(idx)
		}
		c.checkExpr(e.Array)

	case *ast.MatchExpr:
		c.checkExpr(e.Expr)
		c.checkExpr(e.Pattern)

	case *ast.CommaExpr:
		c.checkExpr(e.Left)
		c.checkExpr(e.Right)
	}
}

func (c *Checker) checkAssignExpr(e *ast.AssignExpr) {
	// Left side must be lvalue
	if !ast.IsLValue(e.Left) {
		c.errors.Add(e.Pos(), errAssignToNonLValue)
	}
	c.checkExpr(e.Left)
	c.checkExpr(e.Right)
}

func (c *Checker) checkCallExpr(e *ast.CallExpr) {
	// Function existence already checked in resolver
	funcInfo, ok := c.result.Functions[e.Name]
	if !ok {
		// Error already reported
		for _, arg := range e.Args {
			c.checkExpr(arg)
		}
		return
	}

	// Check argument count
	if len(e.Args) > len(funcInfo.Params) {
		c.errors.Add(e.Pos(), errTooManyArgs, e.Name)
	}

	// Check each argument
	for _, arg := range e.Args {
		c.checkExpr(arg)
	}
}

func (c *Checker) checkBuiltinExpr(e *ast.BuiltinExpr) {
	// Get builtin info by token
	var info BuiltinInfo
	var found bool
	for _, bi := range builtinFuncs {
		if bi.Token == e.Func {
			info = bi
			found = true
			break
		}
	}

	if !found {
		// Unknown builtin - should not happen if parser is correct
		for _, arg := range e.Args {
			c.checkExpr(arg)
		}
		return
	}

	// Check argument count
	if len(e.Args) < info.MinArgs {
		c.errors.Add(e.Pos(), errNotEnoughArgs, info.Name)
	}
	if info.MaxArgs >= 0 && len(e.Args) > info.MaxArgs {
		c.errors.Add(e.Pos(), errTooManyArgs, info.Name)
	}

	// Special checks for specific builtins
	switch e.Func {
	case token.F_SUB, token.F_GSUB:
		// Third argument must be lvalue if provided
		if len(e.Args) >= 3 {
			if !ast.IsLValue(e.Args[2]) {
				c.errors.Add(e.Pos(), errAssignToNonLValue)
			}
		}
	}

	// Check all arguments
	for _, arg := range e.Args {
		c.checkExpr(arg)
	}
}

// ValidateProgram is a convenience function that runs both resolution and checking.
func ValidateProgram(prog *ast.Program) (*ResolveResult, error) {
	result, err := Resolve(prog)
	if err != nil {
		return result, err
	}

	errs := Check(prog, result)
	if len(errs) > 0 {
		// Combine errors
		for _, e := range errs {
			if se, ok := e.(*Error); ok {
				result.Errors = append(result.Errors, se)
			}
		}
		return result, result.Errors.Err()
	}

	return result, nil
}
