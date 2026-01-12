# uawk - Development Roadmap

> **Strategic Focus**: High-performance AWK interpreter with parallel execution

**Last Updated**: 2026-01-12 | **Current Version**: v0.2.0 | **Next**: v0.2.1 | **Target**: v1.0.0 stable

---

## Vision

Build a **production-ready, high-performance AWK interpreter** for Go that significantly outperforms GoAWK and approaches native AWK implementations (gawk, mawk) through bytecode VM optimizations and parallel execution.

### Current State vs Target

| Metric | Current (v0.2.0) | Target (v1.0.0) |
|--------|------------------|-----------------|
| Performance vs GoAWK | **16/16 benchmarks won** | ✅ Achieved |
| Best improvement | **16x faster** (alternation) | ✅ Achieved |
| Bytecode VM | **Yes (~110 opcodes)** | ✅ Achieved |
| Type Specialization | **Yes** | ✅ Achieved |
| Opcode Fusion | **Yes** | ✅ Achieved |
| Parallel execution | **Yes (-j N)** | ✅ Achieved |
| Lazy optimizations | **ENVIRON** | In Progress |
| SIMD field splitting | No | Planned v0.4.0 |
| AOT Compiler | No | Planned v0.5.0 |

---

## Release Strategy

```
v0.1.x (Done) ✅ → Stack VM, VM Perf V2, coregex optimizations
         ↓
v0.2.0 (Done) ✅ → Parallel execution, global opcodes, 16/16 wins
         ↓
v0.2.1 (Current) → Lazy ENVIRON (-56% VM creation)
         ↓
v0.3.0 → I/O optimizations, rand pooling, memory optimizations
         ↓
v0.4.0 → SIMD field splitting, projection pushdown
         ↓
v0.5.0 → AOT Compiler (AWK → Go → Native binary)
         ↓
v1.0.0 STABLE → Production release with API stability guarantee
```

### Completed Milestones

- ✅ **v0.1.0**: Stack VM interpreter, full AWK compatibility, coregex integration
- ✅ **v0.1.1-v0.1.6**: coregex updates, VM Perf V2, type specialization
- ✅ **v0.2.0**: Parallel execution, global array opcodes, 16/16 benchmark wins

---

## v0.2.0 - Parallel Execution (RELEASED)

**Goal**: Multi-file parallel processing + performance optimizations

| ID | Feature | Impact | Status |
|----|---------|--------|--------|
| P2-006 | Parallel execution `-j N` | 37-48% | ✅ Done |
| P2-007 | Global array opcodes | 7% | ✅ Done |
| P2-008 | CompositeSearcher fix | Bug fix | ✅ Done |
| P2-009 | coregex v0.10.3 | Maintenance | ✅ Done |

**Performance (vs GoAWK, 16 benchmarks)**:
- **16/16 benchmarks won**
- Best: alternation **16x faster**
- Worst: wordcount **1.1x faster**

---

## v0.2.1 - Lazy Optimizations (IN PROGRESS)

**Goal**: Reduce VM creation overhead for common cases

| ID | Feature | Impact | Status |
|----|---------|--------|--------|
| P1-010 | Lazy ENVIRON loading | -56% VM creation | ✅ Done |
| P2-011 | rand.NewSource pooling | -15% VM creation | Planned |

**Benchmark Results**:
| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| VM creation (no ENVIRON) | 87μs | 38μs | **-56%** |
| Memory allocation | 63KB | 36KB | **-43%** |
| Allocations | 150 | 25 | **-83%** |

---

## v0.3.0 - I/O & Memory (PLANNED)

**Goal**: Optimize I/O throughput and memory usage

| ID | Feature | Impact | Status |
|----|---------|--------|--------|
| P2-020 | Chunk-based I/O | 20-40% | Planned |
| P2-021 | Buffer pooling | 10-15% | Planned |
| P3-022 | Field string pooling | 5-10% | Planned |
| P3-023 | GC tuning options | Variable | Planned |

---

## v0.4.0 - Advanced Optimizations (PLANNED)

**Goal**: Match or exceed native AWK performance

| ID | Feature | Impact | Status |
|----|---------|--------|--------|
| OPT-001 | SIMD field splitting | 5-10x | Planned |
| OPT-002 | Projection pushdown | 20-50% | Planned |
| OPT-003 | Simple regex GC (vs coregex) | Research | Planned |
| OPT-004 | gawk regex pattern study | 30-80% | Planned |

---

## v0.5.0 - AOT Compiler (PLANNED)

**Goal**: Compile AWK to native binary for 10-50x performance

| ID | Feature | Impact | Status |
|----|---------|--------|--------|
| AOT-001 | Go code generator (AST → Go source) | 10-50x | Planned |
| AOT-002 | Runtime library | - | Planned |
| AOT-003 | Binary caching (~/.cache/uawk/) | Fast startup | Planned |
| AOT-004 | CLI: --compile, --build flags | UX | Planned |

---

## v1.0.0 - Production Ready

**Requirements**:
- [ ] All v0.2.x-v0.5.x features complete
- [ ] API stability guarantee
- [ ] Comprehensive documentation
- [ ] Performance regression tests
- [ ] Security audit
- [ ] 80%+ test coverage

**Guarantees**:
- API stability (no breaking changes in v1.x.x)
- Semantic versioning
- Long-term support

---

## Feature Comparison Matrix

| Feature | gawk | mawk | GoAWK | uawk v0.2.0 | uawk v1.0 |
|---------|------|------|-------|-------------|-----------|
| POSIX AWK | ✅ | ✅ | ✅ | ✅ | ✅ |
| gawk extensions | ✅ | ❌ | Partial | Partial | ✅ |
| Bytecode VM | ❌ | ❌ | ✅ | ✅ | ✅ |
| Type specialization | ❌ | ❌ | ❌ | ✅ | ✅ |
| Opcode fusion | ❌ | ❌ | ❌ | ✅ | ✅ |
| Parallel execution | ❌ | ❌ | ❌ | ✅ | ✅ |
| Lazy ENVIRON | ❌ | ❌ | ❌ | ✅ | ✅ |
| AOT compilation | ❌ | ❌ | ❌ | ❌ | ✅ |
| SIMD field split | ❌ | ❌ | ❌ | ❌ | ✅ |
| Go embedding | ❌ | ❌ | ✅ | ✅ | ✅ |
| Fast regex | ✅ | ✅ | ❌ | ✅ (coregex) | ✅ |

---

## Performance Targets

### Current (v0.2.0) ✅ ACHIEVED

| Benchmark | GoAWK | uawk | Improvement |
|-----------|-------|------|-------------|
| alternation | 909ms | 57ms | **16x faster** |
| email | 340ms | 32ms | **10.6x faster** |
| inner | 324ms | 47ms | **6.9x faster** |
| wordcount | 289ms | 271ms | **1.1x faster** |

### Target (v1.0.0)

| Metric | Target |
|--------|--------|
| vs GoAWK | 2-10x faster |
| vs gawk | 1-2x faster |
| Startup time | <10ms |
| Memory usage | <50MB for 1GB input |

---

## Competitor Analysis

Areas where competitors are faster:

| Benchmark | uawk | mawk | gawk | Gap |
|-----------|------|------|------|-----|
| anchored | 16ms | **7ms** | 31ms | mawk 2.3x |
| charclass | 18ms | **9ms** | 28ms | mawk 2x |
| inner | 23ms | **12ms** | 39ms | mawk 1.9x |
| ipaddr | 48ms | 105ms | **38ms** | gawk 1.3x |
| regex | 77ms | 456ms | **43ms** | gawk 1.8x |

**Analysis**: mawk faster on simple patterns (no GC overhead), gawk faster on specific regex.

---

## Out of Scope

**Not planned**:
- Full gawk compatibility (GNU extensions like @include)
- Network AWK extensions
- GUI/IDE integration
- AWK to other language transpilers

---

## Release History

| Version | Date | Type | Key Changes |
|---------|------|------|-------------|
| **v0.2.0** | 2026-01-12 | Feature | Parallel -j N, global opcodes, 16/16 wins |
| **v0.1.6** | 2026-01-12 | Perf | VM Perf V2, --posix/--no-posix |
| **v0.1.5** | 2026-01-07 | Perf | coregex v0.10.0, Fat Teddy AVX2 |
| **v0.1.4** | 2026-01-07 | Perf | coregex v0.9.5, Teddy 32 patterns |
| **v0.1.3** | 2026-01-06 | Perf | coregex v0.9.2, compile-time strategy |
| **v0.1.2** | 2026-01-05 | Perf | coregex v0.9.1, adaptive DigitPrefilter |
| **v0.1.1** | 2026-01-05 | Perf | coregex v0.9.0, Aho-Corasick |
| **v0.1.0** | 2025-01-04 | Initial | Stack VM, 8/8 benchmark wins |

---

*Current: v0.2.0 | Next: v0.2.1 (Lazy ENVIRON) | Target: v1.0.0*
