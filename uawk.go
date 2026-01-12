package uawk

import (
	"io"

	"github.com/kolkov/uawk/internal/compiler"
	"github.com/kolkov/uawk/internal/parser"
	"github.com/kolkov/uawk/internal/semantic"
)

// Version is the uawk version string.
const Version = "0.1.0"

// Run executes an AWK program with the given input.
// This is a convenience function for one-off execution.
// For repeated execution of the same program, use Compile followed by Program.Run.
//
// Parameters:
//   - program: AWK source code
//   - input: input data reader (can be nil for programs without input)
//   - config: execution configuration (can be nil for defaults)
//
// Returns the program output as a string, or an error if parsing,
// compilation, or execution fails.
//
// Example:
//
//	output, err := uawk.Run(`{ print $1 }`, strings.NewReader("hello world"), nil)
//	// output: "hello\n"
func Run(program string, input io.Reader, config *Config) (string, error) {
	prog, err := Compile(program)
	if err != nil {
		return "", err
	}
	return prog.Run(input, config)
}

// Compile parses and compiles an AWK program for execution.
// The returned Program can be executed multiple times with different inputs.
//
// Example:
//
//	prog, err := uawk.Compile(`{ sum += $1 } END { print sum }`)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	output1, _ := prog.Run(file1, nil)
//	output2, _ := prog.Run(file2, nil)
func Compile(program string) (*Program, error) {
	// Parse
	astProg, err := parser.Parse(program)
	if err != nil {
		// Convert parser error to public type
		if pe, ok := err.(*parser.ParseError); ok {
			return nil, &ParseError{
				Line:    pe.Pos.Line,
				Column:  pe.Pos.Column,
				Message: pe.Message,
			}
		}
		// Handle ErrorList (multiple errors)
		if el, ok := err.(parser.ErrorList); ok && len(el) > 0 {
			return nil, &ParseError{
				Line:    el[0].Pos.Line,
				Column:  el[0].Pos.Column,
				Message: el[0].Message,
			}
		}
		return nil, &ParseError{Message: err.Error()}
	}

	// Resolve symbols
	resolved, err := semantic.Resolve(astProg)
	if err != nil {
		return nil, &CompileError{Message: err.Error()}
	}

	// Check for semantic errors
	if errs := semantic.Check(astProg, resolved); len(errs) > 0 {
		return nil, &CompileError{Message: errs[0].Error()}
	}

	// Compile to bytecode
	compiled, err := compiler.Compile(astProg, resolved)
	if err != nil {
		return nil, &CompileError{Message: err.Error()}
	}

	// Apply peephole optimizations (fuse common instruction patterns)
	compiler.OptimizeProgram(compiled)

	return &Program{
		compiled: compiled,
		source:   program,
	}, nil
}

// Exec is a simplified interface for running an AWK program.
// It reads from input, writes to output, and returns any error.
//
// This function is useful for integration with I/O pipelines
// where you need control over the output writer.
//
// Example:
//
//	err := uawk.Exec(`{ print toupper($0) }`, os.Stdin, os.Stdout, nil)
func Exec(program string, input io.Reader, output io.Writer, config *Config) error {
	prog, err := Compile(program)
	if err != nil {
		return err
	}

	if config == nil {
		config = &Config{}
	}
	config.Output = output

	_, err = prog.Run(input, config)
	return err
}

// MustCompile is like Compile but panics if the program cannot be compiled.
// It simplifies initialization of global program variables.
//
// Example:
//
//	var sumProgram = uawk.MustCompile(`{ sum += $1 } END { print sum }`)
func MustCompile(program string) *Program {
	prog, err := Compile(program)
	if err != nil {
		panic(err)
	}
	return prog
}
