package runtime

import (
	"testing"
)

func TestCompile(t *testing.T) {
	tests := []struct {
		pattern string
		wantErr bool
	}{
		{"hello", false},
		{"^[a-z]+$", false},
		{"[0-9]+", false},
		{"(foo|bar)", false},
		{"\\d+", false},
		{".*\\.txt$", false},
		{"[invalid", true},
		{"(unclosed", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			re, err := Compile(tt.pattern)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for pattern %q", tt.pattern)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if re == nil {
				t.Error("expected non-nil Regex")
			}
			if re.Pattern() != tt.pattern {
				t.Errorf("Pattern() = %q, want %q", re.Pattern(), tt.pattern)
			}
		})
	}
}

func TestMustCompile(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid pattern")
		}
	}()
	MustCompile("[invalid")
}

func TestMatchString(t *testing.T) {
	tests := []struct {
		pattern string
		input   string
		want    bool
	}{
		{"hello", "hello world", true},
		{"hello", "goodbye world", false},
		{"^hello", "hello world", true},
		{"^hello", "say hello", false},
		{"world$", "hello world", true},
		{"world$", "world hello", false},
		{"[0-9]+", "abc123def", true},
		{"[0-9]+", "abcdef", false},
		{"^$", "", true},
		{"^$", "x", false},
		{"foo|bar", "foo", true},
		{"foo|bar", "bar", true},
		{"foo|bar", "baz", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.input, func(t *testing.T) {
			re := MustCompile(tt.pattern)
			if got := re.MatchString(tt.input); got != tt.want {
				t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFindStringIndex(t *testing.T) {
	tests := []struct {
		pattern string
		input   string
		want    []int
	}{
		{"hello", "hello world", []int{0, 5}},
		{"world", "hello world", []int{6, 11}},
		{"[0-9]+", "abc123def", []int{3, 6}},
		{"xyz", "hello world", nil},
		{"^", "hello", []int{0, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.input, func(t *testing.T) {
			re := MustCompile(tt.pattern)
			got := re.FindStringIndex(tt.input)
			if tt.want == nil {
				if got != nil {
					t.Errorf("FindStringIndex(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if got == nil || len(got) != 2 || got[0] != tt.want[0] || got[1] != tt.want[1] {
				t.Errorf("FindStringIndex(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFindAllStringIndex(t *testing.T) {
	re := MustCompile("[0-9]+")
	input := "a1b23c456d"

	got := re.FindAllStringIndex(input, -1)
	want := [][]int{{1, 2}, {3, 5}, {6, 9}}

	if len(got) != len(want) {
		t.Fatalf("FindAllStringIndex got %d matches, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i][0] != want[i][0] || got[i][1] != want[i][1] {
			t.Errorf("match %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestReplaceAllString(t *testing.T) {
	tests := []struct {
		pattern string
		input   string
		repl    string
		want    string
	}{
		{"hello", "hello world", "hi", "hi world"},
		{"[0-9]+", "a1b2c3", "X", "aXbXcX"},
		{"world", "hello world", "universe", "hello universe"},
		{"xyz", "hello world", "XYZ", "hello world"},
		{"(foo)(bar)", "foobar", "$2$1", "barfoo"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			re := MustCompile(tt.pattern)
			if got := re.ReplaceAllString(tt.input, tt.repl); got != tt.want {
				t.Errorf("ReplaceAllString(%q, %q) = %q, want %q", tt.input, tt.repl, got, tt.want)
			}
		})
	}
}

func TestReplaceAllStringFunc(t *testing.T) {
	re := MustCompile("[a-z]+")
	input := "Hello World"
	got := re.ReplaceAllStringFunc(input, func(s string) string {
		return "[" + s + "]"
	})
	want := "H[ello] W[orld]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		pattern string
		input   string
		n       int
		want    []string
	}{
		{",", "a,b,c", -1, []string{"a", "b", "c"}},
		{"[ \t]+", "a  b\tc", -1, []string{"a", "b", "c"}},
		{",", "a,b,c", 2, []string{"a", "b,c"}},
		{":", "", -1, []string{""}},
		{",", "abc", -1, []string{"abc"}},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.input, func(t *testing.T) {
			re := MustCompile(tt.pattern)
			got := re.Split(tt.input, tt.n)
			if len(got) != len(tt.want) {
				t.Errorf("Split(%q, %d) got %d parts, want %d", tt.input, tt.n, len(got), len(tt.want))
				return
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("part %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestRegexCache(t *testing.T) {
	cache := NewRegexCache(3)

	// Test initial state
	if cache.Len() != 0 {
		t.Errorf("new cache Len() = %d, want 0", cache.Len())
	}

	// Test Get and caching
	re1, err := cache.Get("hello")
	if err != nil {
		t.Fatalf("Get(hello): %v", err)
	}
	if cache.Len() != 1 {
		t.Errorf("after first Get, Len() = %d, want 1", cache.Len())
	}

	// Same pattern should return cached
	re2, err := cache.Get("hello")
	if err != nil {
		t.Fatalf("Get(hello) again: %v", err)
	}
	if re1 != re2 {
		t.Error("expected same Regex instance from cache")
	}

	// Test LRU eviction
	cache.Get("world")
	cache.Get("foo")
	if cache.Len() != 3 {
		t.Errorf("after 3 patterns, Len() = %d, want 3", cache.Len())
	}

	// Adding 4th should evict oldest (hello)
	cache.Get("bar")
	if cache.Len() != 3 {
		t.Errorf("after eviction, Len() = %d, want 3", cache.Len())
	}

	// Test Clear
	cache.Clear()
	if cache.Len() != 0 {
		t.Errorf("after Clear(), Len() = %d, want 0", cache.Len())
	}
}

func TestRegexCacheMustGet(t *testing.T) {
	cache := NewRegexCache(10)

	// Valid pattern should work
	re := cache.MustGet("hello")
	if re == nil {
		t.Error("MustGet(hello) returned nil")
	}

	// Invalid pattern should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid pattern")
		}
	}()
	cache.MustGet("[invalid")
}

func TestRegexCacheConcurrency(t *testing.T) {
	cache := NewRegexCache(100)
	patterns := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}

	// Run concurrent Gets
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			for j := 0; j < 100; j++ {
				_, err := cache.Get(patterns[idx])
				if err != nil {
					t.Errorf("concurrent Get error: %v", err)
				}
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// Benchmarks

func BenchmarkCompile(b *testing.B) {
	patterns := []string{
		"hello",
		"^[a-z]+$",
		"[0-9]+",
		".*\\.txt$",
		"([a-z]+)@([a-z]+)\\.com",
	}

	for _, p := range patterns {
		b.Run(p, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = Compile(p)
			}
		})
	}
}

func BenchmarkMatchString(b *testing.B) {
	benchmarks := []struct {
		pattern string
		input   string
	}{
		{"hello", "hello world"},
		{"^[a-z]+$", "helloworld"},
		{"[0-9]+", "abc123def"},
		{".*\\.txt$", "document.txt"},
		{"([a-z]+)@([a-z]+)\\.com", "test@example.com"},
	}

	for _, bm := range benchmarks {
		re := MustCompile(bm.pattern)
		b.Run(bm.pattern, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				re.MatchString(bm.input)
			}
		})
	}
}

func BenchmarkRegexCache(b *testing.B) {
	cache := NewRegexCache(100)

	b.Run("CachedGet", func(b *testing.B) {
		// Pre-populate cache
		cache.Get("hello")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cache.Get("hello")
		}
	})

	b.Run("NewPattern", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cache.Get("[a-z]+")
		}
	})
}
