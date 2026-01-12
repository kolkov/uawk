// Package compiler compiles an AST into bytecode for the VM.
//
// This file implements peephole optimization - a post-compilation pass that
// combines frequent instruction sequences into single fused opcodes.
//
// uawk-specific optimization: This peephole optimizer is unique to uawk
// and not present in GoAWK. It provides significant speedups for common
// AWK patterns like loops and field comparisons.
package compiler

// Fused opcodes for common patterns (added after standard opcodes).
// These combine multiple instructions into one for better performance.
const (
	// Field comparison: FieldInt + Num + Greater
	// FieldIntGreaterNum fieldNum numIdx
	// Pushes 1 if $fieldNum > nums[numIdx], else 0
	FieldIntGreaterNum Opcode = iota + 200

	// Field comparison: FieldInt + Num + Less
	// FieldIntLessNum fieldNum numIdx
	// Pushes 1 if $fieldNum < nums[numIdx], else 0
	FieldIntLessNum

	// Field comparison: FieldInt + Num + Equal
	// FieldIntEqualNum fieldNum numIdx
	// Pushes 1 if $fieldNum == nums[numIdx], else 0
	FieldIntEqualNum

	// Field comparison: FieldInt + Str + Equal
	// FieldIntEqualStr fieldNum strIdx
	// Pushes 1 if $fieldNum == strs[strIdx], else 0
	FieldIntEqualStr

	// Add two fields: FieldInt + FieldInt + Add
	// AddFields field1 field2
	// Pushes $field1 + $field2
	AddFields

	// Loop optimization: LoadGlobal + Num + JumpLess
	// JumpGlobalLessNum globalIdx numIdx offset
	// Jumps if global[globalIdx] < nums[numIdx]
	JumpGlobalLessNum

	// Loop optimization: LoadGlobal + Num + JumpGrEq
	// JumpGlobalGrEqNum globalIdx numIdx offset
	// Jumps if global[globalIdx] >= nums[numIdx]
	JumpGlobalGrEqNum

	// =============================================================================
	// Typed opcodes for static type specialization (P1-003)
	// These opcodes are emitted when the compiler can prove both operands are numeric.
	// They skip type checking and work directly with float64 values.
	// =============================================================================

	// AddNum: typed addition (both operands known numeric)
	// Pops two values, pushes float64 sum
	AddNum Opcode = iota + 300

	// SubNum: typed subtraction (both operands known numeric)
	SubNum

	// MulNum: typed multiplication (both operands known numeric)
	MulNum

	// DivNum: typed division (both operands known numeric)
	DivNum

	// ModNum: typed modulo (both operands known numeric)
	ModNum

	// PowNum: typed power (both operands known numeric)
	PowNum

	// NegNum: typed unary minus (operand known numeric)
	NegNum

	// LessNum: typed less-than comparison
	// Pops two values, pushes 1 if a < b, else 0
	LessNum

	// LessEqNum: typed less-or-equal comparison
	LessEqNum

	// GreaterNum: typed greater-than comparison
	GreaterNum

	// GreaterEqNum: typed greater-or-equal comparison
	GreaterEqNum

	// EqualNum: typed equality comparison
	EqualNum

	// NotEqualNum: typed inequality comparison
	NotEqualNum

	// JumpLessNum: typed conditional jump
	// Pops two values, jumps if a < b (numeric comparison)
	JumpLessNum

	// JumpLessEqNum: typed conditional jump
	JumpLessEqNum

	// JumpGreaterNum: typed conditional jump
	JumpGreaterNum

	// JumpGreaterEqNum: typed conditional jump
	JumpGreaterEqNum

	// JumpEqualNum: typed conditional jump
	JumpEqualNum

	// JumpNotEqualNum: typed conditional jump
	JumpNotEqualNum
)

// fusedJump represents a fused jump that needs offset adjustment
type fusedJump struct {
	newPos       int // Position in new code
	oldOffsetPos int // Position of offset in OLD code (for calculating old target)
}

// OptimizeProgram applies peephole optimizations to a compiled program.
// This is the main entry point for post-compilation optimization.
func OptimizeProgram(p *Program) {
	// Optimize BEGIN
	p.Begin = optimizeCode(p.Begin)

	// Optimize actions
	for i := range p.Actions {
		for j := range p.Actions[i].Pattern {
			p.Actions[i].Pattern[j] = optimizeCode(p.Actions[i].Pattern[j])
		}
		p.Actions[i].Body = optimizeCode(p.Actions[i].Body)
	}

	// Optimize END
	p.End = optimizeCode(p.End)

	// Optimize functions
	for i := range p.Functions {
		p.Functions[i].Body = optimizeCode(p.Functions[i].Body)
	}
}

// optimizeCode applies peephole optimizations to a code sequence.
// Returns optimized code (may be shorter than input).
func optimizeCode(code []Opcode) []Opcode {
	if len(code) < 3 {
		return code
	}

	// Phase 1: Build position mapping and new code
	result := make([]Opcode, 0, len(code))
	posMap := make(map[int]int) // oldPos -> newPos
	oldPos := 0
	newPos := 0

	// Track fused jumps that need offset fixup
	var fusedJumps []fusedJump
	// Track regular jumps that need offset fixup
	type regularJump struct {
		newOffsetPos int // Position of offset in NEW code
		oldOffsetPos int // Position of offset in OLD code
	}
	var regularJumps []regularJump

	for oldPos < len(code) {
		// Record position mapping BEFORE fusion
		posMap[oldPos] = newPos

		consumed, fused, isFusedJump, oldOffsetPosition := tryFuseWithJumpInfo(code, oldPos)
		if consumed > 0 {
			result = append(result, fused...)
			if isFusedJump {
				// Record fused jump for offset fixup
				fusedJumps = append(fusedJumps, fusedJump{
					newPos:       newPos,
					oldOffsetPos: oldOffsetPosition,
				})
			}
			oldPos += consumed
			newPos += len(fused)
		} else {
			// Check if this is a regular jump that needs tracking
			instrLen := instructionLength(code, oldPos)

			// Copy instruction
			result = append(result, code[oldPos:oldPos+instrLen]...)

			// Track regular jumps
			if isJumpOpcode(code[oldPos]) && instrLen >= 2 {
				regularJumps = append(regularJumps, regularJump{
					newOffsetPos: newPos + 1, // offset is at position + 1
					oldOffsetPos: oldPos + 1, // same for old code
				})
			}

			oldPos += instrLen
			newPos += instrLen
		}
	}

	// Record end position
	posMap[oldPos] = newPos

	// Phase 2: Fix jump offsets for fused jumps
	for _, fj := range fusedJumps {
		// Get old offset from original code
		oldOffset := int(code[fj.oldOffsetPos])

		// Calculate old target (relative to position AFTER reading offset)
		// In the original: ip was at oldOffsetPos, after ip++ it would be oldOffsetPos+1
		// then ip += offset, so target = oldOffsetPos + 1 + offset
		oldTarget := fj.oldOffsetPos + 1 + oldOffset

		// Find new position for this target
		newTarget, ok := posMap[oldTarget]
		if !ok {
			// Target position might be between mapped positions, find closest
			newTarget = findClosestNewPos(posMap, oldTarget)
		}

		// The fused jump has format: opcode globalIdx numIdx offset
		// offset is at position fj.newPos + 3
		offsetPos := fj.newPos + 3

		// Calculate new offset (relative to position after fused instruction)
		// After fused instruction, ip will be at fj.newPos + 4
		newOffset := newTarget - (fj.newPos + 4)

		if offsetPos < len(result) {
			result[offsetPos] = Opcode(newOffset)
		}
	}

	// Phase 3: Fix jump offsets for regular jumps
	for _, rj := range regularJumps {
		// Get old offset
		oldOffset := int(code[rj.oldOffsetPos])

		// Calculate old target
		oldTarget := rj.oldOffsetPos + 1 + oldOffset

		// Find new position for this target
		newTarget, ok := posMap[oldTarget]
		if !ok {
			newTarget = findClosestNewPos(posMap, oldTarget)
		}

		// Calculate new offset (relative to position after offset operand)
		newOffset := newTarget - (rj.newOffsetPos + 1)

		if rj.newOffsetPos < len(result) {
			result[rj.newOffsetPos] = Opcode(newOffset)
		}
	}

	return result
}

// tryFuseWithJumpInfo attempts fusion and returns jump info if applicable.
// Returns (consumed, fused, isFusedJump, oldOffsetPosition).
func tryFuseWithJumpInfo(code []Opcode, i int) (int, []Opcode, bool, int) {
	remaining := len(code) - i

	// Pattern: LoadGlobal + Num + JumpLess -> JumpGlobalLessNum
	// Original: LoadGlobal(2) + Num(2) + JumpLess(2) = 6 bytes
	// Fused: JumpGlobalLessNum(4) = 4 bytes
	if remaining >= 6 &&
		code[i] == LoadGlobal &&
		code[i+2] == Num &&
		code[i+4] == JumpLess {
		return 6, []Opcode{
			JumpGlobalLessNum,
			code[i+1], // globalIdx
			code[i+3], // numIdx
			code[i+5], // offset (will be fixed later)
		}, true, i + 5 // oldOffsetPos is at i+5
	}

	// Pattern: LoadGlobal + Num + JumpGrEq -> JumpGlobalGrEqNum
	if remaining >= 6 &&
		code[i] == LoadGlobal &&
		code[i+2] == Num &&
		code[i+4] == JumpGrEq {
		return 6, []Opcode{
			JumpGlobalGrEqNum,
			code[i+1], // globalIdx
			code[i+3], // numIdx
			code[i+5], // offset (will be fixed later)
		}, true, i + 5
	}

	// Non-jump patterns (delegate to simpler function)
	consumed, fused := tryFuse(code, i)
	return consumed, fused, false, 0
}

// tryFuse attempts to fuse instructions starting at position i.
// Returns (consumed, fused). Returns (0, nil) if no fusion possible.
func tryFuse(code []Opcode, i int) (int, []Opcode) {
	remaining := len(code) - i

	// Pattern: FieldInt + Num + Greater -> FieldIntGreaterNum
	if remaining >= 5 &&
		code[i] == FieldInt &&
		code[i+2] == Num &&
		code[i+4] == Greater {
		return 5, []Opcode{
			FieldIntGreaterNum,
			code[i+1], // fieldNum
			code[i+3], // numIdx
		}
	}

	// Pattern: FieldInt + Num + Less -> FieldIntLessNum
	if remaining >= 5 &&
		code[i] == FieldInt &&
		code[i+2] == Num &&
		code[i+4] == Less {
		return 5, []Opcode{
			FieldIntLessNum,
			code[i+1], // fieldNum
			code[i+3], // numIdx
		}
	}

	// Pattern: FieldInt + Num + Equal -> FieldIntEqualNum
	if remaining >= 5 &&
		code[i] == FieldInt &&
		code[i+2] == Num &&
		code[i+4] == Equal {
		return 5, []Opcode{
			FieldIntEqualNum,
			code[i+1], // fieldNum
			code[i+3], // numIdx
		}
	}

	// Pattern: FieldInt + Str + Equal -> FieldIntEqualStr
	if remaining >= 5 &&
		code[i] == FieldInt &&
		code[i+2] == Str &&
		code[i+4] == Equal {
		return 5, []Opcode{
			FieldIntEqualStr,
			code[i+1], // fieldNum
			code[i+3], // strIdx
		}
	}

	// Pattern: FieldInt + FieldInt + Add -> AddFields
	if remaining >= 5 &&
		code[i] == FieldInt &&
		code[i+2] == FieldInt &&
		code[i+4] == Add {
		return 5, []Opcode{
			AddFields,
			code[i+1], // field1
			code[i+3], // field2
		}
	}

	return 0, nil
}

// isJumpOpcode returns true if the opcode is a jump instruction.
func isJumpOpcode(op Opcode) bool {
	switch op {
	case Jump, JumpTrue, JumpFalse, JumpEqual, JumpNotEq,
		JumpLess, JumpLessEq, JumpGreater, JumpGrEq,
		// Typed jump opcodes (P1-003)
		JumpLessNum, JumpLessEqNum, JumpGreaterNum, JumpGreaterEqNum,
		JumpEqualNum, JumpNotEqualNum:
		return true
	default:
		return false
	}
}

// findClosestNewPos finds the new position closest to oldTarget.
func findClosestNewPos(posMap map[int]int, oldTarget int) int {
	// Try exact match first
	if pos, ok := posMap[oldTarget]; ok {
		return pos
	}

	// Find closest position <= oldTarget
	bestOld := -1
	for old := range posMap {
		if old <= oldTarget && old > bestOld {
			bestOld = old
		}
	}

	if bestOld >= 0 {
		return posMap[bestOld]
	}

	// Fallback - shouldn't happen with correct code
	return oldTarget
}

// instructionLength returns the length of instruction at position i.
func instructionLength(code []Opcode, i int) int {
	if i >= len(code) {
		return 0
	}

	switch code[i] {
	case Num, Str, Regex, LoadGlobal, StoreGlobal, LoadLocal, StoreLocal,
		LoadSpecial, StoreSpecial, FieldInt,
		Jump, JumpTrue, JumpFalse, JumpEqual, JumpNotEq,
		JumpLess, JumpLessEq, JumpGreater, JumpGrEq,
		CallBuiltin, Nulls, IndexMulti, ConcatMulti,
		ArrayGetGlobal, ArraySetGlobal, ArrayDeleteGlobal, ArrayInGlobal:
		return 2

	case IncrGlobal, IncrLocal, IncrSpecial, AugGlobal, AugLocal, AugSpecial,
		CallNative, Print, Printf, Getline, GetlineField,
		IncrArrayGlobal, AugArrayGlobal:
		return 3

	case ArrayGet, ArraySet, ArrayDelete, ArrayIn, CallSplit, CallSplitSep,
		CallLength, CallSprintf:
		return 3

	case IncrArray, AugArray:
		return 4

	case GetlineVar, GetlineArray:
		return 4

	case ForIn:
		return 6

	case CallUser:
		// CallUser funcIdx numArrays [scope idx]...
		if i+2 < len(code) {
			numArrays := int(code[i+2])
			return 3 + numArrays*2
		}
		return 3

	// Fused opcodes (non-jump)
	case FieldIntGreaterNum, FieldIntLessNum, FieldIntEqualNum, FieldIntEqualStr, AddFields:
		return 3

	// Jump fused opcodes
	case JumpGlobalLessNum, JumpGlobalGrEqNum:
		return 4

	// Typed numeric opcodes (no operands)
	case AddNum, SubNum, MulNum, DivNum, ModNum, PowNum, NegNum,
		LessNum, LessEqNum, GreaterNum, GreaterEqNum, EqualNum, NotEqualNum:
		return 1

	// Typed jump opcodes (1 operand: offset)
	case JumpLessNum, JumpLessEqNum, JumpGreaterNum, JumpGreaterEqNum,
		JumpEqualNum, JumpNotEqualNum:
		return 2

	default:
		return 1
	}
}
