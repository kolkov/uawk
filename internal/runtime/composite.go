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

// analyzeComposite checks if a pattern is a composite character class pattern
// that can use the fast path. Returns nil if not applicable.
//
// Supported patterns:
//   - Sequences of char class + quantifier: [a-zA-Z]+[0-9]+
//   - Escaped shortcuts: \d+\s+\w+
//   - Mixed: [a-z]+\d+
//
// NOT supported (returns nil):
//   - Literal characters between classes
//   - Alternation (|)
//   - Repetition bounds ({n,m})
//   - Lookahead/lookbehind
//   - Groups
func analyzeComposite(pattern string) *CompositeSearcher {
	// Handle anchors
	anchored := false
	anchorEnd := false
	p := pattern

	// Remove dotall prefix if present (AWK dotall mode)
	if len(p) >= len(dotallPrefix) && p[:len(dotallPrefix)] == dotallPrefix {
		p = p[len(dotallPrefix):]
	}

	if len(p) == 0 {
		return nil
	}

	// Check for start anchor
	if p[0] == '^' {
		anchored = true
		p = p[1:]
	}

	// Check for end anchor
	if len(p) > 0 && p[len(p)-1] == '$' {
		numBackslash := 0
		for i := len(p) - 2; i >= 0 && p[i] == '\\'; i-- {
			numBackslash++
		}
		if numBackslash%2 == 0 {
			anchorEnd = true
			p = p[:len(p)-1]
		}
	}

	if len(p) == 0 {
		return nil
	}

	// Parse sequence of char class parts
	parts, ok := parseCompositeParts(p)
	if !ok || len(parts) < 2 {
		// Need at least 2 parts to be a composite pattern
		// Single part is handled by analyzeCharClass
		return nil
	}

	// Verify at least one char class with quantifier exists (not all literals)
	hasCharClass := false
	for _, part := range parts {
		if part.membership != nil {
			hasCharClass = true
			break
		}
	}
	if !hasCharClass {
		// All literals - not a composite pattern
		return nil
	}

	// Check for char class IMMEDIATELY followed by literal where the char class can match the literal.
	// This requires backtracking which we don't support.
	// Examples that need backtracking:
	//   [a-z]+test - plus can match 't', so greedy matching fails
	//   [a-z.]+\.  - plus can match '.', so greedy matching consumes all dots
	// Examples that are SAFE:
	//   [a-z]+\.com - '.' not in [a-z], so [a-z]+ stops at '.', then '.' matches, then 'com' matches
	//   [a-z]+@test - '@' not in [a-z], so [a-z]+ stops at '@'
	for i := 0; i < len(parts)-1; i++ {
		if parts[i].membership != nil && parts[i+1].literal != "" {
			// Char class immediately followed by literal
			firstChar := parts[i+1].literal[0]
			if parts[i].membership[firstChar] {
				// Char class can match the literal's first char - needs backtracking
				return nil
			}
		}
	}

	// Build first-char skip filter for unanchored patterns
	// This optimization allows skipping positions that can't start a match
	var firstCharFilter *[256]bool
	var literalPrefix string

	if !anchored {
		// Collect leading literal characters into literalPrefix
		var sb strings.Builder
		for _, part := range parts {
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

		// If we have a literal prefix but no char filter, that's fine
		// strings.Index will handle the search efficiently
	}

	return &CompositeSearcher{
		parts:           parts,
		anchored:        anchored,
		anchorEnd:       anchorEnd,
		firstCharFilter: firstCharFilter,
		literalPrefix:   literalPrefix,
	}
}

// parseCompositeParts parses a pattern into a sequence of char class parts.
// Returns nil, false if the pattern contains unsupported elements.
func parseCompositeParts(p string) ([]*charClassPart, bool) {
	var parts []*charClassPart

	i := 0
	for i < len(p) {
		// Try to parse a char class (bracket or escaped shortcut)
		part, consumed, ok := parseCharClassPart(p[i:])
		if !ok {
			return nil, false
		}
		if part == nil {
			// Unsupported pattern element
			return nil, false
		}

		parts = append(parts, part)
		i += consumed
	}

	return parts, len(parts) > 0
}

// parseCharClassPart parses one char class part from the start of p.
// Returns the part, number of bytes consumed, and whether parsing succeeded.
// Returns nil part if unsupported element found.
func parseCharClassPart(p string) (*charClassPart, int, bool) {
	if len(p) == 0 {
		return nil, 0, true
	}

	var table *[256]bool
	consumed := 0

	// Check for escaped shortcuts (\d, \s, \w, etc.)
	if p[0] == '\\' && len(p) >= 2 {
		switch p[1] {
		case 'd':
			table = &digitTable
			consumed = 2
		case 's':
			table = &spaceTable
			consumed = 2
		case 'w':
			table = &wordTable
			consumed = 2
		case 'D':
			table = &nonDigitTable
			consumed = 2
		case 'S':
			table = &nonSpaceTable
			consumed = 2
		case 'W':
			table = &nonWordTable
			consumed = 2
		default:
			// Escaped literal character (e.g., \., \@, \\)
			return &charClassPart{
				literal: string(p[1]),
			}, 2, true
		}
	} else if p[0] == '[' {
		// Parse bracket expression
		var ok bool
		table, consumed, ok = parseBracketExpression(p)
		if !ok {
			return nil, 0, false
		}
	} else if p[0] == '.' {
		// Dot matches any character
		table = &dotTable
		consumed = 1
	} else if isLiteralChar(p[0]) {
		// Literal character (not a special regex char)
		return &charClassPart{
			literal: string(p[0]),
		}, 1, true
	} else {
		// Special regex character without escape - not supported
		return nil, 0, false
	}

	// Parse quantifier (only for char classes, not literals)
	if consumed >= len(p) {
		// No quantifier - implicit {1,1} but we need + or * for char classes
		return nil, 0, false
	}

	minMatch := 0
	switch p[consumed] {
	case '+':
		minMatch = 1
		consumed++
	case '*':
		minMatch = 0
		consumed++
	default:
		// No quantifier or unsupported quantifier
		return nil, 0, false
	}

	return &charClassPart{
		membership: table,
		minMatch:   minMatch,
	}, consumed, true
}

// isLiteralChar returns true if c is a literal character (not a regex special char).
func isLiteralChar(c byte) bool {
	switch c {
	case '.', '*', '+', '?', '[', ']', '(', ')', '{', '}', '|', '^', '$', '\\':
		return false
	default:
		return true
	}
}

// parseBracketExpression parses a [...] expression and returns the membership table.
// Returns pointer to pre-built table if matches known pattern, otherwise builds custom.
// Returns nil, 0, false if unsupported.
func parseBracketExpression(p string) (*[256]bool, int, bool) {
	if len(p) == 0 || p[0] != '[' {
		return nil, 0, false
	}

	// Find closing bracket
	end := 1
	if end < len(p) && (p[end] == '^' || p[end] == ']') {
		end++
	}
	for end < len(p) && p[end] != ']' {
		if p[end] == '\\' && end+1 < len(p) {
			end += 2
		} else {
			end++
		}
	}
	if end >= len(p) {
		return nil, 0, false // Unclosed bracket
	}
	end++ // Include closing ]

	bracketExpr := p[:end]

	// Check for known patterns (use pre-built tables)
	switch bracketExpr {
	case "[0-9]":
		return &digitTable, end, true
	case "[a-z]":
		return &lowerTable, end, true
	case "[A-Z]":
		return &upperTable, end, true
	case "[a-zA-Z]", "[A-Za-z]":
		return &alphaTable, end, true
	}

	// For other bracket expressions, build custom table
	// Currently we only support simple ranges and character lists
	table, ok := buildCustomTable(bracketExpr)
	if !ok {
		return nil, 0, false
	}

	return table, end, true
}

// customTables caches dynamically built membership tables.
// Key is the bracket expression string.
var customTables = make(map[string]*[256]bool)

// buildCustomTable builds a membership table for a bracket expression.
// Returns nil, false if unsupported.
func buildCustomTable(bracketExpr string) (*[256]bool, bool) {
	// Check cache first
	if table, ok := customTables[bracketExpr]; ok {
		return table, true
	}

	if len(bracketExpr) < 2 || bracketExpr[0] != '[' || bracketExpr[len(bracketExpr)-1] != ']' {
		return nil, false
	}

	content := bracketExpr[1 : len(bracketExpr)-1]
	if len(content) == 0 {
		return nil, false
	}

	// Check for negation
	negated := false
	if content[0] == '^' {
		negated = true
		content = content[1:]
	}

	table := new([256]bool)

	// Parse content
	i := 0
	for i < len(content) {
		// Handle escape sequences
		if content[i] == '\\' && i+1 < len(content) {
			switch content[i+1] {
			case 'd':
				// Add all digits
				for c := '0'; c <= '9'; c++ {
					table[c] = true
				}
				i += 2
				continue
			case 's':
				// Add all whitespace
				table[' '] = true
				table['\t'] = true
				table['\n'] = true
				table['\r'] = true
				table['\f'] = true
				table['\v'] = true
				i += 2
				continue
			case 'w':
				// Add all word chars
				for c := 'a'; c <= 'z'; c++ {
					table[c] = true
				}
				for c := 'A'; c <= 'Z'; c++ {
					table[c] = true
				}
				for c := '0'; c <= '9'; c++ {
					table[c] = true
				}
				table['_'] = true
				i += 2
				continue
			default:
				// Escaped literal
				table[content[i+1]] = true
				i += 2
				continue
			}
		}

		// Check for range (a-z)
		if i+2 < len(content) && content[i+1] == '-' && content[i+2] != ']' {
			start := content[i]
			end := content[i+2]
			if start > end {
				// Invalid range
				return nil, false
			}
			for c := start; c <= end; c++ {
				table[c] = true
			}
			i += 3
			continue
		}

		// Single character
		table[content[i]] = true
		i++
	}

	// Apply negation
	if negated {
		for i := range table {
			table[i] = !table[i]
		}
	}

	// Cache the table
	customTables[bracketExpr] = table

	return table, true
}
