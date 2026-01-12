package runtime

import (
	"strings"
	"testing"
)

// CharClass Benchmark Suite
//
// This file compares different approaches to character class matching:
//
// 1. CharClassSearcher (current implementation)
//    - Uses [256]bool lookup table (256 bytes)
//    - Simple loop, Go compiler optimizes well
//    - Best for typical AWK workloads (short strings, early matches)
//
// 2. FastCharClassSearcher (SIMD-like approach)
//    - Uses [4]uint64 bitmap (32 bytes, 8x smaller)
//    - Specialized range checks for common patterns
//    - Interface dispatch adds overhead
//
// Findings (Intel Core i7-1255U, Go 1.22):
//
// | Pattern        | String Length | CharClassSearcher | FastCharClassSearcher |
// |----------------|---------------|-------------------|----------------------|
// | \d+ (digit)    | 13 (short)    | 3.7 ns            | 14 ns (4x slower)    |
// | \d+ (digit)    | 57 (medium)   | 4.2 ns            | 23 ns (5x slower)    |
// | \d+ (digit)    | 2600 (long)   | 760 ns            | 4967 ns (6.5x slower)|
// | \w+ (word)     | 13 (short)    | 1.2 ns            | 4.6 ns (4x slower)   |
// | [a-zA-Z]+      | 13 (short)    | 1.2 ns            | 4.0 ns (3x slower)   |
//
// Raw lookup comparison (62 chars):
// | Approach       | Time      | Notes                              |
// |----------------|-----------|-----------------------------------|
// | [256]bool      | 28 ns     | Simple array access               |
// | [4]uint64      | 20 ns     | Bitmap with bit operations (27% faster) |
// | Range check    | 27 ns     | c >= '0' && c <= '9'              |
//
// Conclusion:
// - [256]bool table is optimal for CharClassSearcher due to Go's array optimization
// - Bitmap lookup is 27% faster per-character but interface dispatch negates gains
// - FastCharClassSearcher could benefit composite patterns where multiple classes
//   are checked (better cache locality with 32-byte bitmaps vs 256-byte tables)
// - SWAR functions useful for future parallel processing or specialized patterns
//
// Future optimizations should focus on:
// - Composite pattern matching (multiple character classes in sequence)
// - Parallel pattern evaluation
// - Memory-constrained scenarios where 32-byte bitmap matters

// Benchmark data of varying sizes
var (
	// Short string (typical AWK field)
	shortStr = "hello123world"

	// Medium string (typical AWK line)
	mediumStr = "The year 2024 is here with 365 days and 8760 hours in it"

	// Long string (larger input)
	longStr = strings.Repeat("abcdefghijklmnopqrstuvwxyz", 100) + "123" + strings.Repeat("ABCDEFGHIJKLMNOPQRSTUVWXYZ", 100)

	// Very long string (stress test)
	veryLongStr = strings.Repeat("The quick brown fox jumps over the lazy dog. ", 1000) + "12345"
)

// ============================================================================
// Benchmark: MatchString - CharClassSearcher vs FastCharClassSearcher
// ============================================================================

// --- Digit pattern ---

func BenchmarkDigitMatch_Old_Short(b *testing.B) {
	s := &CharClassSearcher{membership: digitTable, minMatch: 1}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(shortStr)
	}
}

func BenchmarkDigitMatch_New_Short(b *testing.B) {
	s := NewFastDigitSearcher()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(shortStr)
	}
}

func BenchmarkDigitMatch_Old_Medium(b *testing.B) {
	s := &CharClassSearcher{membership: digitTable, minMatch: 1}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(mediumStr)
	}
}

func BenchmarkDigitMatch_New_Medium(b *testing.B) {
	s := NewFastDigitSearcher()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(mediumStr)
	}
}

func BenchmarkDigitMatch_Old_Long(b *testing.B) {
	s := &CharClassSearcher{membership: digitTable, minMatch: 1}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(longStr)
	}
}

func BenchmarkDigitMatch_New_Long(b *testing.B) {
	s := NewFastDigitSearcher()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(longStr)
	}
}

func BenchmarkDigitMatch_Old_VeryLong(b *testing.B) {
	s := &CharClassSearcher{membership: digitTable, minMatch: 1}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(veryLongStr)
	}
}

func BenchmarkDigitMatch_New_VeryLong(b *testing.B) {
	s := NewFastDigitSearcher()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(veryLongStr)
	}
}

// --- Word pattern ---

func BenchmarkWordMatch_Old_Short(b *testing.B) {
	s := &CharClassSearcher{membership: wordTable, minMatch: 1}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(shortStr)
	}
}

func BenchmarkWordMatch_New_Short(b *testing.B) {
	s := NewFastWordSearcher()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(shortStr)
	}
}

func BenchmarkWordMatch_Old_Long(b *testing.B) {
	s := &CharClassSearcher{membership: wordTable, minMatch: 1}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(longStr)
	}
}

func BenchmarkWordMatch_New_Long(b *testing.B) {
	s := NewFastWordSearcher()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(longStr)
	}
}

// --- Alpha pattern ---

func BenchmarkAlphaMatch_Old_Short(b *testing.B) {
	s := &CharClassSearcher{membership: alphaTable, minMatch: 1}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(shortStr)
	}
}

func BenchmarkAlphaMatch_New_Short(b *testing.B) {
	s := NewFastAlphaSearcher()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(shortStr)
	}
}

func BenchmarkAlphaMatch_Old_Long(b *testing.B) {
	s := &CharClassSearcher{membership: alphaTable, minMatch: 1}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(longStr)
	}
}

func BenchmarkAlphaMatch_New_Long(b *testing.B) {
	s := NewFastAlphaSearcher()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(longStr)
	}
}

// ============================================================================
// Benchmark: FindStringIndex - Old vs New implementations
// ============================================================================

func BenchmarkDigitFind_Old_Short(b *testing.B) {
	s := &CharClassSearcher{membership: digitTable, minMatch: 1}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.FindStringIndex(shortStr)
	}
}

func BenchmarkDigitFind_New_Short(b *testing.B) {
	s := NewFastDigitSearcher()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.FindStringIndex(shortStr)
	}
}

func BenchmarkDigitFind_Old_Long(b *testing.B) {
	s := &CharClassSearcher{membership: digitTable, minMatch: 1}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.FindStringIndex(longStr)
	}
}

func BenchmarkDigitFind_New_Long(b *testing.B) {
	s := NewFastDigitSearcher()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.FindStringIndex(longStr)
	}
}

func BenchmarkDigitFind_Old_VeryLong(b *testing.B) {
	s := &CharClassSearcher{membership: digitTable, minMatch: 1}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.FindStringIndex(veryLongStr)
	}
}

func BenchmarkDigitFind_New_VeryLong(b *testing.B) {
	s := NewFastDigitSearcher()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.FindStringIndex(veryLongStr)
	}
}

// ============================================================================
// Benchmark: Anchored patterns
// ============================================================================

func BenchmarkDigitAnchored_Old(b *testing.B) {
	s := &CharClassSearcher{membership: digitTable, minMatch: 1, anchored: true}
	input := "123abc456def"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(input)
	}
}

func BenchmarkDigitAnchored_New(b *testing.B) {
	s := &FastCharClassSearcher{
		bitmap:     &digitBitmap,
		rangeCheck: digitMatcher{},
		minMatch:   1,
		anchored:   true,
	}
	input := "123abc456def"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(input)
	}
}

func BenchmarkDigitAnchorEnd_Old(b *testing.B) {
	s := &CharClassSearcher{membership: digitTable, minMatch: 1, anchorEnd: true}
	input := "abc123"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(input)
	}
}

func BenchmarkDigitAnchorEnd_New(b *testing.B) {
	s := &FastCharClassSearcher{
		bitmap:     &digitBitmap,
		rangeCheck: digitMatcher{},
		minMatch:   1,
		anchorEnd:  true,
	}
	input := "abc123"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(input)
	}
}

// ============================================================================
// Benchmark: Bitmap vs Bool table lookup
// ============================================================================

func BenchmarkBoolTableLookup(b *testing.B) {
	input := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b.ResetTimer()
	count := 0
	for i := 0; i < b.N; i++ {
		for j := 0; j < len(input); j++ {
			if digitTable[input[j]] {
				count++
			}
		}
	}
	_ = count
}

func BenchmarkBitmapLookup(b *testing.B) {
	input := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b.ResetTimer()
	count := 0
	for i := 0; i < b.N; i++ {
		for j := 0; j < len(input); j++ {
			if digitBitmap.Contains(input[j]) {
				count++
			}
		}
	}
	_ = count
}

// ============================================================================
// Benchmark: Range check vs Bitmap for specific patterns
// ============================================================================

func BenchmarkDigitRangeCheck(b *testing.B) {
	input := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	matcher := digitMatcher{}
	b.ResetTimer()
	count := 0
	for i := 0; i < b.N; i++ {
		for j := 0; j < len(input); j++ {
			if matcher.Match(input[j]) {
				count++
			}
		}
	}
	_ = count
}

func BenchmarkAlphaRangeCheck(b *testing.B) {
	input := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	matcher := alphaMatcher{}
	b.ResetTimer()
	count := 0
	for i := 0; i < b.N; i++ {
		for j := 0; j < len(input); j++ {
			if matcher.Match(input[j]) {
				count++
			}
		}
	}
	_ = count
}

func BenchmarkWordRangeCheck(b *testing.B) {
	input := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_!@#"
	matcher := wordMatcher{}
	b.ResetTimer()
	count := 0
	for i := 0; i < b.N; i++ {
		for j := 0; j < len(input); j++ {
			if matcher.Match(input[j]) {
				count++
			}
		}
	}
	_ = count
}

// ============================================================================
// Benchmark: SWAR operations
// ============================================================================

func BenchmarkSWARContainsDigit(b *testing.B) {
	// 8 bytes representing "abc12def"
	word := uint64('a') |
		uint64('b')<<8 |
		uint64('c')<<16 |
		uint64('1')<<24 |
		uint64('2')<<32 |
		uint64('d')<<40 |
		uint64('e')<<48 |
		uint64('f')<<56
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SWARContainsDigit(word)
	}
}

func BenchmarkSWARContainsSpace(b *testing.B) {
	// 8 bytes representing "abc def "
	word := uint64('a') |
		uint64('b')<<8 |
		uint64('c')<<16 |
		uint64(' ')<<24 |
		uint64('d')<<32 |
		uint64('e')<<40 |
		uint64('f')<<48 |
		uint64(' ')<<56
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SWARContainsSpace(word)
	}
}

// ============================================================================
// Benchmark: No match scenarios (worst case)
// ============================================================================

func BenchmarkDigitNoMatch_Old(b *testing.B) {
	s := &CharClassSearcher{membership: digitTable, minMatch: 1}
	input := strings.Repeat("abcdefghijklmnopqrstuvwxyz", 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(input)
	}
}

func BenchmarkDigitNoMatch_New(b *testing.B) {
	s := NewFastDigitSearcher()
	input := strings.Repeat("abcdefghijklmnopqrstuvwxyz", 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(input)
	}
}

// ============================================================================
// Benchmark: Early match scenarios (best case)
// ============================================================================

func BenchmarkDigitEarlyMatch_Old(b *testing.B) {
	s := &CharClassSearcher{membership: digitTable, minMatch: 1}
	input := "1" + strings.Repeat("abcdefghijklmnopqrstuvwxyz", 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(input)
	}
}

func BenchmarkDigitEarlyMatch_New(b *testing.B) {
	s := NewFastDigitSearcher()
	input := "1" + strings.Repeat("abcdefghijklmnopqrstuvwxyz", 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.MatchString(input)
	}
}
