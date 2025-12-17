// Package types defines runtime value types for uawk.
package types

import (
	"math"
	"testing"
)

func TestValueConstructors(t *testing.T) {
	tests := []struct {
		name string
		v    Value
		kind Kind
	}{
		{"Null", Null(), KindNull},
		{"Num(0)", Num(0), KindNum},
		{"Num(42)", Num(42), KindNum},
		{"Num(-3.14)", Num(-3.14), KindNum},
		{"Str empty", Str(""), KindStr},
		{"Str hello", Str("hello"), KindStr},
		{"NumStr", NumStr("123"), KindNumStr},
		{"Bool true", Bool(true), KindNum},
		{"Bool false", Bool(false), KindNum},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.v.Kind() != tt.kind {
				t.Errorf("Kind() = %v, want %v", tt.v.Kind(), tt.kind)
			}
		})
	}
}

func TestValuePredicates(t *testing.T) {
	null := Null()
	num := Num(42)
	str := Str("hello")
	numStr := NumStr("123")

	if !null.IsNull() {
		t.Error("Null().IsNull() should be true")
	}
	if null.IsNum() || null.IsStr() || null.IsNumStr() {
		t.Error("Null() should not be num/str/numstr")
	}

	if !num.IsNum() {
		t.Error("Num().IsNum() should be true")
	}
	if num.IsNull() || num.IsStr() || num.IsNumStr() {
		t.Error("Num() should not be null/str/numstr")
	}

	if !str.IsStr() {
		t.Error("Str().IsStr() should be true")
	}
	if str.IsNull() || str.IsNum() || str.IsNumStr() {
		t.Error("Str() should not be null/num/numstr")
	}

	if !numStr.IsNumStr() {
		t.Error("NumStr().IsNumStr() should be true")
	}
	if numStr.IsNull() || numStr.IsNum() || numStr.IsStr() {
		t.Error("NumStr() should not be null/num/str")
	}
}

func TestAsNum(t *testing.T) {
	tests := []struct {
		name     string
		v        Value
		expected float64
	}{
		{"Null", Null(), 0},
		{"Num(42)", Num(42), 42},
		{"Num(-3.14)", Num(-3.14), -3.14},
		{"Str empty", Str(""), 0},
		{"Str 123", Str("123"), 123},
		{"Str 3.14", Str("3.14"), 3.14},
		{"Str abc", Str("abc"), 0},
		{"Str 123abc", Str("123abc"), 123},
		{"NumStr 456", NumStr("456"), 456},
		{"NumStr  789 ", NumStr("  789  "), 789},
		{"Bool true", Bool(true), 1},
		{"Bool false", Bool(false), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.AsNum()
			if got != tt.expected {
				t.Errorf("AsNum() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAsNumSpecialValues(t *testing.T) {
	inf := Num(math.Inf(1))
	if !math.IsInf(inf.AsNum(), 1) {
		t.Error("Inf should return Inf")
	}

	negInf := Num(math.Inf(-1))
	if !math.IsInf(negInf.AsNum(), -1) {
		t.Error("-Inf should return -Inf")
	}

	nan := Num(math.NaN())
	if !math.IsNaN(nan.AsNum()) {
		t.Error("NaN should return NaN")
	}

	// String parsing of special values
	infStr := Str("inf")
	if !math.IsInf(infStr.AsNum(), 1) {
		t.Errorf("'inf' string AsNum() = %v, want +Inf", infStr.AsNum())
	}

	nanStr := Str("nan")
	if !math.IsNaN(nanStr.AsNum()) {
		t.Errorf("'nan' string AsNum() = %v, want NaN", nanStr.AsNum())
	}
}

func TestAsStr(t *testing.T) {
	tests := []struct {
		name     string
		v        Value
		format   string
		expected string
	}{
		{"Null", Null(), "%.6g", ""},
		{"Num integer", Num(42), "%.6g", "42"},
		{"Num float", Num(3.14159), "%.6g", "3.14159"},
		{"Num big", Num(1e10), "%.6g", "10000000000"}, // Integer representation
		{"Str hello", Str("hello"), "%.6g", "hello"},
		{"NumStr", NumStr("world"), "%.6g", "world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.AsStr(tt.format)
			if got != tt.expected {
				t.Errorf("AsStr(%q) = %q, want %q", tt.format, got, tt.expected)
			}
		})
	}
}

func TestAsBool(t *testing.T) {
	tests := []struct {
		name     string
		v        Value
		expected bool
	}{
		{"Null", Null(), false},
		{"Num 0", Num(0), false},
		{"Num 1", Num(1), true},
		{"Num -1", Num(-1), true},
		{"Num 0.1", Num(0.1), true},
		{"Str empty", Str(""), false},
		{"Str non-empty", Str("x"), true},
		{"NumStr 0", NumStr("0"), false},
		{"NumStr 1", NumStr("1"), true},
		{"NumStr empty", NumStr(""), false},
		{"NumStr non-numeric", NumStr("abc"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.AsBool()
			if got != tt.expected {
				t.Errorf("AsBool() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsTrueStr(t *testing.T) {
	tests := []struct {
		name     string
		v        Value
		isStr    bool
		numIfNot float64
	}{
		{"Num 42", Num(42), false, 42},
		{"Num 0", Num(0), false, 0},
		{"Null", Null(), false, 0},
		{"Str hello", Str("hello"), true, 0},
		{"Str 123", Str("123"), true, 0},
		{"NumStr 123", NumStr("123"), false, 123},
		{"NumStr abc", NumStr("abc"), true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			num, isStr := tt.v.IsTrueStr()
			if isStr != tt.isStr {
				t.Errorf("IsTrueStr() isStr = %v, want %v", isStr, tt.isStr)
			}
			if !isStr && num != tt.numIfNot {
				t.Errorf("IsTrueStr() num = %v, want %v", num, tt.numIfNot)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name     string
		a, b     Value
		expected int
	}{
		// Numeric comparisons
		{"1 < 2", Num(1), Num(2), -1},
		{"2 > 1", Num(2), Num(1), 1},
		{"1 == 1", Num(1), Num(1), 0},
		{"0 == 0", Num(0), Num(0), 0},
		{"-1 < 1", Num(-1), Num(1), -1},

		// String comparisons
		{"a < b", Str("a"), Str("b"), -1},
		{"b > a", Str("b"), Str("a"), 1},
		{"a == a", Str("a"), Str("a"), 0},
		{"abc < abd", Str("abc"), Str("abd"), -1},

		// Mixed - string takes precedence if either is true string
		{"Str 10 vs Str 9", Str("10"), Str("9"), -1}, // String comparison: "10" < "9"
		{"Num 10 vs Num 9", Num(10), Num(9), 1},      // Numeric: 10 > 9

		// NumStr behaves like number
		{"NumStr 10 vs Num 9", NumStr("10"), Num(9), 1},         // Both numeric: 10 > 9
		{"NumStr 10 vs NumStr 9", NumStr("10"), NumStr("9"), 1}, // Both numeric

		// Null comparison
		{"Null == Null", Null(), Null(), 0},
		{"Null < Num 1", Null(), Num(1), -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Compare(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("Compare() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseNum(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
		hasError bool
	}{
		{"0", 0, false},
		{"123", 123, false},
		{"-456", -456, false},
		{"3.14", 3.14, false},
		{".5", 0.5, false},
		{"1e10", 1e10, false},
		{"1.5e-3", 1.5e-3, false},
		{"0x1a", 26, false},
		{"0X1A", 26, false},
		{"  42  ", 42, false},
		{"", 0, false},
		{"abc", 0, true},
		{"1_000", 0, true}, // Underscores not allowed
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseNum(tt.input)
			if (err != nil) != tt.hasError {
				t.Errorf("ParseNum(%q) error = %v, wantError = %v", tt.input, err, tt.hasError)
			}
			if err == nil && got != tt.expected {
				t.Errorf("ParseNum(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseNumPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"123", 123},
		{"123abc", 123},
		{"  42  ", 42},
		{"3.14rest", 3.14},
		{"0x1aGH", 26},
		{"abc", 0},
		{"", 0},
		{"  ", 0},
		{"+42", 42},
		{"-42", -42},
		{"inf", math.Inf(1)},
		{"-inf", math.Inf(-1)},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseNumPrefix(tt.input)
			if math.IsInf(tt.expected, 1) {
				if !math.IsInf(got, 1) {
					t.Errorf("ParseNumPrefix(%q) = %v, want +Inf", tt.input, got)
				}
			} else if math.IsInf(tt.expected, -1) {
				if !math.IsInf(got, -1) {
					t.Errorf("ParseNumPrefix(%q) = %v, want -Inf", tt.input, got)
				}
			} else if got != tt.expected {
				t.Errorf("ParseNumPrefix(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatNum(t *testing.T) {
	tests := []struct {
		n        float64
		format   string
		expected string
	}{
		{42, "%.6g", "42"},
		{3.14159265, "%.6g", "3.14159"},
		{1e10, "%.6g", "10000000000"},      // Integer representation
		{1e15, "%.6g", "1000000000000000"}, // Still integer representation
		{1e20, "%.6g", "1e+20"},            // Scientific for very large
		{math.NaN(), "%.6g", "nan"},
		{math.Inf(1), "%.6g", "inf"},
		{math.Inf(-1), "%.6g", "-inf"},
		{123.456, "%.2f", "123.46"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := FormatNum(tt.n, tt.format)
			if got != tt.expected {
				t.Errorf("FormatNum(%v, %q) = %q, want %q", tt.n, tt.format, got, tt.expected)
			}
		})
	}
}

func TestKindString(t *testing.T) {
	tests := []struct {
		k        Kind
		expected string
	}{
		{KindNull, "null"},
		{KindNum, "num"},
		{KindStr, "str"},
		{KindNumStr, "numstr"},
		{Kind(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.k.String(); got != tt.expected {
				t.Errorf("Kind.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestValueString(t *testing.T) {
	tests := []struct {
		v        Value
		contains string
	}{
		{Null(), "Null()"},
		{Num(42), "Num(42)"},
		{Str("hello"), `Str("hello")`},
		{NumStr("123"), `NumStr("123")`},
	}

	for _, tt := range tests {
		t.Run(tt.contains, func(t *testing.T) {
			got := tt.v.String()
			if got != tt.contains {
				t.Errorf("Value.String() = %q, want %q", got, tt.contains)
			}
		})
	}
}

func TestNumStrLazyParsing(t *testing.T) {
	// Test that NumStr uses lazy parsing - number is parsed on AsNum() call
	// This matches GoAWK behavior for optimal performance when fields
	// are only used as strings (print, concatenation)
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{"simple integer", "42", 42},
		{"float", "3.14", 3.14},
		{"with whitespace", "  123  ", 123},
		{"with trailing text", "456abc", 456},
		{"negative", "-789", -789},
		{"zero", "0", 0},
		{"non-numeric", "abc", 0},
		{"empty", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NumStr(tt.input)

			// AsNum() parses on demand (lazy)
			got1 := v.AsNum()
			if got1 != tt.expected {
				t.Errorf("AsNum() first call = %v, want %v", got1, tt.expected)
			}

			// Second call - parses again (no caching, by design)
			got2 := v.AsNum()
			if got2 != tt.expected {
				t.Errorf("AsNum() second call = %v, want %v", got2, tt.expected)
			}

			// String representation should be preserved
			gotStr := v.AsStr("%.6g")
			if gotStr != tt.input {
				t.Errorf("AsStr() = %q, want %q", gotStr, tt.input)
			}
		})
	}
}

func TestNumStrPreservesString(t *testing.T) {
	// Ensure NumStr preserves original string with lazy parsing
	tests := []struct {
		input string
	}{
		{"123"},
		{"  456  "},
		{"789abc"},
		{"3.14159265358979"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v := NumStr(tt.input)
			if v.AsStr("%.6g") != tt.input {
				t.Errorf("AsStr() = %q, want %q", v.AsStr("%.6g"), tt.input)
			}
			// Also check Kind is still NumStr
			if v.Kind() != KindNumStr {
				t.Errorf("Kind() = %v, want KindNumStr", v.Kind())
			}
		})
	}
}

// Benchmarks

func BenchmarkAsNum(b *testing.B) {
	v := Str("123.456")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = v.AsNum()
	}
}

func BenchmarkAsStr(b *testing.B) {
	v := Num(123.456)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = v.AsStr("%.6g")
	}
}

func BenchmarkCompareNum(b *testing.B) {
	a, c := Num(1), Num(2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Compare(a, c)
	}
}

func BenchmarkCompareStr(b *testing.B) {
	a, c := Str("hello"), Str("world")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Compare(a, c)
	}
}

func BenchmarkParseNumPrefix(b *testing.B) {
	s := "  123.456abc"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ParseNumPrefix(s)
	}
}

func BenchmarkNumStrCreation(b *testing.B) {
	// Benchmark NumStr creation with lazy parsing
	// No parsing at creation time - just stores the string
	s := "123.456"
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NumStr(s)
	}
}

func BenchmarkNumStrAsNum(b *testing.B) {
	// Benchmark NumStr.AsNum() - parses on each call (lazy)
	// This is the cost when field IS used numerically
	v := NumStr("123.456")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = v.AsNum()
	}
}

func BenchmarkNumStrAsNumMultiple(b *testing.B) {
	// Benchmark multiple AsNum() calls on same NumStr
	// With lazy parsing: each call parses the string
	// Trade-off: faster creation vs slower repeated numeric access
	v := NumStr("123.456")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Simulate typical AWK usage: sum += $1
		// With lazy parsing: 3 ParseNumPrefix calls
		// Trade-off: most AWK programs don't use same field 3x in arithmetic
		_ = v.AsNum()
		_ = v.AsNum()
		_ = v.AsNum()
	}
}

func BenchmarkNumStrOnlyPrint(b *testing.B) {
	// Simulate field access for print (no numeric conversion needed)
	// With lazy parsing: NO parsing happens - big win!
	// This is the common case: print $0, print $1, etc.
	v := NumStr("123.456")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = v.AsStr("%.6g")
	}
}
