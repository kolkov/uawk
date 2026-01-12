// Package vm provides the AWK virtual machine implementation.
// This file implements program analysis for parallel execution safety.
package vm

import (
	"github.com/kolkov/uawk/internal/compiler"
)

// ParallelSafety represents the parallelization safety level of a program.
type ParallelSafety int

const (
	// ParallelUnsafe indicates the program cannot be parallelized.
	// Contains operations that require sequential execution.
	ParallelUnsafe ParallelSafety = iota

	// ParallelStateless indicates the program is stateless (embarrassingly parallel).
	// Each record can be processed independently.
	// Example: { print $1 }
	ParallelStateless

	// ParallelAggregatable indicates the program can be parallelized with aggregation.
	// Workers can run independently, results merged at the end.
	// Example: { sum += $1 } END { print sum }
	ParallelAggregatable
)

// String returns a human-readable description of the safety level.
func (s ParallelSafety) String() string {
	switch s {
	case ParallelUnsafe:
		return "unsafe"
	case ParallelStateless:
		return "stateless"
	case ParallelAggregatable:
		return "aggregatable"
	default:
		return "unknown"
	}
}

// UnsafeReason explains why a program cannot be parallelized.
type UnsafeReason int

const (
	ReasonNone UnsafeReason = iota
	ReasonGetline
	ReasonNext
	ReasonNextFile
	ReasonSystemCall
	ReasonFileOutput
	ReasonPipeOutput
	ReasonRangePattern
	ReasonComplexRS
	ReasonUserFunction
)

// String returns a human-readable explanation.
func (r UnsafeReason) String() string {
	switch r {
	case ReasonNone:
		return "none"
	case ReasonGetline:
		return "uses getline (external input)"
	case ReasonNext:
		return "uses next (control flow between records)"
	case ReasonNextFile:
		return "uses nextfile (control flow between files)"
	case ReasonSystemCall:
		return "uses system() (side effects)"
	case ReasonFileOutput:
		return "uses file output redirection"
	case ReasonPipeOutput:
		return "uses pipe output"
	case ReasonRangePattern:
		return "uses range patterns (stateful matching)"
	case ReasonComplexRS:
		return "uses complex RS (multi-char record separator)"
	case ReasonUserFunction:
		return "uses user-defined functions (may have side effects)"
	default:
		return "unknown reason"
	}
}

// ParallelAnalysis contains the results of parallel safety analysis.
type ParallelAnalysis struct {
	Safety        ParallelSafety
	UnsafeReasons []UnsafeReason

	// HasAggregation indicates the program uses variables in both
	// main loop and END block (requires result merging).
	HasAggregation bool

	// AggregatedVars lists variable indices that need aggregation.
	AggregatedVars []int

	// AggregatedArrays lists array indices that need aggregation.
	AggregatedArrays []int
}

// CanParallelize returns true if the program can be parallelized.
func (a *ParallelAnalysis) CanParallelize() bool {
	return a.Safety != ParallelUnsafe
}

// AnalyzeParallelSafety analyzes a compiled program for parallel execution safety.
func AnalyzeParallelSafety(prog *compiler.Program, rs string) *ParallelAnalysis {
	analysis := &ParallelAnalysis{
		Safety: ParallelStateless,
	}

	// Check RS complexity
	if len(rs) > 1 || rs == "" {
		analysis.Safety = ParallelUnsafe
		analysis.UnsafeReasons = append(analysis.UnsafeReasons, ReasonComplexRS)
		return analysis
	}

	// Check for range patterns
	for _, action := range prog.Actions {
		if len(action.Pattern) == 2 {
			analysis.Safety = ParallelUnsafe
			analysis.UnsafeReasons = append(analysis.UnsafeReasons, ReasonRangePattern)
			return analysis
		}
	}

	// Analyze BEGIN block
	beginVars := analyzeCodeVars(prog.Begin)

	// Analyze main loop
	mainVars := newVarSet()
	for _, action := range prog.Actions {
		for _, pat := range action.Pattern {
			mainVars.merge(analyzeCodeVars(pat))
		}
		if action.Body != nil {
			mainVars.merge(analyzeCodeVars(action.Body))
		}
	}

	// Analyze END block
	endVars := analyzeCodeVars(prog.End)

	// Check for unsafe operations in main loop
	for _, action := range prog.Actions {
		for _, pat := range action.Pattern {
			if reasons := checkUnsafeOps(pat); len(reasons) > 0 {
				analysis.Safety = ParallelUnsafe
				analysis.UnsafeReasons = append(analysis.UnsafeReasons, reasons...)
			}
		}
		if action.Body != nil {
			if reasons := checkUnsafeOps(action.Body); len(reasons) > 0 {
				analysis.Safety = ParallelUnsafe
				analysis.UnsafeReasons = append(analysis.UnsafeReasons, reasons...)
			}
		}
	}

	if analysis.Safety == ParallelUnsafe {
		return analysis
	}

	// Check for user-defined function calls (conservative: treat as unsafe)
	for _, action := range prog.Actions {
		if hasUserFunctionCall(action.Body) {
			analysis.Safety = ParallelUnsafe
			analysis.UnsafeReasons = append(analysis.UnsafeReasons, ReasonUserFunction)
			return analysis
		}
	}

	// Determine if aggregation is needed
	// Variables written in main and read in END need aggregation
	analysis.AggregatedVars = findOverlap(mainVars.writtenScalars, endVars.readScalars)
	analysis.AggregatedArrays = findOverlap(mainVars.writtenArrays, endVars.readArrays)

	if len(analysis.AggregatedVars) > 0 || len(analysis.AggregatedArrays) > 0 {
		analysis.HasAggregation = true
		analysis.Safety = ParallelAggregatable
	}

	// Also check variables from BEGIN that are used in main loop
	beginToMain := findOverlap(beginVars.writtenScalars, mainVars.readScalars)
	if len(beginToMain) > 0 {
		analysis.HasAggregation = true
		analysis.Safety = ParallelAggregatable
	}

	return analysis
}

// varSet tracks variable usage in a code block.
type varSet struct {
	readScalars    map[int]bool
	writtenScalars map[int]bool
	readArrays     map[int]bool
	writtenArrays  map[int]bool
}

func newVarSet() *varSet {
	return &varSet{
		readScalars:    make(map[int]bool),
		writtenScalars: make(map[int]bool),
		readArrays:     make(map[int]bool),
		writtenArrays:  make(map[int]bool),
	}
}

func (v *varSet) merge(other *varSet) {
	for k := range other.readScalars {
		v.readScalars[k] = true
	}
	for k := range other.writtenScalars {
		v.writtenScalars[k] = true
	}
	for k := range other.readArrays {
		v.readArrays[k] = true
	}
	for k := range other.writtenArrays {
		v.writtenArrays[k] = true
	}
}

// analyzeCodeVars analyzes variable usage in bytecode.
//
//nolint:gocognit // switch-case bytecode analysis cannot be split without losing readability
func analyzeCodeVars(code []compiler.Opcode) *varSet {
	vs := newVarSet()
	if len(code) == 0 {
		return vs
	}

	for i := 0; i < len(code); i++ {
		op := code[i]
		switch op {
		case compiler.LoadGlobal:
			if i+1 < len(code) {
				vs.readScalars[int(code[i+1])] = true
				i++
			}
		case compiler.StoreGlobal:
			if i+1 < len(code) {
				vs.writtenScalars[int(code[i+1])] = true
				i++
			}
		case compiler.IncrGlobal:
			if i+2 < len(code) {
				idx := int(code[i+2])
				vs.readScalars[idx] = true
				vs.writtenScalars[idx] = true
				i += 2
			}
		case compiler.AugGlobal:
			if i+2 < len(code) {
				idx := int(code[i+2])
				vs.readScalars[idx] = true
				vs.writtenScalars[idx] = true
				i += 2
			}
		case compiler.ArrayGet:
			if i+2 < len(code) {
				scope := compiler.Scope(code[i+1])
				if scope == compiler.ScopeGlobal {
					vs.readArrays[int(code[i+2])] = true
				}
				i += 2
			}
		case compiler.ArrayGetGlobal:
			if i+1 < len(code) {
				vs.readArrays[int(code[i+1])] = true
				i++
			}
		case compiler.ArraySet:
			if i+2 < len(code) {
				scope := compiler.Scope(code[i+1])
				if scope == compiler.ScopeGlobal {
					vs.writtenArrays[int(code[i+2])] = true
				}
				i += 2
			}
		case compiler.ArraySetGlobal:
			if i+1 < len(code) {
				vs.writtenArrays[int(code[i+1])] = true
				i++
			}
		case compiler.IncrArray:
			if i+3 < len(code) {
				scope := compiler.Scope(code[i+2])
				if scope == compiler.ScopeGlobal {
					idx := int(code[i+3])
					vs.readArrays[idx] = true
					vs.writtenArrays[idx] = true
				}
				i += 3
			}
		case compiler.IncrArrayGlobal:
			if i+2 < len(code) {
				idx := int(code[i+2])
				vs.readArrays[idx] = true
				vs.writtenArrays[idx] = true
				i += 2
			}
		case compiler.AugArray:
			if i+3 < len(code) {
				scope := compiler.Scope(code[i+2])
				if scope == compiler.ScopeGlobal {
					idx := int(code[i+3])
					vs.readArrays[idx] = true
					vs.writtenArrays[idx] = true
				}
				i += 3
			}
		case compiler.AugArrayGlobal:
			if i+2 < len(code) {
				idx := int(code[i+2])
				vs.readArrays[idx] = true
				vs.writtenArrays[idx] = true
				i += 2
			}
		// Skip operands for other opcodes
		case compiler.Num, compiler.Str, compiler.Regex,
			compiler.LoadLocal, compiler.StoreLocal,
			compiler.LoadSpecial, compiler.StoreSpecial,
			compiler.FieldInt:
			i++
		case compiler.Jump, compiler.JumpTrue, compiler.JumpFalse,
			compiler.JumpEqual, compiler.JumpNotEq,
			compiler.JumpLess, compiler.JumpLessEq,
			compiler.JumpGreater, compiler.JumpGrEq:
			i++
		case compiler.IncrLocal, compiler.IncrSpecial, compiler.IncrField:
			i += 2
		case compiler.AugLocal, compiler.AugSpecial, compiler.AugField:
			i += 2
		case compiler.ArrayIn, compiler.ArrayDelete, compiler.ArrayClear:
			i += 2
		case compiler.ArrayInGlobal, compiler.ArrayDeleteGlobal:
			i++
		case compiler.ForIn:
			i += 5
		case compiler.CallBuiltin:
			i++
		case compiler.CallUser:
			if i+2 < len(code) {
				numArrays := int(code[i+2])
				i += 2 + numArrays*2
			}
		case compiler.CallNative, compiler.CallSplit, compiler.CallSplitSep, compiler.CallLength:
			i += 2
		case compiler.CallSprintf, compiler.Print, compiler.Printf:
			i += 2
		case compiler.Getline, compiler.GetlineField:
			i++
		case compiler.GetlineVar, compiler.GetlineArray:
			i += 3
		case compiler.IndexMulti, compiler.ConcatMulti, compiler.Nulls:
			i++
		// Fused opcodes
		case compiler.JumpGlobalLessNum, compiler.JumpGlobalGrEqNum:
			i += 3
		case compiler.FieldIntGreaterNum, compiler.FieldIntLessNum,
			compiler.FieldIntEqualNum, compiler.FieldIntEqualStr,
			compiler.AddFields:
			i += 2
		// Typed opcodes with offset
		case compiler.JumpLessNum, compiler.JumpLessEqNum,
			compiler.JumpGreaterNum, compiler.JumpGreaterEqNum,
			compiler.JumpEqualNum, compiler.JumpNotEqualNum:
			i++
		}
	}

	return vs
}

// checkUnsafeOps checks bytecode for operations that prevent parallelization.
func checkUnsafeOps(code []compiler.Opcode) []UnsafeReason {
	var reasons []UnsafeReason
	if len(code) == 0 {
		return reasons
	}

	for i := 0; i < len(code); i++ {
		op := code[i]
		switch op {
		case compiler.Getline, compiler.GetlineVar, compiler.GetlineField, compiler.GetlineArray:
			reasons = append(reasons, ReasonGetline)
			// Skip operands
			if op == compiler.GetlineVar || op == compiler.GetlineArray {
				i += 3
			} else {
				i++
			}
		case compiler.Next:
			reasons = append(reasons, ReasonNext)
		case compiler.Nextfile:
			reasons = append(reasons, ReasonNextFile)
		case compiler.Print, compiler.Printf:
			if i+2 < len(code) {
				redirect := compiler.Redirect(code[i+2])
				switch redirect {
				case compiler.RedirectWrite, compiler.RedirectAppend:
					reasons = append(reasons, ReasonFileOutput)
				case compiler.RedirectPipe:
					reasons = append(reasons, ReasonPipeOutput)
				}
				i += 2
			}
		case compiler.CallBuiltin:
			if i+1 < len(code) {
				builtin := compiler.BuiltinOp(code[i+1])
				if builtin == compiler.BuiltinSystem {
					reasons = append(reasons, ReasonSystemCall)
				}
				i++
			}
		// Skip operands for other opcodes (same as analyzeCodeVars)
		case compiler.Num, compiler.Str, compiler.Regex,
			compiler.LoadGlobal, compiler.StoreGlobal,
			compiler.LoadLocal, compiler.StoreLocal,
			compiler.LoadSpecial, compiler.StoreSpecial,
			compiler.FieldInt:
			i++
		case compiler.Jump, compiler.JumpTrue, compiler.JumpFalse,
			compiler.JumpEqual, compiler.JumpNotEq,
			compiler.JumpLess, compiler.JumpLessEq,
			compiler.JumpGreater, compiler.JumpGrEq:
			i++
		case compiler.IncrGlobal, compiler.IncrLocal, compiler.IncrSpecial, compiler.IncrField:
			i += 2
		case compiler.AugGlobal, compiler.AugLocal, compiler.AugSpecial, compiler.AugField:
			i += 2
		case compiler.ArrayGet, compiler.ArraySet, compiler.ArrayIn, compiler.ArrayDelete, compiler.ArrayClear:
			i += 2
		case compiler.ArrayGetGlobal, compiler.ArraySetGlobal, compiler.ArrayDeleteGlobal, compiler.ArrayInGlobal:
			i++
		case compiler.IncrArray, compiler.AugArray:
			i += 3
		case compiler.IncrArrayGlobal, compiler.AugArrayGlobal:
			i += 2
		case compiler.ForIn:
			i += 5
		case compiler.CallUser:
			if i+2 < len(code) {
				numArrays := int(code[i+2])
				i += 2 + numArrays*2
			}
		case compiler.CallNative, compiler.CallSplit, compiler.CallSplitSep, compiler.CallLength:
			i += 2
		case compiler.CallSprintf:
			i += 2
		case compiler.IndexMulti, compiler.ConcatMulti, compiler.Nulls:
			i++
		// Fused opcodes
		case compiler.JumpGlobalLessNum, compiler.JumpGlobalGrEqNum:
			i += 3
		case compiler.FieldIntGreaterNum, compiler.FieldIntLessNum,
			compiler.FieldIntEqualNum, compiler.FieldIntEqualStr,
			compiler.AddFields:
			i += 2
		// Typed opcodes with offset
		case compiler.JumpLessNum, compiler.JumpLessEqNum,
			compiler.JumpGreaterNum, compiler.JumpGreaterEqNum,
			compiler.JumpEqualNum, compiler.JumpNotEqualNum:
			i++
		}
	}

	return reasons
}

// hasUserFunctionCall checks if code contains user function calls.
func hasUserFunctionCall(code []compiler.Opcode) bool {
	if len(code) == 0 {
		return false
	}

	for i := 0; i < len(code); i++ {
		op := code[i]
		if op == compiler.CallUser {
			return true
		}
		// Skip operands (same logic as above)
		switch op {
		case compiler.Num, compiler.Str, compiler.Regex,
			compiler.LoadGlobal, compiler.StoreGlobal,
			compiler.LoadLocal, compiler.StoreLocal,
			compiler.LoadSpecial, compiler.StoreSpecial,
			compiler.FieldInt:
			i++
		case compiler.Jump, compiler.JumpTrue, compiler.JumpFalse,
			compiler.JumpEqual, compiler.JumpNotEq,
			compiler.JumpLess, compiler.JumpLessEq,
			compiler.JumpGreater, compiler.JumpGrEq:
			i++
		case compiler.IncrGlobal, compiler.IncrLocal, compiler.IncrSpecial, compiler.IncrField:
			i += 2
		case compiler.AugGlobal, compiler.AugLocal, compiler.AugSpecial, compiler.AugField:
			i += 2
		case compiler.ArrayGet, compiler.ArraySet, compiler.ArrayIn, compiler.ArrayDelete, compiler.ArrayClear:
			i += 2
		case compiler.ArrayGetGlobal, compiler.ArraySetGlobal, compiler.ArrayDeleteGlobal, compiler.ArrayInGlobal:
			i++
		case compiler.IncrArray, compiler.AugArray:
			i += 3
		case compiler.IncrArrayGlobal, compiler.AugArrayGlobal:
			i += 2
		case compiler.ForIn:
			i += 5
		case compiler.CallBuiltin:
			i++
		case compiler.CallNative, compiler.CallSplit, compiler.CallSplitSep, compiler.CallLength:
			i += 2
		case compiler.CallSprintf, compiler.Print, compiler.Printf:
			i += 2
		case compiler.Getline, compiler.GetlineField:
			i++
		case compiler.GetlineVar, compiler.GetlineArray:
			i += 3
		case compiler.IndexMulti, compiler.ConcatMulti, compiler.Nulls:
			i++
		// Fused opcodes
		case compiler.JumpGlobalLessNum, compiler.JumpGlobalGrEqNum:
			i += 3
		case compiler.FieldIntGreaterNum, compiler.FieldIntLessNum,
			compiler.FieldIntEqualNum, compiler.FieldIntEqualStr,
			compiler.AddFields:
			i += 2
		// Typed opcodes
		case compiler.JumpLessNum, compiler.JumpLessEqNum,
			compiler.JumpGreaterNum, compiler.JumpGreaterEqNum,
			compiler.JumpEqualNum, compiler.JumpNotEqualNum:
			i++
		}
	}

	return false
}

// findOverlap returns indices present in both maps.
func findOverlap(a, b map[int]bool) []int {
	var result []int
	for k := range a {
		if b[k] {
			result = append(result, k)
		}
	}
	return result
}
