// Package types defines runtime value types for uawk.
package types

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Kind represents the type of an AWK value.
type Kind uint8

const (
	KindNull   Kind = iota // Uninitialized value
	KindNum                // Numeric value
	KindStr                // String value
	KindNumStr             // Numeric string (from input field)
)

// String returns a string representation of the kind.
func (k Kind) String() string {
	switch k {
	case KindNull:
		return "null"
	case KindNum:
		return "num"
	case KindStr:
		return "str"
	case KindNumStr:
		return "numstr"
	default:
		return "unknown"
	}
}

// Value represents an AWK runtime value.
// Uses tagged union pattern for type safety and performance.
// Values are passed by value (24 bytes on 64-bit systems).
type Value struct {
	kind Kind
	num  float64
	str  string
}

// Constructors

// Null returns a null (uninitialized) value.
func Null() Value {
	return Value{kind: KindNull}
}

// Num creates a numeric value.
func Num(n float64) Value {
	return Value{kind: KindNum, num: n}
}

// Str creates a string value.
func Str(s string) Value {
	return Value{kind: KindStr, str: s}
}

// NumStr creates a numeric string value (from input fields).
// These are treated as numbers in numeric context but preserve the original string.
// Uses lazy parsing: the numeric value is computed on first AsNum() call.
// This avoids unnecessary parsing when fields are only used as strings (print, concat).
func NumStr(s string) Value {
	return Value{kind: KindNumStr, str: s}
}

// Bool creates a numeric value from a boolean (1 for true, 0 for false).
func Bool(b bool) Value {
	if b {
		return Num(1)
	}
	return Num(0)
}

// Accessors

// Kind returns the value's type.
func (v Value) Kind() Kind {
	return v.kind
}

// IsNull returns true if the value is null.
func (v Value) IsNull() bool {
	return v.kind == KindNull
}

// IsNum returns true if the value is a pure number.
func (v Value) IsNum() bool {
	return v.kind == KindNum
}

// IsStr returns true if the value is a pure string.
func (v Value) IsStr() bool {
	return v.kind == KindStr
}

// IsNumStr returns true if the value is a numeric string.
func (v Value) IsNumStr() bool {
	return v.kind == KindNumStr
}

// Conversions

// AsNum returns the numeric representation of the value.
// For NumStr and Str, parses the string using AWK prefix parsing rules.
// This is lazy parsing - the value is computed on demand, not at creation.
func (v Value) AsNum() float64 {
	switch v.kind {
	case KindNum:
		return v.num
	case KindNumStr, KindStr:
		// Lazy parsing: parse on demand
		return ParseNumPrefix(v.str)
	default: // KindNull
		return 0
	}
}

// AsStr returns the string representation using the given format for numbers.
// Common formats: "%.6g" (default CONVFMT), "%.6f"
func (v Value) AsStr(format string) string {
	if v.kind == KindNum {
		return FormatNum(v.num, format)
	}
	// For KindStr, KindNumStr, and KindNull (empty string)
	return v.str
}

// AsBool returns the boolean representation.
// Numbers: 0 is false, everything else is true.
// Strings: empty string is false, everything else is true.
func (v Value) AsBool() bool {
	switch v.kind {
	case KindNum:
		return v.num != 0
	case KindStr:
		return v.str != ""
	case KindNumStr:
		n, err := ParseNum(v.str)
		if err != nil {
			return v.str != ""
		}
		return n != 0
	default: // KindNull
		return false
	}
}

// IsTrueStr returns true if the value should be treated as a true string
// (not convertible to a number). Also returns the numeric value if not a true string.
// For NumStr values, uses lazy parsing to determine if it's a valid number.
func (v Value) IsTrueStr() (float64, bool) {
	switch v.kind {
	case KindStr:
		return 0, true
	case KindNumStr:
		// Lazy parsing: check if string is a valid number (strict parsing).
		// If parsing fails, it's a "true string" (e.g., "10x", "abc").
		n, err := ParseNum(v.str)
		if err != nil {
			return 0, true
		}
		return n, false
	default: // KindNum, KindNull
		return v.num, false
	}
}

// String returns a debug representation of the value.
func (v Value) String() string {
	switch v.kind {
	case KindNum:
		return fmt.Sprintf("Num(%s)", FormatNum(v.num, "%.6g"))
	case KindStr:
		return fmt.Sprintf("Str(%q)", v.str)
	case KindNumStr:
		return fmt.Sprintf("NumStr(%q)", v.str)
	default:
		return "Null()"
	}
}

// Comparison

// Compare compares two values using AWK comparison semantics.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func Compare(a, b Value) int {
	// If both are numeric (or can be converted), compare as numbers
	aNum, aIsStr := a.IsTrueStr()
	bNum, bIsStr := b.IsTrueStr()

	if !aIsStr && !bIsStr {
		// Both numeric - compare as numbers
		switch {
		case aNum < bNum:
			return -1
		case aNum > bNum:
			return 1
		default:
			return 0
		}
	}

	// At least one is a true string - compare as strings
	aStr := a.AsStr("%.6g")
	bStr := b.AsStr("%.6g")
	return strings.Compare(aStr, bStr)
}

// Number Parsing and Formatting

// ParseNum parses a string as a number (strict parsing).
func ParseNum(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	// Handle special cases
	if len(s) >= 3 {
		lower := strings.ToLower(s)
		if lower == "nan" || lower == "+nan" || lower == "-nan" {
			return math.NaN(), nil
		}
		if lower == "inf" || lower == "+inf" {
			return math.Inf(1), nil
		}
		if lower == "-inf" {
			return math.Inf(-1), nil
		}
	}

	// Handle hex without exponent (AWK allows "0x1a", Go requires "0x1ap0")
	if len(s) > 2 && (s[0] == '0' && (s[1] == 'x' || s[1] == 'X')) {
		if !strings.ContainsAny(s, "pP") {
			s += "p0"
		}
	}

	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}

	// AWK doesn't support underscore separators
	if strings.Contains(s, "_") {
		return 0, strconv.ErrSyntax
	}

	return n, nil
}

// ParseNumPrefix parses a number from the beginning of a string.
// Allows trailing non-numeric characters like "123abc" -> 123.
func ParseNumPrefix(s string) float64 {
	// Skip leading whitespace
	i := 0
	for i < len(s) && isSpace(s[i]) {
		i++
	}
	if i >= len(s) {
		return 0
	}

	start := i

	// Handle sign
	if s[i] == '+' || s[i] == '-' {
		i++
	}

	if i >= len(s) {
		return 0
	}

	// Check for special values
	if i+3 <= len(s) {
		rest := strings.ToLower(s[i : i+3])
		if rest == "nan" {
			return math.NaN()
		}
		if rest == "inf" {
			if start < i && s[start] == '-' {
				return math.Inf(-1)
			}
			return math.Inf(1)
		}
	}

	// Check for hex
	if i+2 < len(s) && s[i] == '0' && (s[i+1] == 'x' || s[i+1] == 'X') {
		return parseHexPrefix(s, start, i+2)
	}

	// Parse decimal mantissa
	gotDigit := false
	for i < len(s) && isDigit(s[i]) {
		gotDigit = true
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && isDigit(s[i]) {
			gotDigit = true
			i++
		}
	}
	if !gotDigit {
		return 0
	}

	// Parse exponent
	end := i
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		for i < len(s) && isDigit(s[i]) {
			end = i + 1
			i++
		}
	}

	n, _ := strconv.ParseFloat(s[start:end], 64)
	return n
}

func parseHexPrefix(s string, start, i int) float64 {
	gotDigit := false
	for i < len(s) && isHexDigit(s[i]) {
		gotDigit = true
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && isHexDigit(s[i]) {
			gotDigit = true
			i++
		}
	}
	if !gotDigit {
		return 0
	}

	end := i
	gotExponent := false
	if i < len(s) && (s[i] == 'p' || s[i] == 'P') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		for i < len(s) && isDigit(s[i]) {
			gotExponent = true
			end = i + 1
			i++
		}
	}

	numStr := s[start:end]
	if !gotExponent {
		numStr += "p0" // AWK allows "0x12", ParseFloat requires "0x12p0"
	}
	n, _ := strconv.ParseFloat(numStr, 64)
	return n
}

// FormatNum formats a number as a string using the given format.
func FormatNum(n float64, format string) string {
	switch {
	case math.IsNaN(n):
		return "nan"
	case math.IsInf(n, 1):
		return "inf"
	case math.IsInf(n, -1):
		return "-inf"
	case n == float64(int64(n)):
		// Integer - format without decimal
		return strconv.FormatInt(int64(n), 10)
	case format == "%.6g":
		// Common case - use faster formatting
		return strconv.FormatFloat(n, 'g', 6, 64)
	default:
		return fmt.Sprintf(format, n)
	}
}

// Helper functions

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f'
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isHexDigit(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
