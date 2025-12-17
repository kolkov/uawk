package semantic

import (
	"github.com/kolkov/uawk/internal/token"
)

// SymbolKind defines the category of a symbol.
type SymbolKind int

const (
	SymbolGlobal   SymbolKind = iota // Global variable (created on first use)
	SymbolLocal                      // Local variable (function parameter or extra param)
	SymbolParam                      // Function parameter
	SymbolSpecial                    // Special variable (NR, NF, FS, etc.)
	SymbolFunction                   // User-defined function
	SymbolBuiltin                    // Built-in function
)

// String returns a human-readable name for the symbol kind.
func (k SymbolKind) String() string {
	switch k {
	case SymbolGlobal:
		return "global"
	case SymbolLocal:
		return "local"
	case SymbolParam:
		return "param"
	case SymbolSpecial:
		return "special"
	case SymbolFunction:
		return "function"
	case SymbolBuiltin:
		return "builtin"
	default:
		return "unknown"
	}
}

// VarType represents the type of a variable (scalar or array).
// AWK infers types based on usage context.
type VarType int

const (
	TypeUnknown VarType = iota // Not yet determined
	TypeScalar                 // Scalar value (string or number)
	TypeArray                  // Associative array
)

// String returns a human-readable name for the variable type.
func (t VarType) String() string {
	switch t {
	case TypeUnknown:
		return "unknown"
	case TypeScalar:
		return "scalar"
	case TypeArray:
		return "array"
	default:
		return "invalid"
	}
}

// Symbol holds information about a declared symbol.
type Symbol struct {
	Name  string         // Symbol name
	Kind  SymbolKind     // Category (global, local, special, etc.)
	Type  VarType        // For variables: scalar or array
	Index int            // Index for VM access (within its category)
	Pos   token.Position // Declaration position
	Used  bool           // Whether the symbol is used (for warnings)
}

// IsVariable returns true if the symbol represents a variable.
func (s *Symbol) IsVariable() bool {
	return s.Kind == SymbolGlobal || s.Kind == SymbolLocal ||
		s.Kind == SymbolParam || s.Kind == SymbolSpecial
}

// IsFunction returns true if the symbol represents a function.
func (s *Symbol) IsFunction() bool {
	return s.Kind == SymbolFunction || s.Kind == SymbolBuiltin
}

// SymbolTable implements a hierarchical symbol table with scope support.
// Each scope can have a parent, enabling nested lookups.
type SymbolTable struct {
	parent  *SymbolTable
	symbols map[string]*Symbol
	name    string // Scope name (e.g., function name or "global")
}

// NewSymbolTable creates a new symbol table with the given parent.
// Pass nil for the global scope.
func NewSymbolTable(parent *SymbolTable, name string) *SymbolTable {
	return &SymbolTable{
		parent:  parent,
		symbols: make(map[string]*Symbol),
		name:    name,
	}
}

// Name returns the scope name.
func (st *SymbolTable) Name() string {
	return st.name
}

// Parent returns the parent scope, or nil for the global scope.
func (st *SymbolTable) Parent() *SymbolTable {
	return st.parent
}

// Define adds a new symbol to the current scope.
// Returns the created symbol, or nil if a symbol with that name already exists.
func (st *SymbolTable) Define(name string, kind SymbolKind, pos token.Position) *Symbol {
	if _, exists := st.symbols[name]; exists {
		return nil // Already defined in this scope
	}
	sym := &Symbol{
		Name:  name,
		Kind:  kind,
		Type:  TypeUnknown,
		Index: -1, // Will be assigned later
		Pos:   pos,
		Used:  false,
	}
	st.symbols[name] = sym
	return sym
}

// DefineWithType adds a new symbol with a known type.
func (st *SymbolTable) DefineWithType(name string, kind SymbolKind, typ VarType, pos token.Position) *Symbol {
	sym := st.Define(name, kind, pos)
	if sym != nil {
		sym.Type = typ
	}
	return sym
}

// Lookup searches for a symbol in this scope and all parent scopes.
// Returns the symbol and true if found, nil and false otherwise.
func (st *SymbolTable) Lookup(name string) (*Symbol, bool) {
	for scope := st; scope != nil; scope = scope.parent {
		if sym, ok := scope.symbols[name]; ok {
			return sym, true
		}
	}
	return nil, false
}

// LookupLocal searches for a symbol only in the current scope.
// Returns the symbol and true if found, nil and false otherwise.
func (st *SymbolTable) LookupLocal(name string) (*Symbol, bool) {
	sym, ok := st.symbols[name]
	return sym, ok
}

// Symbols returns all symbols in the current scope (not including parent scopes).
func (st *SymbolTable) Symbols() map[string]*Symbol {
	return st.symbols
}

// ForEach iterates over all symbols in the current scope.
func (st *SymbolTable) ForEach(fn func(name string, sym *Symbol)) {
	for name, sym := range st.symbols {
		fn(name, sym)
	}
}

// Count returns the number of symbols in the current scope.
func (st *SymbolTable) Count() int {
	return len(st.symbols)
}

// FuncInfo holds resolved information about a user-defined function.
type FuncInfo struct {
	Name      string       // Function name
	Params    []string     // Parameter names
	NumParams int          // Number of actual parameters (rest are locals)
	Symbols   *SymbolTable // Local symbol table for this function
	Index     int          // Index for VM dispatch
	Pos       token.Position
	Called    bool // Whether the function is called
}

// NumLocals returns the number of local variables (extra parameters).
func (fi *FuncInfo) NumLocals() int {
	return len(fi.Params) - fi.NumParams
}

// BuiltinInfo holds information about a built-in function.
type BuiltinInfo struct {
	Name    string // Function name
	MinArgs int    // Minimum number of arguments
	MaxArgs int    // Maximum number of arguments (-1 for variadic)
	Token   token.Token
}

// specialVars lists all AWK special variables with their indices.
// These are pre-defined and have special semantics.
var specialVars = map[string]int{
	"ARGC":     1,
	"ARGV":     2, // Array
	"CONVFMT":  3,
	"ENVIRON":  4, // Array
	"FILENAME": 5,
	"FNR":      6,
	"FS":       7,
	"NF":       8,
	"NR":       9,
	"OFMT":     10,
	"OFS":      11,
	"ORS":      12,
	"RLENGTH":  13,
	"RS":       14,
	"RSTART":   15,
	"SUBSEP":   16,
}

// specialArrays lists special variables that are arrays.
var specialArrays = map[string]bool{
	"ARGV":    true,
	"ENVIRON": true,
}

// IsSpecialVar returns true if name is a special AWK variable.
func IsSpecialVar(name string) bool {
	_, ok := specialVars[name]
	return ok
}

// SpecialVarIndex returns the index of a special variable, or -1 if not special.
func SpecialVarIndex(name string) int {
	if idx, ok := specialVars[name]; ok {
		return idx
	}
	return -1
}

// IsSpecialArray returns true if name is a special array variable.
func IsSpecialArray(name string) bool {
	return specialArrays[name]
}

// builtinFuncs maps built-in function names to their argument requirements.
// MinArgs is the minimum, MaxArgs is the maximum (-1 for variadic).
var builtinFuncs = map[string]BuiltinInfo{
	// String functions
	"length":  {Name: "length", MinArgs: 0, MaxArgs: 1, Token: token.F_LENGTH},
	"substr":  {Name: "substr", MinArgs: 2, MaxArgs: 3, Token: token.F_SUBSTR},
	"index":   {Name: "index", MinArgs: 2, MaxArgs: 2, Token: token.F_INDEX},
	"split":   {Name: "split", MinArgs: 2, MaxArgs: 3, Token: token.F_SPLIT},
	"sub":     {Name: "sub", MinArgs: 2, MaxArgs: 3, Token: token.F_SUB},
	"gsub":    {Name: "gsub", MinArgs: 2, MaxArgs: 3, Token: token.F_GSUB},
	"match":   {Name: "match", MinArgs: 2, MaxArgs: 2, Token: token.F_MATCH},
	"sprintf": {Name: "sprintf", MinArgs: 1, MaxArgs: -1, Token: token.F_SPRINTF},
	"tolower": {Name: "tolower", MinArgs: 1, MaxArgs: 1, Token: token.F_TOLOWER},
	"toupper": {Name: "toupper", MinArgs: 1, MaxArgs: 1, Token: token.F_TOUPPER},

	// Math functions
	"sin":   {Name: "sin", MinArgs: 1, MaxArgs: 1, Token: token.F_SIN},
	"cos":   {Name: "cos", MinArgs: 1, MaxArgs: 1, Token: token.F_COS},
	"atan2": {Name: "atan2", MinArgs: 2, MaxArgs: 2, Token: token.F_ATAN2},
	"exp":   {Name: "exp", MinArgs: 1, MaxArgs: 1, Token: token.F_EXP},
	"log":   {Name: "log", MinArgs: 1, MaxArgs: 1, Token: token.F_LOG},
	"sqrt":  {Name: "sqrt", MinArgs: 1, MaxArgs: 1, Token: token.F_SQRT},
	"int":   {Name: "int", MinArgs: 1, MaxArgs: 1, Token: token.F_INT},
	"rand":  {Name: "rand", MinArgs: 0, MaxArgs: 0, Token: token.F_RAND},
	"srand": {Name: "srand", MinArgs: 0, MaxArgs: 1, Token: token.F_SRAND},

	// I/O functions
	"close":  {Name: "close", MinArgs: 1, MaxArgs: 1, Token: token.F_CLOSE},
	"fflush": {Name: "fflush", MinArgs: 0, MaxArgs: 1, Token: token.F_FFLUSH},
	"system": {Name: "system", MinArgs: 1, MaxArgs: 1, Token: token.F_SYSTEM},
}

// IsBuiltinFunc returns true if name is a built-in function.
func IsBuiltinFunc(name string) bool {
	_, ok := builtinFuncs[name]
	return ok
}

// GetBuiltinInfo returns information about a built-in function.
func GetBuiltinInfo(name string) (BuiltinInfo, bool) {
	info, ok := builtinFuncs[name]
	return info, ok
}
