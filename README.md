# uawk - Ultra AWK

[![Go Reference](https://pkg.go.dev/badge/github.com/kolkov/uawk.svg)](https://pkg.go.dev/github.com/kolkov/uawk)
[![CI](https://github.com/kolkov/uawk/actions/workflows/test.yml/badge.svg)](https://github.com/kolkov/uawk/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/kolkov/uawk)](https://goreportcard.com/report/github.com/kolkov/uawk)

A modern, high-performance AWK interpreter written in Go.

## Features

- **Fast**: Outperforms GoAWK in most benchmarks (up to **31x faster** on regex patterns)
- **Compatible**: POSIX AWK compliant with GNU AWK extensions
- **Embeddable**: Clean Go API for embedding in your applications
- **Modern**: Built with Go 1.25+, powered by [coregex](https://github.com/coregx/coregex) v0.10.0
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

uawk v0.1.6 vs GoAWK on 16 regex patterns (lower is better):

### Small Data (1MB)

| Benchmark | uawk | GoAWK | vs GoAWK |
|-----------|------|-------|----------|
| alternation | **12ms** | 95ms | **8x faster** |
| inner | **11ms** | 40ms | **3.6x faster** |
| email | **32ms** | 74ms | **2.3x faster** |
| count | **10ms** | 25ms | **2.5x faster** |
| ipaddr | **19ms** | 33ms | **1.7x faster** |
| regex | **26ms** | 41ms | **1.6x faster** |

### Large Data (100MB)

| Benchmark | uawk | GoAWK | vs GoAWK |
|-----------|------|-------|----------|
| alternation | **264ms** | 8241ms | **31x faster** |
| inner | **439ms** | 3577ms | **8x faster** |
| regex | **1185ms** | 3342ms | **2.8x faster** |
| email | **2382ms** | 5848ms | **2.5x faster** |
| ipaddr | **766ms** | 1690ms | **2.2x faster** |
| select | **983ms** | 1936ms | **2.0x faster** |
| suffix | **302ms** | 568ms | **1.9x faster** |
| charclass | **299ms** | 494ms | **1.7x faster** |
| count | **648ms** | 1018ms | **1.6x faster** |

**uawk wins 13/16 benchmarks vs GoAWK.**

Performance scales with data size - up to **31x faster** on large datasets with Aho-Corasick patterns.

> Benchmarks: Windows, 3 runs median. See [uawk-test](https://github.com/kolkov/uawk-test) for full suite.

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
AWK Source → Lexer → Parser → AST → Semantic Analysis → Compiler → VM → Output
```

- **Lexer**: Context-sensitive tokenizer with UTF-8 support
- **Parser**: Recursive descent parser with comprehensive error messages
- **Compiler**: Generates optimized bytecode (~80 opcodes)
- **VM**: Stack-based virtual machine with inlined operations

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
- `-c` flag for Unicode character operations
- CSV/TSV input/output modes (planned)
- Debug flags (-d, -da, -dt)

## License

MIT License - see [LICENSE](LICENSE) file.

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## Acknowledgments

- [GoAWK](https://github.com/benhoyt/goawk) - Reference implementation and test suite
- [coregex](https://github.com/coregx/coregex) - High-performance regex engine
