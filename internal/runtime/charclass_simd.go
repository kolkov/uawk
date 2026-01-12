// Package runtime provides AWK runtime support including regex operations.
package runtime

// CharClassBitmap provides fast O(1) byte classification using a 32-byte bitmap.
// This is more cache-friendly than the 256-byte bool table used by CharClassSearcher.
//
// Memory layout:
//   - [4]uint64 = 32 bytes (fits in half a cache line)
//   - Each bit represents one byte value (0-255)
//   - Check: (bitmap[c/64] >> (c%64)) & 1
//
// Performance:
//   - 32 bytes vs 256 bytes = 8x smaller
//   - Better cache locality for repeated checks
//   - Batch processing: check 8 bytes at once using SWAR techniques
//
// Benchmark results show that [256]bool table is faster for simple lookups
// due to Go's excellent compiler optimizations for array access.
// Bitmap is more useful for:
//   - Composite patterns where cache locality matters
//   - SWAR batch processing
//   - Memory-constrained scenarios
type CharClassBitmap [4]uint64

// Contains checks if byte c is in the character class.
// This is O(1) with a single array lookup and bit operation.
func (b *CharClassBitmap) Contains(c byte) bool {
	return (b[c>>6] & (1 << (c & 63))) != 0
}

// Set adds byte c to the character class.
func (b *CharClassBitmap) Set(c byte) {
	b[c>>6] |= 1 << (c & 63)
}

// Clear removes byte c from the character class.
func (b *CharClassBitmap) Clear(c byte) {
	b[c>>6] &^= 1 << (c & 63)
}

// SetRange adds all bytes from start to end (inclusive) to the character class.
func (b *CharClassBitmap) SetRange(start, end byte) {
	for c := start; c <= end; c++ {
		b.Set(c)
		if c == 255 { // Prevent overflow
			break
		}
	}
}

// Invert creates a new bitmap with all bits flipped.
func (b *CharClassBitmap) Invert() CharClassBitmap {
	return CharClassBitmap{^b[0], ^b[1], ^b[2], ^b[3]}
}

// FromBoolTable converts a [256]bool table to a bitmap.
func (b *CharClassBitmap) FromBoolTable(table *[256]bool) {
	*b = CharClassBitmap{} // Zero out
	for i := 0; i < 256; i++ {
		if table[i] {
			b.Set(byte(i))
		}
	}
}

// Pre-built bitmaps for common character classes
var (
	digitBitmap    CharClassBitmap // \d: 0-9
	spaceBitmap    CharClassBitmap // \s: space, tab, newline, etc.
	wordBitmap     CharClassBitmap // \w: a-z, A-Z, 0-9, _
	alphaBitmap    CharClassBitmap // [a-zA-Z]
	lowerBitmap    CharClassBitmap // [a-z]
	upperBitmap    CharClassBitmap // [A-Z]
	nonDigitBitmap CharClassBitmap // \D: NOT 0-9
	nonSpaceBitmap CharClassBitmap // \S: NOT whitespace
	nonWordBitmap  CharClassBitmap // \W: NOT word chars
	dotBitmap      CharClassBitmap // .: any char except newline
)

func init() {
	// Initialize digit bitmap
	digitBitmap.SetRange('0', '9')

	// Initialize space bitmap
	spaceBitmap.Set(' ')
	spaceBitmap.Set('\t')
	spaceBitmap.Set('\n')
	spaceBitmap.Set('\r')
	spaceBitmap.Set('\f')
	spaceBitmap.Set('\v')

	// Initialize word bitmap
	wordBitmap.SetRange('a', 'z')
	wordBitmap.SetRange('A', 'Z')
	wordBitmap.SetRange('0', '9')
	wordBitmap.Set('_')

	// Initialize alpha bitmap
	alphaBitmap.SetRange('a', 'z')
	alphaBitmap.SetRange('A', 'Z')

	// Initialize lower bitmap
	lowerBitmap.SetRange('a', 'z')

	// Initialize upper bitmap
	upperBitmap.SetRange('A', 'Z')

	// Initialize negated bitmaps
	nonDigitBitmap = digitBitmap.Invert()
	nonSpaceBitmap = spaceBitmap.Invert()
	nonWordBitmap = wordBitmap.Invert()

	// Initialize dot bitmap (all except newline)
	dotBitmap = CharClassBitmap{^uint64(0), ^uint64(0), ^uint64(0), ^uint64(0)}
	dotBitmap.Clear('\n')
}

// FastCharClassSearcher provides SIMD-like character class matching.
// Uses bitmap lookup and batch processing for better performance.
type FastCharClassSearcher struct {
	bitmap     *CharClassBitmap // 32-byte bitmap for O(1) lookup
	rangeCheck rangeMatcher     // Specialized range check (nil if not applicable)
	minMatch   int              // 1 for +, 0 for *
	anchored   bool             // True if pattern starts with ^
	anchorEnd  bool             // True if pattern ends with $
}

// rangeMatcher provides specialized matching for simple ranges.
// This avoids bitmap lookup overhead for common patterns.
type rangeMatcher interface {
	Match(c byte) bool
}

// digitMatcher matches 0-9 using range check (fastest for digits).
type digitMatcher struct{}

func (digitMatcher) Match(c byte) bool {
	return c >= '0' && c <= '9'
}

// lowerMatcher matches a-z using range check.
type lowerMatcher struct{}

func (lowerMatcher) Match(c byte) bool {
	return c >= 'a' && c <= 'z'
}

// upperMatcher matches A-Z using range check.
type upperMatcher struct{}

func (upperMatcher) Match(c byte) bool {
	return c >= 'A' && c <= 'Z'
}

// alphaMatcher matches a-zA-Z using case-folding trick.
// (c | 0x20) converts uppercase to lowercase, then check range.
type alphaMatcher struct{}

func (alphaMatcher) Match(c byte) bool {
	return (c|0x20) >= 'a' && (c|0x20) <= 'z'
}

// wordMatcher matches word characters (a-zA-Z0-9_).
type wordMatcher struct{}

func (wordMatcher) Match(c byte) bool {
	return (c|0x20) >= 'a' && (c|0x20) <= 'z' ||
		c >= '0' && c <= '9' ||
		c == '_'
}

// NewFastDigitSearcher returns a searcher for \d+ pattern using range check.
func NewFastDigitSearcher() *FastCharClassSearcher {
	return &FastCharClassSearcher{
		bitmap:     &digitBitmap,
		rangeCheck: digitMatcher{},
		minMatch:   1,
	}
}

// NewFastWordSearcher returns a searcher for \w+ pattern.
func NewFastWordSearcher() *FastCharClassSearcher {
	return &FastCharClassSearcher{
		bitmap:     &wordBitmap,
		rangeCheck: wordMatcher{},
		minMatch:   1,
	}
}

// NewFastAlphaSearcher returns a searcher for [a-zA-Z]+ pattern.
func NewFastAlphaSearcher() *FastCharClassSearcher {
	return &FastCharClassSearcher{
		bitmap:     &alphaBitmap,
		rangeCheck: alphaMatcher{},
		minMatch:   1,
	}
}

// MatchString reports whether s contains any match.
// Uses specialized range checks or bitmap lookup.
func (f *FastCharClassSearcher) MatchString(haystack string) bool {
	if f.anchored {
		return f.matchAnchored(haystack)
	}

	if f.anchorEnd {
		return f.matchAnchorEnd(haystack)
	}

	// Unanchored: find first matching byte
	// Use range check if available (faster for simple patterns)
	if f.rangeCheck != nil {
		return f.matchUnanchoredRange(haystack)
	}
	return f.matchUnanchoredBitmap(haystack)
}

// matchAnchored handles patterns anchored at start (^).
func (f *FastCharClassSearcher) matchAnchored(haystack string) bool {
	if len(haystack) == 0 {
		return f.minMatch == 0
	}

	// Check first byte
	if f.rangeCheck != nil {
		if !f.rangeCheck.Match(haystack[0]) {
			return false
		}
	} else if !f.bitmap.Contains(haystack[0]) {
		return false
	}

	if !f.anchorEnd {
		return true
	}

	// Anchored at both ends - must match entire string
	if f.rangeCheck != nil {
		for i := 1; i < len(haystack); i++ {
			if !f.rangeCheck.Match(haystack[i]) {
				return false
			}
		}
	} else {
		for i := 1; i < len(haystack); i++ {
			if !f.bitmap.Contains(haystack[i]) {
				return false
			}
		}
	}
	return true
}

// matchAnchorEnd handles patterns anchored at end ($).
func (f *FastCharClassSearcher) matchAnchorEnd(haystack string) bool {
	if len(haystack) == 0 {
		return f.minMatch == 0
	}

	// Scan backwards to find match length
	end := len(haystack)
	start := end - 1

	if f.rangeCheck != nil {
		for start >= 0 && f.rangeCheck.Match(haystack[start]) {
			start--
		}
	} else {
		for start >= 0 && f.bitmap.Contains(haystack[start]) {
			start--
		}
	}
	start++
	return end-start >= f.minMatch
}

// matchUnanchoredRange finds first match using range check.
func (f *FastCharClassSearcher) matchUnanchoredRange(haystack string) bool {
	for i := 0; i < len(haystack); i++ {
		if f.rangeCheck.Match(haystack[i]) {
			return true
		}
	}
	return f.minMatch == 0
}

// matchUnanchoredBitmap finds first match using bitmap lookup.
// Uses batch processing for strings >= 8 bytes.
func (f *FastCharClassSearcher) matchUnanchoredBitmap(haystack string) bool {
	n := len(haystack)

	// Process 8 bytes at a time for long strings
	// This amortizes the loop overhead
	i := 0
	for ; i+8 <= n; i += 8 {
		if f.bitmap.Contains(haystack[i]) ||
			f.bitmap.Contains(haystack[i+1]) ||
			f.bitmap.Contains(haystack[i+2]) ||
			f.bitmap.Contains(haystack[i+3]) ||
			f.bitmap.Contains(haystack[i+4]) ||
			f.bitmap.Contains(haystack[i+5]) ||
			f.bitmap.Contains(haystack[i+6]) ||
			f.bitmap.Contains(haystack[i+7]) {
			return true
		}
	}

	// Handle remaining bytes
	for ; i < n; i++ {
		if f.bitmap.Contains(haystack[i]) {
			return true
		}
	}
	return f.minMatch == 0
}

// FindStringIndex returns the start and end of the first match, or nil.
func (f *FastCharClassSearcher) FindStringIndex(haystack string) []int {
	start, end, found := f.FindString(haystack)
	if !found {
		return nil
	}
	return []int{start, end}
}

// FindString returns the start, end, and whether a match was found.
func (f *FastCharClassSearcher) FindString(haystack string) (start, end int, found bool) {
	if f.anchored {
		return f.findAnchored(haystack)
	}

	if f.anchorEnd {
		return f.findAnchorEnd(haystack)
	}

	// Unanchored: find first match
	if f.rangeCheck != nil {
		return f.findUnanchoredRange(haystack)
	}
	return f.findUnanchoredBitmap(haystack)
}

// findAnchored handles anchored patterns for FindString.
func (f *FastCharClassSearcher) findAnchored(haystack string) (start, end int, found bool) {
	if len(haystack) == 0 {
		if f.minMatch == 0 {
			return 0, 0, true
		}
		return 0, 0, false
	}

	// Check first byte
	if f.rangeCheck != nil {
		if !f.rangeCheck.Match(haystack[0]) {
			return 0, 0, false
		}
	} else if !f.bitmap.Contains(haystack[0]) {
		return 0, 0, false
	}

	// Greedy scan forward
	end = 1
	if f.rangeCheck != nil {
		for end < len(haystack) && f.rangeCheck.Match(haystack[end]) {
			end++
		}
	} else {
		for end < len(haystack) && f.bitmap.Contains(haystack[end]) {
			end++
		}
	}

	if f.anchorEnd && end != len(haystack) {
		return 0, 0, false
	}

	return 0, end, true
}

// findAnchorEnd handles end-anchored patterns for FindString.
func (f *FastCharClassSearcher) findAnchorEnd(haystack string) (start, end int, found bool) {
	if len(haystack) == 0 {
		if f.minMatch == 0 {
			return 0, 0, true
		}
		return 0, 0, false
	}

	// Scan backwards to find start
	end = len(haystack)
	start = end - 1

	if f.rangeCheck != nil {
		for start >= 0 && f.rangeCheck.Match(haystack[start]) {
			start--
		}
	} else {
		for start >= 0 && f.bitmap.Contains(haystack[start]) {
			start--
		}
	}
	start++

	if end-start < f.minMatch {
		return 0, 0, false
	}
	return start, end, true
}

// findUnanchoredRange finds first match using range check.
func (f *FastCharClassSearcher) findUnanchoredRange(haystack string) (start, end int, found bool) {
	for i := 0; i < len(haystack); i++ {
		if f.rangeCheck.Match(haystack[i]) {
			start = i
			// Greedy scan forward
			for end = i + 1; end < len(haystack) && f.rangeCheck.Match(haystack[end]); end++ {
			}
			return start, end, true
		}
	}

	if f.minMatch == 0 {
		return 0, 0, true
	}
	return 0, 0, false
}

// findUnanchoredBitmap finds first match using bitmap lookup.
func (f *FastCharClassSearcher) findUnanchoredBitmap(haystack string) (start, end int, found bool) {
	n := len(haystack)

	// Process 8 bytes at a time for long strings
	i := 0
	for ; i+8 <= n; i += 8 {
		// Check batch of 8 bytes
		if f.bitmap.Contains(haystack[i]) {
			return f.extendMatch(haystack, i)
		}
		if f.bitmap.Contains(haystack[i+1]) {
			return f.extendMatch(haystack, i+1)
		}
		if f.bitmap.Contains(haystack[i+2]) {
			return f.extendMatch(haystack, i+2)
		}
		if f.bitmap.Contains(haystack[i+3]) {
			return f.extendMatch(haystack, i+3)
		}
		if f.bitmap.Contains(haystack[i+4]) {
			return f.extendMatch(haystack, i+4)
		}
		if f.bitmap.Contains(haystack[i+5]) {
			return f.extendMatch(haystack, i+5)
		}
		if f.bitmap.Contains(haystack[i+6]) {
			return f.extendMatch(haystack, i+6)
		}
		if f.bitmap.Contains(haystack[i+7]) {
			return f.extendMatch(haystack, i+7)
		}
	}

	// Handle remaining bytes
	for ; i < n; i++ {
		if f.bitmap.Contains(haystack[i]) {
			return f.extendMatch(haystack, i)
		}
	}

	if f.minMatch == 0 {
		return 0, 0, true
	}
	return 0, 0, false
}

// extendMatch extends a match from the given start position.
func (f *FastCharClassSearcher) extendMatch(haystack string, start int) (int, int, bool) {
	end := start + 1
	for end < len(haystack) && f.bitmap.Contains(haystack[end]) {
		end++
	}
	return start, end, true
}

// SWARContainsDigit checks if any byte in an 8-byte word is a digit (0-9).
// Uses SWAR (SIMD Within A Register) technique.
// Returns true if any byte is in range 0x30-0x39 ('0'-'9').
//
// Algorithm:
//  1. Subtract 0x30 from each byte (0-9 become 0x00-0x09)
//  2. Check if result < 0x0A using overflow detection
//
// This technique processes 8 bytes in parallel using uint64 arithmetic.
func SWARContainsDigit(word uint64) bool {
	// Subtract '0' (0x30) from each byte
	// If original byte was < 0x30, the subtraction will borrow from the next byte
	const sub = 0x3030303030303030

	// After subtracting, digits become 0x00-0x09
	// We check if each byte is < 0x0A
	// Using the high bit trick: if (byte + 0x76) overflows the high bit, byte >= 0x0A
	const addForCheck = 0x7676767676767676
	const highBits = 0x8080808080808080

	adjusted := word - sub

	// Check for underflow (borrowed into high bit) - means original was < '0'
	underflow := (^word & adjusted) & highBits

	// Check if any byte >= 0x0A after adjustment
	// (adjusted + 0x76) will set high bit if byte >= 0x0A
	overflow := (adjusted + addForCheck) & highBits

	// A digit exists if: no underflow AND no overflow (value is 0x00-0x09)
	// This means: underflow == 0 AND overflow == 0 for at least one byte
	// Equivalently: (underflow | overflow) != highBits

	return (underflow | overflow) != highBits
}

// SWARContainsSpace checks if any byte in an 8-byte word is whitespace.
// Checks for: space (0x20), tab (0x09), newline (0x0A), carriage return (0x0D),
// form feed (0x0C), vertical tab (0x0B).
//
// This is more complex than digit check because whitespace isn't contiguous.
// Uses byte-by-byte extraction for correctness.
func SWARContainsSpace(word uint64) bool {
	// Extract and check each byte
	// This is still faster than 8 separate memory loads
	for i := 0; i < 8; i++ {
		b := byte(word >> (i * 8))
		switch b {
		case ' ', '\t', '\n', '\r', '\f', '\v':
			return true
		}
	}
	return false
}

// SWARContainsAlpha checks if any byte in an 8-byte word is alphabetic (a-zA-Z).
// Uses byte-by-byte extraction with case-folding trick for simplicity and correctness.
func SWARContainsAlpha(word uint64) bool {
	// Extract and check each byte with case-folding
	for i := 0; i < 8; i++ {
		b := byte(word >> (i * 8))
		// Case-fold and check range
		if (b|0x20) >= 'a' && (b|0x20) <= 'z' {
			return true
		}
	}
	return false
}
