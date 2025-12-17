# uawk - Development Roadmap

> **Strategic Focus**: High-performance AWK interpreter with AOT compilation

**Last Updated**: 2025-01-04 | **Current Version**: v0.1.0 | **Target**: v1.0.0 stable

---

## Vision

Build a **production-ready, high-performance AWK interpreter** for Go that significantly outperforms GoAWK and approaches native AWK implementations (gawk, mawk) through bytecode VM and AOT compilation.

### Current State vs Target

| Metric | Current (v0.1.0) | Target (v1.0.0) |
|--------|------------------|-----------------|
| Performance vs GoAWK | **8/8 benchmarks won** | ✅ Achieved |
| select benchmark | **-65%** | ✅ Achieved |
| regex benchmark | **-53%** | ✅ Achieved |
| Bytecode VM | **Yes (~80 opcodes)** | ✅ Achieved |
| CharClass fast paths | **Yes (14-22x speedup)** | ✅ Achieved |
| Composite patterns | **Yes (6x speedup)** | ✅ Achieved |
| AOT Compiler | No | Planned v0.2.0 |
| SIMD field splitting | No | Planned v0.3.0 |
| Parallel execution | No | Planned v0.3.0 |

---

## Release Strategy

```
v0.1.0 (Current) ✅ → Stack VM interpreter, 8/8 benchmark wins
         ↓
v0.2.0 → AOT Compiler (AWK → Go → Native binary)
         ↓
v0.3.0 → Advanced optimizations (SIMD, parallel)
         ↓
v1.0.0 STABLE → Production release with API stability guarantee
```

### Completed Milestones

- ✅ **v0.1.0**: Stack VM interpreter, full AWK compatibility, coregex integration

---

## v0.1.0 - Interpreter (RELEASED)

**Goal**: Production-quality AWK interpreter

| ID | Feature | Status |
|----|---------|--------|
| Core | Lexer, Parser, AST | ✅ Done |
| Semantic | Resolver, Checker | ✅ Done |
| Compiler | AST → Bytecode (~80 opcodes) | ✅ Done |
| VM | Stack-based execution | ✅ Done |
| Builtins | All AWK functions | ✅ Done |
| I/O | Files, pipes, redirections | ✅ Done |
| Regex | coregex integration (3-3000x faster) | ✅ Done |
| Fast paths | CharClass, Composite, Prefilter | ✅ Done |
| CLI | Manual POSIX parsing | ✅ Done |
| Public API | Run(), Compile(), Config | ✅ Done |
| Quality | 71% coverage, 0 lint issues | ✅ Done |

**Performance (vs GoAWK)**:
| Benchmark | Improvement |
|-----------|-------------|
| select | -65% |
| regex | -53% |
| count | -35% |
| sum | -16% |
| csv | -15% |
| groupby | -15% |
| wordcount | -6% |
| filter | -2% |

---

## v0.2.0 - AOT Compiler (PLANNED)

**Goal**: Compile AWK to native binary for 10-50x performance

| ID | Feature | Impact | Status |
|----|---------|--------|--------|
| AOT-001 | Go code generator (AST → Go source) | 10-50x | Planned |
| AOT-002 | Runtime library | - | Planned |
| AOT-003 | Binary caching (~/.cache/uawk/) | Fast startup | Planned |
| AOT-004 | CLI: --compile, --build flags | UX | Planned |
| AOT-005 | Incremental compilation | Build speed | Planned |

**Target**: Q2 2025

---

## v0.3.0 - Advanced Optimizations (PLANNED)

**Goal**: Match or exceed native AWK performance

| ID | Feature | Impact | Status |
|----|---------|--------|--------|
| OPT-001 | SIMD field splitting | 5-10x | Planned |
| OPT-002 | Projection pushdown | 20-50% | Planned |
| OPT-003 | Type inference | 10-20% | Planned |
| OPT-004 | Parallel execution | 2-4x | Planned |
| OPT-005 | Memory pools | 10-15% | Planned |

**Target**: Q3 2025

---

## v1.0.0 - Production Ready

**Requirements**:
- [ ] All v0.2.0-v0.3.0 features complete
- [ ] API stability guarantee
- [ ] Comprehensive documentation
- [ ] Performance regression tests
- [ ] Security audit
- [ ] 80%+ test coverage

**Guarantees**:
- API stability (no breaking changes in v1.x.x)
- Semantic versioning
- Long-term support

**Target**: Q4 2025

---

## Feature Comparison Matrix

| Feature | gawk | mawk | GoAWK | uawk v0.1.0 | uawk v1.0 |
|---------|------|------|-------|-------------|-----------|
| POSIX AWK | ✅ | ✅ | ✅ | ✅ | ✅ |
| gawk extensions | ✅ | ❌ | Partial | Partial | ✅ |
| Bytecode VM | ❌ | ❌ | ✅ | ✅ | ✅ |
| AOT compilation | ❌ | ❌ | ❌ | ❌ | ✅ |
| SIMD field split | ❌ | ❌ | ❌ | ❌ | ✅ |
| Parallel execution | ❌ | ❌ | ❌ | ❌ | ✅ |
| Go embedding | ❌ | ❌ | ✅ | ✅ | ✅ |
| Fast regex | ✅ | ✅ | ❌ | ✅ (coregex) | ✅ |

---

## Performance Targets

### Current (v0.1.0) ✅ ACHIEVED

| Pattern | GoAWK | uawk | Improvement |
|---------|-------|------|-------------|
| select | 193ms | 67ms | **-65%** |
| regex | 238ms | 113ms | **-53%** |
| count | 101ms | 66ms | **-35%** |
| sum | 101ms | 85ms | **-16%** |

### Target (v1.0.0)

| Pattern | Target |
|---------|--------|
| vs GoAWK | 2-5x faster |
| vs gawk | 1-2x faster |
| Startup time | <10ms |
| Memory usage | <50MB for 1GB input |

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
| **v0.1.0** | 2025-01-04 | Initial | Stack VM, 8/8 benchmark wins |

---

*Current: v0.1.0 | Next: v0.2.0 (AOT) | Target: v1.0.0*
