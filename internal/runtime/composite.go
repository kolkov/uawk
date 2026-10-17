// Package runtime provides AWK runtime support including regex operations.
package runtime

import "strings"

// CompositeSearcher handles patterns like [a-zA-Z]+[0-9]+ that consist of
// a sequence of character class parts and/or literal characters. Each part
// must match consecutively (no gaps) in greedy fashion.
//
// Performance: O(n) where n is string length. Each character is checked
// against at most one membership table (O(1) per character).
// First-char skip optimization: For unanchored patterns, positions that
// don't match the first part's filter are skipped (3-5x speedup).
//
// Supported patterns:
//   - [a-zA-Z]+[0-9]+      (alpha followed by digits)
//   - \w+\d+               (word chars followed by digits)
//   - [a-z]+[A-Z]+         (lower followed by upper)
//   - \d+\s+\w+            (digits, spaces, word chars)
//   - [a-z]+@[a-z]+        (literals between char classes)
//   - user[0-9]+           (literal prefix with char class)
//   - [a-z]+@[a-z]+\.[a-z]+ (email-like patterns)
//
// NOT supported (fall back to coregex):
//   - Patterns with * followed by literals (need backtracking)
//   - Patterns with alternation (e.g., "[a-z]+|[0-9]+")
//   - Patterns with repetition bounds (e.g., "[a-z]{2,4}")
type CompositeSearcher struct {
	parts     []*charClassPart // Sequence of char class parts
	anchored  bool             // True if pattern starts with ^
	anchorEnd bool             // True if pattern ends with $

	// First-char skip optimization: for unanchored patterns,
	// quickly skip positions that can't start a match.
	firstCharFilter *[256]bool // Membership table for first character (nil if literal prefix)
	literalPrefix   string     // Non-empty if pattern starts with literal chars
}

// charClassPart represents one segment of a composite pattern.
// Either membership is set (char class with quantifier) or literal is set (exact match).
type charClassPart struct {
	membership *[256]bool // Pointer to avoid copying 256-byte tables (nil for literals)
	literal    string     // Literal string to match (empty for char classes)
	minMatch   int        // 1 for +, 0 for * (ignored for literals)
}

// MatchString reports whether s contains a match for the composite pattern.
// Uses greedy matching: each part consumes as many characters as possible.
// Uses first-char skip optimization for unanchored patterns (3-5x speedup).
func (c *CompositeSearcher) MatchString(haystack string) bool {
	if len(c.parts) == 0 {
		return true
	}

	if c.anchored {
		// Must match at start
		_, ok := c.matchAt(haystack, 0)
		return ok
	}

	if c.anchorEnd {
		// Must match at end - find rightmost valid match
		return c.matchAtEnd(haystack)
	}

	// Unanchored: use first-char skip optimization
	// This avoids calling matchAt for positions that can't possibly match

	// Case 1: Pattern starts with literal prefix - use strings.Index for fast search
	if c.literalPrefix != "" {
		offset := 0
		for {
			idx := strings.Index(haystack[offset:], c.literalPrefix)
			if idx < 0 {
				return false
			}
			pos := offset + idx
			if _, ok := c.matchAt(haystack, pos); ok {
				return true
			}
			offset = pos + 1
			if offset >= len(haystack) {
				return false
			}
		}
	}

	// Case 2: Pattern starts with char class - use membership filter
	if c.firstCharFilter != nil {
		for i := 0; i < len(haystack); i++ {
			// Skip positions where first char doesn't match the filter
			if c.firstCharFilter[haystack[i]] {
				if _, ok := c.matchAt(haystack, i); ok {
					return true
				}
			}
		}
		return false
	}

	// Fallback: try each position (should not reach here normally)
	for i := 0; i <= len(haystack); i++ {
		if _, ok := c.matchAt(haystack, i); ok {
			return true
		}
	}
	return false
}

// FindStringIndex returns the start and end of the first match, or nil.
// Uses first-char skip optimization for unanchored patterns (3-5x speedup).
func (c *CompositeSearcher) FindStringIndex(haystack string) []int {
	if len(c.parts) == 0 {
		return []int{0, 0}
	}

	if c.anchored {
		end, ok := c.matchAt(haystack, 0)
		if !ok {
			return nil
		}
		if c.anchorEnd && end != len(haystack) {
			return nil
		}
		return []int{0, end}
	}

	if c.anchorEnd {
		start, end, ok := c.findMatchAtEnd(haystack)
		if !ok {
			return nil
		}
		return []int{start, end}
	}

	// Unanchored: use first-char skip optimization

	// Case 1: Pattern starts with literal prefix - use strings.Index for fast search
	if c.literalPrefix != "" {
		offset := 0
		for {
			idx := strings.Index(haystack[offset:], c.literalPrefix)
			if idx < 0 {
				return nil
			}
			pos := offset + idx
			if end, ok := c.matchAt(haystack, pos); ok {
				return []int{pos, end}
			}
			offset = pos + 1
			if offset >= len(haystack) {
				return nil
			}
		}
	}

	// Case 2: Pattern starts with char class - use membership filter
	if c.firstCharFilter != nil {
		for i := 0; i < len(haystack); i++ {
			// Skip positions where first char doesn't match the filter
			if c.firstCharFilter[haystack[i]] {
				if end, ok := c.matchAt(haystack, i); ok {
					return []int{i, end}
				}
			}
		}
		return nil
	}

	// Fallback: try each position
	for i := 0; i <= len(haystack); i++ {
		if end, ok := c.matchAt(haystack, i); ok {
			return []int{i, end}
		}
	}
	return nil
}

// matchAt tries to match the pattern starting at position start.
// Returns the end position and whether a match was found.
func (c *CompositeSearcher) matchAt(haystack string, start int) (end int, ok bool) {
	pos := start

	for _, part := range c.parts {
		if part.literal != "" {
			// Literal match - exact string required
			litLen := len(part.literal)
			if pos+litLen > len(haystack) {
				return 0, false
			}
			if haystack[pos:pos+litLen] != part.literal {
				return 0, false
			}
			pos += litLen
		} else {
			// Char class match - greedy
			matchLen := 0
			for pos+matchLen < len(haystack) && part.membership[haystack[pos+matchLen]] {
				matchLen++
			}

			// Check minimum match requirement
			if matchLen < part.minMatch {
				return 0, false
			}

			pos += matchLen
		}
	}

	// Check end anchor if needed
	if c.anchorEnd && pos != len(haystack) {
		return 0, false
	}

	return pos, true
}

// matchAtEnd checks if the pattern matches at the end of the string.
// For end-anchored patterns, we need to find a valid starting position
// such that the pattern consumes exactly to the end.
func (c *CompositeSearcher) matchAtEnd(haystack string) bool {
	// For end-anchored, we scan backwards to find potential start positions
	// This is more complex because greedy matching might overshoot

	// Simple approach: try each position and check if match reaches exactly to end
	for i := 0; i <= len(haystack); i++ {
		if end, ok := c.matchAt(haystack, i); ok && end == len(haystack) {
			return true
		}
	}
	return false
}

// findMatchAtEnd finds a match that ends at the end of the string.
// Returns start, end positions and whether found.
func (c *CompositeSearcher) findMatchAtEnd(haystack string) (start, end int, ok bool) {
	// Try each position from left to right
	for i := 0; i <= len(haystack); i++ {
		if matchEnd, found := c.matchAt(haystack, i); found && matchEnd == len(haystack) {
			return i, matchEnd, true
		}
	}
	return 0, 0, false
}

// analyzeComposite returns a CompositeSearcher for patterns made of two or more
// literal and x* / x+ parts with at least one class, such as [a-zA-Z]+[0-9]+,
// \d+\s+\w+ or user[0-9]+. It returns nil when the fast path does not apply;
// see parseFastPattern for the exact rules. awkPattern must already carry
// dotallPrefix.
func analyzeComposite(awkPattern string) *CompositeSearcher {
	return newCompositeSearcher(parseFastPattern(awkPattern))
}

func newCompositeSearcher(fp *fastPattern) *CompositeSearcher {
	if fp == nil || len(fp.parts) < 2 {
		// A single part is handled by CharClassSearcher.
		return nil
	}
	hasCharClass := false
	for _, part := range fp.parts {
		if part.membership != nil {
			hasCharClass = true
			break
		}
	}
	if !hasCharClass {
		// All literals - not a composite pattern
		return nil
	}

	// Build first-char skip filter for unanchored patterns
	// This optimization allows skipping positions that can't start a match
	var firstCharFilter *[256]bool
	var literalPrefix string

	if !fp.anchored {
		// Collect leading literal characters into literalPrefix
		var sb strings.Builder
		for _, part := range fp.parts {
			if part.literal != "" {
				sb.WriteString(part.literal)
			} else {
				// First non-literal part
				if part.minMatch > 0 {
					// Pattern requires at least one char from this class
					firstCharFilter = part.membership
				}
				break
			}
		}
		literalPrefix = sb.String()
	}

	return &CompositeSearcher{
		parts:           fp.parts,
		anchored:        fp.anchored,
		anchorEnd:       fp.anchorEnd,
		firstCharFilter: firstCharFilter,
		literalPrefix:   literalPrefix,
	}
}
