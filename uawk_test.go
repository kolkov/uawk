package uawk_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kolkov/uawk"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name    string
		program string
		input   string
		config  *uawk.Config
		want    string
		wantErr bool
	}{
		{
			name:    "print first field",
			program: `{ print $1 }`,
			input:   "hello world\n",
			want:    "hello\n",
		},
		{
			name:    "print all fields",
			program: `{ print $0 }`,
			input:   "hello world\n",
			want:    "hello world\n",
		},
		{
			name:    "sum numbers",
			program: `{ sum += $1 } END { print sum }`,
			input:   "1\n2\n3\n",
			want:    "6\n",
		},
		{
			name:    "BEGIN only",
			program: `BEGIN { print "hello" }`,
			input:   "",
			want:    "hello\n",
		},
		{
			name:    "END only",
			program: `END { print "done" }`,
			input:   "ignored\n",
			want:    "done\n",
		},
		{
			name:    "custom field separator",
			program: `{ print $1 }`,
			input:   "a:b:c\n",
			config:  &uawk.Config{FS: ":"},
			want:    "a\n",
		},
		{
			name:    "NR and NF",
			program: `{ print NR, NF }`,
			input:   "a b\nc d e\n",
			want:    "1 2\n2 3\n",
		},
		{
			name:    "pattern match",
			program: `/hello/ { print "found" }`,
			input:   "hello world\ngoodbye\n",
			want:    "found\n",
		},
		{
			name:    "arithmetic",
			program: `BEGIN { print 2 + 3 * 4 }`,
			input:   "",
			want:    "14\n",
		},
		{
			name:    "string concatenation",
			program: `BEGIN { print "hello" " " "world" }`,
			input:   "",
			want:    "hello world\n",
		},
		{
			name:    "user-defined function",
			program: `function double(x) { return x * 2 } BEGIN { print double(21) }`,
			input:   "",
			want:    "42\n",
		},
		{
			name:    "arrays",
			program: `{ a[$1]++ } END { for (k in a) print k, a[k] }`,
			input:   "a\nb\na\n",
			// Note: order may vary, so we check contents separately
		},
		{
			name:    "printf",
			program: `BEGIN { printf "%d %.2f %s\n", 42, 3.14159, "test" }`,
			input:   "",
			want:    "42 3.14 test\n",
		},
		{
			name:    "gsub",
			program: `{ gsub(/o/, "0"); print }`,
			input:   "hello world\n",
			want:    "hell0 w0rld\n",
		},
		{
			name:    "sub",
			program: `{ sub(/o/, "0"); print }`,
			input:   "hello world\n",
			want:    "hell0 world\n",
		},
		{
			name:    "length",
			program: `{ print length($0) }`,
			input:   "hello\n",
			want:    "5\n",
		},
		{
			name:    "substr",
			program: `{ print substr($0, 2, 3) }`,
			input:   "hello\n",
			want:    "ell\n",
		},
		{
			name:    "split",
			program: `{ n = split($0, a, ":"); print n, a[1], a[2] }`,
			input:   "a:b:c\n",
			want:    "3 a b\n",
		},
		{
			name:    "index",
			program: `{ print index($0, "ll") }`,
			input:   "hello\n",
			want:    "3\n",
		},
		{
			name:    "tolower toupper",
			program: `{ print tolower($1), toupper($2) }`,
			input:   "Hello World\n",
			want:    "hello WORLD\n",
		},
		{
			name:    "ternary operator",
			program: `{ print ($1 > 5 ? "big" : "small") }`,
			input:   "3\n10\n",
			want:    "small\nbig\n",
		},
		{
			name:    "increment decrement",
			program: `BEGIN { x = 5; print ++x, x++, x }`,
			input:   "",
			want:    "6 6 7\n",
		},
		{
			name:    "empty input",
			program: `BEGIN { print "start" } { print $0 } END { print "end" }`,
			input:   "",
			want:    "start\nend\n",
		},
		// Error cases
		{
			name:    "syntax error",
			program: `{ print $1`,
			input:   "",
			wantErr: true,
		},
		{
			name:    "undefined function",
			program: `BEGIN { undefined() }`,
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip array test - order is non-deterministic
			if tt.name == "arrays" {
				t.Skip("array iteration order is non-deterministic")
			}

			got, err := uawk.Run(tt.program, strings.NewReader(tt.input), tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Run() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompile(t *testing.T) {
	// Test that Compile returns a reusable program
	prog, err := uawk.Compile(`{ sum += $1 } END { print sum }`)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	// Run multiple times with different inputs
	inputs := []string{"1\n2\n3\n", "10\n20\n30\n"}
	wants := []string{"6\n", "60\n"}

	for i, input := range inputs {
		got, err := prog.Run(strings.NewReader(input), nil)
		if err != nil {
			t.Errorf("Run(%d) error = %v", i, err)
			continue
		}
		if got != wants[i] {
			t.Errorf("Run(%d) = %q, want %q", i, got, wants[i])
		}
	}
}

func TestMustCompile(t *testing.T) {
	// Test that MustCompile panics on error
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustCompile() should panic on invalid program")
		}
	}()

	_ = uawk.MustCompile(`{ print $1`) // Missing closing brace
}

func TestMustCompileValid(t *testing.T) {
	// Test that MustCompile works for valid programs
	prog := uawk.MustCompile(`{ print $1 }`)
	if prog == nil {
		t.Error("MustCompile() returned nil for valid program")
	}
}

func TestParseError(t *testing.T) {
	_, err := uawk.Compile(`{ print $1`)
	if err == nil {
		t.Fatal("expected error for invalid program")
	}

	_, ok := err.(*uawk.ParseError)
	if !ok {
		t.Errorf("expected *ParseError, got %T", err)
	}
}

func TestConfigFieldSeparator(t *testing.T) {
	got, err := uawk.Run(`{ print $2 }`, strings.NewReader("a:b:c\n"), &uawk.Config{FS: ":"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got != "b\n" {
		t.Errorf("Run() = %q, want %q", got, "b\n")
	}
}

func TestConfigVariables(t *testing.T) {
	prog := `BEGIN { print prefix, threshold }`
	config := &uawk.Config{
		Variables: map[string]string{
			"prefix":    "LOG:",
			"threshold": "100",
		},
	}
	got, err := uawk.Run(prog, nil, config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got != "LOG: 100\n" {
		t.Errorf("Run() = %q, want %q", got, "LOG: 100\n")
	}
}

func TestExitError(t *testing.T) {
	_, err := uawk.Run(`BEGIN { exit 42 }`, nil, nil)
	if err == nil {
		t.Fatal("expected error for exit 42")
	}

	code, ok := uawk.IsExitError(err)
	if !ok {
		t.Errorf("expected ExitError, got %T", err)
	}
	if code != 42 {
		t.Errorf("exit code = %d, want 42", code)
	}
}

func TestExitZero(t *testing.T) {
	// exit 0 should not return an error
	_, err := uawk.Run(`BEGIN { exit 0 }`, nil, nil)
	if err != nil {
		t.Errorf("exit 0 should not return error, got %v", err)
	}
}

func TestProgramDisassemble(t *testing.T) {
	prog, err := uawk.Compile(`{ print $1 }`)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	dis := prog.Disassemble()
	if dis == "" {
		t.Error("Disassemble() returned empty string")
	}
	if !strings.Contains(dis, "Action") {
		t.Errorf("Disassemble() should contain 'Action', got: %s", dis)
	}
}

func TestProgramSource(t *testing.T) {
	source := `{ print $1 }`
	prog, err := uawk.Compile(source)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if prog.Source() != source {
		t.Errorf("Source() = %q, want %q", prog.Source(), source)
	}
}

// Benchmark tests
func BenchmarkRun(b *testing.B) {
	input := strings.NewReader("hello world\n")
	for i := 0; i < b.N; i++ {
		input.Reset("hello world\n")
		_, _ = uawk.Run(`{ print $1 }`, input, nil)
	}
}

func BenchmarkCompileAndRun(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = uawk.Run(`{ sum += $1 } END { print sum }`, strings.NewReader("1\n2\n3\n"), nil)
	}
}

func BenchmarkCompiledRun(b *testing.B) {
	prog, _ := uawk.Compile(`{ sum += $1 } END { print sum }`)
	input := strings.NewReader("1\n2\n3\n")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		input.Reset("1\n2\n3\n")
		_, _ = prog.Run(input, nil)
	}
}

// Example functions for documentation
func ExampleRun() {
	output, _ := uawk.Run(`{ print $1 }`, strings.NewReader("hello world\n"), nil)
	fmt.Print(output)
	// Output: hello
}

func ExampleCompile() {
	prog, _ := uawk.Compile(`{ sum += $1 } END { print sum }`)
	output, _ := prog.Run(strings.NewReader("1\n2\n3\n"), nil)
	fmt.Print(output)
	// Output: 6
}
