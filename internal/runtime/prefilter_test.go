package runtime

import (
	"strings"
	"testing"
)

func TestExtractLiterals(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		wantPrefix string
		wantSuffix string
		wantReq    []string
	}{
		{
			name:       "anchored prefix simple",
			pattern:    "^error",
			wantPrefix: "error",
		},
		{
			name:    "anchored suffix simple",
			pattern: "failed$",
			// Note: suffix extraction from unanchored pattern goes to required
			wantReq: []string{"failed"},
		},
		{
			name:       "anchored prefix and suffix",
			pattern:    "^error.*failed$",
			wantPrefix: "error",
			wantSuffix: "failed",
		},
		{
			name:       "anchored prefix with path",
			pattern:    "^/api/v1/",
			wantPrefix: "/api/v1/",
		},
		{
			name:       "anchored prefix with wildcard",
			pattern:    "^GET /api/.*",
			wantPrefix: "GET /api/",
		},
		{
			name:    "required literals in middle",
			pattern: ".*warning.*error.*",
			wantReq: []string{"warning", "error"},
		},
		{
			name:    "required literal after metachar",
			pattern: "\\d+.*test",
			wantReq: []string{"test"},
		},
		{
			name:       "escaped dot in prefix",
			pattern:    "^www\\.example\\.com",
			wantPrefix: "www.example.com",
		},
		{
			name:    "escaped dot in suffix - complex escapes go to required",
			pattern: "test\\.log$",
			// Complex escape sequences in suffix are conservative - extracted as required
			wantSuffix: "log",
			wantReq:    []string{"test."},
		},
		{
			name:    "no useful literals - only metachar",
			pattern: ".*",
		},
		{
			name:    "no useful literals - short required",
			pattern: ".*ab.*",
		},
		{
			name:       "prefix stops at metachar",
			pattern:    "^hello.world",
			wantPrefix: "hello",
			wantReq:    []string{"world"},
		},
		{
			name:       "suffix starts after metachar",
			pattern:    "test.*suffix$",
			wantReq:    []string{"test"},
			wantSuffix: "suffix",
		},
		{
			name:       "with AWK dotall prefix",
			pattern:    "(?s)^error",
			wantPrefix: "error",
		},
		{
			name:       "complex pattern with all components",
			pattern:    "^START.*middle.*END$",
			wantPrefix: "START",
			wantSuffix: "END",
			wantReq:    []string{"middle"},
		},
		{
			name:    "character class patterns - no literals",
			pattern: "[a-z]+",
		},
		{
			name:       "mixed literal and char class",
			pattern:    "^error[0-9]+warning",
			wantPrefix: "error",
			wantReq:    []string{"warning"},
		},
		{
			name:    "alternation breaks required extraction",
			pattern: "foo|bar",
			// Top-level alternation means we cannot safely extract required literals
			// (foo OR bar, not both must be present) - returns nil
		},
		{
			name:    "long literal in middle",
			pattern: ".*authentication_failed.*",
			wantReq: []string{"authentication_failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractLiterals(tt.pattern)

			// Check if we expect nil
			if tt.wantPrefix == "" && tt.wantSuffix == "" && len(tt.wantReq) == 0 {
				if got != nil {
					t.Errorf("extractLiterals(%q) = %+v, want nil", tt.pattern, got)
				}
				return
			}

			if got == nil {
				t.Fatalf("extractLiterals(%q) = nil, want non-nil", tt.pattern)
			}

			if got.Prefix != tt.wantPrefix {
				t.Errorf("Prefix = %q, want %q", got.Prefix, tt.wantPrefix)
			}
			if got.Suffix != tt.wantSuffix {
				t.Errorf("Suffix = %q, want %q", got.Suffix, tt.wantSuffix)
			}
			if !stringSliceEqual(got.Required, tt.wantReq) {
				t.Errorf("Required = %v, want %v", got.Required, tt.wantReq)
			}
		})
	}
}

func TestLiteralPrefiltering(t *testing.T) {
	// These tests verify correctness: prefiltering must never produce false negatives
	tests := []struct {
		name    string
		pattern string
		input   string
		want    bool // Expected match result
	}{
		// Prefix tests
		{
			name:    "prefix match",
			pattern: "^error.*",
			input:   "error: something went wrong",
			want:    true,
		},
		{
			name:    "prefix no match",
			pattern: "^error.*",
			input:   "warning: something went wrong",
			want:    false,
		},
		{
			name:    "prefix exact",
			pattern: "^/api/v1/users",
			input:   "/api/v1/users/123",
			want:    true,
		},
		{
			name:    "prefix wrong path",
			pattern: "^/api/v1/users",
			input:   "/api/v2/users/123",
			want:    false,
		},

		// Suffix tests
		{
			name:    "suffix match",
			pattern: ".*\\.log$",
			input:   "access.log",
			want:    true,
		},
		{
			name:    "suffix no match",
			pattern: ".*\\.log$",
			input:   "access.txt",
			want:    false,
		},
		{
			name:    "suffix in middle no match",
			pattern: ".*\\.log$",
			input:   "access.log.bak",
			want:    false,
		},

		// Prefix + suffix tests
		{
			name:    "prefix and suffix match",
			pattern: "^error.*failed$",
			input:   "error: operation failed",
			want:    true,
		},
		{
			name:    "prefix and suffix - prefix no match",
			pattern: "^error.*failed$",
			input:   "warning: operation failed",
			want:    false,
		},
		{
			name:    "prefix and suffix - suffix no match",
			pattern: "^error.*failed$",
			input:   "error: operation succeeded",
			want:    false,
		},

		// Required literals tests
		{
			name:    "required literal present",
			pattern: ".*authentication.*",
			input:   "user authentication failed",
			want:    true,
		},
		{
			name:    "required literal missing",
			pattern: ".*authentication.*",
			input:   "user login failed",
			want:    false,
		},
		{
			name:    "multiple required literals all present",
			pattern: ".*error.*warning.*",
			input:   "error occurred, warning issued",
			want:    true,
		},
		{
			name:    "multiple required literals one missing",
			pattern: ".*error.*warning.*",
			input:   "error occurred, info issued",
			want:    false,
		},

		// Edge cases
		{
			name:    "empty input prefix pattern",
			pattern: "^test",
			input:   "",
			want:    false,
		},
		{
			name:    "empty input suffix pattern",
			pattern: "test$",
			input:   "",
			want:    false,
		},
		{
			name:    "regex match but prefilter would pass",
			pattern: "^/api/.*users",
			input:   "/api/v1/users",
			want:    true,
		},
		{
			name:    "case sensitive prefix",
			pattern: "^Error",
			input:   "error: test",
			want:    false,
		},
		{
			name:    "unicode in pattern",
			pattern: "^Error:.*",
			input:   "Error: \xe4\xb8\xad\xe6\x96\x87", // Chinese characters
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re, err := Compile(tt.pattern)
			if err != nil {
				t.Fatalf("Compile(%q) error: %v", tt.pattern, err)
			}

			got := re.MatchString(tt.input)
			if got != tt.want {
				t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}

			// Also test FindStringIndex consistency
			idx := re.FindStringIndex(tt.input)
			if tt.want && idx == nil {
				t.Errorf("FindStringIndex(%q) = nil, want non-nil", tt.input)
			}
			if !tt.want && idx != nil {
				t.Errorf("FindStringIndex(%q) = %v, want nil", tt.input, idx)
			}
		})
	}
}

func TestCanReject(t *testing.T) {
	tests := []struct {
		name     string
		literals *LiteralInfo
		input    string
		want     bool // true if should reject
	}{
		{
			name:     "prefix rejects",
			literals: &LiteralInfo{Prefix: "error"},
			input:    "warning: test",
			want:     true,
		},
		{
			name:     "prefix accepts",
			literals: &LiteralInfo{Prefix: "error"},
			input:    "error: test",
			want:     false,
		},
		{
			name:     "suffix rejects",
			literals: &LiteralInfo{Suffix: ".log"},
			input:    "test.txt",
			want:     true,
		},
		{
			name:     "suffix accepts",
			literals: &LiteralInfo{Suffix: ".log"},
			input:    "test.log",
			want:     false,
		},
		{
			name:     "required rejects",
			literals: &LiteralInfo{Required: []string{"error"}},
			input:    "warning: test",
			want:     true,
		},
		{
			name:     "required accepts",
			literals: &LiteralInfo{Required: []string{"error"}},
			input:    "there was an error",
			want:     false,
		},
		{
			name:     "multiple required first missing",
			literals: &LiteralInfo{Required: []string{"error", "warning"}},
			input:    "only warning here",
			want:     true,
		},
		{
			name:     "multiple required second missing",
			literals: &LiteralInfo{Required: []string{"error", "warning"}},
			input:    "only error here",
			want:     true,
		},
		{
			name:     "multiple required all present",
			literals: &LiteralInfo{Required: []string{"error", "warning"}},
			input:    "error and warning both here",
			want:     false,
		},
		{
			name:     "all components reject on prefix",
			literals: &LiteralInfo{Prefix: "start", Suffix: "end", Required: []string{"middle"}},
			input:    "wrong beginning middle end",
			want:     true,
		},
		{
			name:     "all components reject on suffix",
			literals: &LiteralInfo{Prefix: "start", Suffix: "end", Required: []string{"middle"}},
			input:    "start middle wrong",
			want:     true,
		},
		{
			name:     "all components reject on required",
			literals: &LiteralInfo{Prefix: "start", Suffix: "end", Required: []string{"middle"}},
			input:    "start something end",
			want:     true,
		},
		{
			name:     "all components accept",
			literals: &LiteralInfo{Prefix: "start", Suffix: "end", Required: []string{"middle"}},
			input:    "start middle end",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.literals.CanReject(tt.input)
			if got != tt.want {
				t.Errorf("CanReject(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsMetaChar(t *testing.T) {
	metas := ".?*+{}[]()^$|"
	nonMetas := "abcdefghijklmnopqrstuvwxyz0123456789_-=;:'\"<>,/\\"

	for _, c := range metas {
		if !isMetaChar(byte(c)) {
			t.Errorf("isMetaChar(%q) = false, want true", c)
		}
	}

	for _, c := range nonMetas {
		if isMetaChar(byte(c)) {
			t.Errorf("isMetaChar(%q) = true, want false", c)
		}
	}
}

func TestIsLiteralEscape(t *testing.T) {
	// These characters when escaped represent themselves
	literals := ".?*+{}[]()^$\\/"
	// These are special escapes (\d, \s, \w, etc.)
	nonLiterals := "dswDSWnrtfvbB0123456789"

	for _, c := range literals {
		if !isLiteralEscape(byte(c)) {
			t.Errorf("isLiteralEscape(%q) = false, want true", c)
		}
	}

	for _, c := range nonLiterals {
		if isLiteralEscape(byte(c)) {
			t.Errorf("isLiteralEscape(%q) = true, want false", c)
		}
	}
}

// BenchmarkLiteralPrefilter demonstrates the speedup from literal prefiltering
func BenchmarkLiteralPrefilter(b *testing.B) {
	// Pattern with a clear prefix
	pattern := "^/api/v1/users/.*"
	re := MustCompile(pattern)

	// Test data: most lines will NOT match (common case in log filtering)
	lines := []string{
		"/api/v1/users/123/profile",                      // matches
		"/api/v2/users/456/profile",                      // no match - wrong version
		"/api/v1/orders/789",                             // no match - wrong resource
		"GET /api/v1/users/123 HTTP/1.1",                 // no match - doesn't start with path
		"/static/js/app.js",                              // no match
		"/api/v1/products/search?q=test",                 // no match
		"/health",                                        // no match
		"/api/v1/users/list",                             // matches
		"error: connection refused",                      // no match
		strings.Repeat("x", 1000) + "/api/v1/users/test", // no match - prefix not at start
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, line := range lines {
			re.MatchString(line)
		}
	}
}

// BenchmarkLiteralPrefilterVsFullRegex compares prefilter vs full regex
func BenchmarkLiteralPrefilterVsFullRegex(b *testing.B) {
	patterns := []struct {
		name    string
		pattern string
	}{
		{"prefix only", "^/api/v1/"},
		{"suffix only", "\\.json$"},
		{"prefix and suffix", "^error.*failed$"},
		{"required literal", ".*authentication_error.*"},
	}

	// Non-matching input (prefilter should reject quickly)
	nonMatchingInputs := []string{
		"/web/v2/static/file.html",
		"random text without the pattern",
		"warning: operation succeeded",
		strings.Repeat("a", 1000),
	}

	for _, pt := range patterns {
		b.Run(pt.name, func(b *testing.B) {
			re := MustCompile(pt.pattern)
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				for _, input := range nonMatchingInputs {
					re.MatchString(input)
				}
			}
		})
	}
}

// BenchmarkPrefixRejection shows speedup for prefix-based rejection
func BenchmarkPrefixRejection(b *testing.B) {
	pattern := "^ERROR:.*critical.*failure$"
	re := MustCompile(pattern)

	// Input that will be rejected by prefix check
	input := "WARNING: some log message about system status"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		re.MatchString(input)
	}
}

// BenchmarkSuffixRejection shows speedup for suffix-based rejection
func BenchmarkSuffixRejection(b *testing.B) {
	pattern := ".*\\.log$"
	re := MustCompile(pattern)

	// Input that will be rejected by suffix check
	input := "/var/log/application.txt"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		re.MatchString(input)
	}
}

// BenchmarkRequiredLiteralRejection shows speedup for required literal rejection
func BenchmarkRequiredLiteralRejection(b *testing.B) {
	pattern := ".*authentication_failed.*"
	re := MustCompile(pattern)

	// Input that will be rejected by required literal check
	input := "user login successful with token xyz123"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		re.MatchString(input)
	}
}

// BenchmarkPrefilterVsNoPrefilter compares performance with and without prefiltering
// by directly testing the CanReject method vs not using it
func BenchmarkPrefilterVsNoPrefilter(b *testing.B) {
	// Pattern that has a clear prefix for prefiltering
	pattern := "^/api/v1/users/.*$"
	re := MustCompile(pattern)

	// Non-matching input that can be rejected by prefix check
	input := "GET /web/static/file.html HTTP/1.1 - this is a longer line to simulate real log data"

	b.Run("with_prefilter", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			re.MatchString(input)
		}
	})

	b.Run("full_regex_only", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Directly call the underlying regex (bypassing prefilter)
			re.re.MatchString(input)
		}
	})
}

// BenchmarkRealWorldLogFiltering simulates real AWK log filtering scenario
func BenchmarkRealWorldLogFiltering(b *testing.B) {
	// Pattern: find error logs from API service
	pattern := "^.*ERROR.*api.*$"
	re := MustCompile(pattern)

	// Simulated log lines - 90% don't match (common in log filtering)
	logs := []string{
		"2024-01-15 10:23:45 INFO  api.UserService - User login successful",
		"2024-01-15 10:23:46 DEBUG api.AuthService - Token validated",
		"2024-01-15 10:23:47 INFO  web.Controller - Request processed",
		"2024-01-15 10:23:48 WARN  db.ConnectionPool - Connection slow",
		"2024-01-15 10:23:49 INFO  api.OrderService - Order created",
		"2024-01-15 10:23:50 ERROR api.PaymentService - Payment failed: timeout",
		"2024-01-15 10:23:51 INFO  api.NotifyService - Notification sent",
		"2024-01-15 10:23:52 DEBUG cache.Redis - Cache miss",
		"2024-01-15 10:23:53 INFO  api.UserService - User logout",
		"2024-01-15 10:23:54 ERROR api.AuthService - Invalid token format",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, log := range logs {
			re.MatchString(log)
		}
	}
}

// Helper function for comparing string slices
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
