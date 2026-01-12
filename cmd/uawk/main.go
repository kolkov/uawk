// uawk - Ultra AWK interpreter
//
// A modern, high-performance AWK interpreter written in Go.
// Uses manual argument parsing for POSIX compatibility (supports -F: style flags).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/kolkov/uawk"
)

// version is set by GoReleaser at build time via -ldflags.
// For development builds, it will be "dev".
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const (
	shortUsage = "usage: uawk [-F fs] [-v var=value] [-f progfile | 'prog'] [file ...]"
	longUsage  = `Standard AWK arguments:
  -F separator      field separator (default " ")
  -f progfile       load AWK source from progfile (multiple allowed)
  -v var=value      variable assignment (multiple allowed)

Additional uawk features:
  -c                use Unicode chars for index, length, match, substr
  -H                parse header row in CSV input mode
  -i mode           input mode: csv, tsv
  -o mode           output mode: csv, tsv

Performance options:
  --posix           use POSIX leftmost-longest regex matching (default)
  --no-posix        use faster leftmost-first regex matching (Perl-like)
  -j N              use N parallel workers (default: 1 = sequential)
                    parallel execution is automatic for suitable programs

Debugging arguments:
  -d                print parsed AST to stderr and exit
  -da               print bytecode assembly to stderr and exit
  -dt               print type information to stderr and exit
  -dp               print parallel safety analysis to stderr and exit

Other:
  -h, --help        show this help message
  -version          show uawk version and exit
`
)

//nolint:gocyclo,funlen // CLI argument parsing is inherently complex
func main() {
	// Parse command line arguments manually rather than using the
	// "flag" package, so we can support flags with no space between
	// flag and argument, like '-F:' (allowed by POSIX)
	var progFiles []string
	var vars []string
	fieldSep := " "
	inputMode := ""
	outputMode := ""
	header := false
	useChars := false
	debug := false
	debugAsm := false
	debugTypes := false
	debugParallel := false
	var posixRegex *bool // nil = default (true), explicit true/false from flags
	parallelWorkers := 1 // Default: sequential execution

	var i int
	for i = 1; i < len(os.Args); i++ {
		// Stop on explicit end of args or first arg not prefixed with "-"
		arg := os.Args[i]
		if arg == "--" {
			i++
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			break
		}

		switch arg {
		case "-F":
			if i+1 >= len(os.Args) {
				errorExitf("flag needs an argument: -F")
			}
			i++
			fieldSep = os.Args[i]
		case "-f":
			if i+1 >= len(os.Args) {
				errorExitf("flag needs an argument: -f")
			}
			i++
			progFiles = append(progFiles, os.Args[i])
		case "-v":
			if i+1 >= len(os.Args) {
				errorExitf("flag needs an argument: -v")
			}
			i++
			vars = append(vars, os.Args[i])
		case "-i":
			if i+1 >= len(os.Args) {
				errorExitf("flag needs an argument: -i")
			}
			i++
			inputMode = os.Args[i]
		case "-o":
			if i+1 >= len(os.Args) {
				errorExitf("flag needs an argument: -o")
			}
			i++
			outputMode = os.Args[i]
		case "-c":
			useChars = true
		case "-d":
			debug = true
		case "-da":
			debugAsm = true
		case "-dt":
			debugTypes = true
		case "-dp":
			debugParallel = true
		case "-j":
			if i+1 >= len(os.Args) {
				errorExitf("flag needs an argument: -j")
			}
			i++
			n, err := strconv.Atoi(os.Args[i])
			if err != nil || n < 1 {
				errorExitf("invalid number of workers: %s", os.Args[i])
			}
			parallelWorkers = n
		case "-H":
			header = true
		case "--posix":
			t := true
			posixRegex = &t
		case "--no-posix":
			f := false
			posixRegex = &f
		case "-h", "--help":
			fmt.Printf("uawk %s - Ultra AWK Interpreter\n\n%s\n\n%s", version, shortUsage, longUsage)
			os.Exit(0)
		case "-version", "--version":
			fmt.Printf("uawk version %s\n", version)
			fmt.Printf("  commit: %s\n", commit)
			fmt.Printf("  built:  %s\n", date)
			fmt.Println("  regex:  coregex")
			os.Exit(0)
		default:
			// Handle flags with no space: -F:, -ffile, -vvar=val, -j4, etc.
			switch {
			case strings.HasPrefix(arg, "-F"):
				fieldSep = arg[2:]
			case strings.HasPrefix(arg, "-f"):
				progFiles = append(progFiles, arg[2:])
			case strings.HasPrefix(arg, "-i"):
				inputMode = arg[2:]
			case strings.HasPrefix(arg, "-o"):
				outputMode = arg[2:]
			case strings.HasPrefix(arg, "-v"):
				vars = append(vars, arg[2:])
			case strings.HasPrefix(arg, "-j"):
				n, err := strconv.Atoi(arg[2:])
				if err != nil || n < 1 {
					errorExitf("invalid number of workers: %s", arg[2:])
				}
				parallelWorkers = n
			default:
				errorExitf("flag provided but not defined: %s", arg)
			}
		}
	}

	// Remaining args are program and input files
	args := os.Args[i:]

	// Determine program source
	var program string
	var inputFiles []string

	if len(progFiles) > 0 {
		// Read program from files
		var sb strings.Builder
		for _, f := range progFiles {
			content, err := os.ReadFile(f)
			if err != nil {
				errorExitf("cannot read program file %s: %v", f, err)
			}
			sb.Write(content)
			sb.WriteByte('\n')
		}
		program = sb.String()
		inputFiles = args
	} else if len(args) > 0 {
		// First arg is the program
		program = args[0]
		inputFiles = args[1:]
	} else {
		errorExitf(shortUsage)
	}

	// Compile program
	prog, err := uawk.Compile(program)
	if err != nil {
		errorExit(err)
	}

	// Debug output modes
	if debug {
		// TODO: Print AST when available
		fmt.Fprintln(os.Stderr, "AST printing not yet implemented")
		os.Exit(0)
	}
	if debugAsm {
		fmt.Fprintln(os.Stderr, prog.Disassemble())
		os.Exit(0)
	}
	if debugTypes {
		// TODO: Print type info when available
		fmt.Fprintln(os.Stderr, "Type printing not yet implemented")
		os.Exit(0)
	}
	if debugParallel {
		analysis := prog.CanParallelize("\n")
		fmt.Fprintln(os.Stderr, "=== Parallel Safety Analysis ===")
		fmt.Fprintf(os.Stderr, "Can parallelize: %v\n", analysis.CanParallelize)
		fmt.Fprintf(os.Stderr, "Safety level: %v\n", analysis.Safety)
		fmt.Fprintf(os.Stderr, "Has aggregation: %v\n", analysis.HasAggregation)
		if len(analysis.AggregatedVars) > 0 {
			fmt.Fprintf(os.Stderr, "Aggregated vars: %v\n", analysis.AggregatedVars)
		}
		if len(analysis.AggregatedArrays) > 0 {
			fmt.Fprintf(os.Stderr, "Aggregated arrays: %v\n", analysis.AggregatedArrays)
		}
		os.Exit(0)
	}

	// Build configuration with buffered output for performance
	stdout := bufio.NewWriter(os.Stdout)
	defer stdout.Flush()

	config := &uawk.Config{
		FS:         fieldSep,
		Output:     stdout,
		Stderr:     os.Stderr,
		POSIXRegex: posixRegex,
		Parallel:   parallelWorkers,
	}

	// Parse variable assignments
	if len(vars) > 0 {
		config.Variables = make(map[string]string)
		for _, v := range vars {
			parts := strings.SplitN(v, "=", 2)
			if len(parts) != 2 {
				errorExitf("invalid variable assignment: %s (expected var=value)", v)
			}
			config.Variables[parts[0]] = parts[1]
		}
	}

	// Set ARGV
	config.Args = append([]string{"uawk"}, inputFiles...)

	// Determine input source
	var input io.Reader
	if len(inputFiles) == 0 {
		// Read from stdin
		input = os.Stdin
	} else {
		// Concatenate all input files
		readers := make([]io.Reader, 0, len(inputFiles))
		for _, f := range inputFiles {
			if f == "-" {
				readers = append(readers, os.Stdin)
			} else {
				file, err := os.Open(f)
				if err != nil {
					errorExitf("cannot open file %s: %v", f, err)
				}
				defer file.Close()
				readers = append(readers, file)
			}
		}
		input = io.MultiReader(readers...)
	}

	// Execute program
	_, err = prog.Run(input, config)
	if err != nil {
		// Check if it's a normal exit with non-zero code
		if code, ok := uawk.IsExitError(err); ok {
			os.Exit(code)
		}
		errorExit(err)
	}

	// Suppress unused variable warnings (future features)
	_ = inputMode
	_ = outputMode
	_ = header
	_ = useChars
}

// errorExitf prints formatted error message and exits with code 1
func errorExitf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "uawk: "+format+"\n", args...)
	os.Exit(1)
}

// errorExit prints error and exits with code 1
func errorExit(err error) {
	fmt.Fprintf(os.Stderr, "uawk: %v\n", err)
	os.Exit(1)
}
