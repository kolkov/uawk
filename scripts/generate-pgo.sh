#!/bin/bash
# generate-pgo.sh - Generate PGO profile for uawk
#
# This script runs representative AWK workloads with CPU profiling
# and merges the profiles into default.pgo for optimized builds.
#
# Usage: ./scripts/generate-pgo.sh
#
# Requirements:
# - Go 1.22+ (for PGO support)
# - uawk-test repository at ../uawk-test

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
UAWK_TEST_DIR="${PROJECT_DIR}/../uawk-test"
TMP_DIR="${PROJECT_DIR}/tmp/pgo"
PROFILE_DIR="${PROJECT_DIR}/cmd/uawk"

echo "=== uawk PGO Profile Generator ==="
echo "Project: ${PROJECT_DIR}"
echo "Test dir: ${UAWK_TEST_DIR}"
echo ""

# Check uawk-test exists
if [ ! -d "$UAWK_TEST_DIR" ]; then
    echo "ERROR: uawk-test not found at ${UAWK_TEST_DIR}"
    echo "Clone it: git clone https://github.com/kolkov/uawk-test ../uawk-test"
    exit 1
fi

# Create temp directory
mkdir -p "$TMP_DIR"
rm -f "$TMP_DIR"/*.pprof

# Build uawk without PGO first (for profiling)
echo "=== Building uawk (no PGO) ==="
cd "$PROJECT_DIR"
go build -o "$TMP_DIR/uawk-nopgo" ./cmd/uawk
echo "Built: $TMP_DIR/uawk-nopgo"

# Generate test data if needed
echo ""
echo "=== Generating test data (10MB) ==="
cd "$UAWK_TEST_DIR"
if [ ! -f "testdata/numeric_10MB.txt" ]; then
    go run ./cmd/awkbench --generate --size 10MB --data testdata
fi

# Run profiled workloads
echo ""
echo "=== Running profiled workloads ==="

# We'll use Go's built-in benchmarks for profiling
cd "$PROJECT_DIR"

# Profile 1: VM operations (sum, count, filter)
echo "Profile 1: VM operations..."
go test -cpuprofile="$TMP_DIR/vm.pprof" -bench=BenchmarkRun -benchtime=3s ./...  2>/dev/null || true

# Profile 2: Regex operations
echo "Profile 2: Regex operations..."
go test -cpuprofile="$TMP_DIR/regex.pprof" -bench=BenchmarkRegex -benchtime=3s ./internal/runtime/... 2>/dev/null || true

# Profile 3: Lexer operations
echo "Profile 3: Lexer operations..."
go test -cpuprofile="$TMP_DIR/lexer.pprof" -bench=BenchmarkLexer -benchtime=3s ./internal/lexer/... 2>/dev/null || true

# Profile 4: Parser operations
echo "Profile 4: Parser operations..."
go test -cpuprofile="$TMP_DIR/parser.pprof" -bench=BenchmarkParser -benchtime=3s ./internal/parser/... 2>/dev/null || true

# Profile 5: Real AWK workloads via CLI
echo "Profile 5: Real AWK workloads..."
if [ -f "$UAWK_TEST_DIR/testdata/numeric_10MB.txt" ]; then
    # Create a Go program that profiles real workloads
    cat > "$TMP_DIR/profile_workloads.go" << 'GOEOF'
package main

import (
    "os"
    "runtime/pprof"
    "strings"
    "github.com/kolkov/uawk"
)

func main() {
    f, _ := os.Create(os.Args[1])
    pprof.StartCPUProfile(f)
    defer pprof.StopCPUProfile()

    data, _ := os.ReadFile(os.Args[2])
    input := string(data)

    // Run representative workloads
    programs := []string{
        `{ sum += $1 } END { print sum }`,
        `$1 > 500000 { count++ } END { print count }`,
        `{ print $1, $2 }`,
        `/[0-9]+/ { matches++ } END { print matches }`,
        `{ arr[$1]++ } END { for (k in arr) print k, arr[k] }`,
    }

    for i := 0; i < 5; i++ {
        for _, prog := range programs {
            uawk.Run(prog, strings.NewReader(input), nil)
        }
    }
}
GOEOF

    cd "$TMP_DIR"
    go run profile_workloads.go "$TMP_DIR/workloads.pprof" "$UAWK_TEST_DIR/testdata/numeric_10MB.txt" 2>/dev/null || echo "Workload profiling skipped"
    cd "$PROJECT_DIR"
fi

# Merge profiles
echo ""
echo "=== Merging profiles ==="
PROFILES=""
for f in "$TMP_DIR"/*.pprof; do
    if [ -f "$f" ] && [ -s "$f" ]; then
        PROFILES="$PROFILES $f"
        echo "  Found: $(basename $f)"
    fi
done

if [ -z "$PROFILES" ]; then
    echo "ERROR: No profiles generated!"
    exit 1
fi

# Use go tool pprof to merge (Go 1.22+)
echo "Merging into default.pgo..."
go tool pprof -proto $PROFILES > "$PROFILE_DIR/default.pgo" 2>/dev/null || {
    # Fallback: just use the largest profile
    echo "Merge failed, using largest profile..."
    ls -S "$TMP_DIR"/*.pprof | head -1 | xargs cp -t "$PROFILE_DIR/"
    mv "$PROFILE_DIR"/*.pprof "$PROFILE_DIR/default.pgo" 2>/dev/null || true
}

# Verify profile
if [ -f "$PROFILE_DIR/default.pgo" ]; then
    SIZE=$(ls -lh "$PROFILE_DIR/default.pgo" | awk '{print $5}')
    echo ""
    echo "=== PGO Profile Generated ==="
    echo "Location: $PROFILE_DIR/default.pgo"
    echo "Size: $SIZE"
    echo ""
    echo "To build with PGO:"
    echo "  go build ./cmd/uawk"
    echo ""
    echo "Go will automatically use default.pgo if present."
else
    echo "ERROR: Failed to generate profile!"
    exit 1
fi

# Cleanup
rm -rf "$TMP_DIR"

echo "Done!"
