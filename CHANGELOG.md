# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Parallel execution** with `-j N` flag for concurrent multi-file processing
  - Automatic detection of parallel-safe programs (no BEGIN/END, no shared state)
  - Worker pool processes files concurrently with output ordering preserved
  - Speedup scales with CPU cores on I/O-bound workloads

### Changed
- Updated coregex to v0.10.3

### Fixed
- **CompositeSearcher backtracking bug** for overlapping patterns
  - Patterns like `[a-z.]+\.com` now match correctly
  - Email pattern was silently failing due to greedy char class consuming literal dots
  - Added overlap detection: reject patterns where char class can match following literal's first char

### Performance
- **Specialized global array opcodes** eliminate scope switch overhead
  - New opcodes: `ArrayGetGlobal`, `ArraySetGlobal`, `ArrayDeleteGlobal`, `ArrayInGlobal`, `IncrArrayGlobal`, `AugArrayGlobal`
  - wordcount benchmark: 281ms → 271ms (7% improvement, now beats GoAWK)
- **email benchmark**: 181ms → 32ms (5.7x faster, was broken before fix)
- **All 16/16 benchmarks now faster than GoAWK**

## [0.1.6] - 2026-01-11

### Added
- `--posix` / `--no-posix` CLI flags for regex mode selection
- `POSIXRegex` option in `uawk.Config` for library usage
- `RegexConfig` and `VMConfig` for fine-grained control

### Performance
- `--no-posix` mode disables leftmost-longest matching for faster regex
- Up to 12% improvement on some patterns in fast mode

## [0.1.5] - 2026-01-07

### Changed
- Updated coregex to v0.10.0 with Fat Teddy AVX2 prefilter
  - Fat Teddy: 33-64 patterns at **9+ GB/s** (vs 150 MB/s Aho-Corasick)
  - **73x faster** for 40-pattern scenarios
  - Small haystack optimization: **2.4x faster**
  - 5 patterns now faster than Rust regex (char class, IP, suffix, email, anchored)
  - Pure Go scalar fallback for non-AVX2 platforms

### Performance
- alternation: 19x faster vs GoAWK
- Note: inner literal pattern +53% regression in coregex (under investigation)

## [0.1.4] - 2026-01-07

### Changed
- Updated coregex to v0.9.5 with multiple improvements:
  - **v0.9.3**: Teddy 2-byte fingerprint (false positives: 25% → <0.5%)
  - **v0.9.3**: DigitPrefilter priority before tiny NFA fallback
  - **v0.9.4**: Streaming CharClassSearcher (15-30% faster FindAll)
  - **v0.9.5**: Teddy pattern limit 8→32 (Aho-Corasick threshold now >32)
  - **v0.9.5**: Fix for factored prefix patterns (**220x** speedup)

### Performance
- All 10/10 benchmarks faster than GoAWK
- alternation: 16x faster (Teddy improvements)
- regex: 58% faster
- select: 45% faster

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

[Unreleased]: https://github.com/kolkov/uawk/compare/v0.1.6...HEAD
[0.1.6]: https://github.com/kolkov/uawk/compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com/kolkov/uawk/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/kolkov/uawk/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/kolkov/uawk/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/kolkov/uawk/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/kolkov/uawk/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/kolkov/uawk/releases/tag/v0.1.0
