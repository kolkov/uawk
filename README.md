# uawk - Ultra AWK

[![Go Reference](https://pkg.go.dev/badge/github.com/kolkov/uawk.svg)](https://pkg.go.dev/github.com/kolkov/uawk)
[![CI](https://github.com/kolkov/uawk/actions/workflows/test.yml/badge.svg)](https://github.com/kolkov/uawk/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/kolkov/uawk)](https://goreportcard.com/report/github.com/kolkov/uawk)

A modern, high-performance AWK interpreter written in Go.

## Features

- **Fast**: Outperforms GoAWK in all benchmarks (up to **19x faster** on regex patterns)
- **Compatible**: POSIX AWK compliant with GNU AWK extensions
- **Embeddable**: Clean Go API for embedding in your applications
- **Modern**: Built with Go 1.25+, powered by [coregex](https://github.com/coregx/coregex) v0.9.5
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

uawk v0.1.4 vs GoAWK vs gawk on 10MB dataset (lower is better):

| Benchmark | uawk | GoAWK | gawk | vs GoAWK |
|-----------|------|-------|------|----------|
| alternation | **52ms** | 827ms | 87ms | **16x faster** |
| regex | **132ms** | 311ms | 395ms | **2.4x faster** |
| select | **89ms** | 163ms | 262ms | **1.8x faster** |
| ipaddr | **85ms** | 150ms | 109ms | **1.8x faster** |
| csv | **82ms** | 122ms | 151ms | **1.5x faster** |
| count | **75ms** | 95ms | 109ms | **1.3x faster** |
| groupby | **253ms** | 286ms | 390ms | **1.1x faster** |
| sum | **110ms** | 125ms | 139ms | **1.1x faster** |
| filter | **124ms** | 140ms | 169ms | **1.1x faster** |
| wordcount | **239ms** | 256ms | 547ms | **1.1x faster** |

**uawk wins 10/10 benchmarks vs GoAWK.**

> Benchmarks run locally (Windows, 5 runs median).
> See [uawk-test](https://github.com/kolkov/uawk-test) for benchmark suite.

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
