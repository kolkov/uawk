# Contributing to uawk

Thank you for considering contributing to uawk! This document outlines the development workflow and guidelines.

## Quick Start

```bash
# Clone repository
git clone https://github.com/kolkov/uawk.git
cd uawk

# Run tests
go test ./...

# Run linter
golangci-lint run

# Build CLI
go build -o uawk.exe ./cmd/uawk

# Run pre-release checks
bash scripts/pre-release-check.sh
```

## Development Workflow

### Branch Structure

```
main                 # Production-ready code (tagged releases)
  ├─ feature/*       # New features
  ├─ bugfix/*        # Bug fixes
  └─ hotfix/*        # Critical fixes
```

### Starting a New Feature

```bash
git checkout main
git pull origin main
git checkout -b feature/my-new-feature

# Work on your feature...
git add .
git commit -m "feat: add my new feature"

# Create pull request
```

### Fixing a Bug

```bash
git checkout main
git pull origin main
git checkout -b bugfix/fix-issue-123

# Fix the bug...
git add .
git commit -m "fix: resolve issue #123"

# Create pull request
```

## Commit Message Guidelines

Follow [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <description>

[optional body]
```

### Types

- **feat**: New feature
- **fix**: Bug fix
- **docs**: Documentation changes
- **style**: Code style changes (formatting, etc.)
- **refactor**: Code refactoring
- **test**: Adding or updating tests
- **chore**: Maintenance tasks
- **perf**: Performance improvements

### Examples

```bash
feat(vm): add new opcode for field caching
fix(parser): handle empty regex correctly
docs: update README with benchmarks
perf(vm): inline stack operations
test(lexer): add fuzz tests for edge cases
chore: update golangci-lint config
```

### Performance Commits

For optimization commits, include benchmark results:

```
perf(vm): inline stack operations

Бенчмарк awkbench (10 runs, median):
  wordcount: 468ms → 398ms (-15%)
  sum:       202ms → 185ms (-8%)
  filter:    209ms → 195ms (-7%)

Files: internal/vm/vm.go
```

## Code Quality Standards

### Before Committing

1. **Format code**:
   ```bash
   go fmt ./...
   ```

2. **Run linter**:
   ```bash
   golangci-lint run
   ```

3. **Run tests**:
   ```bash
   go test ./...
   ```

4. **Run tests with race detector** (requires GCC or WSL2):
   ```bash
   go test -race ./...
   ```

5. **All-in-one** (use pre-release script):
   ```bash
   bash scripts/pre-release-check.sh
   ```

### Pull Request Requirements

- [ ] Code is formatted (`go fmt ./...`)
- [ ] Linter passes (`golangci-lint run` - 0 issues)
- [ ] All tests pass (`go test ./...`)
- [ ] Race detector passes (`go test -race ./...`)
- [ ] New code has tests (minimum 70% coverage)
- [ ] Documentation updated (if applicable)
- [ ] Commit messages follow conventions
- [ ] Benchmarks added for performance-critical code

## Project Structure

```
uawk/
├── cmd/uawk/             # CLI application
│   └── main.go           # Manual POSIX argument parsing
├── internal/
│   ├── token/            # Tokens and positions
│   ├── lexer/            # Tokenizer
│   ├── types/            # Value system (tagged union)
│   ├── ast/              # AST nodes
│   ├── parser/           # Recursive descent parser
│   ├── semantic/         # Resolver + Checker
│   ├── compiler/         # AST → Bytecode
│   ├── vm/               # Stack VM + Builtins
│   └── runtime/          # Regex, I/O, fast paths
├── uawk.go               # Public API: Run(), Compile()
├── config.go             # Configuration
├── program.go            # Compiled program
├── errors.go             # Error types
├── scripts/              # Development scripts
├── CHANGELOG.md          # Version history
├── LICENSE               # MIT License
└── README.md             # Main documentation
```

## Adding New Features

1. Check if issue exists, if not create one
2. Discuss approach in the issue
3. Create feature branch from `main`
4. Implement feature with tests
5. Update documentation
6. Run quality checks (`bash scripts/pre-release-check.sh`)
7. Create pull request
8. Wait for code review
9. Address feedback
10. Merge when approved

## Code Style Guidelines

### General Principles

- Follow Go conventions and idioms
- Write self-documenting code
- Add comments for complex logic
- Keep functions small and focused
- Use meaningful variable names
- Optimize for clarity first, performance second (except in hot paths)

### Naming Conventions

- **Public types/functions**: `PascalCase` (e.g., `Compile`, `Value`)
- **Private types/functions**: `camelCase` (e.g., `parseExpr`, `emitOpcode`)
- **Constants**: `PascalCase` (e.g., `KindNum`, `OpAdd`)
- **Test functions**: `Test*` (e.g., `TestLexer`)
- **Benchmark functions**: `Benchmark*` (e.g., `BenchmarkVM`)

### Error Handling

- Always check and handle errors
- Use descriptive error messages with context
- Return errors immediately, don't wrap unnecessarily
- Validate inputs before processing

### Testing

- Use table-driven tests when appropriate
- Test both success and error cases
- Test edge cases (empty input, boundaries)
- Add fuzz tests for parsers
- Compare with gawk for correctness
- **Benchmarks are mandatory** for performance-critical code

## VM/Compiler Implementation

### Adding New Opcodes

1. Add opcode constant in `internal/compiler/opcode.go`
2. Add case in `Opcode.String()` for debugging
3. Add compilation logic in `internal/compiler/compiler.go`
4. Add execution logic in `internal/vm/vm.go`
5. Add tests for both compilation and execution
6. Update disassembler if needed

### Performance Guidelines

- Minimize allocations in hot paths
- Use `peek()` + `replaceTop()` instead of `pop()` + `push()` where possible
- Cache frequently accessed values
- Consider fast paths for common cases
- Profile before optimizing

## Running Benchmarks

```bash
# Build uawk
go build -o ~/go/bin/uawk.exe ./cmd/uawk

# Run awkbench (from separate repo)
cd /path/to/uawk-test && ./bin/awkbench.exe --runs 10

# Run Go benchmarks
go test -bench=. -benchmem ./internal/vm/...
```

## Getting Help

- Check existing issues and discussions
- Read ROADMAP.md for project direction
- Read STATUS.md for current state
- Ask questions in GitHub Issues

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

**Thank you for contributing to uawk!** 🎉
