package vm

import (
	"strings"
	"testing"

	"github.com/kolkov/uawk/internal/compiler"
	"github.com/kolkov/uawk/internal/parser"
	"github.com/kolkov/uawk/internal/semantic"
)

// BenchmarkFusedOpcodes tests the performance impact of opcode fusion.
// uawk-specific benchmarks - these patterns are optimized by our peephole optimizer.

// generateNumericData creates test input with numeric fields.
func generateNumericData(lines, fields int) string {
	var sb strings.Builder
	for i := 0; i < lines; i++ {
		for j := 0; j < fields; j++ {
			if j > 0 {
				sb.WriteByte(' ')
			}
			sb.WriteString("12345")
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// compileAndOptimize compiles AWK code with peephole optimization.
func compileAndOptimize(code string) *compiler.Program {
	prog, _ := parser.Parse(code)
	resolved, _ := semantic.Resolve(prog)
	compiled, _ := compiler.Compile(prog, resolved)
	compiler.OptimizeProgram(compiled)
	return compiled
}

// compileWithoutOptimize compiles AWK code without optimization.
func compileWithoutOptimize(code string) *compiler.Program {
	prog, _ := parser.Parse(code)
	resolved, _ := semantic.Resolve(prog)
	compiled, _ := compiler.Compile(prog, resolved)
	// No OptimizeProgram call
	return compiled
}

// BenchmarkFieldIntGreaterNum tests $1 > N pattern.
func BenchmarkFieldIntGreaterNum(b *testing.B) {
	data := generateNumericData(10000, 3)
	code := `$1 > 10000 { count++ } END { print count }`

	b.Run("optimized", func(b *testing.B) {
		compiled := compileAndOptimize(code)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			vm := New(compiled)
			vm.SetInput(strings.NewReader(data))
			var sb strings.Builder
			vm.SetOutput(&sb)
			_ = vm.Run()
		}
	})

	b.Run("unoptimized", func(b *testing.B) {
		compiled := compileWithoutOptimize(code)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			vm := New(compiled)
			vm.SetInput(strings.NewReader(data))
			var sb strings.Builder
			vm.SetOutput(&sb)
			_ = vm.Run()
		}
	})
}

// BenchmarkFieldIntEqualStr tests $1 == "str" pattern.
func BenchmarkFieldIntEqualStr(b *testing.B) {
	var sb strings.Builder
	for i := 0; i < 10000; i++ {
		sb.WriteString("test value\n")
	}
	data := sb.String()
	code := `$1 == "test" { count++ } END { print count }`

	b.Run("optimized", func(b *testing.B) {
		compiled := compileAndOptimize(code)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			vm := New(compiled)
			vm.SetInput(strings.NewReader(data))
			var out strings.Builder
			vm.SetOutput(&out)
			_ = vm.Run()
		}
	})

	b.Run("unoptimized", func(b *testing.B) {
		compiled := compileWithoutOptimize(code)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			vm := New(compiled)
			vm.SetInput(strings.NewReader(data))
			var out strings.Builder
			vm.SetOutput(&out)
			_ = vm.Run()
		}
	})
}

// BenchmarkAddFields tests $1 + $2 pattern.
func BenchmarkAddFields(b *testing.B) {
	data := generateNumericData(10000, 3)
	code := `{ sum += $1 + $2 } END { print sum }`

	b.Run("optimized", func(b *testing.B) {
		compiled := compileAndOptimize(code)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			vm := New(compiled)
			vm.SetInput(strings.NewReader(data))
			var out strings.Builder
			vm.SetOutput(&out)
			_ = vm.Run()
		}
	})

	b.Run("unoptimized", func(b *testing.B) {
		compiled := compileWithoutOptimize(code)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			vm := New(compiled)
			vm.SetInput(strings.NewReader(data))
			var out strings.Builder
			vm.SetOutput(&out)
			_ = vm.Run()
		}
	})
}

// BenchmarkLoopFusion tests loop with fused jump opcodes.
func BenchmarkLoopFusion(b *testing.B) {
	// Loop-heavy computation
	code := `BEGIN { for(i=0; i<100000; i++) sum += i; print sum }`

	b.Run("optimized", func(b *testing.B) {
		compiled := compileAndOptimize(code)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			vm := New(compiled)
			var out strings.Builder
			vm.SetOutput(&out)
			_ = vm.Run()
		}
	})

	b.Run("unoptimized", func(b *testing.B) {
		compiled := compileWithoutOptimize(code)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			vm := New(compiled)
			var out strings.Builder
			vm.SetOutput(&out)
			_ = vm.Run()
		}
	})
}

// BenchmarkNestedLoops tests nested loops with fused jumps.
func BenchmarkNestedLoops(b *testing.B) {
	code := `BEGIN {
		for(i=0; i<100; i++)
			for(j=0; j<100; j++)
				sum += i*j;
		print sum
	}`

	b.Run("optimized", func(b *testing.B) {
		compiled := compileAndOptimize(code)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			vm := New(compiled)
			var out strings.Builder
			vm.SetOutput(&out)
			_ = vm.Run()
		}
	})

	b.Run("unoptimized", func(b *testing.B) {
		compiled := compileWithoutOptimize(code)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			vm := New(compiled)
			var out strings.Builder
			vm.SetOutput(&out)
			_ = vm.Run()
		}
	})
}
