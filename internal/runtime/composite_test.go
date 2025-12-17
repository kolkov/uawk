package runtime

import (
	"testing"
)

func TestCompositeSearcherAlphaDigit(t *testing.T) {
	// The main use case from regex.awk: /[a-zA-Z]+[0-9]+/
	re, err := Compile(`[a-zA-Z]+[0-9]+`)
	if err != nil {
		t.Fatal(err)
	}

	// Verify it uses composite fast path
	if re.composite == nil {
		t.Skip("Pattern not using composite fast path (test still valid)")
	}

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"basic match", "abc123", true},
		{"match in middle", "test abc123 end", true},
		{"uppercase", "ABC123", true},
		{"mixed case", "AbC123", true},
		{"no digits", "abcdef", false},
		{"no letters", "123456", false},
		{"digits first", "123abc", false},
		{"empty", "", false},
		{"only letter", "a", false},
		{"only digit", "1", false},
		{"letter then digit", "a1", true},
		{"multiple sequences", "abc123 def456", true},
		{"separated by space", "abc 123", false},
		{"long sequence", "abcdefghijklmnop1234567890", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := re.MatchString(tt.input); got != tt.expected {
				t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCompositeSearcherFindIndex(t *testing.T) {
	re, err := Compile(`[a-zA-Z]+[0-9]+`)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		input    string
		expected []int
	}{
		{"basic", "abc123", []int{0, 6}},
		{"prefix match", "test123 more", []int{0, 7}},
		{"middle match", "123 abc456 end", []int{4, 10}},
		{"no match", "123 456", nil},
		{"empty", "", nil},
		{"short match", "a1", []int{0, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := re.FindStringIndex(tt.input)
			if tt.expected == nil {
				if got != nil {
					t.Errorf("FindStringIndex(%q) = %v, want nil", tt.input, got)
				}
			} else {
				if got == nil {
					t.Errorf("FindStringIndex(%q) = nil, want %v", tt.input, tt.expected)
				} else if got[0] != tt.expected[0] || got[1] != tt.expected[1] {
					t.Errorf("FindStringIndex(%q) = %v, want %v", tt.input, got, tt.expected)
				}
			}
		})
	}
}

func TestCompositeSearcherAnchored(t *testing.T) {
	// Start anchor
	re1, err := Compile(`^[a-zA-Z]+[0-9]+`)
	if err != nil {
		t.Fatal(err)
	}

	tests1 := []struct {
		name     string
		input    string
		expected bool
	}{
		{"match at start", "abc123", true},
		{"match at start with suffix", "abc123 end", true},
		{"no match at start", "123abc", false},
		{"no match prefix", " abc123", false},
	}

	for _, tt := range tests1 {
		t.Run("start_"+tt.name, func(t *testing.T) {
			if got := re1.MatchString(tt.input); got != tt.expected {
				t.Errorf("^pattern MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}

	// End anchor
	re2, err := Compile(`[a-zA-Z]+[0-9]+$`)
	if err != nil {
		t.Fatal(err)
	}

	tests2 := []struct {
		name     string
		input    string
		expected bool
	}{
		{"match at end", "abc123", true},
		{"match at end with prefix", "start abc123", true},
		{"no match at end", "abc123 ", false},
		{"no match suffix", "abc123x", false},
	}

	for _, tt := range tests2 {
		t.Run("end_"+tt.name, func(t *testing.T) {
			if got := re2.MatchString(tt.input); got != tt.expected {
				t.Errorf("pattern$ MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}

	// Both anchors
	re3, err := Compile(`^[a-zA-Z]+[0-9]+$`)
	if err != nil {
		t.Fatal(err)
	}

	tests3 := []struct {
		name     string
		input    string
		expected bool
	}{
		{"exact match", "abc123", true},
		{"prefix extra", " abc123", false},
		{"suffix extra", "abc123 ", false},
		{"both extra", " abc123 ", false},
	}

	for _, tt := range tests3 {
		t.Run("both_"+tt.name, func(t *testing.T) {
			if got := re3.MatchString(tt.input); got != tt.expected {
				t.Errorf("^pattern$ MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCompositeSearcherThreeParts(t *testing.T) {
	// Three-part pattern: digits, spaces, word chars
	re, err := Compile(`\d+\s+\w+`)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"basic", "123 abc", true},
		{"multiple spaces", "123   abc", true},
		{"tab", "123\tabc", true},
		{"no space", "123abc", false},
		{"no digits", "abc def", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := re.MatchString(tt.input); got != tt.expected {
				t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCompositeSearcherWithStar(t *testing.T) {
	// Pattern with * (zero or more)
	re, err := Compile(`[a-z]*[0-9]+`)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"with letters", "abc123", true},
		{"no letters", "123", true},
		{"just digits", "999", true},
		{"letters after", "abc", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := re.MatchString(tt.input); got != tt.expected {
				t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCompositeSearcherWithLiterals(t *testing.T) {
	// Pattern with literal between char classes
	re, err := Compile(`[a-z]+@[a-z]+`)
	if err != nil {
		t.Fatal(err)
	}

	// Verify it uses composite fast path
	if re.composite == nil {
		t.Skip("Pattern not using composite fast path (test still valid)")
	}

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"basic email-like", "test@domain", true},
		{"in middle", "send to test@domain now", true},
		{"uppercase fails", "TEST@DOMAIN", false},
		{"missing at", "testdomain", false},
		{"empty before at", "@domain", false},
		{"empty after at", "test@", false},
		{"just at", "@", false},
		{"empty", "", false},
		{"multiple at", "a@b@c", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := re.MatchString(tt.input); got != tt.expected {
				t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCompositeSearcherWithPrefixLiteral(t *testing.T) {
	// Pattern with literal prefix
	re, err := Compile(`user[0-9]+`)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"basic", "user123", true},
		{"in text", "create user456 now", true},
		{"no number", "user", false},
		{"wrong prefix", "admin123", false},
		{"partial prefix", "use123", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := re.MatchString(tt.input); got != tt.expected {
				t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCompositeSearcherEmailLike(t *testing.T) {
	// Simple email-like pattern
	re, err := Compile(`[a-z]+@[a-z]+\.[a-z]+`)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"basic email", "test@example.com", true},
		{"in text", "contact test@example.org today", true},
		{"no dot", "test@example", false},
		{"uppercase", "TEST@EXAMPLE.COM", false},
		{"missing domain", "test@.com", false},
		{"missing tld", "test@example.", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := re.MatchString(tt.input); got != tt.expected {
				t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestAnalyzeComposite(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		isComposite bool
	}{
		// Should be composite
		{"alpha_digit", `[a-zA-Z]+[0-9]+`, true},
		{"digit_space_word", `\d+\s+\w+`, true},
		{"lower_upper", `[a-z]+[A-Z]+`, true},
		{"escape_bracket", `\d+[a-z]+`, true},

		// With literals (NEW!)
		{"literal_between", `[a-z]+@[a-z]+`, true},
		{"email_simple", `[a-z]+@[a-z]+\.[a-z]+`, true},
		{"prefix_literal", `user[0-9]+`, true},
		{"suffix_literal", `[a-z]+test`, true},
		{"multi_literal", `[a-z]+abc[0-9]+`, true},
		{"escaped_literal", `[a-z]+\.com`, true},

		// Should NOT be composite (single part - handled by charClass)
		{"single_digit", `\d+`, false},
		{"single_alpha", `[a-zA-Z]+`, false},

		// Should NOT be composite (unsupported)
		{"with_alternation", `[a-z]+|[0-9]+`, false},
		{"no_quantifier", `[a-z][0-9]`, false},
		{"repetition_bound", `[a-z]{2,4}[0-9]+`, false},
		{"all_literals", `abc`, false},
		{"literal_only", `test123`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzeComposite(dotallPrefix + tt.pattern)
			gotComposite := result != nil
			if gotComposite != tt.isComposite {
				t.Errorf("analyzeComposite(%q) composite=%v, want %v", tt.pattern, gotComposite, tt.isComposite)
			}
		})
	}
}

func TestBuildCustomTable(t *testing.T) {
	tests := []struct {
		name    string
		bracket string
		chars   string // Characters that should be in the table
		notIn   string // Characters that should NOT be in the table
	}{
		{"range", "[a-c]", "abc", "defABC123"},
		{"list", "[xyz]", "xyz", "abcABC123"},
		{"mixed", "[a-cx-z]", "abcxyz", "defABC123"},
		{"digit_escape", `[\d]`, "0123456789", "abcABC"},
		{"negated", "[^a-z]", "ABC123!@#", "abcxyz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table, ok := buildCustomTable(tt.bracket)
			if !ok {
				t.Fatalf("buildCustomTable(%q) failed", tt.bracket)
			}

			for _, c := range tt.chars {
				if !table[byte(c)] {
					t.Errorf("table[%q] = false, want true", c)
				}
			}
			for _, c := range tt.notIn {
				if table[byte(c)] {
					t.Errorf("table[%q] = true, want false", c)
				}
			}
		})
	}
}

// BenchmarkCompositeVsCoregex compares composite fast path with full coregex
func BenchmarkCompositeAlphaDigit(b *testing.B) {
	re, _ := Compile(`[a-zA-Z]+[0-9]+`)
	input := "The test123 contains abc456 and xyz789 patterns"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		re.MatchString(input)
	}
}

func BenchmarkCompositeThreeParts(b *testing.B) {
	re, _ := Compile(`\d+\s+\w+`)
	input := "Values: 123 abc and 456 def with 789 ghi"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		re.MatchString(input)
	}
}

func BenchmarkCompositeAnchored(b *testing.B) {
	re, _ := Compile(`^[a-zA-Z]+[0-9]+`)
	input := "test123 followed by other text and more content here"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		re.MatchString(input)
	}
}

func BenchmarkCompositeFindIndex(b *testing.B) {
	re, _ := Compile(`[a-zA-Z]+[0-9]+`)
	input := "Some prefix text then abc123 and more suffix text"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		re.FindStringIndex(input)
	}
}

// Benchmark similar pattern with coregex (for comparison)
func BenchmarkCoregexAlphaDigit(b *testing.B) {
	// Use a pattern that won't be recognized as composite
	re, _ := Compile(`[a-zA-Z]+[0-9]+[a-z]?`)
	input := "The test123 contains abc456 and xyz789 patterns"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		re.MatchString(input)
	}
}
