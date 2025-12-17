# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2025-01-04

### Added
- Initial uawk implementation with full AWK interpreter
- Stack-based bytecode VM with ~80 opcodes
- Coregex integration for high-performance regex (3-3000x faster than stdlib)
- Public API for embedding (Run, Compile, Config, Program)
- POSIX-compatible CLI with manual argument parsing
- Comprehensive test suite (2000+ test runs)
- Full POSIX AWK compatibility
- GNU AWK extensions support

### Performance
- IsTrueStr optimization for comparisons (-39% filter benchmark)
- Field opcode peek+replaceTop optimization
- Unary ops peek+replaceTop optimization
- Inline stack operations (removed pointer indirection)
- Lazy NumStr parsing
- Whitespace lookup table [256]bool
- PeekPop/ReplaceTop for binary operations

### Benchmarks (vs GoAWK)
- select: -65%
- regex: -53%
- count: -35%
- sum: -16%
- csv: -15%
- groupby: -15%
- wordcount: -6%
- filter: -2%

[Unreleased]: https://github.com/kolkov/uawk/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/kolkov/uawk/releases/tag/v0.1.0
