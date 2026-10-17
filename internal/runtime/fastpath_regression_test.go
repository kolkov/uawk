package runtime

import (
	"fmt"
	"regexp"
	"testing"
	"time"
)

// The fast paths (char class, composite, literal prefilter) must agree with
// the regex engine. The reference here is the standard library regexp with
// the same flags uawk uses ((?s), leftmost-longest): it implements the syntax
// coregex follows, and it keeps these tests independent of coregex bugs.

// compareFastPaths checks the given fast paths for pattern against the
// reference result on input. Nil fast paths are skipped.
func compareFastPaths(t *testing.T, pattern, input string, charClass *CharClassSearcher, composite *CompositeSearcher, literals *LiteralInfo) {
	t.Helper()
	ref, err := regexp.Compile(dotallPrefix + pattern)
	if err != nil {
		return
	}
	ref.Longest()
	want := ref.FindStringIndex(input)

	if charClass != nil {
		if got := charClass.FindStringIndex(input); fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("char class %q on %q: FindStringIndex = %v, want %v", pattern, input, got, want)
		}
		if got := charClass.MatchString(input); got != (want != nil) {
			t.Errorf("char class %q on %q: MatchString = %v, want %v", pattern, input, got, want != nil)
		}
	}
	if composite != nil {
		if got := composite.FindStringIndex(input); fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("composite %q on %q: FindStringIndex = %v, want %v", pattern, input, got, want)
		}
		if got := composite.MatchString(input); got != (want != nil) {
			t.Errorf("composite %q on %q: MatchString = %v, want %v", pattern, input, got, want != nil)
		}
	}
	if literals != nil && want != nil && literals.CanReject(input) {
		t.Errorf("prefilter %q rejects %q, but it matches at %v (literals %+v)", pattern, input, want, *literals)
	}
}

// TestFastPathRegressions lists cases the fast paths used to get wrong.
func TestFastPathRegressions(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		input   string
	}{
		// "." matches newline in AWK.
		{"dot crosses newline", `.+`, "a\nbc"},
		{"dot in composite crosses newline", `.+[0-9]+`, "a\nb1"},
		// A class followed by an overlapping class needs backtracking.
		{"overlapping classes", `[a-z]+[a-c]+`, "abc"},
		{"overlapping star then plus", `\w*\D+$`, "x"},
		{"overlapping classes before literal", `\w*[^0-9]*1`, "1\xd1b"},
		// x* matches the empty string at the leftmost position.
		{"star leftmost empty", `[a-z]*`, "1a"},
		{"anchored star empty", `^\d*`, "a"},
		// Non-ASCII brackets were parsed byte by byte.
		// The old table for [а-я] held bytes D0, 8F and B0-D1: р (D1 80) was
		// missed, ð (C3 B0) matched.
		{"non-ASCII range false negative", `[а-я]+[0-9]+`, "р1"},
		{"non-ASCII range false positive", `[а-я]+[0-9]+`, "ð1"},
		{"non-ASCII list", `я[é]*`, "xax_я2caÑ"},
		// Escapes inside brackets follow Go syntax.
		{"newline escape in bracket", `[\n]+x`, "\n\nx"},
		{"tab escape in bracket", `[\t]+x`, "\t\tx"},
		{"vertical tab is not \\s", `\s+x`, "\vx"},
		{"negated escape in bracket", `[\D]+1`, "ab1"},
		// The prefilter took optional literals as required.
		{"optional prefix char", `^a?[a-z]*`, "b_"},
		{"optional last prefix char", `^abc*`, "ab"},
		{"optional last required char", `xyz.*abcd?`, "xyz abc"},
		{"case-insensitive literal", `(?i)error`, "ERROR"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			re, err := Compile(c.pattern)
			if err != nil {
				t.Fatalf("Compile(%q): %v", c.pattern, err)
			}
			compareFastPaths(t, c.pattern, c.input, re.charClass, re.composite, re.literals)
		})
	}
}

// TestCompileTerminates guards against the byte-range loop that never ended
// for a range up to 0xFF ([a-\xff]); such patterns can come from input data
// as dynamic regexes.
func TestCompileTerminates(t *testing.T) {
	for _, pattern := range []string{"[a-\xff]+[0-9]+", "[\x00-\xff]+x", "[^\xfe-\xff]+1", "x[\xff]+"} {
		done := make(chan struct{})
		go func() {
			defer close(done)
			re, err := Compile(pattern)
			if err == nil {
				re.MatchString("a1")
			}
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("Compile(%q) did not return within 5s", pattern)
		}
	}
}
