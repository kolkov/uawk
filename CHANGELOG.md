# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.3] - 2026-01-06

### Changed
- Updated coregex to v0.9.2 with compile-time strategy selection
  - Simple digit patterns (≤100 NFA states): DigitPrefilter
  - Complex digit patterns (>100 NFA states): LazyDFA
  - Eliminates runtime adaptive tracking overhead
  - IP pattern matching: **146x faster** vs v0.9.1

## [0.1.2] - 2026-01-05

### Changed
- Updated coregex to v0.9.1 with adaptive DigitPrefilter
  - Adaptive switching: transitions to DFA after 64 consecutive false positives
  - Sparse data: maintains 300x speedup via SIMD
  - Dense data: 3.5x faster with intelligent DFA fallback
  - No-match scenarios: up to 3000x faster

## [0.1.1] - 2026-01-05

### Changed
- Updated coregex to v0.9.0 with new optimization strategies
  - UseAhoCorasick: 75-113x faster for large alternations (>8 patterns)
  - DigitPrefilter: up to 2500x faster for IP address patterns
  - Paired-byte SIMD: improved rare byte pair searching
- Added ahocorasick transitive dependency for multi-pattern matching

### Performance
- regex benchmark: additional improvements from coregex v0.9.0
- All 7/8 benchmarks faster than GoAWK maintained

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

[Unreleased]: https://github.com/kolkov/uawk/compare/v0.1.3...HEAD
[0.1.3]: https://github.com/kolkov/uawk/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/kolkov/uawk/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/kolkov/uawk/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/kolkov/uawk/releases/tag/v0.1.0
