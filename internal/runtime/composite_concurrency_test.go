package runtime

import (
	"fmt"
	"sync"
	"testing"
)

// TestCompileCompositeConcurrent compiles composite patterns with custom
// bracket expressions from many goroutines at once. Parallel execution (-j N)
// does this: every worker VM compiles the program's regexes on its own.
// Run with -race: shared mutable state in the composite analyzer shows up here.
//
// No pattern is compiled before the goroutines start, so the first compile of
// each pattern happens concurrently.
func TestCompileCompositeConcurrent(t *testing.T) {
	type tc struct {
		pattern string
		match   string
		noMatch string
	}
	cases := []tc{
		{`[a-f]+[0-9]+`, "zz cafe42 zz", "xyz 42"},
		{`[xyz]+[0-9]+`, "id=xy7", "abc 7"},
		{`[^0-9 ]+[0-9]+`, "key=v9", "12 34"},
		{`[a-z_]+\.[0-9]+`, "file_name.42", "file_name.x"},
	}
	// Distinct bracket expressions: [a-b], [a-c], ... [a-y].
	for c := byte('b'); c <= 'y'; c++ {
		cases = append(cases, tc{fmt.Sprintf("[a-%c]+[0-9]+", c), "a1", "1a"})
	}

	const goroutines = 8

	start := make(chan struct{})
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			<-start
			for i := range cases {
				c := cases[(i+g)%len(cases)]
				re, err := Compile(c.pattern)
				if err != nil {
					t.Errorf("Compile(%q): %v", c.pattern, err)
					return
				}
				if !re.MatchString(c.match) {
					t.Errorf("%q: MatchString(%q) = false, want true", c.pattern, c.match)
				}
				if re.MatchString(c.noMatch) {
					t.Errorf("%q: MatchString(%q) = true, want false", c.pattern, c.noMatch)
				}
			}
		}(g)
	}
	close(start)
	wg.Wait()

	// Each pattern must take the composite fast path, otherwise this test
	// does not exercise the code it is meant to guard.
	for _, c := range cases {
		re, err := Compile(c.pattern)
		if err != nil {
			t.Fatalf("Compile(%q): %v", c.pattern, err)
		}
		if re.composite == nil {
			t.Errorf("Compile(%q): composite fast path not used", c.pattern)
		}
	}
}
