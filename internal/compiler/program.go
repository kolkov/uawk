package compiler

import (
	"fmt"
	"strings"
)

// Program represents a compiled AWK program ready for VM execution.
type Program struct {
	// Begin contains bytecode for BEGIN blocks (executed before input processing).
	Begin []Opcode

	// Actions contains pattern-action pairs (executed for each input record).
	Actions []Action

	// End contains bytecode for END blocks (executed after all input).
	End []Opcode

	// Functions contains compiled user-defined functions.
	Functions []Function

	// Constant pools
	Nums    []float64 // Numeric constants
	Strs    []string  // String constants
	Regexes []string  // Regex patterns (compiled at runtime by VM)

	// Variable metadata (for disassembly and debugging)
	ScalarNames []string // Global scalar variable names by index
	ArrayNames  []string // Global array variable names by index

	// Counts for VM allocation
	NumScalars int // Number of global scalar variables
	NumArrays  int // Number of global array variables
}

// Action represents a compiled pattern-action rule.
type Action struct {
	// Pattern contains the pattern bytecode.
	// For range patterns, Pattern has two elements [start, end].
	// Empty Pattern means the action always executes.
	Pattern [][]Opcode

	// Body contains the action bytecode.
	// nil means default action: { print $0 }
	// Empty slice (len=0) means empty action: {} (do nothing)
	Body []Opcode
}

// Function represents a compiled user-defined function.
type Function struct {
	// Name is the function name.
	Name string

	// Params contains parameter names (including locals by AWK convention).
	Params []string

	// Arrays indicates which parameters are arrays (true) vs scalars (false).
	Arrays []bool

	// NumScalars is the count of scalar parameters/locals.
	NumScalars int

	// NumArrays is the count of array parameters.
	NumArrays int

	// Body contains the function bytecode.
	Body []Opcode
}

// NumParams returns the total number of parameters (scalars + arrays).
func (f *Function) NumParams() int {
	return len(f.Params)
}

// Disassemble returns a human-readable disassembly of the program.
func (p *Program) Disassemble() string {
	var sb strings.Builder

	// Constants
	if len(p.Nums) > 0 {
		sb.WriteString("=== Numbers ===\n")
		for i, n := range p.Nums {
			fmt.Fprintf(&sb, "  [%d] %v\n", i, n)
		}
		sb.WriteString("\n")
	}

	if len(p.Strs) > 0 {
		sb.WriteString("=== Strings ===\n")
		for i, s := range p.Strs {
			fmt.Fprintf(&sb, "  [%d] %q\n", i, s)
		}
		sb.WriteString("\n")
	}

	if len(p.Regexes) > 0 {
		sb.WriteString("=== Regexes ===\n")
		for i, r := range p.Regexes {
			fmt.Fprintf(&sb, "  [%d] /%s/\n", i, r)
		}
		sb.WriteString("\n")
	}

	// BEGIN
	if len(p.Begin) > 0 {
		sb.WriteString("=== BEGIN ===\n")
		p.disassembleCode(&sb, p.Begin, "  ")
		sb.WriteString("\n")
	}

	// Actions
	for i, action := range p.Actions {
		fmt.Fprintf(&sb, "=== Action %d ===\n", i)
		for j, pat := range action.Pattern {
			fmt.Fprintf(&sb, "  Pattern[%d]:\n", j)
			p.disassembleCode(&sb, pat, "    ")
		}
		if action.Body != nil {
			sb.WriteString("  Body:\n")
			p.disassembleCode(&sb, action.Body, "    ")
		} else {
			sb.WriteString("  Body: { print $0 }\n")
		}
		sb.WriteString("\n")
	}

	// END
	if len(p.End) > 0 {
		sb.WriteString("=== END ===\n")
		p.disassembleCode(&sb, p.End, "  ")
		sb.WriteString("\n")
	}

	// Functions
	for _, fn := range p.Functions {
		fmt.Fprintf(&sb, "=== Function %s(%s) ===\n", fn.Name, strings.Join(fn.Params, ", "))
		p.disassembleCode(&sb, fn.Body, "  ")
		sb.WriteString("\n")
	}

	return sb.String()
}

// disassembleCode outputs bytecode with proper formatting.
func (p *Program) disassembleCode(sb *strings.Builder, code []Opcode, indent string) {
	for i := 0; i < len(code); i++ {
		op := code[i]
		fmt.Fprintf(sb, "%s%04d: %s", indent, i, op.String())

		// Handle opcodes with arguments
		switch op {
		case Num:
			if i+1 < len(code) {
				i++
				idx := int(code[i])
				if idx < len(p.Nums) {
					fmt.Fprintf(sb, " [%d] = %v", idx, p.Nums[idx])
				} else {
					fmt.Fprintf(sb, " [%d]", idx)
				}
			}
		case Str:
			if i+1 < len(code) {
				i++
				idx := int(code[i])
				if idx < len(p.Strs) {
					fmt.Fprintf(sb, " [%d] = %q", idx, p.Strs[idx])
				} else {
					fmt.Fprintf(sb, " [%d]", idx)
				}
			}
		case Regex:
			if i+1 < len(code) {
				i++
				idx := int(code[i])
				if idx < len(p.Regexes) {
					fmt.Fprintf(sb, " [%d] = /%s/", idx, p.Regexes[idx])
				} else {
					fmt.Fprintf(sb, " [%d]", idx)
				}
			}
		case LoadGlobal, StoreGlobal:
			if i+1 < len(code) {
				i++
				idx := int(code[i])
				if idx < len(p.ScalarNames) && p.ScalarNames[idx] != "" {
					fmt.Fprintf(sb, " %s [%d]", p.ScalarNames[idx], idx)
				} else {
					fmt.Fprintf(sb, " [%d]", idx)
				}
			}
		case LoadLocal, StoreLocal, LoadSpecial, StoreSpecial:
			if i+1 < len(code) {
				i++
				fmt.Fprintf(sb, " [%d]", code[i])
			}
		case FieldInt:
			if i+1 < len(code) {
				i++
				fmt.Fprintf(sb, " $%d", code[i])
			}
		case ArrayGet, ArraySet, ArrayDelete, ArrayIn:
			if i+2 < len(code) {
				i++
				scope := Scope(code[i])
				i++
				idx := code[i]
				fmt.Fprintf(sb, " %s [%d]", scope, idx)
			}
		case IncrGlobal, IncrLocal, IncrSpecial:
			if i+2 < len(code) {
				i++
				amount := code[i]
				i++
				idx := code[i]
				if amount > 0 {
					fmt.Fprintf(sb, " ++ [%d]", idx)
				} else {
					fmt.Fprintf(sb, " -- [%d]", idx)
				}
			}
		case IncrField:
			if i+1 < len(code) {
				i++
				amount := code[i]
				if amount > 0 {
					fmt.Fprintf(sb, " ++")
				} else {
					fmt.Fprintf(sb, " --")
				}
			}
		case AugGlobal, AugLocal, AugSpecial:
			if i+2 < len(code) {
				i++
				augOp := AugOp(code[i])
				i++
				idx := code[i]
				fmt.Fprintf(sb, " %s [%d]", augOp, idx)
			}
		case Jump, JumpTrue, JumpFalse, JumpEqual, JumpNotEq,
			JumpLess, JumpLessEq, JumpGreater, JumpGrEq:
			if i+1 < len(code) {
				i++
				offset := int(code[i])
				target := i + offset
				fmt.Fprintf(sb, " %+d -> %04d", offset, target)
			}
		case ForIn:
			if i+5 < len(code) {
				i++
				varScope := Scope(code[i])
				i++
				varIdx := code[i]
				i++
				arrScope := Scope(code[i])
				i++
				arrIdx := code[i]
				i++
				offset := int(code[i])
				fmt.Fprintf(sb, " var=%s[%d] arr=%s[%d] end=%+d", varScope, varIdx, arrScope, arrIdx, offset)
			}
		case CallBuiltin:
			if i+1 < len(code) {
				i++
				builtinOp := BuiltinOp(code[i])
				fmt.Fprintf(sb, " %s", builtinOp)
			}
		case CallUser:
			if i+2 < len(code) {
				i++
				funcIdx := code[i]
				i++
				numArrays := int(code[i])
				fmt.Fprintf(sb, " func[%d] arrays=%d", funcIdx, numArrays)
				// Skip array scope/index pairs
				for j := 0; j < numArrays*2 && i+1 < len(code); j++ {
					i++
				}
			}
		case CallNative:
			if i+2 < len(code) {
				i++
				funcIdx := code[i]
				i++
				numArgs := code[i]
				fmt.Fprintf(sb, " native[%d] args=%d", funcIdx, numArgs)
			}
		case Nulls, IndexMulti, ConcatMulti:
			if i+1 < len(code) {
				i++
				fmt.Fprintf(sb, " %d", code[i])
			}
		case CallSplit, CallSplitSep, CallLength:
			if i+2 < len(code) {
				i++
				scope := Scope(code[i])
				i++
				idx := code[i]
				fmt.Fprintf(sb, " %s [%d]", scope, idx)
			}
		case CallSprintf, Print, Printf:
			if i+2 < len(code) {
				i++
				numArgs := code[i]
				i++
				redirect := Redirect(code[i])
				fmt.Fprintf(sb, " args=%d redirect=%s", numArgs, redirect)
			}
		case Getline, GetlineField:
			if i+1 < len(code) {
				i++
				redirect := Redirect(code[i])
				fmt.Fprintf(sb, " redirect=%s", redirect)
			}
		case GetlineVar:
			if i+3 < len(code) {
				i++
				redirect := Redirect(code[i])
				i++
				scope := Scope(code[i])
				i++
				idx := code[i]
				fmt.Fprintf(sb, " redirect=%s %s[%d]", redirect, scope, idx)
			}
		case GetlineArray:
			if i+3 < len(code) {
				i++
				redirect := Redirect(code[i])
				i++
				scope := Scope(code[i])
				i++
				idx := code[i]
				fmt.Fprintf(sb, " redirect=%s %s[%d]", redirect, scope, idx)
			}
		// Fused opcodes (peephole optimization)
		case JumpGlobalLessNum, JumpGlobalGrEqNum:
			if i+3 < len(code) {
				i++
				globalIdx := code[i]
				i++
				numIdx := code[i]
				i++
				offset := int(code[i])
				target := i + offset
				var cmp string
				if op == JumpGlobalLessNum {
					cmp = "<"
				} else {
					cmp = ">="
				}
				if int(globalIdx) < len(p.ScalarNames) && p.ScalarNames[globalIdx] != "" {
					fmt.Fprintf(sb, " %s %s %v -> %04d", p.ScalarNames[globalIdx], cmp, p.Nums[numIdx], target)
				} else {
					fmt.Fprintf(sb, " global[%d] %s num[%d] -> %04d", globalIdx, cmp, numIdx, target)
				}
			}
		case FieldIntGreaterNum, FieldIntLessNum, FieldIntEqualNum:
			if i+2 < len(code) {
				i++
				fieldNum := code[i]
				i++
				numIdx := code[i]
				var cmp string
				switch op {
				case FieldIntGreaterNum:
					cmp = ">"
				case FieldIntLessNum:
					cmp = "<"
				case FieldIntEqualNum:
					cmp = "=="
				}
				fmt.Fprintf(sb, " $%d %s %v", fieldNum, cmp, p.Nums[numIdx])
			}
		case FieldIntEqualStr:
			if i+2 < len(code) {
				i++
				fieldNum := code[i]
				i++
				strIdx := code[i]
				fmt.Fprintf(sb, " $%d == %q", fieldNum, p.Strs[strIdx])
			}
		case AddFields:
			if i+2 < len(code) {
				i++
				field1 := code[i]
				i++
				field2 := code[i]
				fmt.Fprintf(sb, " $%d + $%d", field1, field2)
			}
		}

		sb.WriteString("\n")
	}
}
