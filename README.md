# uawk - Ultra AWK

[![Go Reference](https://pkg.go.dev/badge/github.com/kolkov/uawk.svg)](https://pkg.go.dev/github.com/kolkov/uawk)
[![CI](https://github.com/kolkov/uawk/actions/workflows/test.yml/badge.svg)](https://github.com/kolkov/uawk/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/kolkov/uawk)](https://goreportcard.com/report/github.com/kolkov/uawk)

A modern, high-performance AWK interpreter written in Go.

## Features

- **Fast**: Outperforms GoAWK in all benchmarks (up to **19x faster** on regex patterns)
- **Compatible**: POSIX AWK compliant with GNU AWK extensions
- **Embeddable**: Clean Go API for embedding in your applications
- **Modern**: Built with Go 1.25+, powered by [coregex](https://github.com/coregx/coregex) v0.9.1
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

uawk v0.1.2 vs GoAWK vs gawk vs mawk on 10MB dataset (lower is better):

| Benchmark | uawk | GoAWK | gawk | mawk | vs GoAWK |
|-----------|------|-------|------|------|----------|
| alternation | **37ms** | 708ms | 33ms | 31ms | **19x faster** |
| ipaddr | **49ms** | 140ms | 39ms | 104ms | **2.9x faster** |
| regex | **78ms** | 248ms | 44ms | 453ms | **3.2x faster** |
| count | **38ms** | 63ms | 61ms | 44ms | **1.7x faster** |
| csv | **67ms** | 97ms | 117ms | 91ms | **1.4x faster** |
| select | **75ms** | 96ms | 140ms | 77ms | **1.3x faster** |
| sum | **76ms** | 99ms | 117ms | 80ms | **1.3x faster** |
| groupby | **199ms** | 271ms | 307ms | 144ms | **1.4x faster** |
| filter | **106ms** | 113ms | 126ms | 98ms | **1.1x faster** |
| wordcount | **210ms** | 237ms | 299ms | 165ms | **1.1x faster** |

**uawk wins 10/10 benchmarks vs GoAWK.**

> Benchmarks run on GitHub Actions (Ubuntu, 10 runs median).
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
