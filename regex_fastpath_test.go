package uawk_test

import (
	"strings"
	"testing"

	"github.com/kolkov/uawk"
)

// TestRegexFastPaths checks AWK regex matching for pattern shapes that uawk
// matches with its byte-table fast paths or literal prefilter instead of the
// regex engine. Most cases disagreed with gawk before the fast paths were
// made exact; the others guard results that must not change. Expected values
// are gawk 5.1.0 output. Non-ASCII text is in input data and regex literals
// only, and positions are checked only where bytes and characters coincide.
func TestRegexFastPaths(t *testing.T) {
	tests := []struct {
		name    string
		program string
		input   string
		want    string
	}{
		{"dot crosses newline in match", `BEGIN { s = "a" "\n" "bc"; print match(s, /.+/), RLENGTH }`, "", "1 4\n"},
		{"dot in class sequence crosses newline", `BEGIN { s = "a" "\n" "b1"; print match(s, /.+[0-9]+/), RLENGTH }`, "", "1 4\n"},
		{"overlapping classes", `BEGIN { print match("abc", /[a-z]+[a-c]+/), RLENGTH }`, "", "1 3\n"},
		{"star matches empty at start", `BEGIN { print match("1a", /[a-z]*/), RLENGTH }`, "", "1 0\n"},
		{"anchored star matches empty", `BEGIN { print ("a" ~ /^[0-9]*/) }`, "", "1\n"},
		{"optional first char", `BEGIN { print ("b_" ~ /^a?[a-z]*/) }`, "", "1\n"},
		{"optional last prefix char", `BEGIN { print ("ab" ~ /^abc*/) }`, "", "1\n"},
		{"optional last required char", `BEGIN { print ("xyz abc" ~ /xyz.*abcd?/) }`, "", "1\n"},
		{"newline escape in bracket", `BEGIN { s = "\n\nx"; print match(s, /[\n]+x/), RLENGTH }`, "", "1 3\n"},
		{"non-ASCII range in input", `{ print ($0 ~ /[а-я]+[0-9]+/) }`, "я1", "1\n"},
		{"non-ASCII range second letter", `{ print ($0 ~ /[а-я]+[0-9]+/) }`, "р1", "1\n"},
		{"non-ASCII outside range", `{ print ($0 ~ /[а-я]+[0-9]+/) }`, "ð1", "0\n"},
		{"non-ASCII outside range Latin", `{ print ($0 ~ /[а-я]+[0-9]+/) }`, "Ñ1", "0\n"},
		{"common pattern stays correct", `{ print match($0, /[a-zA-Z]+[0-9]+/), RLENGTH }`, "The test123 here", "5 7\n"},
		{"non-ASCII list in class sequence", `{ print ($0 ~ /я[é]*/), ($0 ~ /я[é]+2/), ($0 ~ /^xax_я[é]*/) }`, "xax_яé2", "1 1 1\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := ""
			if tt.input != "" {
				input = tt.input + "\n"
			}
			got, err := uawk.Run(tt.program, strings.NewReader(input), nil)
			if err != nil {
				t.Fatalf("Run(%s) error = %v", tt.program, err)
			}
			if got != tt.want {
				t.Errorf("Run(%s) on %q = %q, want %q", tt.program, tt.input, got, tt.want)
			}
		})
	}
}
