package vm_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kolkov/uawk/internal/compiler"
	"github.com/kolkov/uawk/internal/parser"
	"github.com/kolkov/uawk/internal/semantic"
	"github.com/kolkov/uawk/internal/vm"
)

func BenchmarkVMFilter(b *testing.B) {
	source := `$1 > 500 && $2 < 500 { print }`
	prog, _ := parser.Parse(source)
	resolved, _ := semantic.Resolve(prog)
	compiled, _ := compiler.Compile(prog, resolved)

	var input strings.Builder
	for i := 0; i < 10000; i++ {
		input.WriteString("305 66.000497 668 208.818703 423\n")
	}
	inputStr := input.String()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		v := vm.New(compiled)
		v.SetInput(strings.NewReader(inputStr))
		var buf bytes.Buffer
		v.SetOutput(&buf)
		v.Run()
	}
}

func BenchmarkVMCount(b *testing.B) {
	source := `{ fields += NF } END { print NR, fields }`
	prog, _ := parser.Parse(source)
	resolved, _ := semantic.Resolve(prog)
	compiled, _ := compiler.Compile(prog, resolved)

	var input strings.Builder
	for i := 0; i < 10000; i++ {
		input.WriteString("305 66.000497 668 208.818703 423\n")
	}
	inputStr := input.String()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		v := vm.New(compiled)
		v.SetInput(strings.NewReader(inputStr))
		var buf bytes.Buffer
		v.SetOutput(&buf)
		v.Run()
	}
}

func BenchmarkVMCSV(b *testing.B) {
	source := `BEGIN { FS = "," } { sum += $3 } END { print sum }`
	prog, _ := parser.Parse(source)
	resolved, _ := semantic.Resolve(prog)
	compiled, _ := compiler.Compile(prog, resolved)

	var input strings.Builder
	for i := 0; i < 10000; i++ {
		input.WriteString("305,66.000497,668,208.818703,423\n")
	}
	inputStr := input.String()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		v := vm.New(compiled)
		v.SetInput(strings.NewReader(inputStr))
		var buf bytes.Buffer
		v.SetOutput(&buf)
		v.Run()
	}
}

func BenchmarkVMWordCount(b *testing.B) {
	source := `{ for (i = 1; i <= NF; i++) words[tolower($i)]++ } END { for (w in words) print words[w], w }`
	prog, _ := parser.Parse(source)
	resolved, _ := semantic.Resolve(prog)
	compiled, _ := compiler.Compile(prog, resolved)

	var input strings.Builder
	for i := 0; i < 10000; i++ {
		input.WriteString("The quick brown fox jumps over lazy dog\n")
	}
	inputStr := input.String()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		v := vm.New(compiled)
		v.SetInput(strings.NewReader(inputStr))
		var buf bytes.Buffer
		v.SetOutput(&buf)
		v.Run()
	}
}
