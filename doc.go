// Package uawk provides a high-performance AWK interpreter.
//
// uawk is a modern AWK implementation written in Go, featuring:
//   - Full POSIX AWK compatibility
//   - High-performance regex engine (coregex)
//   - Zero external dependencies for core functionality
//   - Embeddable library for Go applications
//
// # Quick Start
//
// For simple one-off execution:
//
//	output, err := uawk.Run(`{ print $1 }`, strings.NewReader("hello world"), nil)
//
// With configuration:
//
//	output, err := uawk.Run(program, input, &uawk.Config{
//	    FS: ":",
//	    Variables: map[string]string{"threshold": "100"},
//	})
//
// # Compiled Programs
//
// For repeated execution of the same program:
//
//	prog, err := uawk.Compile(`$1 > threshold { print $2 }`)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for _, file := range files {
//	    output, err := prog.Run(file, &uawk.Config{
//	        Variables: map[string]string{"threshold": "100"},
//	    })
//	    // ...
//	}
//
// # Configuration
//
// The [Config] type allows customization of AWK execution:
//   - Field and record separators (FS, RS, OFS, ORS)
//   - Pre-defined variables
//   - Custom I/O writers
//
// # Error Handling
//
// Errors are returned as specific types for detailed handling:
//   - [ParseError]: syntax errors in AWK source
//   - [CompileError]: semantic errors during compilation
//   - [RuntimeError]: errors during execution
//
// # Thread Safety
//
// Compiled [Program] objects are safe for concurrent use.
// Each call to [Program.Run] creates an independent execution context.
package uawk
