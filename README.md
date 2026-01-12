# uawk - Ultra AWK

[![Go Reference](https://pkg.go.dev/badge/github.com/kolkov/uawk.svg)](https://pkg.go.dev/github.com/kolkov/uawk)
[![CI](https://github.com/kolkov/uawk/actions/workflows/test.yml/badge.svg)](https://github.com/kolkov/uawk/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/kolkov/uawk)](https://goreportcard.com/report/github.com/kolkov/uawk)

A modern, high-performance AWK interpreter written in Go.

## Features

- **Fast**: Outperforms GoAWK on **all 16/16 benchmarks** (up to **16x faster** on regex patterns)
- **Parallel**: Multi-file processing with `-j N` flag for concurrent execution
- **Compatible**: POSIX AWK compliant with GNU AWK extensions
- **Embeddable**: Clean Go API for embedding in your applications
- **Modern**: Built with Go 1.25+, powered by [coregex](https://github.com/coregx/coregex) v0.10.3
- **Minimal**: Zero CGO, easy cross-compilation

## Installation

```bash
go install github.com/kolkov/uawk/cmd/uawk@latest
```

## Usage

### Command Line

```bash
# Basic usage
uawk '{ print $1 }' file.txt

# Field separator
uawk -F: '{ print $1 }' /etc/passwd
uawk -F':' '{ print $1 }' /etc/passwd  # POSIX style

# Variables
uawk -v name=World 'BEGIN { print "Hello, " name }'

# Program from file
uawk -f script.awk input.txt

# Multiple input files
uawk '{ print FILENAME, $0 }' file1.txt file2.txt

# Performance mode (faster regex, non-POSIX)
uawk --no-posix '/pattern/ { print }' file.txt

# Parallel processing (multi-file)
uawk -j 4 '{ sum += $1 } END { print sum }' *.log
uawk -j 8 '/error/ { print FILENAME, $0 }' logs/*.txt
```

### As a Library

```go
package main

import (
    "fmt"
    "strings"
    "github.com/kolkov/uawk"
)

func main() {
    // Simple execution
    output, err := uawk.Run(`{ print $1 }`, strings.NewReader("hello world"), nil)
    if err != nil {
        panic(err)
    }
    fmt.Print(output) // "hello\n"

    // With configuration
    config := &uawk.Config{
        FS: ":",
        Variables: map[string]string{"threshold": "100"},
    }
    output, err = uawk.Run(`$2 > threshold { print $1 }`, input, config)

    // Fast mode (non-POSIX regex for better performance)
    fast := false
    config = &uawk.Config{POSIXRegex: &fast}
    output, err = uawk.Run(`/pattern/ { print }`, input, config)

    // Compile once, run multiple times
    prog, err := uawk.Compile(`{ sum += $1 } END { print sum }`)
    if err != nil {
        panic(err)
    }
    
    for _, file := range files {
        result, _ := prog.Run(file, nil)
        fmt.Println(result)
    }
}
```

## Benchmarks

uawk vs GoAWK on 16 benchmarks (10MB dataset, lower is better):

| Benchmark | uawk | GoAWK | vs GoAWK |
|-----------|------|-------|----------|
| alternation | **57ms** | 909ms | **16x faster** |
| email | **32ms** | 340ms | **10.6x faster** |
| inner | **47ms** | 324ms | **6.9x faster** |
| ipaddr | **57ms** | 167ms | **2.9x faster** |
| charclass | **36ms** | 80ms | **2.2x faster** |
| version | **63ms** | 134ms | **2.1x faster** |
| regex | **117ms** | 241ms | **2.1x faster** |
| select | **105ms** | 157ms | **1.5x faster** |
| suffix | **42ms** | 61ms | **1.5x faster** |
| count | **60ms** | 87ms | **1.4x faster** |
| anchored | **31ms** | 41ms | **1.3x faster** |
| sum | **95ms** | 119ms | **1.3x faster** |
| csv | **82ms** | 95ms | **1.2x faster** |
| groupby | **236ms** | 284ms | **1.2x faster** |
| filter | **105ms** | 121ms | **1.2x faster** |
| wordcount | **271ms** | 289ms | **1.1x faster** |

**uawk wins all 16/16 benchmarks vs GoAWK.**

### Performance Features

- **Parallel Execution**: Multi-file processing with automatic result merging (`-j N`)
- **Specialized Global Opcodes**: Direct array access without scope dispatch overhead
- **Static Type Specialization**: Compile-time type inference for numeric operations
- **Opcode Fusion**: Peephole optimizer combines common instruction sequences
- **CompositeSearcher**: Optimized matching for composite patterns (e.g., `[a-z]+@[a-z]+`)
- **CharClass Fast Path**: Optimized matching for character classes (`\d+`, `[a-z]+`)
- **PGO Support**: Profile-guided optimization for hot paths

> Benchmarks: Windows, 5 runs median. See [uawk-test](https://github.com/kolkov/uawk-test) for full suite.

## Building from Source

```bash
git clone https://github.com/kolkov/uawk
cd uawk
go build -o uawk ./cmd/uawk
```

### Requirements

- Go 1.25 or later

## Architecture

```
AWK Source → Lexer → Parser → AST → Semantic Analysis → Type Inference → Compiler → Optimizer → VM
```

- **Lexer**: Context-sensitive tokenizer with UTF-8 support
- **Parser**: Recursive descent parser with comprehensive error messages
- **Type Inference**: Static analysis for numeric type specialization
- **Compiler**: Generates optimized bytecode (~110 opcodes including fused ops)
- **Optimizer**: Peephole optimizer for instruction fusion
- **VM**: Stack-based virtual machine with typed operations

## Supported Features

### Standard AWK
- Pattern-action rules, BEGIN/END blocks
- Field splitting and assignment ($1, $2, $NF, etc.)
- Built-in variables (NR, NF, FS, RS, OFS, ORS, FILENAME, etc.)
- Arithmetic, string, and regex operators
- Control flow (if/else, while, for, do-while)
- Arrays (associative)
- Built-in functions (print, printf, sprintf, length, substr, split, sub, gsub, match, tolower, toupper, sin, cos, exp, log, sqrt, int, rand, srand, system, etc.)
- User-defined functions
- I/O redirection (>, >>, |, getline)

### Extensions
- `-j N` parallel execution for multi-file processing
- `-c` flag for Unicode character operations
- `--posix` / `--no-posix` regex mode selection
- Debug flags (-d, -da, -dt)
- CSV/TSV input/output modes (planned)

## License

MIT License - see [LICENSE](LICENSE) file.

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## Acknowledgments

- [GoAWK](https://github.com/benhoyt/goawk) - Reference implementation and test suite
- [coregex](https://github.com/coregx/coregex) - High-performance regex engine
