package ast

// Visitor defines the generic visitor pattern for AST traversal.
// Type parameter T is the return type of visit methods.
//
// Example usage for type checking:
//
//	type TypeChecker struct{}
//	func (tc *TypeChecker) VisitNumLit(n *NumLit) Type { return TypeNum }
//	func (tc *TypeChecker) VisitStrLit(n *StrLit) Type { return TypeStr }
//	// ... other methods
//
// Example usage for code generation:
//
//	type CodeGen struct{ code []Instruction }
//	func (cg *CodeGen) VisitNumLit(n *NumLit) error {
//	    cg.code = append(cg.code, PushNum(n.Value))
//	    return nil
//	}
type Visitor[T any] interface {
	// Program-level
	VisitProgram(*Program) T
	VisitRule(*Rule) T
	VisitFuncDecl(*FuncDecl) T

	// Expressions - Literals
	VisitNumLit(*NumLit) T
	VisitStrLit(*StrLit) T
	VisitRegexLit(*RegexLit) T

	// Expressions - References
	VisitIdent(*Ident) T
	VisitFieldExpr(*FieldExpr) T
	VisitIndexExpr(*IndexExpr) T

	// Expressions - Operations
	VisitBinaryExpr(*BinaryExpr) T
	VisitUnaryExpr(*UnaryExpr) T
	VisitTernaryExpr(*TernaryExpr) T
	VisitAssignExpr(*AssignExpr) T
	VisitConcatExpr(*ConcatExpr) T
	VisitGroupExpr(*GroupExpr) T

	// Expressions - Calls
	VisitCallExpr(*CallExpr) T
	VisitBuiltinExpr(*BuiltinExpr) T
	VisitGetlineExpr(*GetlineExpr) T

	// Expressions - Special
	VisitInExpr(*InExpr) T
	VisitMatchExpr(*MatchExpr) T
	VisitCommaExpr(*CommaExpr) T

	// Statements
	VisitExprStmt(*ExprStmt) T
	VisitPrintStmt(*PrintStmt) T
	VisitBlockStmt(*BlockStmt) T
	VisitIfStmt(*IfStmt) T
	VisitWhileStmt(*WhileStmt) T
	VisitDoWhileStmt(*DoWhileStmt) T
	VisitForStmt(*ForStmt) T
	VisitForInStmt(*ForInStmt) T
	VisitBreakStmt(*BreakStmt) T
	VisitContinueStmt(*ContinueStmt) T
	VisitNextStmt(*NextStmt) T
	VisitNextFileStmt(*NextFileStmt) T
	VisitReturnStmt(*ReturnStmt) T
	VisitExitStmt(*ExitStmt) T
	VisitDeleteStmt(*DeleteStmt) T
}

// Walk traverses an AST in depth-first order.
// For each node, it calls fn(node). If fn returns false,
// the children of that node are not visited.
//
// Example: Count all identifiers
//
//	count := 0
//	ast.Walk(program, func(n ast.Node) bool {
//	    if _, ok := n.(*ast.Ident); ok {
//	        count++
//	    }
//	    return true // continue traversal
//	})
func Walk(node Node, fn func(Node) bool) {
	if node == nil || !fn(node) {
		return
	}

	switch n := node.(type) {
	// Program-level
	case *Program:
		for _, b := range n.Begin {
			Walk(b, fn)
		}
		for _, r := range n.Rules {
			Walk(r, fn)
		}
		for _, b := range n.EndBlocks {
			Walk(b, fn)
		}
		for _, f := range n.Functions {
			Walk(f, fn)
		}

	case *Rule:
		Walk(n.Pattern, fn)
		Walk(n.Action, fn)

	case *FuncDecl:
		Walk(n.Body, fn)

	// Expressions - Literals (no children)
	case *NumLit, *StrLit, *RegexLit:
		// no children

	// Expressions - References
	case *Ident:
		// no children

	case *FieldExpr:
		Walk(n.Index, fn)

	case *IndexExpr:
		Walk(n.Array, fn)
		for _, idx := range n.Index {
			Walk(idx, fn)
		}

	// Expressions - Operations
	case *BinaryExpr:
		Walk(n.Left, fn)
		Walk(n.Right, fn)

	case *UnaryExpr:
		Walk(n.Expr, fn)

	case *TernaryExpr:
		Walk(n.Cond, fn)
		Walk(n.Then, fn)
		Walk(n.Else, fn)

	case *AssignExpr:
		Walk(n.Left, fn)
		Walk(n.Right, fn)

	case *ConcatExpr:
		for _, e := range n.Exprs {
			Walk(e, fn)
		}

	case *GroupExpr:
		Walk(n.Expr, fn)

	// Expressions - Calls
	case *CallExpr:
		for _, arg := range n.Args {
			Walk(arg, fn)
		}

	case *BuiltinExpr:
		for _, arg := range n.Args {
			Walk(arg, fn)
		}

	case *GetlineExpr:
		Walk(n.Target, fn)
		Walk(n.File, fn)
		Walk(n.Command, fn)

	// Expressions - Special
	case *InExpr:
		for _, idx := range n.Index {
			Walk(idx, fn)
		}
		Walk(n.Array, fn)

	case *MatchExpr:
		Walk(n.Expr, fn)
		Walk(n.Pattern, fn)

	case *CommaExpr:
		Walk(n.Left, fn)
		Walk(n.Right, fn)

	// Statements
	case *ExprStmt:
		Walk(n.Expr, fn)

	case *PrintStmt:
		for _, arg := range n.Args {
			Walk(arg, fn)
		}
		Walk(n.Dest, fn)

	case *BlockStmt:
		for _, s := range n.Stmts {
			Walk(s, fn)
		}

	case *IfStmt:
		Walk(n.Cond, fn)
		Walk(n.Then, fn)
		Walk(n.Else, fn)

	case *WhileStmt:
		Walk(n.Cond, fn)
		Walk(n.Body, fn)

	case *DoWhileStmt:
		Walk(n.Body, fn)
		Walk(n.Cond, fn)

	case *ForStmt:
		Walk(n.Init, fn)
		Walk(n.Cond, fn)
		Walk(n.Post, fn)
		Walk(n.Body, fn)

	case *ForInStmt:
		Walk(n.Var, fn)
		Walk(n.Array, fn)
		Walk(n.Body, fn)

	case *BreakStmt, *ContinueStmt, *NextStmt, *NextFileStmt:
		// no children

	case *ReturnStmt:
		Walk(n.Value, fn)

	case *ExitStmt:
		Walk(n.Code, fn)

	case *DeleteStmt:
		Walk(n.Array, fn)
		for _, idx := range n.Index {
			Walk(idx, fn)
		}
	}
}

// Inspect traverses an AST with parent tracking.
// For each node, it calls fn(node, parent). The parent is nil for the root node.
// If fn returns false, the children of that node are not visited.
//
// Example: Find all identifiers inside field expressions
//
//	ast.Inspect(program, func(n, parent ast.Node) bool {
//	    if id, ok := n.(*ast.Ident); ok {
//	        if _, inField := parent.(*ast.FieldExpr); inField {
//	            fmt.Printf("Identifier %s is in field expr\n", id.Name)
//	        }
//	    }
//	    return true
//	})
func Inspect(node Node, fn func(node, parent Node) bool) {
	inspect(node, nil, fn)
}

func inspect(node, parent Node, fn func(node, parent Node) bool) {
	if node == nil || !fn(node, parent) {
		return
	}

	switch n := node.(type) {
	case *Program:
		for _, b := range n.Begin {
			inspect(b, n, fn)
		}
		for _, r := range n.Rules {
			inspect(r, n, fn)
		}
		for _, b := range n.EndBlocks {
			inspect(b, n, fn)
		}
		for _, f := range n.Functions {
			inspect(f, n, fn)
		}

	case *Rule:
		inspect(n.Pattern, n, fn)
		inspect(n.Action, n, fn)

	case *FuncDecl:
		inspect(n.Body, n, fn)

	case *NumLit, *StrLit, *RegexLit, *Ident:
		// no children

	case *FieldExpr:
		inspect(n.Index, n, fn)

	case *IndexExpr:
		inspect(n.Array, n, fn)
		for _, idx := range n.Index {
			inspect(idx, n, fn)
		}

	case *BinaryExpr:
		inspect(n.Left, n, fn)
		inspect(n.Right, n, fn)

	case *UnaryExpr:
		inspect(n.Expr, n, fn)

	case *TernaryExpr:
		inspect(n.Cond, n, fn)
		inspect(n.Then, n, fn)
		inspect(n.Else, n, fn)

	case *AssignExpr:
		inspect(n.Left, n, fn)
		inspect(n.Right, n, fn)

	case *ConcatExpr:
		for _, e := range n.Exprs {
			inspect(e, n, fn)
		}

	case *GroupExpr:
		inspect(n.Expr, n, fn)

	case *CallExpr:
		for _, arg := range n.Args {
			inspect(arg, n, fn)
		}

	case *BuiltinExpr:
		for _, arg := range n.Args {
			inspect(arg, n, fn)
		}

	case *GetlineExpr:
		inspect(n.Target, n, fn)
		inspect(n.File, n, fn)
		inspect(n.Command, n, fn)

	case *InExpr:
		for _, idx := range n.Index {
			inspect(idx, n, fn)
		}
		inspect(n.Array, n, fn)

	case *MatchExpr:
		inspect(n.Expr, n, fn)
		inspect(n.Pattern, n, fn)

	case *CommaExpr:
		inspect(n.Left, n, fn)
		inspect(n.Right, n, fn)

	case *ExprStmt:
		inspect(n.Expr, n, fn)

	case *PrintStmt:
		for _, arg := range n.Args {
			inspect(arg, n, fn)
		}
		inspect(n.Dest, n, fn)

	case *BlockStmt:
		for _, s := range n.Stmts {
			inspect(s, n, fn)
		}

	case *IfStmt:
		inspect(n.Cond, n, fn)
		inspect(n.Then, n, fn)
		inspect(n.Else, n, fn)

	case *WhileStmt:
		inspect(n.Cond, n, fn)
		inspect(n.Body, n, fn)

	case *DoWhileStmt:
		inspect(n.Body, n, fn)
		inspect(n.Cond, n, fn)

	case *ForStmt:
		inspect(n.Init, n, fn)
		inspect(n.Cond, n, fn)
		inspect(n.Post, n, fn)
		inspect(n.Body, n, fn)

	case *ForInStmt:
		inspect(n.Var, n, fn)
		inspect(n.Array, n, fn)
		inspect(n.Body, n, fn)

	case *BreakStmt, *ContinueStmt, *NextStmt, *NextFileStmt:
		// no children

	case *ReturnStmt:
		inspect(n.Value, n, fn)

	case *ExitStmt:
		inspect(n.Code, n, fn)

	case *DeleteStmt:
		inspect(n.Array, n, fn)
		for _, idx := range n.Index {
			inspect(idx, n, fn)
		}
	}
}

// WalkFunc is a convenience type for walk callbacks.
type WalkFunc func(Node) bool

// InspectFunc is a convenience type for inspect callbacks.
type InspectFunc func(node, parent Node) bool

// Accept dispatches to the appropriate visitor method based on node type.
// This implements the double-dispatch pattern for the visitor.
//
// Example:
//
//	result := ast.Accept[int](node, myVisitor)
func Accept[T any](node Node, v Visitor[T]) T {
	switch n := node.(type) {
	case *Program:
		return v.VisitProgram(n)
	case *Rule:
		return v.VisitRule(n)
	case *FuncDecl:
		return v.VisitFuncDecl(n)

	case *NumLit:
		return v.VisitNumLit(n)
	case *StrLit:
		return v.VisitStrLit(n)
	case *RegexLit:
		return v.VisitRegexLit(n)

	case *Ident:
		return v.VisitIdent(n)
	case *FieldExpr:
		return v.VisitFieldExpr(n)
	case *IndexExpr:
		return v.VisitIndexExpr(n)

	case *BinaryExpr:
		return v.VisitBinaryExpr(n)
	case *UnaryExpr:
		return v.VisitUnaryExpr(n)
	case *TernaryExpr:
		return v.VisitTernaryExpr(n)
	case *AssignExpr:
		return v.VisitAssignExpr(n)
	case *ConcatExpr:
		return v.VisitConcatExpr(n)
	case *GroupExpr:
		return v.VisitGroupExpr(n)

	case *CallExpr:
		return v.VisitCallExpr(n)
	case *BuiltinExpr:
		return v.VisitBuiltinExpr(n)
	case *GetlineExpr:
		return v.VisitGetlineExpr(n)

	case *InExpr:
		return v.VisitInExpr(n)
	case *MatchExpr:
		return v.VisitMatchExpr(n)
	case *CommaExpr:
		return v.VisitCommaExpr(n)

	case *ExprStmt:
		return v.VisitExprStmt(n)
	case *PrintStmt:
		return v.VisitPrintStmt(n)
	case *BlockStmt:
		return v.VisitBlockStmt(n)
	case *IfStmt:
		return v.VisitIfStmt(n)
	case *WhileStmt:
		return v.VisitWhileStmt(n)
	case *DoWhileStmt:
		return v.VisitDoWhileStmt(n)
	case *ForStmt:
		return v.VisitForStmt(n)
	case *ForInStmt:
		return v.VisitForInStmt(n)
	case *BreakStmt:
		return v.VisitBreakStmt(n)
	case *ContinueStmt:
		return v.VisitContinueStmt(n)
	case *NextStmt:
		return v.VisitNextStmt(n)
	case *NextFileStmt:
		return v.VisitNextFileStmt(n)
	case *ReturnStmt:
		return v.VisitReturnStmt(n)
	case *ExitStmt:
		return v.VisitExitStmt(n)
	case *DeleteStmt:
		return v.VisitDeleteStmt(n)

	default:
		var zero T
		return zero
	}
}
