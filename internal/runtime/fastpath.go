package runtime

import (
	"regexp/syntax"
	"unicode"
	"unicode/utf8"
)

// The char-class and composite fast paths match bytes against 256-entry
// tables and never backtrack. They are an optimization of coregex, so for every
// pattern they accept they must return exactly what coregex returns for the
// same pattern ((?s) prefix, Perl syntax, leftmost-longest).
//
// To guarantee that, admission works on the syntax tree from regexp/syntax,
// parsed with the same flags coregex uses, instead of on the pattern text.
// A pattern is accepted only when all of the following hold:
//
//   - It is a concatenation of literals and x* / x+ where x is a character
//     class, "." or a single literal character, with optional ^ and $.
//   - Every literal is ASCII, and every class either contains no character
//     >= U+0080 or contains all of them. Then a multi-byte UTF-8 character,
//     or an invalid byte that coregex reads as U+FFFD, is either matched
//     whole or not at all, so byte tables agree with rune matching.
//   - The pattern is one-pass: no class can match a byte that could start
//     what follows it. Then greedy matching without backtracking finds the
//     same match as the regex engine (same idea as regexp's onepass check).
//
// Everything else falls back to coregex.

// fastPattern is a pattern accepted for the fast paths.
type fastPattern struct {
	parts     []*charClassPart
	anchored  bool // ^ at start
	anchorEnd bool // $ at end
}

// parseFastPattern returns the fast-path form of an AWK regex (already
// prefixed with dotallPrefix), or nil when the fast paths must not be used.
func parseFastPattern(awkPattern string) *fastPattern {
	re, err := syntax.Parse(awkPattern, syntax.Perl) // same flags as coregex.Compile
	if err != nil {
		return nil
	}

	var subs []*syntax.Regexp
	switch re.Op {
	case syntax.OpConcat:
		subs = re.Sub
	default:
		subs = []*syntax.Regexp{re}
	}

	fp := &fastPattern{}
	if len(subs) > 0 && subs[0].Op == syntax.OpBeginText {
		fp.anchored = true
		subs = subs[1:]
	}
	if len(subs) > 0 && subs[len(subs)-1].Op == syntax.OpEndText {
		fp.anchorEnd = true
		subs = subs[:len(subs)-1]
	}
	if len(subs) == 0 {
		return nil
	}

	for _, sub := range subs {
		part, ok := fastPart(sub)
		if !ok {
			return nil
		}
		// Merge adjacent literals so a literal part is one string.
		if n := len(fp.parts); part.literal != "" && n > 0 && fp.parts[n-1].literal != "" {
			fp.parts[n-1].literal += part.literal
			continue
		}
		fp.parts = append(fp.parts, part)
	}

	if !isOnePass(fp.parts) {
		return nil
	}
	return fp
}

// fastPart converts one element of the concatenation.
func fastPart(re *syntax.Regexp) (*charClassPart, bool) {
	switch re.Op {
	case syntax.OpLiteral:
		lit, ok := asciiLiteral(re)
		if !ok {
			return nil, false
		}
		return &charClassPart{literal: lit}, true

	case syntax.OpStar, syntax.OpPlus:
		if re.Flags&syntax.NonGreedy != 0 || len(re.Sub) != 1 {
			return nil, false
		}
		table, ok := byteTable(re.Sub[0])
		if !ok {
			return nil, false
		}
		minMatch := 0
		if re.Op == syntax.OpPlus {
			minMatch = 1
		}
		return &charClassPart{membership: table, minMatch: minMatch}, true
	}
	return nil, false
}

// asciiLiteral returns the literal as a byte string if it is plain ASCII.
func asciiLiteral(re *syntax.Regexp) (string, bool) {
	if re.Flags&syntax.FoldCase != 0 || len(re.Rune) == 0 {
		return "", false
	}
	b := make([]byte, 0, len(re.Rune))
	for _, r := range re.Rune {
		if r >= utf8.RuneSelf {
			return "", false
		}
		b = append(b, byte(r))
	}
	return string(b), true
}

// byteTable returns the membership table for the single-character element of
// x* or x+: a character class, "." or one literal character.
func byteTable(re *syntax.Regexp) (*[256]bool, bool) {
	switch re.Op {
	case syntax.OpAnyChar:
		table := new([256]bool)
		for i := range table {
			table[i] = true
		}
		return table, true

	case syntax.OpAnyCharNotNL:
		table := new([256]bool)
		for i := range table {
			table[i] = i != '\n'
		}
		return table, true

	case syntax.OpLiteral:
		if len(re.Rune) != 1 {
			return nil, false
		}
		lit, ok := asciiLiteral(re)
		if !ok {
			return nil, false
		}
		table := new([256]bool)
		table[lit[0]] = true
		return table, true

	case syntax.OpCharClass:
		return classTable(re.Rune)
	}
	return nil, false
}

// classTable builds a byte table from the rune ranges of a character class
// (pairs lo, hi). It fails when the class contains some but not all
// characters >= U+0080: such a class cannot be matched byte by byte.
func classTable(ranges []rune) (*[256]bool, bool) {
	table := new([256]bool)
	for i := 0; i+1 < len(ranges); i += 2 {
		lo, hi := ranges[i], ranges[i+1]
		for r := lo; r <= hi && r < utf8.RuneSelf; r++ {
			table[r] = true
		}
	}

	switch nonASCIICoverage(ranges) {
	case coverNone:
	case coverAll:
		for i := utf8.RuneSelf; i < len(table); i++ {
			table[i] = true
		}
	default:
		return nil, false
	}
	return table, true
}

const (
	coverNone = iota
	coverAll
	coverPartial
)

// nonASCIICoverage reports whether the class contains none, all or only some
// of the characters U+0080..U+10FFFF. Surrogates (U+D800..U+DFFF) are ignored:
// they are not valid in UTF-8, and coregex reads such bytes as U+FFFD.
func nonASCIICoverage(ranges []rune) int {
	segments := [][2]rune{{utf8.RuneSelf, 0xD7FF}, {0xE000, unicode.MaxRune}}

	covered, total := 0, 0
	for _, seg := range segments {
		total += int(seg[1] - seg[0] + 1)
		for i := 0; i+1 < len(ranges); i += 2 {
			lo, hi := max(ranges[i], seg[0]), min(ranges[i+1], seg[1])
			if lo <= hi {
				covered += int(hi - lo + 1)
			}
		}
	}
	switch covered {
	case 0:
		return coverNone
	case total:
		return coverAll
	}
	return coverPartial
}

// isOnePass reports whether greedy matching without backtracking is exact:
// for every class part, none of the bytes it matches can start the rest of
// the pattern. The rest starts with the first byte of its first literal or
// with any byte of its leading classes, up to and including the first class
// that must match at least once.
func isOnePass(parts []*charClassPart) bool {
	for i, part := range parts {
		if part.membership == nil {
			continue
		}
		for _, next := range parts[i+1:] {
			if next.literal != "" {
				if part.membership[next.literal[0]] {
					return false
				}
				break
			}
			for b := range next.membership {
				if next.membership[b] && part.membership[b] {
					return false
				}
			}
			if next.minMatch > 0 {
				break
			}
		}
	}
	return true
}
