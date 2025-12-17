package semantic

import (
	"sort"

	"github.com/kolkov/uawk/internal/ast"
	"github.com/kolkov/uawk/internal/token"
)

// ResolveResult contains the results of semantic analysis.
type ResolveResult struct {
	// Global symbol table (includes specials)
	Globals *SymbolTable

	// User-defined functions by name
	Functions map[string]*FuncInfo

	// Ordered list of global variable names (for VM allocation)
	GlobalVars []string

	// Ordered list of special variable names
	SpecialVars []string

	// Errors encountered during resolution
	Errors ErrorList

	// Warnings (non-fatal issues)
	Warnings WarningList
}

// Resolver performs semantic analysis on an AST.
type Resolver struct {
	result *ResolveResult

	// Current scope for variable resolution
	currentScope *SymbolTable

	// Current function being analyzed (nil if in global scope)
	currentFunc *FuncInfo

	// Context tracking
	inLoop   int  // Loop nesting depth
	inFunc   bool // Whether we're inside a function
	inBegin  bool // Whether we're in BEGIN block
	inEnd    bool // Whether we're in END block
	inAction bool // Whether we're in a pattern-action rule

	// Type inference iteration count
	typeUpdates int
}

// Resolve performs semantic analysis on the given program.
// Returns the resolution result containing symbol tables, errors, and warnings.
func Resolve(prog *ast.Program) (*ResolveResult, error) {
	r := &Resolver{
		result: &ResolveResult{
			Globals:   NewSymbolTable(nil, "global"),
			Functions: make(map[string]*FuncInfo),
		},
	}

	// Initialize current scope to global
	r.currentScope = r.result.Globals

	// Phase 1: Pre-define special variables
	r.defineSpecials()

	// Phase 2: Collect all function declarations (first pass)
	r.collectFunctions(prog)

	// Phase 3: Resolve all scopes (second pass)
	r.resolveProgram(prog)

	// Phase 4: Type inference iterations (for complex call graphs)
	r.inferTypes(prog)

	// Phase 5: Finalize - assign indices and check for unused symbols
	r.finalize()

	if err := r.result.Errors.Err(); err != nil {
		return r.result, err
	}
	return r.result, nil
}

// defineSpecials pre-defines all AWK special variables.
func (r *Resolver) defineSpecials() {
	for name, idx := range specialVars {
		typ := TypeScalar
		if IsSpecialArray(name) {
			typ = TypeArray
		}
		sym := r.result.Globals.DefineWithType(name, SymbolSpecial, typ, token.Position{Line: 1, Column: 1})
		if sym != nil {
			sym.Index = idx
			sym.Used = true // Specials are always considered "used"
		}
	}
}

// collectFunctions collects all function declarations without resolving bodies.
func (r *Resolver) collectFunctions(prog *ast.Program) {
	for _, fn := range prog.Functions {
		if _, exists := r.result.Functions[fn.Name]; exists {
			r.result.Errors.Add(fn.NamePos, errDuplicateFunc, fn.Name)
			continue
		}

		// Check if function name conflicts with builtin
		if IsBuiltinFunc(fn.Name) {
			// AWK allows shadowing builtins with user functions
		}

		funcInfo := &FuncInfo{
			Name:      fn.Name,
			Params:    fn.Params,
			NumParams: fn.NumParams,
			Symbols:   NewSymbolTable(r.result.Globals, fn.Name),
			Index:     len(r.result.Functions),
			Pos:       fn.NamePos,
		}

		// Define parameters as local symbols
		seen := make(map[string]bool)
		for i, param := range fn.Params {
			if param == fn.Name {
				r.result.Errors.Add(fn.NamePos, errParamShadowsFunc, param)
				continue
			}
			if seen[param] {
				r.result.Errors.Add(fn.NamePos, errDuplicateParam, param, fn.Name)
				continue
			}
			seen[param] = true

			kind := SymbolParam
			if i >= fn.NumParams {
				kind = SymbolLocal // Extra parameters are locals
			}
			funcInfo.Symbols.Define(param, kind, fn.NamePos)
		}

		r.result.Functions[fn.Name] = funcInfo
	}
}

// resolveProgram resolves all parts of the program.
func (r *Resolver) resolveProgram(prog *ast.Program) {
	// Resolve BEGIN blocks
	for _, block := range prog.Begin {
		r.inBegin = true
		r.resolveBlock(block)
		r.inBegin = false
	}

	// Resolve pattern-action rules
	for _, rule := range prog.Rules {
		r.inAction = true
		r.resolveRule(rule)
		r.inAction = false
	}

	// Resolve END blocks
	for _, block := range prog.EndBlocks {
		r.inEnd = true
		r.resolveBlock(block)
		r.inEnd = false
	}

	// Resolve function bodies
	for _, fn := range prog.Functions {
		r.resolveFunction(fn)
	}
}

// resolveFunction resolves a function definition.
func (r *Resolver) resolveFunction(fn *ast.FuncDecl) {
	funcInfo, ok := r.result.Functions[fn.Name]
	if !ok {
		return // Error already reported
	}

	// Enter function scope
	oldScope := r.currentScope
	oldFunc := r.currentFunc
	r.currentScope = funcInfo.Symbols
	r.currentFunc = funcInfo
	r.inFunc = true

	// Resolve body
	if fn.Body != nil {
		r.resolveBlock(fn.Body)
	}

	// Restore scope
	r.currentScope = oldScope
	r.currentFunc = oldFunc
	r.inFunc = false
}

// resolveRule resolves a pattern-action rule.
func (r *Resolver) resolveRule(rule *ast.Rule) {
	if rule.Pattern != nil {
		r.resolveExpr(rule.Pattern)
	}
	if rule.Action != nil {
		r.resolveBlock(rule.Action)
	}
}

// resolveBlock resolves a block of statements.
func (r *Resolver) resolveBlock(block *ast.BlockStmt) {
	if block == nil {
		return
	}
	for _, stmt := range block.Stmts {
		r.resolveStmt(stmt)
	}
}

// resolveStmt resolves a single statement.
func (r *Resolver) resolveStmt(stmt ast.Stmt) {
	if stmt == nil {
		return
	}

	switch s := stmt.(type) {
	case *ast.ExprStmt:
		r.resolveExpr(s.Expr)

	case *ast.PrintStmt:
		for _, arg := range s.Args {
			r.resolveExpr(arg)
		}
		if s.Dest != nil {
			r.resolveExpr(s.Dest)
		}

	case *ast.BlockStmt:
		r.resolveBlock(s)

	case *ast.IfStmt:
		r.resolveExpr(s.Cond)
		r.resolveStmt(s.Then)
		if s.Else != nil {
			r.resolveStmt(s.Else)
		}

	case *ast.WhileStmt:
		r.resolveExpr(s.Cond)
		r.inLoop++
		r.resolveStmt(s.Body)
		r.inLoop--

	case *ast.DoWhileStmt:
		r.inLoop++
		r.resolveStmt(s.Body)
		r.inLoop--
		r.resolveExpr(s.Cond)

	case *ast.ForStmt:
		if s.Init != nil {
			r.resolveStmt(s.Init)
		}
		if s.Cond != nil {
			r.resolveExpr(s.Cond)
		}
		if s.Post != nil {
			r.resolveStmt(s.Post)
		}
		r.inLoop++
		r.resolveStmt(s.Body)
		r.inLoop--

	case *ast.ForInStmt:
		// The loop variable is a scalar
		r.resolveVarRef(s.Var.Name, TypeScalar, s.Var.Pos())

		// The array must be... an array
		if ident, ok := s.Array.(*ast.Ident); ok {
			r.resolveVarRef(ident.Name, TypeArray, ident.Pos())
		} else {
			r.resolveExpr(s.Array)
		}

		r.inLoop++
		r.resolveStmt(s.Body)
		r.inLoop--

	case *ast.BreakStmt:
		if r.inLoop == 0 {
			r.result.Errors.Add(s.Pos(), errBreakOutsideLoop)
		}

	case *ast.ContinueStmt:
		if r.inLoop == 0 {
			r.result.Errors.Add(s.Pos(), errContinueOutsideLoop)
		}

	case *ast.NextStmt:
		if r.inBegin || r.inEnd {
			r.result.Errors.Add(s.Pos(), errNextInBeginEnd)
		}

	case *ast.NextFileStmt:
		if r.inBegin || r.inEnd {
			r.result.Errors.Add(s.Pos(), errNextInBeginEnd)
		}

	case *ast.ReturnStmt:
		if !r.inFunc {
			r.result.Errors.Add(s.Pos(), errReturnOutsideFunc)
		}
		if s.Value != nil {
			r.resolveExpr(s.Value)
		}

	case *ast.ExitStmt:
		if s.Code != nil {
			r.resolveExpr(s.Code)
		}

	case *ast.DeleteStmt:
		// delete target must be an array
		if ident, ok := s.Array.(*ast.Ident); ok {
			r.resolveVarRef(ident.Name, TypeArray, ident.Pos())
		}
		for _, idx := range s.Index {
			r.resolveExpr(idx)
		}
	}
}

// resolveExpr resolves an expression.
func (r *Resolver) resolveExpr(expr ast.Expr) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *ast.NumLit, *ast.StrLit, *ast.RegexLit:
		// Literals need no resolution

	case *ast.Ident:
		// Variable reference - scalar by default in expression context
		r.resolveVarRef(e.Name, TypeScalar, e.Pos())

	case *ast.FieldExpr:
		r.resolveExpr(e.Index)

	case *ast.IndexExpr:
		// Array access
		if ident, ok := e.Array.(*ast.Ident); ok {
			r.resolveVarRef(ident.Name, TypeArray, ident.Pos())
		} else {
			r.resolveExpr(e.Array)
		}
		for _, idx := range e.Index {
			r.resolveExpr(idx)
		}

	case *ast.BinaryExpr:
		r.resolveExpr(e.Left)
		r.resolveExpr(e.Right)

	case *ast.UnaryExpr:
		r.resolveExpr(e.Expr)

	case *ast.TernaryExpr:
		r.resolveExpr(e.Cond)
		r.resolveExpr(e.Then)
		r.resolveExpr(e.Else)

	case *ast.AssignExpr:
		r.resolveExpr(e.Left)
		r.resolveExpr(e.Right)

	case *ast.ConcatExpr:
		for _, sub := range e.Exprs {
			r.resolveExpr(sub)
		}

	case *ast.GroupExpr:
		r.resolveExpr(e.Expr)

	case *ast.CallExpr:
		r.resolveCall(e)

	case *ast.BuiltinExpr:
		r.resolveBuiltin(e)

	case *ast.GetlineExpr:
		if e.Target != nil {
			r.resolveExpr(e.Target)
		}
		if e.File != nil {
			r.resolveExpr(e.File)
		}
		if e.Command != nil {
			r.resolveExpr(e.Command)
		}

	case *ast.InExpr:
		for _, idx := range e.Index {
			r.resolveExpr(idx)
		}
		if ident, ok := e.Array.(*ast.Ident); ok {
			r.resolveVarRef(ident.Name, TypeArray, ident.Pos())
		} else {
			r.resolveExpr(e.Array)
		}

	case *ast.MatchExpr:
		r.resolveExpr(e.Expr)
		r.resolveExpr(e.Pattern)

	case *ast.CommaExpr:
		r.resolveExpr(e.Left)
		r.resolveExpr(e.Right)
	}
}

// resolveVarRef resolves a variable reference, creating the symbol if needed.
// AWK auto-creates global variables on first use.
func (r *Resolver) resolveVarRef(name string, expectedType VarType, pos token.Position) *Symbol {
	// First check if it's a special variable
	if IsSpecialVar(name) {
		sym, _ := r.result.Globals.LookupLocal(name)
		if sym != nil {
			sym.Used = true
			// Check type compatibility
			if expectedType != TypeUnknown && sym.Type != TypeUnknown && sym.Type != expectedType {
				r.result.Errors.Add(pos, errArrayScalarConflict, name)
			}
		}
		return sym
	}

	// Check local scope first (if in function)
	if r.currentFunc != nil {
		if sym, ok := r.currentFunc.Symbols.LookupLocal(name); ok {
			sym.Used = true
			// Update type if needed
			if sym.Type == TypeUnknown && expectedType != TypeUnknown {
				sym.Type = expectedType
				r.typeUpdates++
			} else if sym.Type != TypeUnknown && expectedType != TypeUnknown && sym.Type != expectedType {
				r.result.Errors.Add(pos, errArrayScalarConflict, name)
			}
			return sym
		}
	}

	// Check global scope
	if sym, ok := r.result.Globals.LookupLocal(name); ok {
		sym.Used = true
		// Update type if needed
		if sym.Type == TypeUnknown && expectedType != TypeUnknown {
			sym.Type = expectedType
			r.typeUpdates++
		} else if sym.Type != TypeUnknown && expectedType != TypeUnknown && sym.Type != expectedType {
			r.result.Errors.Add(pos, errArrayScalarConflict, name)
		}
		return sym
	}

	// Check if it's a function name being used as variable
	if _, isFunc := r.result.Functions[name]; isFunc {
		r.result.Errors.Add(pos, errVarShadowsFunc, name)
		return nil
	}

	// Auto-create as global (AWK semantics)
	sym := r.result.Globals.DefineWithType(name, SymbolGlobal, expectedType, pos)
	if sym != nil {
		sym.Used = true
		r.typeUpdates++
	}
	return sym
}

// resolveCall resolves a user-defined function call.
func (r *Resolver) resolveCall(call *ast.CallExpr) {
	funcInfo, ok := r.result.Functions[call.Name]
	if !ok {
		r.result.Errors.Add(call.Pos(), errUndefinedFunc, call.Name)
		// Still resolve arguments
		for _, arg := range call.Args {
			r.resolveExpr(arg)
		}
		return
	}

	funcInfo.Called = true

	// Check argument count
	if len(call.Args) > len(funcInfo.Params) {
		r.result.Errors.Add(call.Pos(), errTooManyArgs, call.Name)
	}

	// Resolve arguments and propagate types bidirectionally between args and params
	for i, arg := range call.Args {
		// Special handling for Ident arguments: they might be arrays passed to functions
		// In AWK, arrays are passed by reference and the parameter takes on the array type
		if ident, ok := arg.(*ast.Ident); ok {
			// Don't force TypeScalar - use TypeUnknown to allow array arguments
			argSym := r.resolveVarRef(ident.Name, TypeUnknown, ident.Pos())

			// Bidirectional type propagation between argument and parameter
			if i < len(funcInfo.Params) {
				paramName := funcInfo.Params[i]
				if paramSym, ok := funcInfo.Symbols.LookupLocal(paramName); ok {
					// Forward: argument type -> parameter type
					if argSym != nil && argSym.Type != TypeUnknown && paramSym.Type == TypeUnknown {
						paramSym.Type = argSym.Type
						r.typeUpdates++
					}
					// Backward: parameter type -> argument type (for deep call chains)
					if argSym != nil && paramSym.Type != TypeUnknown && argSym.Type == TypeUnknown {
						argSym.Type = paramSym.Type
						r.typeUpdates++
					}
				}
			}
		} else {
			// For non-Ident arguments, resolve normally
			r.resolveExpr(arg)

			// Type propagation for array elements
			if i < len(funcInfo.Params) {
				paramName := funcInfo.Params[i]
				if paramSym, ok := funcInfo.Symbols.LookupLocal(paramName); ok {
					if _, isIndex := arg.(*ast.IndexExpr); isIndex {
						// Array element is always scalar
						if paramSym.Type == TypeUnknown {
							paramSym.Type = TypeScalar
							r.typeUpdates++
						}
					}
				}
			}
		}
	}
}

// resolveBuiltin resolves a built-in function call.
func (r *Resolver) resolveBuiltin(builtin *ast.BuiltinExpr) {
	// Special handling for split() - second arg is always array
	if builtin.Func == token.F_SPLIT && len(builtin.Args) >= 2 {
		r.resolveExpr(builtin.Args[0])
		if ident, ok := builtin.Args[1].(*ast.Ident); ok {
			r.resolveVarRef(ident.Name, TypeArray, ident.Pos())
		}
		for _, arg := range builtin.Args[2:] {
			r.resolveExpr(arg)
		}
		return
	}

	// Special handling for length() - argument may be array or scalar
	if builtin.Func == token.F_LENGTH && len(builtin.Args) > 0 {
		if ident, ok := builtin.Args[0].(*ast.Ident); ok {
			// Don't force a type - could be either
			r.resolveVarRef(ident.Name, TypeUnknown, ident.Pos())
			return
		}
	}

	// Default: resolve all arguments
	for _, arg := range builtin.Args {
		r.resolveExpr(arg)
	}
}

// inferTypes performs additional type inference passes for complex call graphs.
func (r *Resolver) inferTypes(prog *ast.Program) {
	// Do additional passes while types are being updated
	for i := 0; i < 100; i++ {
		prevUpdates := r.typeUpdates
		r.resolveProgram(prog)
		if r.typeUpdates == prevUpdates {
			break
		}
	}

	// Set any remaining unknown types to scalar
	r.result.Globals.ForEach(func(name string, sym *Symbol) {
		if sym.Type == TypeUnknown {
			sym.Type = TypeScalar
		}
	})

	for _, funcInfo := range r.result.Functions {
		funcInfo.Symbols.ForEach(func(name string, sym *Symbol) {
			if sym.Type == TypeUnknown {
				sym.Type = TypeScalar
			}
		})
	}
}

// finalize assigns indices and generates warnings.
func (r *Resolver) finalize() {
	// Collect and sort global variables (excluding specials)
	var globals []string
	r.result.Globals.ForEach(func(name string, sym *Symbol) {
		if sym.Kind == SymbolGlobal {
			globals = append(globals, name)
		}
	})
	sort.Strings(globals)

	// Assign indices by type (scalars first, then arrays)
	scalarIdx := 0
	arrayIdx := 0
	for _, name := range globals {
		sym, _ := r.result.Globals.LookupLocal(name)
		if sym.Type == TypeArray {
			sym.Index = arrayIdx
			arrayIdx++
		} else {
			sym.Index = scalarIdx
			scalarIdx++
		}
	}
	r.result.GlobalVars = globals

	// Collect special variable names
	var specials []string
	r.result.Globals.ForEach(func(name string, sym *Symbol) {
		if sym.Kind == SymbolSpecial {
			specials = append(specials, name)
		}
	})
	sort.Strings(specials)
	r.result.SpecialVars = specials

	// Assign indices to function local variables
	for _, funcInfo := range r.result.Functions {
		scalarIdx := 0
		arrayIdx := 0
		for _, paramName := range funcInfo.Params {
			if sym, ok := funcInfo.Symbols.LookupLocal(paramName); ok {
				if sym.Type == TypeArray {
					sym.Index = arrayIdx
					arrayIdx++
				} else {
					sym.Index = scalarIdx
					scalarIdx++
				}
			}
		}
	}

	// Generate warnings for unused symbols
	r.result.Globals.ForEach(func(name string, sym *Symbol) {
		if sym.Kind == SymbolGlobal && !sym.Used {
			r.result.Warnings.Add(sym.Pos, warnUnusedVar, name)
		}
	})

	for _, funcInfo := range r.result.Functions {
		if !funcInfo.Called {
			r.result.Warnings.Add(funcInfo.Pos, warnUnusedFunc, funcInfo.Name)
		}

		funcInfo.Symbols.ForEach(func(name string, sym *Symbol) {
			if !sym.Used {
				r.result.Warnings.Add(sym.Pos, warnUnusedParam, name)
			}
		})
	}
}

// LookupVar looks up a variable by name in the current resolution context.
// Returns the symbol, its scope kind, and whether it was found.
func (r *ResolveResult) LookupVar(funcName, varName string) (*Symbol, SymbolKind, bool) {
	// If in a function, check local scope first
	if funcName != "" {
		if funcInfo, ok := r.Functions[funcName]; ok {
			if sym, ok := funcInfo.Symbols.LookupLocal(varName); ok {
				return sym, sym.Kind, true
			}
		}
	}

	// Check global scope
	if sym, ok := r.Globals.LookupLocal(varName); ok {
		return sym, sym.Kind, true
	}

	return nil, 0, false
}

// GetFunction returns function info by name.
func (r *ResolveResult) GetFunction(name string) (*FuncInfo, bool) {
	fi, ok := r.Functions[name]
	return fi, ok
}
