package compiler

import (
	"testing"

	"github.com/kolkov/uawk/internal/parser"
	"github.com/kolkov/uawk/internal/semantic"
)

// TestOptimizerPatterns tests that optimization produces correct patterns.
func TestOptimizerPatterns(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantOp   Opcode // Expected fused opcode
		wantLen  int    // Expected instruction count (rough)
		location string // Where to look: "begin", "action-pattern", "action-body", "end"
	}{
		{
			name:     "FieldIntGreaterNum",
			code:     `$1 > 500 { print }`,
			wantOp:   FieldIntGreaterNum,
			location: "action-pattern",
		},
		{
			name:     "FieldIntLessNum",
			code:     `$1 < 100 { print }`,
			wantOp:   FieldIntLessNum,
			location: "action-pattern",
		},
		{
			name:     "FieldIntEqualNum",
			code:     `$1 == 42 { print }`,
			wantOp:   FieldIntEqualNum,
			location: "action-pattern",
		},
		{
			name:     "FieldIntEqualStr",
			code:     `$1 == "test" { print }`,
			wantOp:   FieldIntEqualStr,
			location: "action-pattern",
		},
		{
			name:     "AddFields",
			code:     `{ x = $1 + $2 }`,
			wantOp:   AddFields,
			location: "action-body",
		},
		{
			name:     "JumpGlobalLessNum in loop",
			code:     `BEGIN { for(i=0; i<10; i++) x++ }`,
			wantOp:   JumpGlobalLessNum,
			location: "begin",
		},
		{
			name:     "JumpGlobalGrEqNum in loop",
			code:     `BEGIN { for(i=0; i<10; i++) x++ }`,
			wantOp:   JumpGlobalGrEqNum,
			location: "begin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, err := parser.Parse(tt.code)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			resolved, err := semantic.Resolve(prog)
			if err != nil {
				t.Fatalf("Resolve error: %v", err)
			}

			compiled, err := Compile(prog, resolved)
			if err != nil {
				t.Fatalf("Compile error: %v", err)
			}

			// Apply optimizer
			OptimizeProgram(compiled)

			// Find the target code
			var code []Opcode
			switch tt.location {
			case "begin":
				code = compiled.Begin
			case "action-pattern":
				if len(compiled.Actions) > 0 && len(compiled.Actions[0].Pattern) > 0 {
					code = compiled.Actions[0].Pattern[0]
				}
			case "action-body":
				if len(compiled.Actions) > 0 {
					code = compiled.Actions[0].Body
				}
			case "end":
				code = compiled.End
			}

			if code == nil {
				t.Fatalf("No code found in %s", tt.location)
			}

			// Check if the expected opcode is present
			found := false
			for _, op := range code {
				if op == tt.wantOp {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Expected opcode %s not found in %s:\n%s",
					tt.wantOp, tt.location, compiled.Disassemble())
			}
		})
	}
}

// TestOptimizerNoChange tests that non-matching patterns are unchanged.
func TestOptimizerNoChange(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{
			name: "dynamic field access",
			code: `{ print $x }`,
		},
		{
			name: "regex pattern",
			code: `/test/ { print }`,
		},
		{
			name: "local variable loop",
			code: `function f() { for(i=0; i<10; i++) x++ }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, err := parser.Parse(tt.code)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			resolved, err := semantic.Resolve(prog)
			if err != nil {
				t.Fatalf("Resolve error: %v", err)
			}

			compiled, err := Compile(prog, resolved)
			if err != nil {
				t.Fatalf("Compile error: %v", err)
			}

			// Get bytecode before optimization
			before := compiled.Disassemble()

			// Apply optimizer (should not change anything)
			OptimizeProgram(compiled)

			after := compiled.Disassemble()

			// For these patterns, optimizer shouldn't introduce fused opcodes
			// (though the code might be the same or slightly different)
			fusedOps := []Opcode{
				JumpGlobalLessNum, JumpGlobalGrEqNum,
				FieldIntGreaterNum, FieldIntLessNum, FieldIntEqualNum,
				FieldIntEqualStr, AddFields,
			}

			// Preallocate with estimated capacity
			allCode := make([]Opcode, 0, len(compiled.Begin)+len(compiled.End)+256)
			allCode = append(allCode, compiled.Begin...)
			for _, a := range compiled.Actions {
				for _, p := range a.Pattern {
					allCode = append(allCode, p...)
				}
				allCode = append(allCode, a.Body...)
			}
			allCode = append(allCode, compiled.End...)

			for _, fusedOp := range fusedOps {
				for _, op := range allCode {
					if op == fusedOp {
						t.Errorf("Unexpected fused opcode %s in:\nBefore:\n%s\nAfter:\n%s",
							fusedOp, before, after)
					}
				}
			}
		})
	}
}

// BenchmarkOptimizer measures optimizer overhead.
func BenchmarkOptimizer(b *testing.B) {
	code := `BEGIN { for(i=0; i<1000; i++) sum += i } $1 > 500 { count++ } END { print sum, count }`

	prog, _ := parser.Parse(code)
	resolved, _ := semantic.Resolve(prog)
	compiled, _ := Compile(prog, resolved)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create a copy to avoid measuring the same optimization twice
		progCopy := *compiled
		OptimizeProgram(&progCopy)
	}
}
