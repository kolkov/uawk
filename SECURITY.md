# Security Policy

## Supported Versions

uawk is currently in experimental release (v0.x.x). We provide security updates for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |
| < 0.1.0 | :x:                |

Future stable releases (v1.0+) will follow semantic versioning with LTS support.

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability in uawk, please report it responsibly.

### How to Report

**DO NOT** open a public GitHub issue for security vulnerabilities.

Instead, please report security issues by:

1. **Private Security Advisory** (preferred):
   https://github.com/kolkov/uawk/security/advisories/new

2. **Email** to maintainers:
   Create a private GitHub issue or contact via discussions

### What to Include

Please include the following information in your report:

- **Description** of the vulnerability
- **Steps to reproduce** the issue (include malicious AWK program or regex pattern if applicable)
- **Affected versions** (which versions are impacted)
- **Potential impact** (DoS, memory exhaustion, unexpected behavior, etc.)
- **Suggested fix** (if you have one)
- **Your contact information** (for follow-up questions)

### Response Timeline

- **Initial Response**: Within 48-72 hours
- **Triage & Assessment**: Within 1 week
- **Fix & Disclosure**: Coordinated with reporter

We aim to:
1. Acknowledge receipt within 72 hours
2. Provide an initial assessment within 1 week
3. Work with you on a coordinated disclosure timeline
4. Credit you in the security advisory (unless you prefer to remain anonymous)

## Security Considerations for AWK Interpreter

uawk is an AWK interpreter that executes untrusted AWK programs. This introduces security risks that users should be aware of.

### 1. Malicious AWK Programs

**Risk**: Crafted AWK programs can cause excessive CPU usage or memory exhaustion.

**Attack Vectors**:
- **Infinite loops**: `BEGIN { while(1) {} }`
- **Memory exhaustion**: Building huge arrays or strings
- **System command injection**: `system()` and pipe redirections
- **File operations**: Reading/writing arbitrary files

**Mitigation in Library**:
- ✅ **Bounded bytecode execution** - Stack-based VM with predictable behavior
- ✅ **No eval()** - AWK programs cannot generate and execute code
- ⚠️ **system() enabled by default** - Can be disabled at application level

**User Recommendations**:
```go
// ❌ BAD - Running untrusted AWK on sensitive systems
output, _ := uawk.Run(userInput, data, nil)

// ✅ GOOD - Validate program before execution
if containsDangerousOps(userInput) { // system(), pipes, file writes
    return errors.New("program contains dangerous operations")
}

// ✅ GOOD - Use timeout for execution (application-level)
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

done := make(chan string)
go func() {
    output, _ := uawk.Run(program, input, nil)
    done <- output
}()

select {
case result := <-done:
    // Execution completed
case <-ctx.Done():
    // Timeout - potential infinite loop
    return errors.New("execution timeout")
}
```

### 2. Regex-Based Attacks (ReDoS)

**Risk**: Crafted regex patterns in AWK programs can cause catastrophic backtracking.

**Example Attack**:
```awk
# Potential ReDoS pattern
/^(a+)+$/ { print }
```

**Mitigation**:
- ✅ **coregex engine** - Uses Thompson's NFA, immune to catastrophic backtracking
- ✅ **O(n) matching** - DFA-based search with O(n) time complexity
- ✅ **CharClass fast paths** - Common patterns bypass regex engine entirely
- ✅ **Lazy DFA** - Graceful degradation for complex patterns

uawk uses coregex which is immune to ReDoS attacks by design.

### 3. File and Pipe Operations

**Risk**: AWK programs can read/write files and execute shell commands.

**Attack Vectors**:
```awk
# Write arbitrary files
{ print > "/etc/passwd" }

# Execute shell commands
{ system("rm -rf /") }

# Read sensitive files
{ while ((getline line < "/etc/shadow") > 0) print line }
```

**Mitigation (Application Level)**:
```go
// ✅ GOOD - Sandbox AWK execution
// Option 1: Run in restricted container/namespace
// Option 2: Use seccomp/AppArmor profiles
// Option 3: Validate program AST before execution

// Example: Check for dangerous operations
func isSafeProgram(program string) bool {
    dangerous := []string{
        "system(", "getline", "|", ">", ">>", "<",
    }
    for _, op := range dangerous {
        if strings.Contains(program, op) {
            return false
        }
    }
    return true
}
```

### 4. Memory Exhaustion

**Risk**: AWK programs can allocate unbounded memory.

**Attack Vectors**:
```awk
# Huge array
BEGIN { for (i=1; i<=1000000000; i++) arr[i] = i }

# String concatenation bomb
BEGIN { s = "a"; for (i=1; i<=30; i++) s = s s; print length(s) }

# Infinite field splitting
BEGIN { FS = "" }  # Every character becomes a field
{ for (i=1; i<=NF; i++) arr[i] = $i }
```

**Mitigation**:
- ✅ **Go runtime limits** - GOMEMLIMIT environment variable
- ✅ **Container limits** - Use cgroups memory limits
- 🔄 **VM limits** - Planned for v0.2.0 (max array size, max string length)

**User Best Practices**:
```go
// Set memory limit for Go runtime
os.Setenv("GOMEMLIMIT", "100MiB")

// Or run in container with memory limits
// docker run --memory=100m uawk-container
```

### 5. Input Size Attacks

**Risk**: Large input files can cause memory exhaustion.

**Mitigation**:
```go
// ✅ GOOD - Limit input size
const maxInputSize = 10 * 1024 * 1024 // 10MB

info, _ := os.Stat(inputFile)
if info.Size() > maxInputSize {
    return errors.New("input file too large")
}
```

## Security Best Practices for Users

### Running Untrusted AWK Programs

1. **Never run untrusted AWK on production systems** without sandboxing
2. **Use containers** with resource limits (CPU, memory, disk)
3. **Disable dangerous features** at application level (system(), pipes, file writes)
4. **Set execution timeouts** to prevent infinite loops
5. **Validate program structure** before execution

### Embedding uawk in Applications

```go
// Example: Safe AWK execution for web applications
func safeExec(program, input string) (string, error) {
    // 1. Validate program (no dangerous operations)
    if !isSafeProgram(program) {
        return "", errors.New("program contains dangerous operations")
    }

    // 2. Limit input size
    if len(input) > maxInputSize {
        return "", errors.New("input too large")
    }

    // 3. Execute with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    done := make(chan string)
    errChan := make(chan error)

    go func() {
        output, err := uawk.Run(program, strings.NewReader(input), nil)
        if err != nil {
            errChan <- err
            return
        }
        done <- output
    }()

    select {
    case output := <-done:
        return output, nil
    case err := <-errChan:
        return "", err
    case <-ctx.Done():
        return "", errors.New("execution timeout")
    }
}
```

## Dependency Security

uawk has minimal dependencies:

| Dependency | Purpose | Risk |
|------------|---------|------|
| `github.com/coregx/coregex` | Regex engine | Low (no network, no syscalls) |

**Monitoring**:
- ✅ Minimal dependency surface (only 1 dependency)
- ✅ coregex is a pure Go library with no transitive dependencies
- 🔄 Dependabot enabled (planned when public)

## Security Testing

### Current Testing

- ✅ Unit tests with edge cases
- ✅ Fuzz tests for lexer and parser
- ✅ Comparison tests vs gawk (correctness)
- ✅ Race detector (0 data races)
- ✅ golangci-lint with 15+ linters (0 issues)

### Planned for v1.0

- 🔄 Security-focused fuzzing (malicious programs)
- 🔄 Resource exhaustion tests
- 🔄 Static analysis with gosec
- 🔄 SAST scanning in CI

## Security Disclosure History

### v0.1.0 (2025-01-04)

**Initial release** - No security issues reported yet.

uawk v0.1.0 is a new project with production-quality code but experimental API stability.

**Recommendation**: Use with caution in production. API may change in v0.2+.

## Security Contact

- **GitHub Security Advisory**: https://github.com/kolkov/uawk/security/advisories/new
- **Public Issues** (for non-sensitive bugs): https://github.com/kolkov/uawk/issues
- **Discussions**: https://github.com/kolkov/uawk/discussions

## Bug Bounty Program

uawk does not currently have a bug bounty program. We rely on responsible disclosure from the security community.

If you report a valid security vulnerability:
- ✅ Public credit in security advisory (if desired)
- ✅ Acknowledgment in CHANGELOG
- ✅ Our gratitude and recognition in README
- ✅ Priority review and quick fix

---

**Thank you for helping keep uawk secure!** 🔒

*Security is a journey, not a destination. We continuously improve our security posture with each release.*
