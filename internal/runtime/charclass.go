// Package runtime provides AWK runtime support including regex operations.
package runtime

// CharClassSearcher provides fast O(1) byte classification for simple patterns.
// Uses a 256-byte lookup table instead of NFA execution.
// Achieves 14-22x speedup for patterns like \d+, \s+, \w+.
//
// It is used only for patterns accepted by parseFastPattern with a single
// x* or x+ part, so its results equal coregex's (see fastpath.go).
type CharClassSearcher struct {
	membership [256]bool // O(1) byte classification
	minMatch   int       // 1 for +, 0 for *
	anchored   bool      // True if pattern starts with ^
	anchorEnd  bool      // True if pattern ends with $
}

// run returns the end of the longest run of member bytes starting at start.
func (s *CharClassSearcher) run(haystack string, start int) int {
	end := start
	for end < len(haystack) && s.membership[haystack[end]] {
		end++
	}
	return end
}

// runBefore returns the start of the longest run of member bytes ending at end.
func (s *CharClassSearcher) runBefore(haystack string, end int) int {
	start := end
	for start > 0 && s.membership[haystack[start-1]] {
		start--
	}
	return start
}

// MatchString reports whether s contains any match.
func (s *CharClassSearcher) MatchString(haystack string) bool {
	if s.anchored {
		end := s.run(haystack, 0)
		if s.anchorEnd {
			return end == len(haystack) && end >= s.minMatch
		}
		return end >= s.minMatch
	}

	if s.anchorEnd {
		return len(haystack)-s.runBefore(haystack, len(haystack)) >= s.minMatch
	}

	// x* matches the empty string at position 0.
	if s.minMatch == 0 {
		return true
	}
	for i := 0; i < len(haystack); i++ {
		if s.membership[haystack[i]] {
			return true
		}
	}
	return false
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
// The match is the leftmost one, and the longest at that position.
func (s *CharClassSearcher) FindString(haystack string) (start, end int, found bool) {
	if s.anchored {
		end = s.run(haystack, 0)
		if end < s.minMatch || (s.anchorEnd && end != len(haystack)) {
			return 0, 0, false
		}
		return 0, end, true
	}

	if s.anchorEnd {
		// The leftmost match ending at the end of the text starts where the
		// final run of member bytes starts.
		start = s.runBefore(haystack, len(haystack))
		if len(haystack)-start < s.minMatch {
			return 0, 0, false
		}
		return start, len(haystack), true
	}

	// x* matches at position 0, possibly empty.
	if s.minMatch == 0 {
		return 0, s.run(haystack, 0), true
	}

	for i := 0; i < len(haystack); i++ {
		if s.membership[haystack[i]] {
			return i, s.run(haystack, i), true
		}
	}
	return 0, 0, false
}

// newCharClassSearcher returns a CharClassSearcher for patterns that consist
// of a single x* or x+ (with optional ^ and $), or nil if the fast path does
// not apply.
func newCharClassSearcher(fp *fastPattern) *CharClassSearcher {
	if fp == nil || len(fp.parts) != 1 || fp.parts[0].membership == nil {
		return nil
	}
	part := fp.parts[0]
	return &CharClassSearcher{
		membership: *part.membership,
		minMatch:   part.minMatch,
		anchored:   fp.anchored,
		anchorEnd:  fp.anchorEnd,
	}
}
