// Package runtime provides AWK runtime support including regex operations.
package runtime

import (
	"regexp/syntax"
	"strings"
)

// LiteralInfo holds extracted literal substrings from a regex pattern.
// These literals enable fast rejection using SIMD-optimized string functions
// before falling back to full NFA execution.
type LiteralInfo struct {
	Prefix   string   // Anchored prefix (^prefix) - use HasPrefix
	Suffix   string   // Anchored suffix (suffix$) - use HasSuffix
	Required []string // Must appear somewhere - use strings.Contains
}

// minRequiredLen is the shortest unanchored literal worth a strings.Contains
// check before running the regex.
const minRequiredLen = 3

// extractLiterals analyzes a regex pattern and extracts literal substrings
// that can be used for fast prefiltering. Returns nil if no useful literals found.
//
// IMPORTANT: it must NEVER produce false negatives (reject strings that match).
// It therefore works on the syntax tree from regexp/syntax, parsed with the
// same flags as coregex, and takes only literals every match must contain:
// case-sensitive literals that are direct elements of the top-level
// concatenation. A literal right after ^ is a prefix, one right before $ is a
// suffix. Anything inside groups, alternations or repetitions is skipped.
//
// Examples:
//   - "^error.*failed$" -> prefix="error", suffix="failed"
//   - "^/api/v1/" -> prefix="/api/v1/"
//   - "warning.*error" -> required=["warning", "error"]
//   - "\\d+.*test" -> required=["test"] (\\d is not a literal)
//   - "^abc*" -> prefix="ab" (c is optional)
func extractLiterals(pattern string) *LiteralInfo {
	re, err := syntax.Parse(pattern, syntax.Perl) // same flags as coregex.Compile
	if err != nil {
		return nil
	}

	var subs []*syntax.Regexp
	switch re.Op {
	case syntax.OpConcat:
		subs = re.Sub
	case syntax.OpLiteral:
		subs = []*syntax.Regexp{re}
	default:
		return nil
	}

	anchored := len(subs) > 0 && subs[0].Op == syntax.OpBeginText
	if anchored {
		subs = subs[1:]
	}
	anchorEnd := len(subs) > 0 && subs[len(subs)-1].Op == syntax.OpEndText
	if anchorEnd {
		subs = subs[:len(subs)-1]
	}

	// Split the concatenation into runs of adjacent literals. Each run is a
	// substring every match contains.
	type run struct {
		text       string
		start, end int // index range in subs
	}
	var runs []run
	var current strings.Builder
	runStart := -1
	flush := func(end int) {
		if runStart >= 0 {
			runs = append(runs, run{current.String(), runStart, end})
		}
		current.Reset()
		runStart = -1
	}
	for i, sub := range subs {
		if sub.Op == syntax.OpLiteral && sub.Flags&syntax.FoldCase == 0 {
			if runStart < 0 {
				runStart = i
			}
			current.WriteString(string(sub.Rune))
			continue
		}
		flush(i)
	}
	flush(len(subs))

	info := &LiteralInfo{}
	for _, r := range runs {
		switch {
		case anchored && r.start == 0:
			info.Prefix = r.text
		case anchorEnd && r.end == len(subs):
			info.Suffix = r.text
		case len(r.text) >= minRequiredLen:
			info.Required = append(info.Required, r.text)
		}
	}

	if info.Prefix == "" && info.Suffix == "" && len(info.Required) == 0 {
		return nil
	}
	return info
}

// CanReject checks if the literal info can definitely reject the string
// without running the full regex. Returns true if the string cannot match.
// This is the hot path - must have zero allocations.
func (li *LiteralInfo) CanReject(s string) bool {
	if li.Prefix != "" && !strings.HasPrefix(s, li.Prefix) {
		return true
	}
	if li.Suffix != "" && !strings.HasSuffix(s, li.Suffix) {
		return true
	}
	for _, req := range li.Required {
		if !strings.Contains(s, req) {
			return true
		}
	}
	return false
}
