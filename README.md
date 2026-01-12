# uawk

[![Go Reference](https://pkg.go.dev/badge/github.com/kolkov/uawk.svg)](https://pkg.go.dev/github.com/kolkov/uawk)
[![CI](https://github.com/kolkov/uawk/actions/workflows/test.yml/badge.svg)](https://github.com/kolkov/uawk/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/kolkov/uawk)](https://goreportcard.com/report/github.com/kolkov/uawk)

AWK interpreter written in Go with [coregex](https://github.com/coregx/coregex) regex engine.

## Features

- POSIX AWK compliant with GNU AWK extensions
- Parallel file processing (`-j N`)
- Embeddable Go API
- Zero CGO dependencies

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

# Variables
uawk -v name=World 'BEGIN { print "Hello, " name }'

# Program from file
uawk -f script.awk input.txt

# Parallel processing
uawk -j 4 '{ sum += $1 } END { print sum }' *.log

# Non-POSIX regex mode
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
    output, err := uawk.Run(`{ print $1 }`, strings.NewReader("hello world"), nil)
    if err != nil {
        panic(err)
    }
    fmt.Print(output)

    // With configuration
    config := &uawk.Config{
        FS: ":",
        Variables: map[string]string{"threshold": "100"},
    }
    output, err = uawk.Run(`$2 > threshold { print $1 }`, input, config)

    // Compile once, run multiple times
    prog, err := uawk.Compile(`{ sum += $1 } END { print sum }`)
    for _, file := range files {
        result, _ := prog.Run(file, nil)
        fmt.Println(result)
    }
}
```

## Benchmarks

See [uawk-test](https://github.com/kolkov/uawk-test) for benchmark suite and methodology.

Results vary by workload. Regex-heavy patterns benefit from coregex optimizations. I/O-bound workloads show smaller differences between implementations.

## Building

```bash
git clone https://github.com/kolkov/uawk
cd uawk
go build -o uawk ./cmd/uawk
```

Requires Go 1.25+.

## Architecture

```
Source → Lexer → Parser → AST → Semantic Analysis → Compiler → Optimizer → VM
```

| Component | Description |
|-----------|-------------|
| Lexer | Context-sensitive tokenizer, UTF-8 |
| Parser | Recursive descent |
| Compiler | Bytecode generation (~110 opcodes) |
| Optimizer | Peephole optimization |
| VM | Stack-based execution |

## Supported Features

### Standard AWK
- Pattern-action rules, BEGIN/END blocks
- Field splitting and assignment
- Built-in variables (NR, NF, FS, RS, OFS, ORS, FILENAME, etc.)
- Arithmetic, string, and regex operators
- Control flow (if/else, while, for, do-while)
- Associative arrays
- Built-in functions (print, printf, sprintf, length, substr, split, sub, gsub, match, tolower, toupper, sin, cos, exp, log, sqrt, int, rand, srand, system, etc.)
- User-defined functions
- I/O redirection (>, >>, |, getline)

### Extensions
- `-j N` parallel execution
- `-c` Unicode character operations
- `--posix` / `--no-posix` regex mode
- Debug flags (-d, -da, -dt)

## License

MIT

## Acknowledgments

- [GoAWK](https://github.com/benhoyt/goawk) by Ben Hoyt — reference implementation and test suite
- [coregex](https://github.com/coregx/coregex) — regex engine
