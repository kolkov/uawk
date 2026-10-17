package uawk_test

import (
	"strings"
	"testing"

	"github.com/kolkov/uawk"
)

// TestRegexAcrossNewlines checks regex matching on strings that contain
// newlines. In AWK "." matches newline (uawk compiles every regex with (?s)),
// so ".*X" must start at the beginning of the string and end after the last
// X, and the leftmost match wins. Expected values are gawk 5.1.0 output.
//
// coregex v0.12.15-v0.12.29 clipped (?s).*X matches to the line of X
// (coregx/coregex#191); some cases were already wrong with v0.12.8.
func TestRegexAcrossNewlines(t *testing.T) {
	tests := []struct {
		name    string
		program string
		want    string
	}{
		{"match .* prefix across newline", `BEGIN { match("x\ny@example.com", /.*@example\.com/); print RSTART, RLENGTH }`, "1 15\n"},
		{"match .* greedy over several lines", `BEGIN { match("q.txt\nz\nb.txt", /.*\.txt/); print RSTART, RLENGTH }`, "1 13\n"},
		{"gsub .* one match over several lines", `BEGIN { t = "q.txt\nz\nb.txt"; n = gsub(/.*\.txt/, "<>", t); print n, t }`, "1 <>\n"},
		{"sub .* prefix across newline", `BEGIN { t = "x\ny@example.com"; sub(/.*@/, "<>", t); print t }`, "<>example.com\n"},
		{"split on .* prefix across newline", `BEGIN { n = split("a\nb.txt", a, /.*\./); print n, a[1] "|" a[2] }`, "2 |txt\n"},
		{"match .* with suffix alternation", `BEGIN { match("a\nb.log", /.*\.(txt|log)/); print RSTART, RLENGTH }`, "1 7\n"},
		{"match leftmost with suffix alternation", `BEGIN { match("a.txt b.log", /[a-z]+\.(txt|log)/); print RSTART, RLENGTH }`, "1 5\n"},
		{"gsub .* with class before suffix", `BEGIN { t = "a\n12.log\nb3.log"; n = gsub(/.*[0-9]+\.log/, "<>", t); print n, t }`, "1 <>\n"},
		{"match .* after leading newlines", `BEGIN { match("\n\nx\n", /.*x/); print RSTART, RLENGTH }`, "1 3\n"},
		{"match .* alternation no match", `BEGIN { match("xbaz", /.*(foo|bar)baz/); print RSTART, RLENGTH }`, "0 -1\n"},
		{"~ .* alternation no match", `BEGIN { print ("xbaz" ~ /.*(foo|bar)baz/) }`, "0\n"},
		{"sub .* alternation no match", `BEGIN { t = "xbaz"; n = sub(/.*(foo|bar)baz/, "<>", t); print n, t }`, "0 xbaz\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := uawk.Run(tt.program, strings.NewReader(""), nil)
			if err != nil {
				t.Fatalf("Run(%s) error = %v", tt.program, err)
			}
			if got != tt.want {
				t.Errorf("Run(%s) = %q, want %q", tt.program, got, tt.want)
			}
		})
	}
}
