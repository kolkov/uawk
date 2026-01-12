package runtime

import (
	"testing"
)

func TestCharClassBitmapContains(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() CharClassBitmap
		testVal byte
		want    bool
	}{
		{
			name: "digit bitmap contains 5",
			setup: func() CharClassBitmap {
				var b CharClassBitmap
				b.SetRange('0', '9')
				return b
			},
			testVal: '5',
			want:    true,
		},
		{
			name: "digit bitmap not contains a",
			setup: func() CharClassBitmap {
				var b CharClassBitmap
				b.SetRange('0', '9')
				return b
			},
			testVal: 'a',
			want:    false,
		},
		{
			name: "alpha bitmap contains Z",
			setup: func() CharClassBitmap {
				var b CharClassBitmap
				b.SetRange('a', 'z')
				b.SetRange('A', 'Z')
				return b
			},
			testVal: 'Z',
			want:    true,
		},
		{
			name: "word bitmap contains underscore",
			setup: func() CharClassBitmap {
				var b CharClassBitmap
				b.SetRange('a', 'z')
				b.SetRange('A', 'Z')
				b.SetRange('0', '9')
				b.Set('_')
				return b
			},
			testVal: '_',
			want:    true,
		},
		{
			name: "empty bitmap contains nothing",
			setup: func() CharClassBitmap {
				return CharClassBitmap{}
			},
			testVal: 'x',
			want:    false,
		},
		{
			name: "byte 0",
			setup: func() CharClassBitmap {
				var b CharClassBitmap
				b.Set(0)
				return b
			},
			testVal: 0,
			want:    true,
		},
		{
			name: "byte 255",
			setup: func() CharClassBitmap {
				var b CharClassBitmap
				b.Set(255)
				return b
			},
			testVal: 255,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := tt.setup()
			if got := b.Contains(tt.testVal); got != tt.want {
				t.Errorf("Contains(%d) = %v, want %v", tt.testVal, got, tt.want)
			}
		})
	}
}

func TestCharClassBitmapInvert(t *testing.T) {
	var b CharClassBitmap
	b.SetRange('0', '9')

	inverted := b.Invert()

	// Digits should not be in inverted
	for c := byte('0'); c <= '9'; c++ {
		if inverted.Contains(c) {
			t.Errorf("inverted.Contains(%q) = true, want false", c)
		}
	}

	// Letters should be in inverted
	for c := byte('a'); c <= 'z'; c++ {
		if !inverted.Contains(c) {
			t.Errorf("inverted.Contains(%q) = false, want true", c)
		}
	}
}

func TestCharClassBitmapFromBoolTable(t *testing.T) {
	// Test that bitmap matches the original bool table
	var b CharClassBitmap
	b.FromBoolTable(&digitTable)

	for i := 0; i < 256; i++ {
		c := byte(i)
		got := b.Contains(c)
		want := digitTable[c]
		if got != want {
			t.Errorf("bitmap.Contains(%d) = %v, digitTable[%d] = %v", c, got, c, want)
		}
	}
}

func TestPrebuiltBitmaps(t *testing.T) {
	tests := []struct {
		name   string
		bitmap *CharClassBitmap
		table  *[256]bool
	}{
		{"digit", &digitBitmap, &digitTable},
		{"space", &spaceBitmap, &spaceTable},
		{"word", &wordBitmap, &wordTable},
		{"alpha", &alphaBitmap, &alphaTable},
		{"lower", &lowerBitmap, &lowerTable},
		{"upper", &upperBitmap, &upperTable},
		{"nonDigit", &nonDigitBitmap, &nonDigitTable},
		{"nonSpace", &nonSpaceBitmap, &nonSpaceTable},
		{"nonWord", &nonWordBitmap, &nonWordTable},
		{"dot", &dotBitmap, &dotTable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < 256; i++ {
				c := byte(i)
				got := tt.bitmap.Contains(c)
				want := tt.table[c]
				if got != want {
					t.Errorf("bitmap.Contains(%d) = %v, table[%d] = %v", c, got, c, want)
				}
			}
		})
	}
}

func TestFastCharClassSearcherDigit(t *testing.T) {
	searcher := NewFastDigitSearcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"digits only", "123", true},
		{"letters only", "abc", false},
		{"digits at start", "123abc", true},
		{"digits at end", "abc123", true},
		{"empty", "", false},
		{"single digit", "5", true},
		{"mixed", "a1b2c3", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := searcher.MatchString(tt.input); got != tt.expected {
				t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFastCharClassSearcherWord(t *testing.T) {
	searcher := NewFastWordSearcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"letters", "hello", true},
		{"with underscore", "hello_world", true},
		{"digits", "123", true},
		{"spaces only", "   ", false},
		{"empty", "", false},
		{"mixed", "hello world", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := searcher.MatchString(tt.input); got != tt.expected {
				t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFastCharClassSearcherAlpha(t *testing.T) {
	searcher := NewFastAlphaSearcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"lowercase", "abc", true},
		{"uppercase", "ABC", true},
		{"mixed case", "AbC", true},
		{"digits only", "123", false},
		{"empty", "", false},
		{"with spaces", "a b c", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := searcher.MatchString(tt.input); got != tt.expected {
				t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFastCharClassSearcherAnchored(t *testing.T) {
	searcher := &FastCharClassSearcher{
		bitmap:     &digitBitmap,
		rangeCheck: digitMatcher{},
		minMatch:   1,
		anchored:   true,
	}

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"starts with digit", "123abc", true},
		{"ends with digit", "abc123", false},
		{"all digits", "123", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := searcher.MatchString(tt.input); got != tt.expected {
				t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFastCharClassSearcherAnchorEnd(t *testing.T) {
	searcher := &FastCharClassSearcher{
		bitmap:     &digitBitmap,
		rangeCheck: digitMatcher{},
		minMatch:   1,
		anchorEnd:  true,
	}

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"ends with digit", "abc123", true},
		{"starts with digit", "123abc", false},
		{"all digits", "123", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := searcher.MatchString(tt.input); got != tt.expected {
				t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFastCharClassSearcherFindString(t *testing.T) {
	searcher := NewFastDigitSearcher()

	tests := []struct {
		name      string
		input     string
		wantStart int
		wantEnd   int
		wantFound bool
	}{
		{"digits in middle", "abc123def", 3, 6, true},
		{"all digits", "123", 0, 3, true},
		{"no digits", "abc", 0, 0, false},
		{"empty", "", 0, 0, false},
		{"multiple groups", "12abc34", 0, 2, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, found := searcher.FindString(tt.input)
			if found != tt.wantFound {
				t.Errorf("FindString(%q) found = %v, want %v", tt.input, found, tt.wantFound)
			}
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("FindString(%q) = (%d, %d), want (%d, %d)", tt.input, start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestFastCharClassSearcherFindStringIndex(t *testing.T) {
	searcher := NewFastDigitSearcher()

	tests := []struct {
		name     string
		input    string
		expected []int
	}{
		{"digits in middle", "abc123def", []int{3, 6}},
		{"all digits", "123", []int{0, 3}},
		{"no digits", "abc", nil},
		{"empty", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := searcher.FindStringIndex(tt.input)
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

func TestSWARContainsDigit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"all digits", "12345678", true},
		{"no digits", "abcdefgh", false},
		{"one digit at start", "1abcdefg", true},
		{"one digit at end", "abcdefg7", true},
		{"one digit middle", "abc5defg", true},
		{"spaces", "        ", false},
		{"mixed", "abc12def", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.input) != 8 {
				t.Fatalf("test input must be 8 bytes, got %d", len(tt.input))
			}
			word := uint64(tt.input[0]) |
				uint64(tt.input[1])<<8 |
				uint64(tt.input[2])<<16 |
				uint64(tt.input[3])<<24 |
				uint64(tt.input[4])<<32 |
				uint64(tt.input[5])<<40 |
				uint64(tt.input[6])<<48 |
				uint64(tt.input[7])<<56

			if got := SWARContainsDigit(word); got != tt.expected {
				t.Errorf("SWARContainsDigit(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSWARContainsSpace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"all spaces", "        ", true},
		{"no spaces", "abcdefgh", false},
		{"one space", "abcd efg", true},
		{"tab", "abcdefg\t", true},
		{"newline", "abcdef\ng", true},
		{"digits", "12345678", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.input) != 8 {
				t.Fatalf("test input must be 8 bytes, got %d", len(tt.input))
			}
			word := uint64(tt.input[0]) |
				uint64(tt.input[1])<<8 |
				uint64(tt.input[2])<<16 |
				uint64(tt.input[3])<<24 |
				uint64(tt.input[4])<<32 |
				uint64(tt.input[5])<<40 |
				uint64(tt.input[6])<<48 |
				uint64(tt.input[7])<<56

			if got := SWARContainsSpace(word); got != tt.expected {
				t.Errorf("SWARContainsSpace(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// TestRangeMatcherCorrectness verifies range matchers match bitmap results
func TestRangeMatcherCorrectness(t *testing.T) {
	tests := []struct {
		name    string
		matcher rangeMatcher
		bitmap  *CharClassBitmap
	}{
		{"digit", digitMatcher{}, &digitBitmap},
		{"lower", lowerMatcher{}, &lowerBitmap},
		{"upper", upperMatcher{}, &upperBitmap},
		{"alpha", alphaMatcher{}, &alphaBitmap},
		{"word", wordMatcher{}, &wordBitmap},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < 256; i++ {
				c := byte(i)
				matcherResult := tt.matcher.Match(c)
				bitmapResult := tt.bitmap.Contains(c)
				if matcherResult != bitmapResult {
					t.Errorf("byte %d (%q): matcher=%v, bitmap=%v",
						c, c, matcherResult, bitmapResult)
				}
			}
		})
	}
}

// TestConsistencyWithCharClassSearcher verifies FastCharClassSearcher matches CharClassSearcher
func TestConsistencyWithCharClassSearcher(t *testing.T) {
	testCases := []struct {
		name        string
		fast        *FastCharClassSearcher
		original    *CharClassSearcher
		description string
	}{
		{
			name:        "digit unanchored",
			fast:        NewFastDigitSearcher(),
			original:    &CharClassSearcher{membership: digitTable, minMatch: 1},
			description: "\\d+",
		},
		{
			name:        "word unanchored",
			fast:        NewFastWordSearcher(),
			original:    &CharClassSearcher{membership: wordTable, minMatch: 1},
			description: "\\w+",
		},
		{
			name: "digit anchored start",
			fast: &FastCharClassSearcher{
				bitmap:     &digitBitmap,
				rangeCheck: digitMatcher{},
				minMatch:   1,
				anchored:   true,
			},
			original:    &CharClassSearcher{membership: digitTable, minMatch: 1, anchored: true},
			description: "^\\d+",
		},
		{
			name: "digit anchored end",
			fast: &FastCharClassSearcher{
				bitmap:     &digitBitmap,
				rangeCheck: digitMatcher{},
				minMatch:   1,
				anchorEnd:  true,
			},
			original:    &CharClassSearcher{membership: digitTable, minMatch: 1, anchorEnd: true},
			description: "\\d+$",
		},
	}

	inputs := []string{
		"",
		"123",
		"abc",
		"abc123",
		"123abc",
		"abc123def",
		"   ",
		"a1b2c3",
		"The year 2024 is here with 365 days",
		"no digits here at all",
		"12345678901234567890",
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, input := range inputs {
				// Test MatchString
				fastMatch := tc.fast.MatchString(input)
				origMatch := tc.original.MatchString(input)
				if fastMatch != origMatch {
					t.Errorf("%s: MatchString(%q): fast=%v, original=%v",
						tc.description, input, fastMatch, origMatch)
				}

				// Test FindStringIndex
				fastIdx := tc.fast.FindStringIndex(input)
				origIdx := tc.original.FindStringIndex(input)
				if (fastIdx == nil) != (origIdx == nil) {
					t.Errorf("%s: FindStringIndex(%q): fast=%v, original=%v",
						tc.description, input, fastIdx, origIdx)
				} else if fastIdx != nil && (fastIdx[0] != origIdx[0] || fastIdx[1] != origIdx[1]) {
					t.Errorf("%s: FindStringIndex(%q): fast=%v, original=%v",
						tc.description, input, fastIdx, origIdx)
				}
			}
		})
	}
}

// TestLongStrings verifies batch processing works correctly
func TestLongStrings(t *testing.T) {
	searcher := NewFastDigitSearcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"100 chars no match", "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!@#$%^&*()_+-={}[]|\\:\";<>?,./`~abcdefghij", false},
		{"100 chars digit at end", "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!@#$%^&*()_+-={}[]|\\:\";<>?,./`~abcdefghi5", true},
		{"100 chars digit at start", "5bcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!@#$%^&*()_+-={}[]|\\:\";<>?,./`~abcdefghij", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := searcher.MatchString(tt.input); got != tt.expected {
				t.Errorf("MatchString(%q...) = %v, want %v", tt.input[:20], got, tt.expected)
			}
		})
	}
}
