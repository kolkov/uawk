#!/usr/bin/env bash
# Pre-Release Validation Script for uawk
# This script runs all quality checks before creating a release
# EXACTLY matches CI checks + additional validations
# Based on coregex pre-release-check.sh

set -e  # Exit on first error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Header
echo ""
echo "================================================"
echo "  uawk - Pre-Release Check"
echo "================================================"
echo ""

# Track overall status
ERRORS=0
WARNINGS=0

# 1. Check Go version
log_info "Checking Go version..."
GO_VERSION=$(go version | awk '{print $3}')
REQUIRED_VERSION="go1.25"
if [[ "$GO_VERSION" < "$REQUIRED_VERSION" ]]; then
    log_error "Go version $REQUIRED_VERSION+ required, found $GO_VERSION"
    ERRORS=$((ERRORS + 1))
else
    log_success "Go version: $GO_VERSION"
fi
echo ""

# 2. Check git status
log_info "Checking git status..."
if git diff-index --quiet HEAD --; then
    log_success "Working directory is clean"
else
    log_warning "Uncommitted changes detected"
    git status --short
    WARNINGS=$((WARNINGS + 1))
fi
echo ""

# 3. Code formatting check (EXACT CI command)
# Exclude tmp/ (scratch files), reference/ (external code)
log_info "Checking code formatting (gofmt -l .)..."
UNFORMATTED=$(find . -name "*.go" -not -path "./tmp/*" -not -path "./reference/*" -not -path "./vendor/*" | xargs gofmt -l 2>/dev/null || true)
if [ -n "$UNFORMATTED" ]; then
    log_error "The following files need formatting:"
    echo "$UNFORMATTED"
    echo ""
    log_info "Run: go fmt ./..."
    ERRORS=$((ERRORS + 1))
else
    log_success "All files are properly formatted"
fi
echo ""

# 4. Go vet (exclude tmp/ - scratch files with conflicting packages)
log_info "Running go vet..."
# Explicitly list packages to avoid tmp/ conflict
PACKAGES=". ./cmd/uawk/... ./internal/..."
if go vet $PACKAGES 2>&1; then
    log_success "go vet passed"
else
    log_error "go vet failed"
    ERRORS=$((ERRORS + 1))
fi
echo ""

# 5. Build all packages (exclude tmp/)
log_info "Building all packages..."
if go build $PACKAGES 2>&1; then
    log_success "Build successful"
else
    log_error "Build failed"
    ERRORS=$((ERRORS + 1))
fi
echo ""

# 6. go.mod validation
log_info "Validating go.mod..."
go mod verify
if [ $? -eq 0 ]; then
    log_success "go.mod verified"
else
    log_error "go.mod verification failed"
    ERRORS=$((ERRORS + 1))
fi

# Check if go.mod needs tidying
go mod tidy
if git diff --quiet go.mod go.sum 2>/dev/null; then
    log_success "go.mod is tidy"
else
    log_warning "go.mod needs tidying (run 'go mod tidy')"
    git diff go.mod go.sum 2>/dev/null || true
    WARNINGS=$((WARNINGS + 1))
fi
echo ""

# 6.5. Verify golangci-lint configuration (optional)
log_info "Verifying golangci-lint configuration..."
if command -v golangci-lint &> /dev/null; then
    if [ -f ".golangci.yml" ] || [ -f ".golangci.yaml" ]; then
        if golangci-lint config verify 2>&1; then
            log_success "golangci-lint config is valid"
        else
            log_error "golangci-lint config is invalid"
            ERRORS=$((ERRORS + 1))
        fi
    else
        log_info "No golangci-lint config file (using defaults)"
    fi
else
    log_warning "golangci-lint not installed (optional but recommended)"
    log_info "Install: https://golangci-lint.run/welcome/install/"
    WARNINGS=$((WARNINGS + 1))
fi
echo ""

# 7. Run tests with race detector (supports WSL2 fallback)
USE_WSL=0
WSL_DISTRO=""

# Helper function to find WSL distro with Go installed
find_wsl_distro() {
    if ! command -v wsl &> /dev/null; then
        return 1
    fi

    # Try common distros first
    for distro in "Gentoo" "Ubuntu" "Debian" "Alpine"; do
        if wsl -d "$distro" bash -c "command -v go &> /dev/null" 2>/dev/null; then
            echo "$distro"
            return 0
        fi
    done

    return 1
}

if command -v gcc &> /dev/null || command -v clang &> /dev/null; then
    log_info "Running tests with race detector..."
    RACE_FLAG="-race"
    TEST_CMD="go test -race ./... 2>&1"
else
    # Try to find WSL distro with Go
    WSL_DISTRO=$(find_wsl_distro)
    if [ -n "$WSL_DISTRO" ]; then
        log_info "GCC not found locally, but WSL2 ($WSL_DISTRO) detected!"
        log_info "Running tests with race detector via WSL2 $WSL_DISTRO..."
        USE_WSL=1
        RACE_FLAG="-race"

        # Convert Windows path to WSL path (D:\projects\... -> /mnt/d/projects/...)
        CURRENT_DIR=$(pwd)
        if [[ "$CURRENT_DIR" =~ ^/([a-z])/ ]]; then
            # Already in /d/... format (MSYS), convert to /mnt/d/...
            WSL_PATH="/mnt${CURRENT_DIR}"
        else
            # Windows format D:\... convert to /mnt/d/...
            DRIVE_LETTER=$(echo "$CURRENT_DIR" | cut -d: -f1 | tr '[:upper:]' '[:lower:]')
            PATH_WITHOUT_DRIVE=${CURRENT_DIR#*:}
            WSL_PATH="/mnt/$DRIVE_LETTER${PATH_WITHOUT_DRIVE//\\//}"
        fi

        TEST_CMD="wsl -d \"$WSL_DISTRO\" bash -c \"cd \\\"$WSL_PATH\\\" && go test -race -ldflags '-linkmode=external' ./... 2>&1\""
    else
        log_warning "GCC not found, running tests WITHOUT race detector"
        log_info "Install GCC (mingw-w64) or setup WSL2 with Go for race detection"
        log_info "  Windows: https://www.mingw-w64.org/"
        log_info "  WSL2: https://docs.microsoft.com/en-us/windows/wsl/install"
        WARNINGS=$((WARNINGS + 1))
        RACE_FLAG=""
        TEST_CMD="go test ./... 2>&1"
    fi
fi

log_info "Running tests..."
# Explicitly list test packages to avoid tmp/ conflict
TEST_PACKAGES=". ./cmd/uawk/... ./internal/..."
if [ $USE_WSL -eq 1 ]; then
    # WSL2: Use timeout (3 min) and unbuffered output with external linkmode
    TEST_OUTPUT=$(wsl -d "$WSL_DISTRO" bash -c "cd $WSL_PATH && timeout 180 stdbuf -oL -eL go test -race -ldflags '-linkmode=external' $TEST_PACKAGES 2>&1" || true)
    if [ -z "$TEST_OUTPUT" ]; then
        log_error "WSL2 tests timed out or failed to run"
        ERRORS=$((ERRORS + 1))
    fi
else
    TEST_OUTPUT=$(eval "$TEST_CMD")
fi

# Check if race detector failed to build
if echo "$TEST_OUTPUT" | grep -q "hole in findfunctab\|build failed.*race"; then
    log_warning "Race detector build failed"
    log_info "Falling back to tests without race detector..."

    if [ $USE_WSL -eq 1 ]; then
        TEST_OUTPUT=$(wsl -d "$WSL_DISTRO" bash -c "cd \"$WSL_PATH\" && go test $TEST_PACKAGES 2>&1")
    else
        TEST_OUTPUT=$(go test $TEST_PACKAGES 2>&1)
    fi

    RACE_FLAG=""
    WARNINGS=$((WARNINGS + 1))
fi

if echo "$TEST_OUTPUT" | grep -q "FAIL"; then
    log_error "Tests failed or race conditions detected"
    echo "$TEST_OUTPUT"
    echo ""
    ERRORS=$((ERRORS + 1))
elif echo "$TEST_OUTPUT" | grep -q "PASS\|ok"; then
    if [ $USE_WSL -eq 1 ] && [ -n "$RACE_FLAG" ]; then
        log_success "All tests passed with race detector (via WSL2 $WSL_DISTRO)"
    elif [ -n "$RACE_FLAG" ]; then
        log_success "All tests passed with race detector (0 races)"
    else
        log_success "All tests passed (race detector not available)"
    fi
else
    log_error "Unexpected test output"
    echo "$TEST_OUTPUT"
    ERRORS=$((ERRORS + 1))
fi
echo ""

# 8. Test coverage check
log_info "Checking test coverage..."
COVERAGE=$(go test -cover $TEST_PACKAGES 2>&1 | grep "coverage:" | grep -v "\[no statements\]" | head -1 | awk '{print $5}' | sed 's/%//')
if [ -n "$COVERAGE" ]; then
    echo "  - overall coverage: ${COVERAGE}%"
    if awk -v cov="$COVERAGE" 'BEGIN {exit !(cov >= 70.0)}'; then
        log_success "Coverage meets requirement (>70%)"
    else
        log_warning "Coverage below 70% (${COVERAGE}%) - acceptable for early versions"
        WARNINGS=$((WARNINGS + 1))
    fi
else
    log_warning "Could not determine coverage"
    WARNINGS=$((WARNINGS + 1))
fi
echo ""

# 9. Dependency check
log_info "Checking dependencies..."
COREGEX_VERSION=$(grep "github.com/coregx/coregex" go.mod | awk '{print $2}')
if [ -n "$COREGEX_VERSION" ]; then
    log_success "coregex dependency: $COREGEX_VERSION"
else
    log_error "coregex not found in go.mod (required for regex)"
    ERRORS=$((ERRORS + 1))
fi

# Check total dependency count (should be minimal)
DEP_COUNT=$(grep -c "^\s*github.com/" go.mod 2>/dev/null) || DEP_COUNT=0
if [ "$DEP_COUNT" -le 2 ]; then
    log_success "Minimal dependencies: $DEP_COUNT external packages"
else
    log_warning "Found $DEP_COUNT external dependencies - consider reducing"
    grep "^\s*github.com/" go.mod || true
    WARNINGS=$((WARNINGS + 1))
fi
echo ""

# 10. golangci-lint (optional, same as CI)
log_info "Running golangci-lint..."
if command -v golangci-lint &> /dev/null; then
    if command -v timeout &> /dev/null; then
        LINT_OUTPUT=$(timeout 180 golangci-lint run --timeout=2m $PACKAGES 2>&1) || true
    else
        LINT_OUTPUT=$(golangci-lint run --timeout=2m $PACKAGES 2>&1) || true
    fi
    if [ -z "$LINT_OUTPUT" ] || echo "$LINT_OUTPUT" | grep -q "congratulations"; then
        log_success "golangci-lint passed"
    elif echo "$LINT_OUTPUT" | grep -q "0 issues"; then
        log_success "golangci-lint passed with 0 issues"
    else
        log_warning "Linter found issues"
        echo "$LINT_OUTPUT" | grep -v "^level=error.*tmp" | tail -10
        WARNINGS=$((WARNINGS + 1))
    fi
else
    log_warning "golangci-lint not installed (optional)"
    log_info "Install: https://golangci-lint.run/welcome/install/"
    WARNINGS=$((WARNINGS + 1))
fi
echo ""

# 11. Check for TODO/FIXME comments (informational)
log_info "Checking for TODO/FIXME comments..."
TODO_COUNT=$(grep -r "TODO\|FIXME" --include="*.go" --exclude-dir=vendor --exclude-dir=tmp --exclude-dir=reference . 2>/dev/null | wc -l)
TODO_COUNT=${TODO_COUNT:-0}
if [ "$TODO_COUNT" -gt 0 ]; then
    log_info "Found $TODO_COUNT TODO/FIXME comments (informational)"
    grep -r "TODO\|FIXME" --include="*.go" --exclude-dir=vendor --exclude-dir=tmp --exclude-dir=reference . 2>/dev/null | head -5
else
    log_success "No TODO/FIXME comments found"
fi
echo ""

# 12. Check critical documentation files
log_info "Checking documentation..."
DOCS_MISSING=0
REQUIRED_DOCS="README.md LICENSE"

for doc in $REQUIRED_DOCS; do
    if [ ! -f "$doc" ]; then
        log_error "Missing: $doc"
        DOCS_MISSING=1
        ERRORS=$((ERRORS + 1))
    fi
done

# CHANGELOG is optional for now
if [ ! -f "CHANGELOG.md" ]; then
    log_warning "CHANGELOG.md not found (recommended for releases)"
    WARNINGS=$((WARNINGS + 1))
fi

if [ $DOCS_MISSING -eq 0 ]; then
    log_success "All critical documentation files present"
fi
echo ""

# 13. Build CLI binary and check size
log_info "Building CLI binary..."
mkdir -p tmp
go build -o tmp/uawk.exe ./cmd/uawk 2>&1
if [ -f "tmp/uawk.exe" ]; then
    SIZE=$(ls -la tmp/uawk.exe | awk '{print $5}')
    SIZE_MB=$(awk "BEGIN {printf \"%.2f\", $SIZE / 1048576}")
    log_success "CLI binary built: ${SIZE_MB}MB"

    # Check if size is reasonable (< 10MB)
    if awk -v size="$SIZE_MB" 'BEGIN {exit !(size < 10.0)}'; then
        log_success "Binary size is reasonable (<10MB)"
    else
        log_warning "Binary size is large (${SIZE_MB}MB) - consider optimization"
        WARNINGS=$((WARNINGS + 1))
    fi
    rm -f tmp/uawk.exe
else
    log_error "Failed to build CLI binary"
    ERRORS=$((ERRORS + 1))
fi
echo ""

# Summary
echo "========================================"
echo "  Summary"
echo "========================================"
echo ""

if [ $ERRORS -eq 0 ] && [ $WARNINGS -eq 0 ]; then
    log_success "All checks passed! Ready for release."
    echo ""
    exit 0
elif [ $ERRORS -eq 0 ]; then
    log_warning "Checks completed with $WARNINGS warning(s)"
    echo ""
    log_info "Review warnings above before proceeding with release"
    echo ""
    exit 0
else
    log_error "Checks failed with $ERRORS error(s) and $WARNINGS warning(s)"
    echo ""
    log_error "Fix errors before creating release"
    echo ""
    exit 1
fi
