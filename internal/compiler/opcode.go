// Package compiler compiles an AST into bytecode for the VM.
package compiler

import "fmt"

// Opcode represents a virtual machine instruction.
// Each opcode is a 32-bit signed integer, allowing for large jump offsets
// and constant indices without overflow concerns.
type Opcode int32

const (
	// Nop does nothing (used for empty blocks to distinguish from nil).
	Nop Opcode = iota

	// Stack operations
	Num  // Push number constant: Num numIndex
	Str  // Push string constant: Str strIndex
	Dupe // Duplicate top of stack
	Drop // Discard top of stack
	Swap // Swap top two stack values
	Rote // Rotate top three: [a b c] -> [c a b]

	// Variable access
	LoadGlobal  // Load global scalar: LoadGlobal index
	LoadLocal   // Load local scalar: LoadLocal index
	LoadSpecial // Load special variable: LoadSpecial index

	// Variable assignment
	StoreGlobal  // Store to global scalar: StoreGlobal index
	StoreLocal   // Store to local scalar: StoreLocal index
	StoreSpecial // Store to special variable: StoreSpecial index

	// Field access
	Field      // Get field $N (N on stack): Field
	FieldInt   // Get field $N (constant): FieldInt index
	StoreField // Set field $N (value and N on stack): StoreField

	// Array access
	ArrayGet    // Get array element: ArrayGet scope index (key on stack)
	ArraySet    // Set array element: ArraySet scope index (value and key on stack)
	ArrayDelete // Delete element: ArrayDelete scope index (key on stack)
	ArrayClear  // Delete entire array: ArrayClear scope index
	ArrayIn     // Test membership: ArrayIn scope index (key on stack)

	// Specialized global array access (no scope switch overhead)
	ArrayGetGlobal    // Get global array element: ArrayGetGlobal index (key on stack)
	ArraySetGlobal    // Set global array element: ArraySetGlobal index (value and key on stack)
	ArrayDeleteGlobal // Delete from global array: ArrayDeleteGlobal index (key on stack)
	ArrayInGlobal     // Test membership in global array: ArrayInGlobal index (key on stack)

	// Increment/Decrement (optimized for standalone statements)
	IncrGlobal  // Increment global: IncrGlobal amount index
	IncrLocal   // Increment local: IncrLocal amount index
	IncrSpecial // Increment special: IncrSpecial amount index
	IncrField   // Increment field: IncrField amount (field index on stack)
	IncrArray   // Increment array element: IncrArray amount scope index (key on stack)

	// Specialized global array increment (no scope switch overhead)
	IncrArrayGlobal // Increment global array element: IncrArrayGlobal amount index (key on stack)

	// Augmented assignment
	AugGlobal  // op= global: AugGlobal augOp index (value on stack)
	AugLocal   // op= local: AugLocal augOp index (value on stack)
	AugSpecial // op= special: AugSpecial augOp index (value on stack)
	AugField   // op= field: AugField augOp (field index and value on stack)
	AugArray   // op= array: AugArray augOp scope index (key and value on stack)

	// Specialized global array augmented assignment (no scope switch overhead)
	AugArrayGlobal // op= global array: AugArrayGlobal augOp index (key and value on stack)

	// Regex
	Regex // Push regex match against $0: Regex regexIndex

	// Multi-value operations
	IndexMulti  // Concatenate array indices with SUBSEP: IndexMulti count
	ConcatMulti // Concatenate multiple strings: ConcatMulti count

	// Arithmetic operators
	Add      // a + b
	Subtract // a - b
	Multiply // a * b
	Divide   // a / b
	Power    // a ^ b
	Modulo   // a % b

	// Comparison operators
	Equal        // a == b
	NotEqual     // a != b
	Less         // a < b
	LessEqual    // a <= b
	Greater      // a > b
	GreaterEqual // a >= b

	// String operators
	Concat   // a b (concatenation)
	Match    // a ~ b (regex match)
	NotMatch // a !~ b (regex not match)

	// Unary operators
	UnaryMinus // -a
	UnaryPlus  // +a (convert to number)
	Not        // !a
	Boolean    // Convert to boolean (0 or 1)

	// Control flow
	Jump        // Unconditional jump: Jump offset
	JumpTrue    // Jump if true: JumpTrue offset
	JumpFalse   // Jump if false: JumpFalse offset
	JumpEqual   // Jump if equal: JumpEqual offset (two values on stack)
	JumpNotEq   // Jump if not equal: JumpNotEq offset
	JumpLess    // Jump if less: JumpLess offset
	JumpLessEq  // Jump if less or equal: JumpLessEq offset
	JumpGreater // Jump if greater: JumpGreater offset
	JumpGrEq    // Jump if greater or equal: JumpGrEq offset

	// AWK control flow
	Next     // Skip to next record
	Nextfile // Skip to next file
	Exit     // Exit with status 0: Exit
	ExitCode // Exit with status: ExitCode (status on stack)

	// Loop control (for-in special handling)
	ForIn      // Begin for-in loop: ForIn varScope varIndex arrayScope arrayIndex offset
	BreakForIn // Break from for-in loop

	// Function calls
	CallBuiltin // Call builtin: CallBuiltin builtinOp
	CallUser    // Call user function: CallUser funcIndex numArrayArgs [scope index]...
	CallNative  // Call native function: CallNative funcIndex numArgs
	Return      // Return with value (value on stack)
	ReturnNull  // Return without value (return "")
	Nulls       // Push N null values: Nulls count

	// Special builtins (need special handling)
	CallSplit    // split(s, a): CallSplit scope index (string on stack)
	CallSplitSep // split(s, a, sep): CallSplitSep scope index (string and sep on stack)
	CallSprintf  // sprintf(fmt, ...): CallSprintf numArgs
	CallLength   // length(array): CallLength scope index

	// I/O operations
	Print  // print: Print numArgs redirect
	Printf // printf: Printf numArgs redirect

	// Getline operations
	Getline      // getline: Getline redirect
	GetlineVar   // getline var: GetlineVar redirect scope index
	GetlineField // getline field: GetlineField redirect (field index on stack)
	GetlineArray // getline arr[k]: GetlineArray redirect scope index (key on stack)

	// Halt marks end of execution
	Halt
)

// String returns a human-readable name for the opcode.
func (op Opcode) String() string {
	switch op {
	case Nop:
		return "Nop"
	case Num:
		return "Num"
	case Str:
		return "Str"
	case Dupe:
		return "Dupe"
	case Drop:
		return "Drop"
	case Swap:
		return "Swap"
	case Rote:
		return "Rote"
	case LoadGlobal:
		return "LoadGlobal"
	case LoadLocal:
		return "LoadLocal"
	case LoadSpecial:
		return "LoadSpecial"
	case StoreGlobal:
		return "StoreGlobal"
	case StoreLocal:
		return "StoreLocal"
	case StoreSpecial:
		return "StoreSpecial"
	case Field:
		return "Field"
	case FieldInt:
		return "FieldInt"
	case StoreField:
		return "StoreField"
	case ArrayGet:
		return "ArrayGet"
	case ArraySet:
		return "ArraySet"
	case ArrayDelete:
		return "ArrayDelete"
	case ArrayClear:
		return "ArrayClear"
	case ArrayIn:
		return "ArrayIn"
	case ArrayGetGlobal:
		return "ArrayGetGlobal"
	case ArraySetGlobal:
		return "ArraySetGlobal"
	case ArrayDeleteGlobal:
		return "ArrayDeleteGlobal"
	case ArrayInGlobal:
		return "ArrayInGlobal"
	case IncrGlobal:
		return "IncrGlobal"
	case IncrLocal:
		return "IncrLocal"
	case IncrSpecial:
		return "IncrSpecial"
	case IncrField:
		return "IncrField"
	case IncrArray:
		return "IncrArray"
	case IncrArrayGlobal:
		return "IncrArrayGlobal"
	case AugGlobal:
		return "AugGlobal"
	case AugLocal:
		return "AugLocal"
	case AugSpecial:
		return "AugSpecial"
	case AugField:
		return "AugField"
	case AugArray:
		return "AugArray"
	case AugArrayGlobal:
		return "AugArrayGlobal"
	case Regex:
		return "Regex"
	case IndexMulti:
		return "IndexMulti"
	case ConcatMulti:
		return "ConcatMulti"
	case Add:
		return "Add"
	case Subtract:
		return "Subtract"
	case Multiply:
		return "Multiply"
	case Divide:
		return "Divide"
	case Power:
		return "Power"
	case Modulo:
		return "Modulo"
	case Equal:
		return "Equal"
	case NotEqual:
		return "NotEqual"
	case Less:
		return "Less"
	case LessEqual:
		return "LessEqual"
	case Greater:
		return "Greater"
	case GreaterEqual:
		return "GreaterEqual"
	case Concat:
		return "Concat"
	case Match:
		return "Match"
	case NotMatch:
		return "NotMatch"
	case UnaryMinus:
		return "UnaryMinus"
	case UnaryPlus:
		return "UnaryPlus"
	case Not:
		return "Not"
	case Boolean:
		return "Boolean"
	case Jump:
		return "Jump"
	case JumpTrue:
		return "JumpTrue"
	case JumpFalse:
		return "JumpFalse"
	case JumpEqual:
		return "JumpEqual"
	case JumpNotEq:
		return "JumpNotEq"
	case JumpLess:
		return "JumpLess"
	case JumpLessEq:
		return "JumpLessEq"
	case JumpGreater:
		return "JumpGreater"
	case JumpGrEq:
		return "JumpGrEq"
	case Next:
		return "Next"
	case Nextfile:
		return "Nextfile"
	case Exit:
		return "Exit"
	case ExitCode:
		return "ExitCode"
	case ForIn:
		return "ForIn"
	case BreakForIn:
		return "BreakForIn"
	case CallBuiltin:
		return "CallBuiltin"
	case CallUser:
		return "CallUser"
	case CallNative:
		return "CallNative"
	case Return:
		return "Return"
	case ReturnNull:
		return "ReturnNull"
	case Nulls:
		return "Nulls"
	case CallSplit:
		return "CallSplit"
	case CallSplitSep:
		return "CallSplitSep"
	case CallSprintf:
		return "CallSprintf"
	case CallLength:
		return "CallLength"
	case Print:
		return "Print"
	case Printf:
		return "Printf"
	case Getline:
		return "Getline"
	case GetlineVar:
		return "GetlineVar"
	case GetlineField:
		return "GetlineField"
	case GetlineArray:
		return "GetlineArray"
	case Halt:
		return "Halt"
	// Fused opcodes (peephole optimization)
	case JumpGlobalLessNum:
		return "JumpGlobalLessNum"
	case JumpGlobalGrEqNum:
		return "JumpGlobalGrEqNum"
	case FieldIntGreaterNum:
		return "FieldIntGreaterNum"
	case FieldIntLessNum:
		return "FieldIntLessNum"
	case FieldIntEqualNum:
		return "FieldIntEqualNum"
	case FieldIntEqualStr:
		return "FieldIntEqualStr"
	case AddFields:
		return "AddFields"
	// Typed opcodes (static type specialization)
	case AddNum:
		return "AddNum"
	case SubNum:
		return "SubNum"
	case MulNum:
		return "MulNum"
	case DivNum:
		return "DivNum"
	case ModNum:
		return "ModNum"
	case PowNum:
		return "PowNum"
	case NegNum:
		return "NegNum"
	case LessNum:
		return "LessNum"
	case LessEqNum:
		return "LessEqNum"
	case GreaterNum:
		return "GreaterNum"
	case GreaterEqNum:
		return "GreaterEqNum"
	case EqualNum:
		return "EqualNum"
	case NotEqualNum:
		return "NotEqualNum"
	case JumpLessNum:
		return "JumpLessNum"
	case JumpLessEqNum:
		return "JumpLessEqNum"
	case JumpGreaterNum:
		return "JumpGreaterNum"
	case JumpGreaterEqNum:
		return "JumpGreaterEqNum"
	case JumpEqualNum:
		return "JumpEqualNum"
	case JumpNotEqualNum:
		return "JumpNotEqualNum"
	default:
		return fmt.Sprintf("Opcode(%d)", op)
	}
}

// AugOp represents an augmented assignment operation.
type AugOp Opcode

const (
	AugAdd AugOp = iota // +=
	AugSub              // -=
	AugMul              // *=
	AugDiv              // /=
	AugPow              // ^=
	AugMod              // %=
)

// String returns a human-readable name for the augmented operation.
func (op AugOp) String() string {
	switch op {
	case AugAdd:
		return "AugAdd"
	case AugSub:
		return "AugSub"
	case AugMul:
		return "AugMul"
	case AugDiv:
		return "AugDiv"
	case AugPow:
		return "AugPow"
	case AugMod:
		return "AugMod"
	default:
		return fmt.Sprintf("AugOp(%d)", op)
	}
}

// BuiltinOp represents a built-in function identifier.
type BuiltinOp Opcode

const (
	BuiltinAtan2 BuiltinOp = iota
	BuiltinClose
	BuiltinCos
	BuiltinExp
	BuiltinFflush
	BuiltinFflushAll
	BuiltinGsub
	BuiltinIndex
	BuiltinInt
	BuiltinLength
	BuiltinLengthArg
	BuiltinLog
	BuiltinMatch
	BuiltinRand
	BuiltinSin
	BuiltinSqrt
	BuiltinSrand
	BuiltinSrandSeed
	BuiltinSub
	BuiltinSubstr
	BuiltinSubstrLen
	BuiltinSystem
	BuiltinTolower
	BuiltinToupper
)

// String returns a human-readable name for the builtin operation.
func (op BuiltinOp) String() string {
	switch op {
	case BuiltinAtan2:
		return "atan2"
	case BuiltinClose:
		return "close"
	case BuiltinCos:
		return "cos"
	case BuiltinExp:
		return "exp"
	case BuiltinFflush:
		return "fflush"
	case BuiltinFflushAll:
		return "fflush()"
	case BuiltinGsub:
		return "gsub"
	case BuiltinIndex:
		return "index"
	case BuiltinInt:
		return "int"
	case BuiltinLength:
		return "length()"
	case BuiltinLengthArg:
		return "length"
	case BuiltinLog:
		return "log"
	case BuiltinMatch:
		return "match"
	case BuiltinRand:
		return "rand"
	case BuiltinSin:
		return "sin"
	case BuiltinSqrt:
		return "sqrt"
	case BuiltinSrand:
		return "srand()"
	case BuiltinSrandSeed:
		return "srand"
	case BuiltinSub:
		return "sub"
	case BuiltinSubstr:
		return "substr"
	case BuiltinSubstrLen:
		return "substr3"
	case BuiltinSystem:
		return "system"
	case BuiltinTolower:
		return "tolower"
	case BuiltinToupper:
		return "toupper"
	default:
		return fmt.Sprintf("BuiltinOp(%d)", op)
	}
}

// Scope represents variable scope for array operations.
type Scope int32

const (
	ScopeGlobal  Scope = iota // Global variable
	ScopeLocal                // Local variable (function parameter or local)
	ScopeSpecial              // Special variable (ARGC, ARGV, etc.)
)

// String returns a human-readable name for the scope.
func (s Scope) String() string {
	switch s {
	case ScopeGlobal:
		return "Global"
	case ScopeLocal:
		return "Local"
	case ScopeSpecial:
		return "Special"
	default:
		return fmt.Sprintf("Scope(%d)", s)
	}
}

// Redirect represents I/O redirection type for print/getline.
type Redirect int32

const (
	RedirectNone   Redirect = iota // No redirection
	RedirectWrite                  // > file
	RedirectAppend                 // >> file
	RedirectPipe                   // | command
	RedirectInput                  // < file (for getline)
)

// String returns a human-readable name for the redirect type.
func (r Redirect) String() string {
	switch r {
	case RedirectNone:
		return "none"
	case RedirectWrite:
		return ">"
	case RedirectAppend:
		return ">>"
	case RedirectPipe:
		return "|"
	case RedirectInput:
		return "<"
	default:
		return fmt.Sprintf("Redirect(%d)", r)
	}
}
