// Package runtime provides AWK runtime support including regex operations.
package runtime

import (
	"strings"
)

// LiteralInfo holds extracted literal substrings from a regex pattern.
// These literals enable fast rejection using SIMD-optimized string functions
// before falling back to full NFA execution.
type LiteralInfo struct {
	Prefix   string   // Anchored prefix (^prefix) - use HasPrefix
	Suffix   string   // Anchored suffix (suffix$) - use HasSuffix
	Required []string // Must appear somewhere - use strings.Contains
}

// extractLiterals analyzes a regex pattern and extracts literal substrings
// that can be used for fast prefiltering. Returns nil if no useful literals found.
//
// IMPORTANT: This is a conservative extractor. It may miss some literals,
// but it must NEVER produce false negatives (reject strings that match).
//
// Examples:
//   - "^error.*failed$" -> prefix="error", suffix="failed"
//   - "^/api/v1/" -> prefix="/api/v1/"
//   - "warning.*error" -> required=["warning", "error"]
//   - "\\d+.*test" -> required=["test"] (\\d is not a literal)
func extractLiterals(pattern string) *LiteralInfo {
	// Remove dotall prefix if present (AWK dotall mode)
	p := pattern
	if len(p) >= len(dotallPrefix) && p[:len(dotallPrefix)] == dotallPrefix {
		p = p[len(dotallPrefix):]
	}

	if len(p) == 0 {
		return nil
	}

	// Check for alternation at top level - if present, we cannot safely
	// extract required literals (foo|bar means foo OR bar, not both)
	hasTopLevelAlternation := containsTopLevelAlternation(p)

	info := &LiteralInfo{}

	// Check for anchored prefix (^...)
	if p[0] == '^' {
		prefix, rest := extractLiteralPrefix(p[1:])
		if len(prefix) > 0 {
			info.Prefix = prefix
		}
		p = rest
		// If prefix consumed everything, we're done
		if len(p) == 0 {
			if len(info.Prefix) > 0 {
				return info
			}
			return nil
		}
	}

	// Check for anchored suffix (...$)
	if len(p) > 0 && p[len(p)-1] == '$' && !isEscaped(p, len(p)-1) {
		suffix, rest := extractLiteralSuffix(p[:len(p)-1])
		if len(suffix) > 0 {
			info.Suffix = suffix
		}
		p = rest
	}

	// Extract required literals from the middle
	// Skip if there's top-level alternation (foo|bar means only ONE is required)
	if !hasTopLevelAlternation {
		required := extractRequiredLiterals(p)
		if len(required) > 0 {
			info.Required = required
		}
	}

	// Return nil if no useful literals found
	if info.Prefix == "" && info.Suffix == "" && len(info.Required) == 0 {
		return nil
	}

	return info
}

// containsTopLevelAlternation checks if pattern contains | outside of groups.
func containsTopLevelAlternation(p string) bool {
	depth := 0
	inCharClass := false

	for i := 0; i < len(p); i++ {
		c := p[i]

		// Handle escapes
		if c == '\\' && i+1 < len(p) {
			i++ // Skip escaped character
			continue
		}

		// Handle character classes
		if c == '[' && !inCharClass {
			inCharClass = true
			continue
		}
		if c == ']' && inCharClass {
			inCharClass = false
			continue
		}
		if inCharClass {
			continue
		}

		// Handle groups
		if c == '(' {
			depth++
			continue
		}
		if c == ')' && depth > 0 {
			depth--
			continue
		}

		// Check for top-level alternation
		if c == '|' && depth == 0 {
			return true
		}
	}

	return false
}

// extractLiteralPrefix extracts the literal prefix from a pattern.
// Returns the literal prefix and the remaining pattern.
func extractLiteralPrefix(p string) (prefix string, rest string) {
	var sb strings.Builder
	i := 0
	for i < len(p) {
		if isMetaChar(p[i]) {
			break
		}
		if p[i] == '\\' {
			// Check for escape sequence
			if i+1 < len(p) {
				escaped := p[i+1]
				if isLiteralEscape(escaped) {
					// Escaped literal character (e.g., \. \* \+)
					sb.WriteByte(escaped)
					i += 2
					continue
				}
				// Non-literal escape (e.g., \d \s \w) - stop here
				break
			}
			break
		}
		sb.WriteByte(p[i])
		i++
	}
	return sb.String(), p[i:]
}

// extractLiteralSuffix extracts the literal suffix from a pattern (from the end).
// Returns the literal suffix and the remaining pattern (without the suffix).
func extractLiteralSuffix(p string) (suffix string, rest string) {
	if len(p) == 0 {
		return "", ""
	}

	// Find where the literal suffix starts by scanning backwards
	// We need to find the last position where a metachar or special escape occurs
	literalStart := len(p)

	for i := len(p) - 1; i >= 0; i-- {
		c := p[i]

		// Character class or group ends - stop, suffix starts after this
		if c == ']' || c == ')' {
			literalStart = i + 1
			break
		}

		// Metachar - suffix starts after this
		if isMetaChar(c) {
			literalStart = i + 1
			break
		}

		// Check for escape sequence
		if i > 0 && p[i-1] == '\\' {
			// Check how many backslashes precede
			numBackslash := 0
			for j := i - 1; j >= 0 && p[j] == '\\'; j-- {
				numBackslash++
			}
			if numBackslash%2 == 1 {
				// Odd backslashes - this char is escaped
				if isLiteralEscape(c) {
					// Escaped literal (like \.) - include in suffix
					// Continue scanning backwards
					i-- // Skip the backslash too
					continue
				}
				// Non-literal escape (like \d) - suffix starts after this
				literalStart = i + 1
				break
			}
		}
	}

	if literalStart >= len(p) {
		return "", p
	}

	// Build suffix from the literal portion
	suffix = unescapeLiterals(p[literalStart:])
	if suffix == "" {
		return "", p
	}
	return suffix, p[:literalStart]
}

// extractRequiredLiterals extracts literal sequences from the middle of a pattern.
// These are substrings that must appear somewhere in the input.
func extractRequiredLiterals(p string) []string {
	if len(p) == 0 {
		return nil
	}

	var result []string
	var current strings.Builder
	i := 0

	for i < len(p) {
		c := p[i]

		// Handle character classes [...]
		if c == '[' {
			// Flush current literal
			if current.Len() >= 3 {
				result = append(result, current.String())
			}
			current.Reset()
			// Skip entire character class
			i = skipCharClass(p, i)
			continue
		}

		// Handle groups (...)
		if c == '(' {
			// Flush current literal
			if current.Len() >= 3 {
				result = append(result, current.String())
			}
			current.Reset()
			// Skip entire group (we don't extract from groups for safety)
			i = skipGroup(p, i)
			continue
		}

		if isMetaChar(c) {
			// Flush current literal if long enough
			if current.Len() >= 3 { // Minimum useful length
				result = append(result, current.String())
			}
			current.Reset()
			i++
			continue
		}

		if c == '\\' {
			if i+1 < len(p) {
				escaped := p[i+1]
				if isLiteralEscape(escaped) {
					// Escaped literal character
					current.WriteByte(escaped)
					i += 2
					continue
				}
				// Non-literal escape - flush and skip
				if current.Len() >= 3 {
					result = append(result, current.String())
				}
				current.Reset()
				i += 2
				continue
			}
			// Trailing backslash - skip
			i++
			continue
		}

		current.WriteByte(c)
		i++
	}

	// Flush final literal
	if current.Len() >= 3 {
		result = append(result, current.String())
	}

	return result
}

// skipCharClass returns the index after the closing ] of a character class.
func skipCharClass(p string, start int) int {
	if start >= len(p) || p[start] != '[' {
		return start + 1
	}

	i := start + 1

	// Handle ] at start (literal ])
	if i < len(p) && p[i] == ']' {
		i++
	}
	// Handle ^] at start
	if i < len(p) && p[i] == '^' {
		i++
		if i < len(p) && p[i] == ']' {
			i++
		}
	}

	for i < len(p) {
		if p[i] == '\\' && i+1 < len(p) {
			i += 2 // Skip escaped char
			continue
		}
		if p[i] == ']' {
			return i + 1
		}
		i++
	}

	return len(p) // Unclosed - return end
}

// skipGroup returns the index after the closing ) of a group.
func skipGroup(p string, start int) int {
	if start >= len(p) || p[start] != '(' {
		return start + 1
	}

	depth := 1
	i := start + 1

	for i < len(p) && depth > 0 {
		if p[i] == '\\' && i+1 < len(p) {
			i += 2 // Skip escaped char
			continue
		}
		if p[i] == '[' {
			i = skipCharClass(p, i)
			continue
		}
		if p[i] == '(' {
			depth++
		} else if p[i] == ')' {
			depth--
		}
		i++
	}

	return i
}

// isMetaChar returns true if c is a regex metacharacter that cannot be a literal.
func isMetaChar(c byte) bool {
	switch c {
	case '.', '*', '+', '?', '{', '}', '[', ']', '(', ')', '|', '^', '$':
		return true
	}
	return false
}

// isLiteralEscape returns true if the character after backslash represents
// a literal character (not a special escape like \d, \s, \w).
func isLiteralEscape(c byte) bool {
	switch c {
	// These are literal escapes - the backslash just quotes the metachar
	case '.', '*', '+', '?', '{', '}', '[', ']', '(', ')', '|', '^', '$', '\\', '/':
		return true
	}
	return false
}

// isEscaped checks if the character at position i is preceded by an odd number
// of backslashes (meaning it's escaped).
func isEscaped(s string, i int) bool {
	numBackslash := 0
	for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
		numBackslash++
	}
	return numBackslash%2 == 1
}

// unescapeLiterals converts escaped metacharacters back to their literal form.
func unescapeLiterals(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}

	var sb strings.Builder
	sb.Grow(len(s))

	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && isLiteralEscape(s[i+1]) {
			sb.WriteByte(s[i+1])
			i++
		} else {
			sb.WriteByte(s[i])
		}
	}

	return sb.String()
}

// CanReject checks if the literal info can definitely reject the string
// without running the full regex. Returns true if the string cannot match.
// This is the hot path - must have zero allocations.
func (li *LiteralInfo) CanReject(s string) bool {
	if li.Prefix != "" && !strings.HasPrefix(s, li.Prefix) {
		return true
	}
	if li.Suffix != "" && !strings.HasSuffix(s, li.Suffix) {
		return true
	}
	for _, req := range li.Required {
		if !strings.Contains(s, req) {
			return true
		}
	}
	return false
}
