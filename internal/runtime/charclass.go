// Package runtime provides AWK runtime support including regex operations.
package runtime

// CharClassSearcher provides fast O(1) byte classification for simple patterns.
// Uses a 256-byte lookup table instead of NFA execution.
// Achieves 14-22x speedup for patterns like \d+, \s+, \w+.
type CharClassSearcher struct {
	membership [256]bool // O(1) byte classification
	minMatch   int       // 1 for +, 0 for *
	anchored   bool      // True if pattern starts with ^
	anchorEnd  bool      // True if pattern ends with $
}

// Pre-built character class tables for common patterns
var (
	digitTable    [256]bool // \d: 0-9
	spaceTable    [256]bool // \s: space, tab, newline, etc.
	wordTable     [256]bool // \w: a-z, A-Z, 0-9, _
	alphaTable    [256]bool // [a-zA-Z]
	lowerTable    [256]bool // [a-z]
	upperTable    [256]bool // [A-Z]
	nonDigitTable [256]bool // \D: NOT 0-9
	nonSpaceTable [256]bool // \S: NOT whitespace
	nonWordTable  [256]bool // \W: NOT word chars
	dotTable      [256]bool // .: any char except newline
)

func init() {
	// Initialize digit table (\d)
	for c := '0'; c <= '9'; c++ {
		digitTable[c] = true
	}

	// Initialize space table (\s)
	spaceTable[' '] = true
	spaceTable['\t'] = true
	spaceTable['\n'] = true
	spaceTable['\r'] = true
	spaceTable['\f'] = true
	spaceTable['\v'] = true

	// Initialize word table (\w)
	for c := 'a'; c <= 'z'; c++ {
		wordTable[c] = true
	}
	for c := 'A'; c <= 'Z'; c++ {
		wordTable[c] = true
	}
	for c := '0'; c <= '9'; c++ {
		wordTable[c] = true
	}
	wordTable['_'] = true

	// Initialize alpha table ([a-zA-Z])
	for c := 'a'; c <= 'z'; c++ {
		alphaTable[c] = true
	}
	for c := 'A'; c <= 'Z'; c++ {
		alphaTable[c] = true
	}

	// Initialize lower table ([a-z])
	for c := 'a'; c <= 'z'; c++ {
		lowerTable[c] = true
	}

	// Initialize upper table ([A-Z])
	for c := 'A'; c <= 'Z'; c++ {
		upperTable[c] = true
	}

	// Initialize negated tables (inversions)
	for i := 0; i < 256; i++ {
		nonDigitTable[i] = !digitTable[i]
		nonSpaceTable[i] = !spaceTable[i]
		nonWordTable[i] = !wordTable[i]
	}

	// Initialize dot table (any char except newline)
	for i := 0; i < 256; i++ {
		dotTable[i] = true
	}
	dotTable['\n'] = false
}

// NewDigitSearcher returns a searcher for \d+ pattern.
func NewDigitSearcher() *CharClassSearcher {
	return &CharClassSearcher{membership: digitTable, minMatch: 1}
}

// NewSpaceSearcher returns a searcher for \s+ pattern.
func NewSpaceSearcher() *CharClassSearcher {
	return &CharClassSearcher{membership: spaceTable, minMatch: 1}
}

// NewWordSearcher returns a searcher for \w+ pattern.
func NewWordSearcher() *CharClassSearcher {
	return &CharClassSearcher{membership: wordTable, minMatch: 1}
}

// MatchString reports whether s contains any match.
func (s *CharClassSearcher) MatchString(haystack string) bool {
	if s.anchored {
		// Must match at start
		if len(haystack) == 0 {
			return s.minMatch == 0
		}
		if !s.membership[haystack[0]] {
			return false
		}
		// Find extent of match
		end := 1
		for end < len(haystack) && s.membership[haystack[end]] {
			end++
		}
		if s.anchorEnd {
			return end == len(haystack)
		}
		return true
	}

	if s.anchorEnd {
		// Must match at end
		if len(haystack) == 0 {
			return s.minMatch == 0
		}
		// Scan backwards
		end := len(haystack)
		start := end - 1
		for start >= 0 && s.membership[haystack[start]] {
			start--
		}
		start++
		return end-start >= s.minMatch
	}

	// Unanchored: find first matching byte
	for i := 0; i < len(haystack); i++ {
		if s.membership[haystack[i]] {
			return true
		}
	}
	return s.minMatch == 0
}

// FindStringIndex returns the start and end of the first match, or nil.
func (s *CharClassSearcher) FindStringIndex(haystack string) []int {
	start, end, found := s.FindString(haystack)
	if !found {
		return nil
	}
	return []int{start, end}
}

// FindString returns the start, end, and whether a match was found.
func (s *CharClassSearcher) FindString(haystack string) (start, end int, found bool) {
	if s.anchored {
		// Must match at start
		if len(haystack) == 0 {
			if s.minMatch == 0 {
				return 0, 0, true
			}
			return 0, 0, false
		}
		if !s.membership[haystack[0]] {
			return 0, 0, false
		}
		// Greedy scan forward
		end = 1
		for end < len(haystack) && s.membership[haystack[end]] {
			end++
		}
		if s.anchorEnd && end != len(haystack) {
			return 0, 0, false
		}
		return 0, end, true
	}

	if s.anchorEnd {
		// Must match at end
		if len(haystack) == 0 {
			if s.minMatch == 0 {
				return 0, 0, true
			}
			return 0, 0, false
		}
		// Scan backwards to find start
		end = len(haystack)
		start = end - 1
		for start >= 0 && s.membership[haystack[start]] {
			start--
		}
		start++
		if end-start < s.minMatch {
			return 0, 0, false
		}
		return start, end, true
	}

	// Unanchored: find first matching byte
	for i := 0; i < len(haystack); i++ {
		if s.membership[haystack[i]] {
			start = i
			// Greedy scan forward
			for end = i + 1; end < len(haystack) && s.membership[haystack[end]]; end++ {
			}
			return start, end, true
		}
	}

	// No match found
	if s.minMatch == 0 {
		return 0, 0, true // * matches empty string
	}
	return 0, 0, false
}

// analyzeCharClass checks if a pattern is a simple character class pattern
// that can use the fast path. Returns nil if not applicable.
func analyzeCharClass(pattern string) *CharClassSearcher {
	// Handle anchors
	anchored := false
	anchorEnd := false
	p := pattern

	// Remove dotall prefix if present (AWK dotall mode)
	if len(p) >= len(dotallPrefix) && p[:len(dotallPrefix)] == dotallPrefix {
		p = p[len(dotallPrefix):]
	}

	if len(p) > 0 && p[0] == '^' {
		anchored = true
		p = p[1:]
	}

	// Check for trailing $ anchor
	if len(p) > 0 && p[len(p)-1] == '$' {
		// Make sure it's not escaped
		numBackslash := 0
		for i := len(p) - 2; i >= 0 && p[i] == '\\'; i-- {
			numBackslash++
		}
		if numBackslash%2 == 0 {
			anchorEnd = true
			p = p[:len(p)-1]
		}
	}

	// Check for simple character class patterns
	var searcher *CharClassSearcher

	switch p {
	// \d patterns
	case `\d+`, `\d*`:
		searcher = &CharClassSearcher{membership: digitTable}
		if p[len(p)-1] == '+' {
			searcher.minMatch = 1
		}

	// \s patterns
	case `\s+`, `\s*`:
		searcher = &CharClassSearcher{membership: spaceTable}
		if p[len(p)-1] == '+' {
			searcher.minMatch = 1
		}

	// \w patterns
	case `\w+`, `\w*`:
		searcher = &CharClassSearcher{membership: wordTable}
		if p[len(p)-1] == '+' {
			searcher.minMatch = 1
		}

	// \D patterns (negated digit)
	case `\D+`, `\D*`:
		searcher = &CharClassSearcher{membership: nonDigitTable}
		if p[len(p)-1] == '+' {
			searcher.minMatch = 1
		}

	// \S patterns (negated space)
	case `\S+`, `\S*`:
		searcher = &CharClassSearcher{membership: nonSpaceTable}
		if p[len(p)-1] == '+' {
			searcher.minMatch = 1
		}

	// \W patterns (negated word)
	case `\W+`, `\W*`:
		searcher = &CharClassSearcher{membership: nonWordTable}
		if p[len(p)-1] == '+' {
			searcher.minMatch = 1
		}

	// . patterns (any char except newline)
	case `.+`, `.*`:
		searcher = &CharClassSearcher{membership: dotTable}
		if p[len(p)-1] == '+' {
			searcher.minMatch = 1
		}

	// [0-9] patterns
	case `[0-9]+`, `[0-9]*`:
		searcher = &CharClassSearcher{membership: digitTable}
		if p[len(p)-1] == '+' {
			searcher.minMatch = 1
		}

	// Common AWK patterns - now using cached tables
	case `[a-zA-Z]+`, `[a-zA-Z]*`:
		searcher = &CharClassSearcher{membership: alphaTable}
		if p[len(p)-1] == '+' {
			searcher.minMatch = 1
		}

	case `[a-z]+`, `[a-z]*`:
		searcher = &CharClassSearcher{membership: lowerTable}
		if p[len(p)-1] == '+' {
			searcher.minMatch = 1
		}

	case `[A-Z]+`, `[A-Z]*`:
		searcher = &CharClassSearcher{membership: upperTable}
		if p[len(p)-1] == '+' {
			searcher.minMatch = 1
		}

	default:
		return nil
	}

	if searcher != nil {
		searcher.anchored = anchored
		searcher.anchorEnd = anchorEnd
	}

	return searcher
}
