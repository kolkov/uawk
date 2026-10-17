package runtime

import (
	"math/rand"
	"strings"
	"testing"
)

// checkFastPaths compares the fast path Compile selects for pattern with the
// reference result on input. It calls selectFastPaths directly, so it skips
// the coregex compile that the fast paths do not depend on.
func checkFastPaths(t *testing.T, pattern, input string) {
	t.Helper()
	charClass, composite, literals := selectFastPaths(dotallPrefix + pattern)
	compareFastPaths(t, pattern, input, charClass, composite, literals)
}

// TestFastPathAdmission checks that common AWK patterns keep their fast path,
// and that patterns it cannot match exactly do not get one.
func TestFastPathAdmission(t *testing.T) {
	fast := []string{
		`\d+`, `\s+`, `\w+`, `[a-z]+`, `[A-Za-z]+`, `.+`, `\D+`, `[^0-9]+`, `^[0-9]+$`,
		`[a-zA-Z]+[0-9]+`, `\d+\s+\w+`, `user[0-9]+`, `[a-z]+@[a-z]+\.[a-z]+`, `[a-z]+\.com`,
	}
	slow := []string{
		`[a-z]+[a-c]+`, `[a-z]+test`, `[а-я]+`, `[а-я]+[0-9]+`, `\pL+`, `[a-z]+?[0-9]+`,
		`(?i)[a-z]+[0-9]+`, `[a-z]{2}[0-9]+`, `([a-z]+)[0-9]+`, `[a-z]+|[0-9]+`,
	}
	for _, p := range fast {
		re := MustCompile(p)
		if re.charClass == nil && re.composite == nil {
			t.Errorf("%q: expected a fast path", p)
		}
	}
	for _, p := range slow {
		re := MustCompile(p)
		if re.charClass != nil || re.composite != nil {
			t.Errorf("%q: expected no char-class or composite fast path", p)
		}
	}
}

// Building blocks for generated patterns: ASCII and non-ASCII classes,
// escapes, anchors and quantifiers the fast paths accept or must reject.
var (
	diffAtoms = []string{
		`\d`, `\s`, `\w`, `\D`, `\S`, `\W`, `.`, `[a-z]`, `[A-Z]`, `[0-9]`, `[a-zA-Z]`,
		`[a-f]`, `[xyz]`, `[^0-9]`, `[^a]`, `[a-c]`, `[а-я]`, `[^я]`, `[é]`, `[[:alpha:]]`,
		`[[:space:]]`, `\pL`, `[a-ÿ]`, `[\s]`, `[^\s]`, `[\n]`, `[^\n]`, `[ -~]`,
		`[\x00-\x7f]`, `[^\x00-\x7f]`, `a`, `\.`,
	}
	diffQuants = []string{"+", "*", "+", "*", "+?", "?", "{2}"}
	diffLits   = []string{`a`, `b`, `x`, `1`, `@`, `\.`, `-`, `_`, `я`, `\n`, ` `, `(?i)a`, `abc`}
	diffBytes  = []string{"a", "b", "c", "x", "1", "2", "_", " ", "\n", "\t", "\v", ".", "@", "-", "я", "Ñ", "é", "\xff", "\xd1"}
)

func genDiffPattern(r *rand.Rand) string {
	var sb strings.Builder
	if r.Intn(5) == 0 {
		sb.WriteString("^")
	}
	for n := 1 + r.Intn(4); n > 0; n-- {
		if r.Intn(3) == 0 {
			sb.WriteString(diffLits[r.Intn(len(diffLits))])
		} else {
			sb.WriteString(diffAtoms[r.Intn(len(diffAtoms))])
			sb.WriteString(diffQuants[r.Intn(len(diffQuants))])
		}
	}
	if r.Intn(5) == 0 {
		sb.WriteString("$")
	}
	return sb.String()
}

func genDiffInput(r *rand.Rand) string {
	var sb strings.Builder
	for n := r.Intn(12); n > 0; n-- {
		if r.Intn(4) == 0 {
			sb.WriteByte(byte(r.Intn(128)))
		} else {
			sb.WriteString(diffBytes[r.Intn(len(diffBytes))])
		}
	}
	return sb.String()
}

// TestFastPathMatchesStdlib compares the fast paths with the reference on
// generated patterns and inputs (fixed seed, so failures reproduce).
func TestFastPathMatchesStdlib(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	patterns, inputs := 3000, 25
	if testing.Short() {
		patterns = 500
	}
	for i := 0; i < patterns; i++ {
		p := genDiffPattern(r)
		for j := 0; j < inputs; j++ {
			checkFastPaths(t, p, genDiffInput(r))
		}
		if t.Failed() {
			return
		}
	}
}

// FuzzFastPath runs the same comparison on fuzzer-chosen patterns and inputs:
//
//	go test -run '^$' -fuzz FuzzFastPath -fuzztime 60s ./internal/runtime/
func FuzzFastPath(f *testing.F) {
	seeds := [][2]string{
		{`[a-zA-Z]+[0-9]+`, "abc123"}, {`\d+\s+\w+`, "12  ab"}, {`[a-z]+[a-c]+`, "abc"},
		{`.+`, "a\nb"}, {`[а-я]+[0-9]+`, "я1"}, {`^a?[a-z]*`, "b"}, {`[a-z]*`, "1a"},
		{`user[0-9]+`, "user42"}, {`[^\s]+ \S+`, "\xff  z"},
	}
	for _, s := range seeds {
		f.Add(s[0], s[1])
	}
	f.Fuzz(func(t *testing.T, pattern, input string) {
		if len(pattern) > 64 || len(input) > 256 {
			return
		}
		checkFastPaths(t, pattern, input)
	})
}
