package runtime

import (
	"testing"
)

func TestCharClassSearcherDigit(t *testing.T) {
	re, err := Compile(`\d+`)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		input    string
		expected bool
	}{
		{"123", true},
		{"abc", false},
		{"abc123", true},
		{"123abc", true},
		{"", false},
		{"a1b2c3", true},
	}

	for _, tt := range tests {
		if got := re.MatchString(tt.input); got != tt.expected {
			t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestCharClassSearcherSpace(t *testing.T) {
	re, err := Compile(`\s+`)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		input    string
		expected bool
	}{
		{"  ", true},
		{"abc", false},
		{"abc def", true},
		{"\t\n", true},
		{"", false},
	}

	for _, tt := range tests {
		if got := re.MatchString(tt.input); got != tt.expected {
			t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestCharClassSearcherWord(t *testing.T) {
	re, err := Compile(`\w+`)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		input    string
		expected bool
	}{
		{"hello", true},
		{"hello_world", true},
		{"123", true},
		{"   ", false},
		{"", false},
		{"hello world", true},
	}

	for _, tt := range tests {
		if got := re.MatchString(tt.input); got != tt.expected {
			t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestCharClassSearcherAnchored(t *testing.T) {
	re, err := Compile(`^\d+`)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		input    string
		expected bool
	}{
		{"123abc", true},
		{"abc123", false},
		{"123", true},
		{"", false},
	}

	for _, tt := range tests {
		if got := re.MatchString(tt.input); got != tt.expected {
			t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestCharClassSearcherEndAnchored(t *testing.T) {
	re, err := Compile(`\d+$`)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		input    string
		expected bool
	}{
		{"abc123", true},
		{"123abc", false},
		{"123", true},
		{"", false},
	}

	for _, tt := range tests {
		if got := re.MatchString(tt.input); got != tt.expected {
			t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestCharClassSearcherFindIndex(t *testing.T) {
	re, err := Compile(`\d+`)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		input    string
		expected []int
	}{
		{"abc123def", []int{3, 6}},
		{"123", []int{0, 3}},
		{"abc", nil},
		{"", nil},
		{"12abc34", []int{0, 2}},
	}

	for _, tt := range tests {
		got := re.FindStringIndex(tt.input)
		if tt.expected == nil {
			if got != nil {
				t.Errorf("FindStringIndex(%q) = %v, want nil", tt.input, got)
			}
		} else {
			if got == nil || got[0] != tt.expected[0] || got[1] != tt.expected[1] {
				t.Errorf("FindStringIndex(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		}
	}
}

// BenchmarkCharClassVsRegex compares CharClassSearcher with full regex
func BenchmarkCharClassDigit(b *testing.B) {
	re, _ := Compile(`\d+`)
	input := "The year 2024 is here with 365 days and 8760 hours"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		re.MatchString(input)
	}
}

func BenchmarkCharClassSpace(b *testing.B) {
	re, _ := Compile(`\s+`)
	input := "The year 2024 is here with 365 days and 8760 hours"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		re.MatchString(input)
	}
}

func BenchmarkCharClassWord(b *testing.B) {
	re, _ := Compile(`\w+`)
	input := "The year 2024 is here with 365 days and 8760 hours"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		re.MatchString(input)
	}
}

// Benchmark complex regex that doesn't use fast path
func BenchmarkComplexRegex(b *testing.B) {
	re, _ := Compile(`\d{4}`)
	input := "The year 2024 is here with 365 days and 8760 hours"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		re.MatchString(input)
	}
}

func BenchmarkFindDigit(b *testing.B) {
	re, _ := Compile(`\d+`)
	input := "The year 2024 is here with 365 days and 8760 hours"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		re.FindStringIndex(input)
	}
}

// Tests for negated character classes and dot patterns (P2-011)

func TestCharClassSearcherNonDigit(t *testing.T) {
	re, err := Compile(`\D+`)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"letters only", "abc", true},
		{"digits only", "123", false},
		{"mixed starts letter", "abc123", true},
		{"mixed starts digit", "123abc", true},
		{"empty", "", false},
		{"spaces", "   ", true},
		{"special chars", "!@#", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := re.MatchString(tt.input); got != tt.expected {
				t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCharClassSearcherNonSpace(t *testing.T) {
	re, err := Compile(`\S+`)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"letters", "abc", true},
		{"spaces only", "   ", false},
		{"tabs only", "\t\t", false},
		{"mixed", "abc def", true},
		{"empty", "", false},
		{"digits", "123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := re.MatchString(tt.input); got != tt.expected {
				t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCharClassSearcherNonWord(t *testing.T) {
	re, err := Compile(`\W+`)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"letters", "abc", false},
		{"digits", "123", false},
		{"underscore", "_", false},
		{"spaces", "   ", true},
		{"punctuation", "!@#", true},
		{"mixed", "abc!def", true},
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

func TestCharClassSearcherDot(t *testing.T) {
	re, err := Compile(`.+`)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"letters", "abc", true},
		{"digits", "123", true},
		{"spaces", "   ", true},
		{"special", "!@#", true},
		{"empty", "", false},
		{"newline only", "\n", false},
		{"with newline", "abc\ndef", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := re.MatchString(tt.input); got != tt.expected {
				t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCharClassSearcherDotStar(t *testing.T) {
	re, err := Compile(`.*`)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"letters", "abc", true},
		{"empty", "", true},          // * matches zero chars
		{"newline only", "\n", true}, // * matches empty before newline
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := re.MatchString(tt.input); got != tt.expected {
				t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCharClassSearcherNegatedFindIndex(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		input    string
		expected []int
	}{
		{"nondigit in digits", `\D+`, "123abc456", []int{3, 6}},
		{"nonspace in spaces", `\S+`, "   hello   ", []int{3, 8}},
		{"nonword in words", `\W+`, "abc...def", []int{3, 6}},
		{"dot match", `.+`, "hello", []int{0, 5}},
		{"dot stops at newline", `.+`, "hello\nworld", []int{0, 5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re, err := Compile(tt.pattern)
			if err != nil {
				t.Fatal(err)
			}
			got := re.FindStringIndex(tt.input)
			if tt.expected == nil {
				if got != nil {
					t.Errorf("FindStringIndex(%q) = %v, want nil", tt.input, got)
				}
			} else {
				if got == nil || got[0] != tt.expected[0] || got[1] != tt.expected[1] {
					t.Errorf("FindStringIndex(%q) = %v, want %v", tt.input, got, tt.expected)
				}
			}
		})
	}
}

// Benchmarks for new patterns

func BenchmarkCharClassNonDigit(b *testing.B) {
	re, _ := Compile(`\D+`)
	input := "The year 2024 is here with 365 days and 8760 hours"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		re.MatchString(input)
	}
}

func BenchmarkCharClassNonSpace(b *testing.B) {
	re, _ := Compile(`\S+`)
	input := "The year 2024 is here with 365 days and 8760 hours"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		re.MatchString(input)
	}
}

func BenchmarkCharClassDot(b *testing.B) {
	re, _ := Compile(`.+`)
	input := "The year 2024 is here with 365 days and 8760 hours"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		re.MatchString(input)
	}
}
