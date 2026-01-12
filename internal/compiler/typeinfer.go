// Package compiler - Type inference for static type specialization.
//
// This file implements compile-time type inference to enable specialized
// numeric opcodes. The key insight from frawk (Rust AWK) is that many AWK
// programs have variables that are provably numeric at compile time.
//
// uawk-specific optimization: This type inference is unique to uawk
// and not present in GoAWK. It enables significant speedups for
// numeric-heavy workloads like loops and accumulation.
//
// Type inference rules (conservative - only specialize when 100% certain):
//   - Numeric literals -> TypeNum
//   - String literals -> TypeStr
//   - Variables assigned only numeric values -> TypeNum
//   - Loop counters (i in `for(i=0;...)`) -> TypeNum
//   - Arithmetic results (+, -, *, /, %, ^) -> TypeNum
//   - Field access ($1, $2) -> TypeUnknown (could be anything)
//   - Variables read from input -> TypeUnknown
//   - Function parameters -> TypeUnknown (unless all call sites agree)
package compiler

import (
	"github.com/kolkov/uawk/internal/ast"
	"github.com/kolkov/uawk/internal/semantic"
	"github.com/kolkov/uawk/internal/token"
)

// InferredType represents the inferred type of an expression or variable.
// This is distinct from AWK's runtime types - it's used for compile-time
// optimization decisions.
type InferredType uint8

const (
	// TypeUnknown means the type cannot be determined at compile time.
	// Falls back to generic opcodes.
	TypeUnknown InferredType = iota

	// TypeInferNum means the value is provably numeric at compile time.
	// Can use specialized numeric opcodes.
	TypeInferNum

	// TypeInferStr means the value is provably a string at compile time.
	// Future: could enable string-specific optimizations.
	TypeInferStr
)

// String returns a human-readable name for the inferred type.
func (t InferredType) String() string {
	switch t {
	case TypeUnknown:
		return "unknown"
	case TypeInferNum:
		return "num"
	case TypeInferStr:
		return "str"
	default:
		return "invalid"
	}
}

// TypeInfo holds type inference results for a program.
// Maps expressions and variables to their inferred types.
type TypeInfo struct {
	// ExprTypes maps expression AST nodes to inferred types.
	// Key is the expression's address (pointer identity).
	ExprTypes map[ast.Expr]InferredType

	// VarTypes maps variable names to inferred types.
	// Key format: "funcName:varName" or ":varName" for globals.
	VarTypes map[string]InferredType

	// NumericLoopVars tracks variables that are loop counters.
	// These are provably numeric in the loop context.
	NumericLoopVars map[string]bool
}

// NewTypeInfo creates a new TypeInfo.
func NewTypeInfo() *TypeInfo {
	return &TypeInfo{
		ExprTypes:       make(map[ast.Expr]InferredType),
		VarTypes:        make(map[string]InferredType),
		NumericLoopVars: make(map[string]bool),
	}
}

// GetExprType returns the inferred type for an expression.
func (ti *TypeInfo) GetExprType(expr ast.Expr) InferredType {
	if t, ok := ti.ExprTypes[expr]; ok {
		return t
	}
	return TypeUnknown
}

// GetVarType returns the inferred type for a variable.
func (ti *TypeInfo) GetVarType(funcName, varName string) InferredType {
	key := funcName + ":" + varName
	if t, ok := ti.VarTypes[key]; ok {
		return t
	}
	// Check global
	key = ":" + varName
	if t, ok := ti.VarTypes[key]; ok {
		return t
	}
	return TypeUnknown
}

// IsNumericLoopVar returns true if the variable is a numeric loop counter.
func (ti *TypeInfo) IsNumericLoopVar(funcName, varName string) bool {
	key := funcName + ":" + varName
	return ti.NumericLoopVars[key]
}

// typeInferrer performs type inference on an AST.
type typeInferrer struct {
	resolved *semantic.ResolveResult
	info     *TypeInfo

	// Current function being analyzed ("" for global scope)
	currentFunc string

	// Track assignments to variables (for inferring variable types)
	// Key: "funcName:varName", Value: all types assigned to it
	varAssignments map[string][]InferredType

	// Track whether a variable has been read from unknown source
	varHasUnknownRead map[string]bool
}

// InferTypes performs type inference on a resolved program.
// Returns TypeInfo that can be used during code generation to
// emit specialized opcodes for provably-typed expressions.
func InferTypes(prog *ast.Program, resolved *semantic.ResolveResult) *TypeInfo {
	ti := &typeInferrer{
		resolved:          resolved,
		info:              NewTypeInfo(),
		varAssignments:    make(map[string][]InferredType),
		varHasUnknownRead: make(map[string]bool),
	}

	// Phase 1: Collect type information from all code paths
	ti.collectTypes(prog)

	// Phase 2: Finalize variable types based on all assignments
	ti.finalizeVarTypes()

	return ti.info
}

// collectTypes collects type information from all program parts.
func (ti *typeInferrer) collectTypes(prog *ast.Program) {
	// Process BEGIN blocks
	for _, block := range prog.Begin {
		ti.inferBlock(block)
	}

	// Process rules
	for _, rule := range prog.Rules {
		if rule.Pattern != nil {
			ti.inferExpr(rule.Pattern)
		}
		if rule.Action != nil {
			ti.inferBlock(rule.Action)
		}
	}

	// Process END blocks
	for _, block := range prog.EndBlocks {
		ti.inferBlock(block)
	}

	// Process functions
	for _, fn := range prog.Functions {
		ti.currentFunc = fn.Name
		if fn.Body != nil {
			ti.inferBlock(fn.Body)
		}
		ti.currentFunc = ""
	}
}

// finalizeVarTypes determines final variable types from all assignments.
func (ti *typeInferrer) finalizeVarTypes() {
	for key, types := range ti.varAssignments {
		// If variable has unknown read, it's unknown
		if ti.varHasUnknownRead[key] {
			ti.info.VarTypes[key] = TypeUnknown
			continue
		}

		// If all assignments are numeric, variable is numeric
		allNum := true
		allStr := true
		for _, t := range types {
			if t != TypeInferNum {
				allNum = false
			}
			if t != TypeInferStr {
				allStr = false
			}
		}

		if len(types) > 0 && allNum {
			ti.info.VarTypes[key] = TypeInferNum
		} else if len(types) > 0 && allStr {
			ti.info.VarTypes[key] = TypeInferStr
		} else {
			ti.info.VarTypes[key] = TypeUnknown
		}
	}
}

// inferBlock infers types in a block statement.
func (ti *typeInferrer) inferBlock(block *ast.BlockStmt) {
	if block == nil {
		return
	}
	for _, stmt := range block.Stmts {
		ti.inferStmt(stmt)
	}
}

// inferStmt infers types in a statement.
func (ti *typeInferrer) inferStmt(stmt ast.Stmt) {
	if stmt == nil {
		return
	}

	switch s := stmt.(type) {
	case *ast.ExprStmt:
		ti.inferExpr(s.Expr)

	case *ast.PrintStmt:
		for _, arg := range s.Args {
			ti.inferExpr(arg)
		}
		if s.Dest != nil {
			ti.inferExpr(s.Dest)
		}

	case *ast.BlockStmt:
		ti.inferBlock(s)

	case *ast.IfStmt:
		ti.inferExpr(s.Cond)
		ti.inferStmt(s.Then)
		if s.Else != nil {
			ti.inferStmt(s.Else)
		}

	case *ast.WhileStmt:
		ti.inferExpr(s.Cond)
		ti.inferStmt(s.Body)

	case *ast.DoWhileStmt:
		ti.inferStmt(s.Body)
		ti.inferExpr(s.Cond)

	case *ast.ForStmt:
		// Special handling for classic for loop: for (i=0; i<n; i++)
		// The loop variable is provably numeric
		ti.inferForLoop(s)

	case *ast.ForInStmt:
		// for (k in arr) - k is a string key
		key := ti.varKey(s.Var.Name)
		ti.recordAssignment(key, TypeInferStr)
		ti.inferStmt(s.Body)

	case *ast.ReturnStmt:
		if s.Value != nil {
			ti.inferExpr(s.Value)
		}

	case *ast.ExitStmt:
		if s.Code != nil {
			ti.inferExpr(s.Code)
		}

	case *ast.DeleteStmt:
		for _, idx := range s.Index {
			ti.inferExpr(idx)
		}
	}
}

// inferForLoop handles type inference for classic for loops.
// Detects patterns like: for (i=0; i<n; i++)
func (ti *typeInferrer) inferForLoop(s *ast.ForStmt) {
	// Check if init is numeric assignment to a variable
	var loopVar string
	if s.Init != nil {
		if exprStmt, ok := s.Init.(*ast.ExprStmt); ok {
			if assign, ok := exprStmt.Expr.(*ast.AssignExpr); ok {
				if ident, ok := assign.Left.(*ast.Ident); ok {
					// Check if RHS is numeric
					rhsType := ti.inferExpr(assign.Right)
					if rhsType == TypeInferNum {
						loopVar = ident.Name
						key := ti.varKey(loopVar)
						ti.recordAssignment(key, TypeInferNum)
						ti.info.NumericLoopVars[key] = true
					}
				}
			}
		}
	}

	// Infer condition
	if s.Cond != nil {
		ti.inferExpr(s.Cond)
	}

	// Check post statement (typically i++ or i += 1)
	if s.Post != nil {
		if exprStmt, ok := s.Post.(*ast.ExprStmt); ok {
			switch expr := exprStmt.Expr.(type) {
			case *ast.UnaryExpr:
				// i++ or ++i
				if (expr.Op == token.INCR || expr.Op == token.DECR) && loopVar != "" {
					if ident, ok := expr.Expr.(*ast.Ident); ok && ident.Name == loopVar {
						// Confirmed numeric loop pattern
						key := ti.varKey(loopVar)
						ti.info.NumericLoopVars[key] = true
					}
				}
			case *ast.AssignExpr:
				// i += 1 or i = i + 1
				if ident, ok := expr.Left.(*ast.Ident); ok && ident.Name == loopVar {
					rhsType := ti.inferExpr(expr.Right)
					if rhsType == TypeInferNum {
						key := ti.varKey(loopVar)
						ti.info.NumericLoopVars[key] = true
					}
				}
			}
		}
		ti.inferStmt(s.Post)
	}

	// Infer body
	ti.inferStmt(s.Body)
}

// inferExpr infers the type of an expression and returns it.
func (ti *typeInferrer) inferExpr(expr ast.Expr) InferredType {
	if expr == nil {
		return TypeUnknown
	}

	var t InferredType

	switch e := expr.(type) {
	case *ast.NumLit:
		t = TypeInferNum

	case *ast.StrLit:
		t = TypeInferStr

	case *ast.RegexLit:
		// Regex in pattern context returns bool (num)
		t = TypeInferNum

	case *ast.Ident:
		// Variable reference - check if we know its type
		key := ti.varKey(e.Name)
		t = ti.info.VarTypes[key]
		if t == 0 {
			t = TypeUnknown
		}

	case *ast.FieldExpr:
		// Field access - always unknown (comes from input)
		ti.inferExpr(e.Index)
		t = TypeUnknown

	case *ast.IndexExpr:
		// Array access - always unknown
		for _, idx := range e.Index {
			ti.inferExpr(idx)
		}
		t = TypeUnknown

	case *ast.BinaryExpr:
		t = ti.inferBinaryExpr(e)

	case *ast.UnaryExpr:
		t = ti.inferUnaryExpr(e)

	case *ast.TernaryExpr:
		ti.inferExpr(e.Cond)
		thenType := ti.inferExpr(e.Then)
		elseType := ti.inferExpr(e.Else)
		// Result is numeric only if both branches are numeric
		if thenType == TypeInferNum && elseType == TypeInferNum {
			t = TypeInferNum
		} else if thenType == TypeInferStr && elseType == TypeInferStr {
			t = TypeInferStr
		} else {
			t = TypeUnknown
		}

	case *ast.AssignExpr:
		t = ti.inferAssignExpr(e)

	case *ast.ConcatExpr:
		// Concatenation always produces string
		for _, sub := range e.Exprs {
			ti.inferExpr(sub)
		}
		t = TypeInferStr

	case *ast.GroupExpr:
		t = ti.inferExpr(e.Expr)

	case *ast.CallExpr:
		// User function call - unknown return type
		for _, arg := range e.Args {
			ti.inferExpr(arg)
		}
		t = TypeUnknown

	case *ast.BuiltinExpr:
		t = ti.inferBuiltinExpr(e)

	case *ast.GetlineExpr:
		// Getline reads from input - marks target as unknown
		if e.Target != nil {
			if ident, ok := e.Target.(*ast.Ident); ok {
				key := ti.varKey(ident.Name)
				ti.varHasUnknownRead[key] = true
			}
		}
		if e.File != nil {
			ti.inferExpr(e.File)
		}
		if e.Command != nil {
			ti.inferExpr(e.Command)
		}
		t = TypeInferNum // getline returns 0, 1, or -1

	case *ast.InExpr:
		// "key in array" returns boolean (0 or 1)
		for _, idx := range e.Index {
			ti.inferExpr(idx)
		}
		t = TypeInferNum

	case *ast.MatchExpr:
		// Match returns boolean (0 or 1)
		ti.inferExpr(e.Expr)
		ti.inferExpr(e.Pattern)
		t = TypeInferNum

	case *ast.CommaExpr:
		ti.inferExpr(e.Left)
		t = ti.inferExpr(e.Right)

	default:
		t = TypeUnknown
	}

	// Record the type
	ti.info.ExprTypes[expr] = t
	return t
}

// inferBinaryExpr infers the type of a binary expression.
func (ti *typeInferrer) inferBinaryExpr(e *ast.BinaryExpr) InferredType {
	leftType := ti.inferExpr(e.Left)
	rightType := ti.inferExpr(e.Right)

	switch e.Op {
	// Arithmetic operators always produce numbers
	case token.ADD, token.SUB, token.MUL, token.DIV, token.MOD, token.POW:
		return TypeInferNum

	// Comparison operators produce boolean (numeric 0 or 1)
	case token.EQUALS, token.NOT_EQUALS, token.LESS, token.LTE, token.GREATER, token.GTE:
		return TypeInferNum

	// Logical operators produce boolean (numeric 0 or 1)
	case token.AND, token.OR:
		return TypeInferNum

	// Match operators produce boolean
	case token.MATCH, token.NOT_MATCH:
		return TypeInferNum

	default:
		// Unknown binary operator - if both sides are same type, result might be that type
		if leftType == rightType {
			return leftType
		}
		return TypeUnknown
	}
}

// inferUnaryExpr infers the type of a unary expression.
func (ti *typeInferrer) inferUnaryExpr(e *ast.UnaryExpr) InferredType {
	ti.inferExpr(e.Expr)

	switch e.Op {
	// Numeric unary operators
	case token.SUB, token.ADD:
		return TypeInferNum

	// Logical not produces boolean
	case token.NOT:
		return TypeInferNum

	// Increment/decrement
	case token.INCR, token.DECR:
		// These modify a variable and return numeric
		if ident, ok := e.Expr.(*ast.Ident); ok {
			key := ti.varKey(ident.Name)
			ti.recordAssignment(key, TypeInferNum)
		}
		return TypeInferNum

	default:
		return TypeUnknown
	}
}

// inferAssignExpr infers the type of an assignment and records variable type.
func (ti *typeInferrer) inferAssignExpr(e *ast.AssignExpr) InferredType {
	rhsType := ti.inferExpr(e.Right)

	// For augmented assignments (+= -= etc), result is always numeric
	if e.Op != token.ASSIGN {
		rhsType = TypeInferNum
	}

	// Record assignment to variable
	if ident, ok := e.Left.(*ast.Ident); ok {
		key := ti.varKey(ident.Name)
		ti.recordAssignment(key, rhsType)
	}

	// Infer left side (for array indices, etc.)
	ti.inferExpr(e.Left)

	return rhsType
}

// inferBuiltinExpr infers the type of a builtin function call.
func (ti *typeInferrer) inferBuiltinExpr(e *ast.BuiltinExpr) InferredType {
	// Infer all arguments
	for _, arg := range e.Args {
		ti.inferExpr(arg)
	}

	// Classify builtins by return type
	switch e.Func {
	// Numeric return type
	case token.F_ATAN2, token.F_COS, token.F_EXP, token.F_INT, token.F_LOG,
		token.F_RAND, token.F_SIN, token.F_SQRT, token.F_SRAND,
		token.F_INDEX, token.F_LENGTH, token.F_MATCH, token.F_SPLIT,
		token.F_SUB, token.F_GSUB, token.F_SYSTEM:
		return TypeInferNum

	// String return type
	case token.F_SPRINTF, token.F_SUBSTR, token.F_TOLOWER, token.F_TOUPPER:
		return TypeInferStr

	// Unknown/varies
	default:
		return TypeUnknown
	}
}

// varKey returns the key for a variable in the current scope.
func (ti *typeInferrer) varKey(name string) string {
	return ti.currentFunc + ":" + name
}

// recordAssignment records an assignment to a variable.
func (ti *typeInferrer) recordAssignment(key string, t InferredType) {
	ti.varAssignments[key] = append(ti.varAssignments[key], t)
}

// IsNumericExpr returns true if the expression is provably numeric.
// This is the main entry point for the compiler to check if specialized
// opcodes can be used.
func (ti *TypeInfo) IsNumericExpr(expr ast.Expr) bool {
	return ti.GetExprType(expr) == TypeInferNum
}

// BothNumeric returns true if both expressions are provably numeric.
// Used for binary operations to determine if specialized opcodes can be used.
func (ti *TypeInfo) BothNumeric(left, right ast.Expr) bool {
	return ti.GetExprType(left) == TypeInferNum && ti.GetExprType(right) == TypeInferNum
}
