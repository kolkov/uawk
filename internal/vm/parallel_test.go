package vm

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/kolkov/uawk/internal/compiler"
	"github.com/kolkov/uawk/internal/parser"
	"github.com/kolkov/uawk/internal/semantic"
)

// Helper to compile an AWK program for testing.
func compileAWK(t *testing.T, source string) *compiler.Program {
	t.Helper()
	astProg, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	resolved, err := semantic.Resolve(astProg)
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	if errs := semantic.Check(astProg, resolved); len(errs) > 0 {
		t.Fatalf("semantic error: %v", errs[0])
	}
	prog, err := compiler.Compile(astProg, resolved)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	compiler.OptimizeProgram(prog)
	return prog
}

func TestParallelSafetyAnalysis(t *testing.T) {
	tests := []struct {
		name           string
		program        string
		rs             string
		wantSafety     ParallelSafety
		wantReasons    []UnsafeReason
		wantAggregated bool
	}{
		{
			name:       "stateless print",
			program:    `{ print $1 }`,
			rs:         "\n",
			wantSafety: ParallelStateless,
		},
		{
			name:       "stateless filter",
			program:    `$1 > 100 { print $0 }`,
			rs:         "\n",
			wantSafety: ParallelStateless,
		},
		{
			name:           "sum with END",
			program:        `{ sum += $1 } END { print sum }`,
			rs:             "\n",
			wantSafety:     ParallelAggregatable,
			wantAggregated: true,
		},
		{
			name:           "count by key",
			program:        `{ count[$1]++ } END { for (k in count) print k, count[k] }`,
			rs:             "\n",
			wantSafety:     ParallelAggregatable,
			wantAggregated: true,
		},
		{
			name:        "getline is unsafe",
			program:     `{ getline x < "file" }`,
			rs:          "\n",
			wantSafety:  ParallelUnsafe,
			wantReasons: []UnsafeReason{ReasonGetline},
		},
		{
			name:        "next is unsafe",
			program:     `$1 == "skip" { next }`,
			rs:          "\n",
			wantSafety:  ParallelUnsafe,
			wantReasons: []UnsafeReason{ReasonNext},
		},
		{
			name:        "system is unsafe",
			program:     `{ system("echo " $0) }`,
			rs:          "\n",
			wantSafety:  ParallelUnsafe,
			wantReasons: []UnsafeReason{ReasonSystemCall},
		},
		{
			name:        "file output is unsafe",
			program:     `{ print $0 > "output.txt" }`,
			rs:          "\n",
			wantSafety:  ParallelUnsafe,
			wantReasons: []UnsafeReason{ReasonFileOutput},
		},
		{
			name:        "pipe output is unsafe",
			program:     `{ print $0 | "sort" }`,
			rs:          "\n",
			wantSafety:  ParallelUnsafe,
			wantReasons: []UnsafeReason{ReasonPipeOutput},
		},
		{
			name:        "complex RS is unsafe",
			program:     `{ print $0 }`,
			rs:          "\n\n",
			wantSafety:  ParallelUnsafe,
			wantReasons: []UnsafeReason{ReasonComplexRS},
		},
		{
			name:        "empty RS is unsafe",
			program:     `{ print $0 }`,
			rs:          "",
			wantSafety:  ParallelUnsafe,
			wantReasons: []UnsafeReason{ReasonComplexRS},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog := compileAWK(t, tt.program)
			analysis := AnalyzeParallelSafety(prog, tt.rs)

			if analysis.Safety != tt.wantSafety {
				t.Errorf("safety = %v, want %v", analysis.Safety, tt.wantSafety)
			}

			if tt.wantReasons != nil {
				if len(analysis.UnsafeReasons) != len(tt.wantReasons) {
					t.Errorf("reasons = %v, want %v", analysis.UnsafeReasons, tt.wantReasons)
				} else {
					for i, r := range tt.wantReasons {
						if analysis.UnsafeReasons[i] != r {
							t.Errorf("reason[%d] = %v, want %v", i, analysis.UnsafeReasons[i], r)
						}
					}
				}
			}

			if analysis.HasAggregation != tt.wantAggregated {
				t.Errorf("hasAggregation = %v, want %v", analysis.HasAggregation, tt.wantAggregated)
			}
		})
	}
}

func TestParallelExecutor_BasicPrint(t *testing.T) {
	prog := compileAWK(t, `{ print $1 }`)

	input := strings.NewReader("hello world\nfoo bar\nbaz qux\n")
	var output bytes.Buffer

	config := DefaultParallelConfig()
	config.NumWorkers = 2

	exec := NewParallelExecutor(prog, DefaultVMConfig(), config)
	err := exec.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// Output should contain all first fields (order may vary with parallel)
	got := output.String()
	if !strings.Contains(got, "hello") || !strings.Contains(got, "foo") || !strings.Contains(got, "baz") {
		t.Errorf("output missing expected fields: %q", got)
	}
}

func TestParallelExecutor_Sum(t *testing.T) {
	prog := compileAWK(t, `{ sum += $1 } END { print sum }`)

	// Create input with known sum: 1+2+3+...+100 = 5050
	var inputLines []string
	for i := 1; i <= 100; i++ {
		inputLines = append(inputLines, strconv.Itoa(i))
	}
	input := strings.NewReader(strings.Join(inputLines, "\n") + "\n")

	var output bytes.Buffer

	config := DefaultParallelConfig()
	config.NumWorkers = 4
	config.ChunkSize = 100 // Small chunks to force parallelism

	exec := NewParallelExecutor(prog, DefaultVMConfig(), config)
	err := exec.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// Expected: 1+2+3+...+100 = 5050
	got := strings.TrimSpace(output.String())
	if got != "5050" {
		t.Errorf("sum = %q, want 5050", got)
	}
}

func TestParallelExecutor_Filter(t *testing.T) {
	prog := compileAWK(t, `$1 > 50 { print $1 }`)

	var inputLines []string
	for i := 1; i <= 100; i++ {
		inputLines = append(inputLines, strconv.Itoa(i))
	}
	input := strings.NewReader(strings.Join(inputLines, "\n") + "\n")

	var output bytes.Buffer

	config := DefaultParallelConfig()
	config.NumWorkers = 4
	config.ChunkSize = 100

	exec := NewParallelExecutor(prog, DefaultVMConfig(), config)
	err := exec.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// Should have numbers 51-100 in output
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 50 {
		t.Errorf("got %d lines, want 50", len(lines))
	}
}

func TestParallelExecutor_BEGIN(t *testing.T) {
	prog := compileAWK(t, `BEGIN { x = 10 } { sum += $1 + x } END { print sum }`)

	input := strings.NewReader("1\n2\n3\n")
	var output bytes.Buffer

	config := DefaultParallelConfig()
	config.NumWorkers = 2

	exec := NewParallelExecutor(prog, DefaultVMConfig(), config)
	err := exec.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// sum = (1+10) + (2+10) + (3+10) = 36
	got := strings.TrimSpace(output.String())
	if got != "36" {
		t.Errorf("sum = %q, want 36", got)
	}
}

func TestParallelExecutor_CountByKey(t *testing.T) {
	prog := compileAWK(t, `{ count[$1]++ } END { for (k in count) print k, count[k] }`)

	input := strings.NewReader("a\nb\na\nc\nb\na\n")
	var output bytes.Buffer

	config := DefaultParallelConfig()
	config.NumWorkers = 2
	config.ChunkSize = 10 // Small chunks

	exec := NewParallelExecutor(prog, DefaultVMConfig(), config)
	err := exec.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// Should have counts: a=3, b=2, c=1
	got := output.String()
	if !strings.Contains(got, "a 3") {
		t.Errorf("output missing 'a 3': %q", got)
	}
	if !strings.Contains(got, "b 2") {
		t.Errorf("output missing 'b 2': %q", got)
	}
	if !strings.Contains(got, "c 1") {
		t.Errorf("output missing 'c 1': %q", got)
	}
}

func TestParallelExecutor_CancelContext(t *testing.T) {
	prog := compileAWK(t, `{ print $0 }`)

	// Large input to ensure we have time to cancel
	input := strings.NewReader(strings.Repeat("test\n", 10000))
	var output bytes.Buffer

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	config := DefaultParallelConfig()
	config.NumWorkers = 4

	exec := NewParallelExecutor(prog, DefaultVMConfig(), config)
	err := exec.Run(ctx, input, &output)

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestParallelExecutor_EmptyInput(t *testing.T) {
	prog := compileAWK(t, `{ print $0 } END { print "done" }`)

	input := strings.NewReader("")
	var output bytes.Buffer

	config := DefaultParallelConfig()
	config.NumWorkers = 2

	exec := NewParallelExecutor(prog, DefaultVMConfig(), config)
	err := exec.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	got := strings.TrimSpace(output.String())
	if got != "done" {
		t.Errorf("output = %q, want 'done'", got)
	}
}

func TestParallelExecutor_SingleRecord(t *testing.T) {
	prog := compileAWK(t, `{ print $1, $2 }`)

	input := strings.NewReader("hello world\n")
	var output bytes.Buffer

	config := DefaultParallelConfig()
	config.NumWorkers = 4

	exec := NewParallelExecutor(prog, DefaultVMConfig(), config)
	err := exec.Run(context.Background(), input, &output)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	got := strings.TrimSpace(output.String())
	if got != "hello world" {
		t.Errorf("output = %q, want 'hello world'", got)
	}
}

func BenchmarkParallelExecutor_Sum_Small(b *testing.B) {
	prog := compileAWKBench(b, `{ sum += $1 } END { print sum }`)

	// Generate small input (100K lines)
	var inputLines []string
	for i := 0; i < 100000; i++ {
		inputLines = append(inputLines, "1")
	}
	inputData := strings.Join(inputLines, "\n") + "\n"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		input := strings.NewReader(inputData)
		var output bytes.Buffer

		config := DefaultParallelConfig()
		config.NumWorkers = 4

		exec := NewParallelExecutor(prog, DefaultVMConfig(), config)
		err := exec.Run(context.Background(), input, &output)
		if err != nil {
			b.Fatalf("Run error: %v", err)
		}
	}
}

func BenchmarkSequentialExecutor_Sum_Small(b *testing.B) {
	prog := compileAWKBench(b, `{ sum += $1 } END { print sum }`)

	// Generate small input (100K lines)
	var inputLines []string
	for i := 0; i < 100000; i++ {
		inputLines = append(inputLines, "1")
	}
	inputData := strings.Join(inputLines, "\n") + "\n"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		input := strings.NewReader(inputData)
		var output bytes.Buffer

		vm := NewWithConfig(prog, DefaultVMConfig())
		vm.SetInput(input)
		vm.SetOutput(&output)
		if err := vm.Run(); err != nil {
			b.Fatalf("Run error: %v", err)
		}
	}
}

func BenchmarkParallelExecutor_Sum_Large(b *testing.B) {
	prog := compileAWKBench(b, `{ sum += $1 } END { print sum }`)

	// Generate large input (1M lines)
	var sb strings.Builder
	for i := 0; i < 1000000; i++ {
		sb.WriteString("1\n")
	}
	inputData := sb.String()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		input := strings.NewReader(inputData)
		var output bytes.Buffer

		config := DefaultParallelConfig()
		config.NumWorkers = 4

		exec := NewParallelExecutor(prog, DefaultVMConfig(), config)
		err := exec.Run(context.Background(), input, &output)
		if err != nil {
			b.Fatalf("Run error: %v", err)
		}
	}
}

func BenchmarkSequentialExecutor_Sum_Large(b *testing.B) {
	prog := compileAWKBench(b, `{ sum += $1 } END { print sum }`)

	// Generate large input (1M lines)
	var sb strings.Builder
	for i := 0; i < 1000000; i++ {
		sb.WriteString("1\n")
	}
	inputData := sb.String()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		input := strings.NewReader(inputData)
		var output bytes.Buffer

		vm := NewWithConfig(prog, DefaultVMConfig())
		vm.SetInput(input)
		vm.SetOutput(&output)
		if err := vm.Run(); err != nil {
			b.Fatalf("Run error: %v", err)
		}
	}
}

func BenchmarkParallelExecutor_Filter_Large(b *testing.B) {
	prog := compileAWKBench(b, `$1 > 500000 { print $1 }`)

	// Generate large input (1M lines)
	var sb strings.Builder
	for i := 0; i < 1000000; i++ {
		sb.WriteString(strconv.Itoa(i))
		sb.WriteByte('\n')
	}
	inputData := sb.String()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		input := strings.NewReader(inputData)
		var output bytes.Buffer

		config := DefaultParallelConfig()
		config.NumWorkers = 4

		exec := NewParallelExecutor(prog, DefaultVMConfig(), config)
		err := exec.Run(context.Background(), input, &output)
		if err != nil {
			b.Fatalf("Run error: %v", err)
		}
	}
}

func BenchmarkSequentialExecutor_Filter_Large(b *testing.B) {
	prog := compileAWKBench(b, `$1 > 500000 { print $1 }`)

	// Generate large input (1M lines)
	var sb strings.Builder
	for i := 0; i < 1000000; i++ {
		sb.WriteString(strconv.Itoa(i))
		sb.WriteByte('\n')
	}
	inputData := sb.String()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		input := strings.NewReader(inputData)
		var output bytes.Buffer

		vm := NewWithConfig(prog, DefaultVMConfig())
		vm.SetInput(input)
		vm.SetOutput(&output)
		if err := vm.Run(); err != nil {
			b.Fatalf("Run error: %v", err)
		}
	}
}

// Helper for benchmarks
func compileAWKBench(b *testing.B, source string) *compiler.Program {
	b.Helper()
	astProg, err := parser.Parse(source)
	if err != nil {
		b.Fatalf("parse error: %v", err)
	}
	resolved, err := semantic.Resolve(astProg)
	if err != nil {
		b.Fatalf("resolve error: %v", err)
	}
	if errs := semantic.Check(astProg, resolved); len(errs) > 0 {
		b.Fatalf("semantic error: %v", errs[0])
	}
	prog, err := compiler.Compile(astProg, resolved)
	if err != nil {
		b.Fatalf("compile error: %v", err)
	}
	compiler.OptimizeProgram(prog)
	return prog
}
