package compiler

import (
	"fmt"
	"math"
	"strconv"

	"github.com/kolkov/uawk/internal/ast"
	"github.com/kolkov/uawk/internal/semantic"
	"github.com/kolkov/uawk/internal/token"
)

// CompileError represents a compilation error.
type CompileError struct {
	Message string
}

func (e *CompileError) Error() string {
	return e.Message
}

// Compile transforms a resolved AST into bytecode.
func Compile(prog *ast.Program, resolved *semantic.ResolveResult) (compiledProg *Program, err error) {
	defer func() {
		if r := recover(); r != nil {
			if ce, ok := r.(*CompileError); ok {
				err = ce
			} else {
				panic(r) // Re-panic for non-compile errors
			}
		}
	}()

	p := &Program{}

	// Initialize constant indexes for deduplication.
	indexes := &constantIndexes{
		nums:    make(map[float64]int),
		strs:    make(map[string]int),
		regexes: make(map[string]int),
	}

	// Run type inference for static type specialization (P1-003 optimization).
	// This enables specialized numeric opcodes for provably-typed expressions.
	typeInfo := InferTypes(prog, resolved)

	// Phase 1: Pre-compile function metadata.
	// Use funcInfo.Index to ensure correct placement regardless of map iteration order.
	p.Functions = make([]Function, len(resolved.Functions))
	for _, funcInfo := range resolved.Functions {
		arrays := make([]bool, len(funcInfo.Params))
		numArrays := 0
		for j, param := range funcInfo.Params {
			if sym, ok := funcInfo.Symbols.LookupLocal(param); ok && sym.Type == semantic.TypeArray {
				arrays[j] = true
				numArrays++
			}
		}
		compiledFunc := Function{
			Name:       funcInfo.Name,
			Params:     funcInfo.Params,
			Arrays:     arrays,
			NumScalars: len(funcInfo.Params) - numArrays,
			NumArrays:  numArrays,
		}
		p.Functions[funcInfo.Index] = compiledFunc
	}

	// Phase 2: Compile function bodies.
	for _, funcInfo := range resolved.Functions {
		// Find corresponding AST function
		var astFunc *ast.FuncDecl
		for _, f := range prog.Functions {
			if f.Name == funcInfo.Name {
				astFunc = f
				break
			}
		}
		if astFunc != nil && astFunc.Body != nil {
			c := newCompiler(resolved, p, indexes, funcInfo.Name, typeInfo)
			c.compileBlock(astFunc.Body)
			p.Functions[funcInfo.Index].Body = c.finish()
		}
	}

	// Phase 3: Compile BEGIN blocks.
	for _, block := range prog.Begin {
		c := newCompiler(resolved, p, indexes, "", typeInfo)
		c.compileBlock(block)
		p.Begin = append(p.Begin, c.finish()...)
	}

	// Phase 4: Compile pattern-action rules.
	for _, rule := range prog.Rules {
		var pattern [][]Opcode
		if rule.Pattern != nil {
			// Check for range pattern (CommaExpr)
			if comma, ok := rule.Pattern.(*ast.CommaExpr); ok {
				// Range pattern: /start/, /end/
				c := newCompiler(resolved, p, indexes, "", typeInfo)
				c.compileExpr(comma.Left)
				pattern = append(pattern, c.finish())

				c = newCompiler(resolved, p, indexes, "", typeInfo)
				c.compileExpr(comma.Right)
				pattern = append(pattern, c.finish())
			} else {
				// Single pattern
				c := newCompiler(resolved, p, indexes, "", typeInfo)
				c.compileExpr(rule.Pattern)
				pattern = [][]Opcode{c.finish()}
			}
		}

		var body []Opcode
		if rule.Action == nil {
			// No action means { print $0 } - nil body signals this to VM
			body = nil
		} else if len(rule.Action.Stmts) == 0 {
			// Empty action {} - add Nop so VM knows it's not nil
			c := newCompiler(resolved, p, indexes, "", typeInfo)
			c.add(Nop)
			body = c.finish()
		} else {
			c := newCompiler(resolved, p, indexes, "", typeInfo)
			c.compileBlock(rule.Action)
			body = c.finish()
		}

		p.Actions = append(p.Actions, Action{
			Pattern: pattern,
			Body:    body,
		})
	}

	// Phase 5: Compile END blocks.
	for _, block := range prog.EndBlocks {
		c := newCompiler(resolved, p, indexes, "", typeInfo)
		if len(block.Stmts) > 0 {
			c.compileBlock(block)
		} else {
			c.add(Nop) // Ensure empty END {} isn't treated as no END
		}
		p.End = append(p.End, c.finish()...)
	}

	// Phase 6: Build variable name tables for debugging.
	resolved.Globals.ForEach(func(name string, sym *semantic.Symbol) {
		if sym.Kind == semantic.SymbolGlobal {
			if sym.Type == semantic.TypeArray {
				for len(p.ArrayNames) <= sym.Index {
					p.ArrayNames = append(p.ArrayNames, "")
				}
				p.ArrayNames[sym.Index] = name
				p.NumArrays = max(p.NumArrays, sym.Index+1)
			} else {
				for len(p.ScalarNames) <= sym.Index {
					p.ScalarNames = append(p.ScalarNames, "")
				}
				p.ScalarNames[sym.Index] = name
				p.NumScalars = max(p.NumScalars, sym.Index+1)
			}
		}
	})

	return p, nil
}

// constantIndexes tracks constant pool indices for deduplication.
type constantIndexes struct {
	nums    map[float64]int
	strs    map[string]int
	regexes map[string]int
}

// compiler holds the state for compiling a single code block.
type compiler struct {
	resolved *semantic.ResolveResult
	program  *Program
	indexes  *constantIndexes
	funcName string // Current function name ("" for global scope)

	// Type information for specialization (may be nil)
	typeInfo *TypeInfo

	code      []Opcode
	breaks    [][]int // Stack of break target lists
	continues [][]int // Stack of continue target lists
}

// newCompiler creates a new compiler for a code block.
func newCompiler(resolved *semantic.ResolveResult, program *Program, indexes *constantIndexes, funcName string, typeInfo *TypeInfo) *compiler {
	return &compiler{
		resolved: resolved,
		program:  program,
		indexes:  indexes,
		funcName: funcName,
		typeInfo: typeInfo,
	}
}

// add appends opcodes to the current code block.
func (c *compiler) add(ops ...Opcode) {
	c.code = append(c.code, ops...)
}

// finish returns the compiled code block.
func (c *compiler) finish() []Opcode {
	return c.code
}

// opcodeInt converts an int to Opcode, checking for overflow.
func opcodeInt(n int) Opcode {
	if n > math.MaxInt32 || n < math.MinInt32 {
		panic(&CompileError{Message: fmt.Sprintf("value %d overflows int32", n)})
	}
	return Opcode(n)
}

// numIndex adds or reuses a numeric constant.
func (c *compiler) numIndex(n float64) int {
	if idx, ok := c.indexes.nums[n]; ok {
		return idx
	}
	idx := len(c.program.Nums)
	c.program.Nums = append(c.program.Nums, n)
	c.indexes.nums[n] = idx
	return idx
}

// strIndex adds or reuses a string constant.
func (c *compiler) strIndex(s string) int {
	if idx, ok := c.indexes.strs[s]; ok {
		return idx
	}
	idx := len(c.program.Strs)
	c.program.Strs = append(c.program.Strs, s)
	c.indexes.strs[s] = idx
	return idx
}

// regexIndex adds or reuses a regex pattern.
func (c *compiler) regexIndex(pattern string) int {
	if idx, ok := c.indexes.regexes[pattern]; ok {
		return idx
	}
	idx := len(c.program.Regexes)
	c.program.Regexes = append(c.program.Regexes, pattern)
	c.indexes.regexes[pattern] = idx
	return idx
}

// lookupScalar returns scope and index for a scalar variable.
func (c *compiler) lookupScalar(name string) (Scope, int) {
	sym, kind, ok := c.resolved.LookupVar(c.funcName, name)
	if !ok {
		panic(&CompileError{Message: fmt.Sprintf("undefined variable: %s", name)})
	}
	if sym.Type == semantic.TypeArray {
		panic(&CompileError{Message: fmt.Sprintf("expected scalar, got array: %s", name)})
	}
	return kindToScope(kind), sym.Index
}

// lookupArray returns scope and index for an array variable.
func (c *compiler) lookupArray(name string) (Scope, int) {
	sym, kind, ok := c.resolved.LookupVar(c.funcName, name)
	if !ok {
		panic(&CompileError{Message: fmt.Sprintf("undefined variable: %s", name)})
	}
	if sym.Type != semantic.TypeArray {
		panic(&CompileError{Message: fmt.Sprintf("expected array, got scalar: %s", name)})
	}
	return kindToScope(kind), sym.Index
}

// kindToScope converts semantic.SymbolKind to compiler.Scope.
func kindToScope(kind semantic.SymbolKind) Scope {
	switch kind {
	case semantic.SymbolGlobal:
		return ScopeGlobal
	case semantic.SymbolLocal, semantic.SymbolParam:
		return ScopeLocal
	case semantic.SymbolSpecial:
		return ScopeSpecial
	default:
		return ScopeGlobal
	}
}

// jumpForward emits a forward jump and returns its patch location.
func (c *compiler) jumpForward(op Opcode, args ...Opcode) int {
	c.add(op)
	c.add(args...)
	c.add(0) // Placeholder for offset
	return len(c.code)
}

// patchForward patches a forward jump to the current position.
func (c *compiler) patchForward(mark int) {
	offset := len(c.code) - mark
	c.code[mark-1] = opcodeInt(offset)
}

// labelBackward returns the current position for a backward jump.
func (c *compiler) labelBackward() int {
	return len(c.code)
}

// jumpBackward emits a backward jump to a label.
func (c *compiler) jumpBackward(label int, op Opcode, args ...Opcode) {
	offset := label - (len(c.code) + len(args) + 2)
	c.add(op)
	c.add(args...)
	c.add(opcodeInt(offset))
}

// patchBreaks patches all break jumps in the current loop.
func (c *compiler) patchBreaks() {
	breaks := c.breaks[len(c.breaks)-1]
	for _, mark := range breaks {
		c.patchForward(mark)
	}
	c.breaks = c.breaks[:len(c.breaks)-1]
}

// patchContinues patches all continue jumps in the current loop.
func (c *compiler) patchContinues() {
	continues := c.continues[len(c.continues)-1]
	for _, mark := range continues {
		c.patchForward(mark)
	}
	c.continues = c.continues[:len(c.continues)-1]
}

// compileBlock compiles a block statement.
func (c *compiler) compileBlock(block *ast.BlockStmt) {
	if block == nil {
		return
	}
	for _, stmt := range block.Stmts {
		c.compileStmt(stmt)
	}
}

// compileStmt compiles a statement.
func (c *compiler) compileStmt(stmt ast.Stmt) {
	if stmt == nil {
		return
	}

	switch s := stmt.(type) {
	case *ast.ExprStmt:
		c.compileExprStmt(s)

	case *ast.PrintStmt:
		c.compilePrintStmt(s)

	case *ast.BlockStmt:
		c.compileBlock(s)

	case *ast.IfStmt:
		c.compileIfStmt(s)

	case *ast.WhileStmt:
		c.compileWhileStmt(s)

	case *ast.DoWhileStmt:
		c.compileDoWhileStmt(s)

	case *ast.ForStmt:
		c.compileForStmt(s)

	case *ast.ForInStmt:
		c.compileForInStmt(s)

	case *ast.BreakStmt:
		if len(c.breaks) == 0 {
			panic(&CompileError{Message: "break outside loop"})
		}
		i := len(c.breaks) - 1
		if c.breaks[i] == nil {
			// In for-in loop, use BreakForIn
			c.add(BreakForIn)
		} else {
			mark := c.jumpForward(Jump)
			c.breaks[i] = append(c.breaks[i], mark)
		}

	case *ast.ContinueStmt:
		if len(c.continues) == 0 {
			panic(&CompileError{Message: "continue outside loop"})
		}
		i := len(c.continues) - 1
		mark := c.jumpForward(Jump)
		c.continues[i] = append(c.continues[i], mark)

	case *ast.NextStmt:
		c.add(Next)

	case *ast.NextFileStmt:
		c.add(Nextfile)

	case *ast.ReturnStmt:
		if s.Value != nil {
			c.compileExpr(s.Value)
			c.add(Return)
		} else {
			c.add(ReturnNull)
		}

	case *ast.ExitStmt:
		if s.Code != nil {
			c.compileExpr(s.Code)
			c.add(ExitCode)
		} else {
			c.add(Exit)
		}

	case *ast.DeleteStmt:
		if ident, ok := s.Array.(*ast.Ident); ok {
			scope, idx := c.lookupArray(ident.Name)
			if len(s.Index) > 0 {
				c.compileIndex(s.Index)
				if scope == ScopeGlobal {
					c.add(ArrayDeleteGlobal, opcodeInt(idx))
				} else {
					c.add(ArrayDelete, Opcode(scope), opcodeInt(idx))
				}
			} else {
				c.add(ArrayClear, Opcode(scope), opcodeInt(idx))
			}
		}

	default:
		panic(&CompileError{Message: fmt.Sprintf("unexpected statement type: %T", stmt)})
	}
}

// compileExprStmt compiles an expression statement, with optimizations.
func (c *compiler) compileExprStmt(s *ast.ExprStmt) {
	switch expr := s.Expr.(type) {
	case *ast.AssignExpr:
		// Optimize: avoid Dupe/Drop for assignments
		c.compileExpr(expr.Right)
		c.compileAssign(expr.Left, expr.Op)
		return

	case *ast.UnaryExpr:
		if expr.Op == token.INCR || expr.Op == token.DECR {
			// Optimize: use Incr* opcodes for standalone ++/--
			c.compileIncr(expr)
			return
		}
	}

	// Default: compile expression and drop result
	c.compileExpr(s.Expr)
	c.add(Drop)
}

// compileIncr compiles an increment/decrement expression.
func (c *compiler) compileIncr(expr *ast.UnaryExpr) {
	amount := Opcode(1)
	if expr.Op == token.DECR {
		amount = Opcode(-1)
	}

	switch target := expr.Expr.(type) {
	case *ast.Ident:
		scope, idx := c.lookupScalar(target.Name)
		switch scope {
		case ScopeGlobal:
			c.add(IncrGlobal, amount, opcodeInt(idx))
		case ScopeLocal:
			c.add(IncrLocal, amount, opcodeInt(idx))
		case ScopeSpecial:
			c.add(IncrSpecial, amount, opcodeInt(idx))
		}
	case *ast.FieldExpr:
		c.compileExpr(target.Index)
		c.add(IncrField, amount)
	case *ast.IndexExpr:
		c.compileIndex(target.Index)
		if ident, ok := target.Array.(*ast.Ident); ok {
			scope, idx := c.lookupArray(ident.Name)
			if scope == ScopeGlobal {
				c.add(IncrArrayGlobal, amount, opcodeInt(idx))
			} else {
				c.add(IncrArray, amount, Opcode(scope), opcodeInt(idx))
			}
		}
	}
}

// compilePrintStmt compiles a print or printf statement.
func (c *compiler) compilePrintStmt(s *ast.PrintStmt) {
	// Compile redirect destination first (if any)
	redirect := RedirectNone
	if s.Redirect != token.ILLEGAL {
		if s.Dest != nil {
			c.compileExpr(s.Dest)
		}
		redirect = tokenToRedirect(s.Redirect)
	}

	// Compile arguments
	for _, arg := range s.Args {
		c.compileExpr(arg)
	}

	// Emit print/printf
	if s.Printf {
		c.add(Printf, opcodeInt(len(s.Args)), Opcode(redirect))
	} else {
		c.add(Print, opcodeInt(len(s.Args)), Opcode(redirect))
	}
}

// tokenToRedirect converts a token to a Redirect.
func tokenToRedirect(tok token.Token) Redirect {
	switch tok {
	case token.GREATER:
		return RedirectWrite
	case token.APPEND:
		return RedirectAppend
	case token.PIPE:
		return RedirectPipe
	case token.LESS:
		return RedirectInput
	default:
		return RedirectNone
	}
}

// compileIfStmt compiles an if statement.
func (c *compiler) compileIfStmt(s *ast.IfStmt) {
	if s.Else == nil {
		// if without else
		jumpOp := c.compileCondition(s.Cond, true)
		ifMark := c.jumpForward(jumpOp)
		c.compileStmt(s.Then)
		c.patchForward(ifMark)
	} else {
		// if with else
		jumpOp := c.compileCondition(s.Cond, true)
		ifMark := c.jumpForward(jumpOp)
		c.compileStmt(s.Then)
		elseMark := c.jumpForward(Jump)
		c.patchForward(ifMark)
		c.compileStmt(s.Else)
		c.patchForward(elseMark)
	}
}

// compileWhileStmt compiles a while loop.
func (c *compiler) compileWhileStmt(s *ast.WhileStmt) {
	c.breaks = append(c.breaks, []int{})
	c.continues = append(c.continues, []int{})

	// Optimization: condition at start and end to avoid extra jump
	jumpOp := c.compileCondition(s.Cond, true)
	mark := c.jumpForward(jumpOp)

	loopStart := c.labelBackward()
	c.compileStmt(s.Body)
	c.patchContinues()

	jumpOp = c.compileCondition(s.Cond, false)
	c.jumpBackward(loopStart, jumpOp)
	c.patchForward(mark)

	c.patchBreaks()
}

// compileDoWhileStmt compiles a do-while loop.
func (c *compiler) compileDoWhileStmt(s *ast.DoWhileStmt) {
	c.breaks = append(c.breaks, []int{})
	c.continues = append(c.continues, []int{})

	loopStart := c.labelBackward()
	c.compileStmt(s.Body)
	c.patchContinues()

	jumpOp := c.compileCondition(s.Cond, false)
	c.jumpBackward(loopStart, jumpOp)

	c.patchBreaks()
}

// compileForStmt compiles a for loop.
func (c *compiler) compileForStmt(s *ast.ForStmt) {
	// Init
	if s.Init != nil {
		c.compileStmt(s.Init)
	}

	c.breaks = append(c.breaks, []int{})
	c.continues = append(c.continues, []int{})

	// Condition at start (for early exit)
	var mark int
	if s.Cond != nil {
		jumpOp := c.compileCondition(s.Cond, true)
		mark = c.jumpForward(jumpOp)
	}

	loopStart := c.labelBackward()
	c.compileStmt(s.Body)
	c.patchContinues()

	// Post
	if s.Post != nil {
		c.compileStmt(s.Post)
	}

	// Condition at end
	if s.Cond != nil {
		jumpOp := c.compileCondition(s.Cond, false)
		c.jumpBackward(loopStart, jumpOp)
		c.patchForward(mark)
	} else {
		c.jumpBackward(loopStart, Jump)
	}

	c.patchBreaks()
}

// compileForInStmt compiles a for-in loop.
func (c *compiler) compileForInStmt(s *ast.ForInStmt) {
	varScope, varIdx := c.lookupScalar(s.Var.Name)

	var arrScope Scope
	var arrIdx int
	if ident, ok := s.Array.(*ast.Ident); ok {
		arrScope, arrIdx = c.lookupArray(ident.Name)
	} else {
		panic(&CompileError{Message: "for-in requires array identifier"})
	}

	// ForIn handles iteration internally
	mark := c.jumpForward(ForIn,
		Opcode(varScope), opcodeInt(varIdx),
		Opcode(arrScope), opcodeInt(arrIdx))

	// nil breaks indicates for-in loop
	c.breaks = append(c.breaks, nil)
	c.continues = append(c.continues, []int{})

	c.compileStmt(s.Body)

	c.patchForward(mark)
	c.patchContinues()
	c.breaks = c.breaks[:len(c.breaks)-1]
}

// compileCondition compiles a boolean condition with jump optimization.
func (c *compiler) compileCondition(expr ast.Expr, invert bool) Opcode {
	jumpOp := func(normal, inverted Opcode) Opcode {
		if invert {
			return inverted
		}
		return normal
	}

	// Optimize comparison expressions into conditional jumps
	if binary, ok := expr.(*ast.BinaryExpr); ok {
		// Check if we can use typed jump opcodes (P1-003 optimization)
		useTyped := c.typeInfo != nil && c.typeInfo.BothNumeric(binary.Left, binary.Right)

		switch binary.Op {
		case token.EQUALS:
			c.compileExpr(binary.Left)
			c.compileExpr(binary.Right)
			if useTyped {
				return jumpOp(JumpEqualNum, JumpNotEqualNum)
			}
			return jumpOp(JumpEqual, JumpNotEq)
		case token.NOT_EQUALS:
			c.compileExpr(binary.Left)
			c.compileExpr(binary.Right)
			if useTyped {
				return jumpOp(JumpNotEqualNum, JumpEqualNum)
			}
			return jumpOp(JumpNotEq, JumpEqual)
		case token.LESS:
			c.compileExpr(binary.Left)
			c.compileExpr(binary.Right)
			if useTyped {
				return jumpOp(JumpLessNum, JumpGreaterEqNum)
			}
			return jumpOp(JumpLess, JumpGrEq)
		case token.LTE:
			c.compileExpr(binary.Left)
			c.compileExpr(binary.Right)
			if useTyped {
				return jumpOp(JumpLessEqNum, JumpGreaterNum)
			}
			return jumpOp(JumpLessEq, JumpGreater)
		case token.GREATER:
			c.compileExpr(binary.Left)
			c.compileExpr(binary.Right)
			if useTyped {
				return jumpOp(JumpGreaterNum, JumpLessEqNum)
			}
			return jumpOp(JumpGreater, JumpLessEq)
		case token.GTE:
			c.compileExpr(binary.Left)
			c.compileExpr(binary.Right)
			if useTyped {
				return jumpOp(JumpGreaterEqNum, JumpLessNum)
			}
			return jumpOp(JumpGrEq, JumpLess)
		}
	}

	// Default: evaluate expression and use JumpTrue/JumpFalse
	c.compileExpr(expr)
	return jumpOp(JumpTrue, JumpFalse)
}

// compileExpr compiles an expression.
func (c *compiler) compileExpr(expr ast.Expr) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *ast.NumLit:
		c.add(Num, opcodeInt(c.numIndex(e.Value)))

	case *ast.StrLit:
		c.add(Str, opcodeInt(c.strIndex(e.Value)))

	case *ast.RegexLit:
		c.add(Regex, opcodeInt(c.regexIndex(e.Pattern)))

	case *ast.Ident:
		scope, idx := c.lookupScalar(e.Name)
		switch scope {
		case ScopeGlobal:
			c.add(LoadGlobal, opcodeInt(idx))
		case ScopeLocal:
			c.add(LoadLocal, opcodeInt(idx))
		case ScopeSpecial:
			c.add(LoadSpecial, opcodeInt(idx))
		}

	case *ast.FieldExpr:
		// Optimize constant field index
		if num, ok := e.Index.(*ast.NumLit); ok {
			if num.Value == float64(int(num.Value)) {
				c.add(FieldInt, opcodeInt(int(num.Value)))
				return
			}
		}
		c.compileExpr(e.Index)
		c.add(Field)

	case *ast.IndexExpr:
		c.compileIndex(e.Index)
		if ident, ok := e.Array.(*ast.Ident); ok {
			scope, idx := c.lookupArray(ident.Name)
			if scope == ScopeGlobal {
				c.add(ArrayGetGlobal, opcodeInt(idx))
			} else {
				c.add(ArrayGet, Opcode(scope), opcodeInt(idx))
			}
		}

	case *ast.BinaryExpr:
		c.compileBinaryExpr(e)

	case *ast.UnaryExpr:
		c.compileUnaryExpr(e)

	case *ast.TernaryExpr:
		jump := c.compileCondition(e.Cond, true)
		ifMark := c.jumpForward(jump)
		c.compileExpr(e.Then)
		elseMark := c.jumpForward(Jump)
		c.patchForward(ifMark)
		c.compileExpr(e.Else)
		c.patchForward(elseMark)

	case *ast.AssignExpr:
		// Assignment in expression context: compute value, dupe, store
		if e.Op == token.ASSIGN {
			// Simple assignment: dupe RHS, one copy stored, one for expression
			c.compileExpr(e.Right)
			c.add(Dupe)
			c.compileAssign(e.Left, e.Op)
		} else {
			// Augmented assignment: compute result, dupe, store
			// For x += 3: push x, push 3, add, dupe, store to x
			// Key: expression value is the FINAL result, not the original RHS
			c.compileAugAssignExpr(e.Left, e.Op, e.Right)
		}

	case *ast.ConcatExpr:
		// Compile all parts
		for _, part := range e.Exprs {
			c.compileExpr(part)
		}
		if len(e.Exprs) == 2 {
			c.add(Concat)
		} else {
			c.add(ConcatMulti, opcodeInt(len(e.Exprs)))
		}

	case *ast.GroupExpr:
		c.compileExpr(e.Expr)

	case *ast.CallExpr:
		c.compileCallExpr(e)

	case *ast.BuiltinExpr:
		c.compileBuiltinExpr(e)

	case *ast.GetlineExpr:
		c.compileGetlineExpr(e)

	case *ast.InExpr:
		c.compileIndex(e.Index)
		if ident, ok := e.Array.(*ast.Ident); ok {
			scope, idx := c.lookupArray(ident.Name)
			if scope == ScopeGlobal {
				c.add(ArrayInGlobal, opcodeInt(idx))
			} else {
				c.add(ArrayIn, Opcode(scope), opcodeInt(idx))
			}
		}

	case *ast.MatchExpr:
		c.compileExpr(e.Expr)
		// For match expressions, push the pattern as a string, not evaluate it
		if regex, ok := e.Pattern.(*ast.RegexLit); ok {
			c.add(Str, opcodeInt(c.strIndex(regex.Pattern)))
		} else {
			c.compileExpr(e.Pattern)
		}
		if e.Op == token.MATCH {
			c.add(Match)
		} else {
			c.add(NotMatch)
		}

	case *ast.CommaExpr:
		// Comma expressions are rare; evaluate both, keep right
		c.compileExpr(e.Left)
		c.add(Drop)
		c.compileExpr(e.Right)

	default:
		panic(&CompileError{Message: fmt.Sprintf("unexpected expression type: %T", expr)})
	}
}

// compileBinaryExpr compiles a binary expression.
func (c *compiler) compileBinaryExpr(e *ast.BinaryExpr) {
	// Short-circuit operators
	switch e.Op {
	case token.AND:
		c.compileExpr(e.Left)
		c.add(Dupe)
		mark := c.jumpForward(JumpFalse)
		c.add(Drop)
		c.compileExpr(e.Right)
		c.patchForward(mark)
		c.add(Boolean)
		return

	case token.OR:
		c.compileExpr(e.Left)
		c.add(Dupe)
		mark := c.jumpForward(JumpTrue)
		c.add(Drop)
		c.compileExpr(e.Right)
		c.patchForward(mark)
		c.add(Boolean)
		return
	}

	// Compile operands
	c.compileExpr(e.Left)
	c.compileExpr(e.Right)

	// Check if we can use specialized numeric opcodes (P1-003 optimization).
	// This is only safe when BOTH operands are provably numeric.
	useTyped := c.typeInfo != nil && c.typeInfo.BothNumeric(e.Left, e.Right)

	// Emit operator (with typed variants for numeric operations)
	switch e.Op {
	case token.ADD:
		if useTyped {
			c.add(AddNum)
		} else {
			c.add(Add)
		}
	case token.SUB:
		if useTyped {
			c.add(SubNum)
		} else {
			c.add(Subtract)
		}
	case token.MUL:
		if useTyped {
			c.add(MulNum)
		} else {
			c.add(Multiply)
		}
	case token.DIV:
		if useTyped {
			c.add(DivNum)
		} else {
			c.add(Divide)
		}
	case token.MOD:
		if useTyped {
			c.add(ModNum)
		} else {
			c.add(Modulo)
		}
	case token.POW:
		if useTyped {
			c.add(PowNum)
		} else {
			c.add(Power)
		}
	case token.EQUALS:
		if useTyped {
			c.add(EqualNum)
		} else {
			c.add(Equal)
		}
	case token.NOT_EQUALS:
		if useTyped {
			c.add(NotEqualNum)
		} else {
			c.add(NotEqual)
		}
	case token.LESS:
		if useTyped {
			c.add(LessNum)
		} else {
			c.add(Less)
		}
	case token.LTE:
		if useTyped {
			c.add(LessEqNum)
		} else {
			c.add(LessEqual)
		}
	case token.GREATER:
		if useTyped {
			c.add(GreaterNum)
		} else {
			c.add(Greater)
		}
	case token.GTE:
		if useTyped {
			c.add(GreaterEqNum)
		} else {
			c.add(GreaterEqual)
		}
	case token.MATCH:
		c.add(Match)
	case token.NOT_MATCH:
		c.add(NotMatch)
	default:
		panic(&CompileError{Message: fmt.Sprintf("unknown binary operator: %v", e.Op)})
	}
}

// compileUnaryExpr compiles a unary expression.
func (c *compiler) compileUnaryExpr(e *ast.UnaryExpr) {
	switch e.Op {
	case token.INCR, token.DECR:
		// Pre/post increment in expression context
		op := Add
		if e.Op == token.DECR {
			op = Subtract
		}

		if e.Post {
			// Post: return original value, then increment
			c.compileDupeIndexLValue(e.Expr)
			c.add(UnaryPlus) // Coerce to number
			c.add(Dupe)
			c.add(Num, opcodeInt(c.numIndex(1)))
			c.add(op)
			c.compileAssignRoteIndex(e.Expr)
		} else {
			// Pre: increment first, then return new value
			c.compileDupeIndexLValue(e.Expr)
			c.add(Num, opcodeInt(c.numIndex(1)))
			c.add(op)
			c.add(Dupe)
			c.compileAssignRoteIndex(e.Expr)
		}
		return
	}

	c.compileExpr(e.Expr)

	// Check if we can use typed opcode for unary minus
	useTyped := c.typeInfo != nil && c.typeInfo.IsNumericExpr(e.Expr)

	switch e.Op {
	case token.SUB:
		if useTyped {
			c.add(NegNum)
		} else {
			c.add(UnaryMinus)
		}
	case token.ADD:
		c.add(UnaryPlus)
	case token.NOT:
		c.add(Not)
	default:
		panic(&CompileError{Message: fmt.Sprintf("unknown unary operator: %v", e.Op)})
	}
}

// compileCallExpr compiles a user-defined function call.
func (c *compiler) compileCallExpr(e *ast.CallExpr) {
	funcInfo, ok := c.resolved.GetFunction(e.Name)
	if !ok {
		panic(&CompileError{Message: fmt.Sprintf("undefined function: %s", e.Name)})
	}

	// Find compiled function to get array info
	var compiledFunc *Function
	for i := range c.program.Functions {
		if c.program.Functions[i].Name == e.Name {
			compiledFunc = &c.program.Functions[i]
			break
		}
	}

	var arrayOpcodes []Opcode
	numScalarArgs := 0

	for i, arg := range e.Args {
		if compiledFunc != nil && i < len(compiledFunc.Arrays) && compiledFunc.Arrays[i] {
			// Array argument
			if ident, ok := arg.(*ast.Ident); ok {
				scope, idx := c.lookupArray(ident.Name)
				arrayOpcodes = append(arrayOpcodes, Opcode(scope), opcodeInt(idx))
			}
		} else {
			// Scalar argument
			c.compileExpr(arg)
			numScalarArgs++
		}
	}

	// Push nulls for missing scalar arguments
	if compiledFunc != nil && numScalarArgs < compiledFunc.NumScalars {
		c.add(Nulls, opcodeInt(compiledFunc.NumScalars-numScalarArgs))
	}

	c.add(CallUser, opcodeInt(funcInfo.Index), opcodeInt(len(arrayOpcodes)/2))
	c.add(arrayOpcodes...)
}

// compileBuiltinExpr compiles a built-in function call.
func (c *compiler) compileBuiltinExpr(e *ast.BuiltinExpr) {
	// Special cases that need array or lvalue handling
	switch e.Func {
	case token.F_SPLIT:
		c.compileExpr(e.Args[0])
		if ident, ok := e.Args[1].(*ast.Ident); ok {
			scope, idx := c.lookupArray(ident.Name)
			if len(e.Args) > 2 {
				// Separator: if regex literal, push pattern as string
				if regex, ok := e.Args[2].(*ast.RegexLit); ok {
					c.add(Str, opcodeInt(c.strIndex(regex.Pattern)))
				} else {
					c.compileExpr(e.Args[2])
				}
				c.add(CallSplitSep, Opcode(scope), opcodeInt(idx))
			} else {
				c.add(CallSplit, Opcode(scope), opcodeInt(idx))
			}
		}
		return

	case token.F_SUB, token.F_GSUB:
		op := BuiltinSub
		if e.Func == token.F_GSUB {
			op = BuiltinGsub
		}
		// Default target is $0
		var target ast.Expr = &ast.FieldExpr{Index: &ast.NumLit{Value: 0}}
		if len(e.Args) == 3 {
			target = e.Args[2]
		}
		// Helper to compile pattern (regex literal must be pushed as string)
		compilePattern := func() {
			if regex, ok := e.Args[0].(*ast.RegexLit); ok {
				c.add(Str, opcodeInt(c.strIndex(regex.Pattern)))
			} else {
				c.compileExpr(e.Args[0])
			}
		}
		// Different compilation for different target types (like GoAWK)
		switch target.(type) {
		case *ast.FieldExpr, *ast.IndexExpr:
			// For fields/arrays: need to preserve index with Rote
			c.compileDupeIndexLValue(target)
			compilePattern()         // pattern as string
			c.compileExpr(e.Args[1]) // replacement
			c.add(Rote)
			c.add(CallBuiltin, Opcode(op))
			c.compileAssignRoteIndex(target)
		case *ast.Ident:
			// For simple variables: no Rote needed
			compilePattern()         // pattern as string
			c.compileExpr(e.Args[1]) // replacement
			c.compileExpr(target)    // target value
			c.add(CallBuiltin, Opcode(op))
			c.compileAssign(target, token.ASSIGN)
		default:
			// Fallback for other expressions
			compilePattern()
			c.compileExpr(e.Args[1])
			c.compileExpr(target)
			c.add(CallBuiltin, Opcode(op))
		}
		return

	case token.F_LENGTH:
		if len(e.Args) > 0 {
			// Check if argument is an array
			if ident, ok := e.Args[0].(*ast.Ident); ok {
				sym, _, ok := c.resolved.LookupVar(c.funcName, ident.Name)
				if ok && sym.Type == semantic.TypeArray {
					scope, idx := c.lookupArray(ident.Name)
					c.add(CallLength, Opcode(scope), opcodeInt(idx))
					return
				}
			}
			c.compileExpr(e.Args[0])
			c.add(CallBuiltin, Opcode(BuiltinLengthArg))
		} else {
			c.add(CallBuiltin, Opcode(BuiltinLength))
		}
		return

	case token.F_SPRINTF:
		for _, arg := range e.Args {
			c.compileExpr(arg)
		}
		c.add(CallSprintf, opcodeInt(len(e.Args)))
		return

	case token.F_MATCH:
		// match(str, pattern) - pattern must be pushed as string, not executed
		c.compileExpr(e.Args[0]) // str
		if regex, ok := e.Args[1].(*ast.RegexLit); ok {
			c.add(Str, opcodeInt(c.strIndex(regex.Pattern)))
		} else {
			c.compileExpr(e.Args[1])
		}
		c.add(CallBuiltin, Opcode(BuiltinMatch))
		return
	}

	// Generic builtins
	for _, arg := range e.Args {
		c.compileExpr(arg)
	}

	var op BuiltinOp
	switch e.Func {
	case token.F_ATAN2:
		op = BuiltinAtan2
	case token.F_CLOSE:
		op = BuiltinClose
	case token.F_COS:
		op = BuiltinCos
	case token.F_EXP:
		op = BuiltinExp
	case token.F_FFLUSH:
		if len(e.Args) > 0 {
			op = BuiltinFflush
		} else {
			op = BuiltinFflushAll
		}
	case token.F_INDEX:
		op = BuiltinIndex
	case token.F_INT:
		op = BuiltinInt
	case token.F_LOG:
		op = BuiltinLog
	case token.F_RAND:
		op = BuiltinRand
	case token.F_SIN:
		op = BuiltinSin
	case token.F_SQRT:
		op = BuiltinSqrt
	case token.F_SRAND:
		if len(e.Args) > 0 {
			op = BuiltinSrandSeed
		} else {
			op = BuiltinSrand
		}
	case token.F_SUBSTR:
		if len(e.Args) > 2 {
			op = BuiltinSubstrLen
		} else {
			op = BuiltinSubstr
		}
	case token.F_SYSTEM:
		op = BuiltinSystem
	case token.F_TOLOWER:
		op = BuiltinTolower
	case token.F_TOUPPER:
		op = BuiltinToupper
	default:
		panic(&CompileError{Message: fmt.Sprintf("unknown builtin: %v", e.Func)})
	}
	c.add(CallBuiltin, Opcode(op))
}

// compileGetlineExpr compiles a getline expression.
func (c *compiler) compileGetlineExpr(e *ast.GetlineExpr) {
	// Determine redirect type
	redirect := RedirectNone
	if e.Command != nil {
		c.compileExpr(e.Command)
		redirect = RedirectPipe
	} else if e.File != nil {
		c.compileExpr(e.File)
		redirect = RedirectInput
	}

	// Handle target
	switch target := e.Target.(type) {
	case *ast.Ident:
		scope, idx := c.lookupScalar(target.Name)
		c.add(GetlineVar, Opcode(redirect), Opcode(scope), opcodeInt(idx))
	case *ast.FieldExpr:
		c.compileExpr(target.Index)
		c.add(GetlineField, Opcode(redirect))
	case *ast.IndexExpr:
		c.compileIndex(target.Index)
		if ident, ok := target.Array.(*ast.Ident); ok {
			scope, idx := c.lookupArray(ident.Name)
			c.add(GetlineArray, Opcode(redirect), Opcode(scope), opcodeInt(idx))
		}
	default:
		c.add(Getline, Opcode(redirect))
	}
}

// compileIndex compiles array index expressions.
func (c *compiler) compileIndex(indexes []ast.Expr) {
	for _, idx := range indexes {
		// Optimize integer constants to string form
		if num, ok := idx.(*ast.NumLit); ok && num.Value == float64(int64(num.Value)) {
			s := strconv.FormatInt(int64(num.Value), 10)
			c.add(Str, opcodeInt(c.strIndex(s)))
			continue
		}
		c.compileExpr(idx)
	}
	if len(indexes) > 1 {
		c.add(IndexMulti, opcodeInt(len(indexes)))
	}
}

// compileAssign compiles an assignment to an lvalue.
func (c *compiler) compileAssign(target ast.Expr, op token.Token) {
	// Handle augmented assignment
	if op != token.ASSIGN {
		c.compileAugAssign(target, op)
		return
	}

	// Simple assignment
	switch t := target.(type) {
	case *ast.Ident:
		scope, idx := c.lookupScalar(t.Name)
		switch scope {
		case ScopeGlobal:
			c.add(StoreGlobal, opcodeInt(idx))
		case ScopeLocal:
			c.add(StoreLocal, opcodeInt(idx))
		case ScopeSpecial:
			c.add(StoreSpecial, opcodeInt(idx))
		}
	case *ast.FieldExpr:
		c.compileExpr(t.Index)
		c.add(StoreField)
	case *ast.IndexExpr:
		c.compileIndex(t.Index)
		if ident, ok := t.Array.(*ast.Ident); ok {
			scope, idx := c.lookupArray(ident.Name)
			if scope == ScopeGlobal {
				c.add(ArraySetGlobal, opcodeInt(idx))
			} else {
				c.add(ArraySet, Opcode(scope), opcodeInt(idx))
			}
		}
	}
}

// compileAugAssign compiles an augmented assignment (+=, -=, etc.).
func (c *compiler) compileAugAssign(target ast.Expr, op token.Token) {
	var augOp AugOp
	switch op {
	case token.ADD_ASSIGN:
		augOp = AugAdd
	case token.SUB_ASSIGN:
		augOp = AugSub
	case token.MUL_ASSIGN:
		augOp = AugMul
	case token.DIV_ASSIGN:
		augOp = AugDiv
	case token.MOD_ASSIGN:
		augOp = AugMod
	case token.POW_ASSIGN:
		augOp = AugPow
	default:
		panic(&CompileError{Message: fmt.Sprintf("unknown assignment operator: %v", op)})
	}

	switch t := target.(type) {
	case *ast.Ident:
		scope, idx := c.lookupScalar(t.Name)
		switch scope {
		case ScopeGlobal:
			c.add(AugGlobal, Opcode(augOp), opcodeInt(idx))
		case ScopeLocal:
			c.add(AugLocal, Opcode(augOp), opcodeInt(idx))
		case ScopeSpecial:
			c.add(AugSpecial, Opcode(augOp), opcodeInt(idx))
		}
	case *ast.FieldExpr:
		c.compileExpr(t.Index)
		c.add(AugField, Opcode(augOp))
	case *ast.IndexExpr:
		c.compileIndex(t.Index)
		if ident, ok := t.Array.(*ast.Ident); ok {
			scope, idx := c.lookupArray(ident.Name)
			if scope == ScopeGlobal {
				c.add(AugArrayGlobal, Opcode(augOp), opcodeInt(idx))
			} else {
				c.add(AugArray, Opcode(augOp), Opcode(scope), opcodeInt(idx))
			}
		}
	}
}

// compileDupeIndexLValue compiles an lvalue, duplicating the index for reuse.
func (c *compiler) compileDupeIndexLValue(expr ast.Expr) {
	switch e := expr.(type) {
	case *ast.Ident:
		c.compileExpr(expr)
	case *ast.FieldExpr:
		c.compileExpr(e.Index)
		c.add(Dupe)
		c.add(Field)
	case *ast.IndexExpr:
		c.compileIndex(e.Index)
		c.add(Dupe)
		if ident, ok := e.Array.(*ast.Ident); ok {
			scope, idx := c.lookupArray(ident.Name)
			if scope == ScopeGlobal {
				c.add(ArrayGetGlobal, opcodeInt(idx))
			} else {
				c.add(ArrayGet, Opcode(scope), opcodeInt(idx))
			}
		}
	}
}

// compileAugAssignExpr compiles augmented assignment in expression context.
// For `x += y`, this pushes x, y, computes x+y, dupes result, stores back to x.
// The expression value is the final result (x+y), not the original RHS.
func (c *compiler) compileAugAssignExpr(target ast.Expr, op token.Token, rhs ast.Expr) {
	switch t := target.(type) {
	case *ast.Ident:
		// Simple variable: load, compute, dupe, store
		scope, idx := c.lookupScalar(t.Name)
		switch scope {
		case ScopeGlobal:
			c.add(LoadGlobal, opcodeInt(idx))
		case ScopeLocal:
			c.add(LoadLocal, opcodeInt(idx))
		case ScopeSpecial:
			c.add(LoadSpecial, opcodeInt(idx))
		}
		c.compileExpr(rhs)
		c.compileAugOp(op)
		c.add(Dupe)
		switch scope {
		case ScopeGlobal:
			c.add(StoreGlobal, opcodeInt(idx))
		case ScopeLocal:
			c.add(StoreLocal, opcodeInt(idx))
		case ScopeSpecial:
			c.add(StoreSpecial, opcodeInt(idx))
		}

	case *ast.FieldExpr:
		// Field: compile index, dupe for later, get field, compute, dupe result, rote, store
		c.compileExpr(t.Index)
		c.add(Dupe)
		c.add(Field)
		c.compileExpr(rhs)
		c.compileAugOp(op)
		c.add(Dupe)
		c.add(Rote)
		c.add(StoreField)

	case *ast.IndexExpr:
		// Array element: compile index, dupe for later, get element, compute, dupe, rote, store
		c.compileIndex(t.Index)
		c.add(Dupe)
		if ident, ok := t.Array.(*ast.Ident); ok {
			scope, idx := c.lookupArray(ident.Name)
			if scope == ScopeGlobal {
				c.add(ArrayGetGlobal, opcodeInt(idx))
				c.compileExpr(rhs)
				c.compileAugOp(op)
				c.add(Dupe)
				c.add(Rote)
				c.add(ArraySetGlobal, opcodeInt(idx))
			} else {
				c.add(ArrayGet, Opcode(scope), opcodeInt(idx))
				c.compileExpr(rhs)
				c.compileAugOp(op)
				c.add(Dupe)
				c.add(Rote)
				c.add(ArraySet, Opcode(scope), opcodeInt(idx))
			}
		}
	}
}

// compileAugOp emits the arithmetic operator for augmented assignment.
func (c *compiler) compileAugOp(op token.Token) {
	switch op {
	case token.ADD_ASSIGN:
		c.add(Add)
	case token.SUB_ASSIGN:
		c.add(Subtract)
	case token.MUL_ASSIGN:
		c.add(Multiply)
	case token.DIV_ASSIGN:
		c.add(Divide)
	case token.MOD_ASSIGN:
		c.add(Modulo)
	case token.POW_ASSIGN:
		c.add(Power)
	}
}

// compileAssignRoteIndex assigns with the index already on stack (via Rote).
func (c *compiler) compileAssignRoteIndex(expr ast.Expr) {
	switch e := expr.(type) {
	case *ast.Ident:
		c.compileAssign(expr, token.ASSIGN)
	case *ast.FieldExpr:
		c.add(Rote)
		c.add(StoreField)
	case *ast.IndexExpr:
		c.add(Rote)
		if ident, ok := e.Array.(*ast.Ident); ok {
			scope, idx := c.lookupArray(ident.Name)
			if scope == ScopeGlobal {
				c.add(ArraySetGlobal, opcodeInt(idx))
			} else {
				c.add(ArraySet, Opcode(scope), opcodeInt(idx))
			}
		}
	}
}
