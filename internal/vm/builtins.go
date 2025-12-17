package vm

import (
	"fmt"
	"math"
	"math/rand"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/kolkov/uawk/internal/compiler"
	"github.com/kolkov/uawk/internal/types"
)

// callBuiltin executes a built-in function.
func (vm *VM) callBuiltin(op compiler.BuiltinOp) error {
	switch op {
	case compiler.BuiltinAtan2:
		// atan2(y, x) - args pushed in order, so pop in reverse
		x := vm.pop().AsNum()
		y := vm.pop().AsNum()
		vm.push(types.Num(math.Atan2(y, x)))

	case compiler.BuiltinClose:
		name := vm.pop().AsStr(vm.convfmt)
		result := vm.closeFile(name)
		vm.push(types.Num(float64(result)))

	case compiler.BuiltinCos:
		x := vm.pop().AsNum()
		vm.push(types.Num(math.Cos(x)))

	case compiler.BuiltinExp:
		x := vm.pop().AsNum()
		vm.push(types.Num(math.Exp(x)))

	case compiler.BuiltinFflush:
		name := vm.pop().AsStr(vm.convfmt)
		result := vm.flushFile(name)
		vm.push(types.Num(float64(result)))

	case compiler.BuiltinFflushAll:
		result := vm.flushAll()
		vm.push(types.Num(float64(result)))

	case compiler.BuiltinGsub:
		target := vm.pop().AsStr(vm.convfmt)
		replacement := vm.pop().AsStr(vm.convfmt)
		pattern := vm.pop().AsStr(vm.convfmt)
		result, count := vm.builtinGsub(pattern, replacement, target)
		// Push both count and result (result on top for assignment)
		vm.push(types.Num(float64(count)))
		vm.push(types.Str(result))

	case compiler.BuiltinIndex:
		substr := vm.pop().AsStr(vm.convfmt)
		str := vm.pop().AsStr(vm.convfmt)
		idx := strings.Index(str, substr)
		if idx < 0 {
			vm.push(types.Num(0))
		} else {
			// AWK uses 1-based indexing
			vm.push(types.Num(float64(idx + 1)))
		}

	case compiler.BuiltinInt:
		x := vm.pop().AsNum()
		vm.push(types.Num(math.Trunc(x)))

	case compiler.BuiltinLength:
		// length() with no args - length of $0
		vm.push(types.Num(float64(len(vm.line))))

	case compiler.BuiltinLengthArg:
		s := vm.pop().AsStr(vm.convfmt)
		vm.push(types.Num(float64(len(s))))

	case compiler.BuiltinLog:
		x := vm.pop().AsNum()
		vm.push(types.Num(math.Log(x)))

	case compiler.BuiltinMatch:
		// match(str, pattern) - args pushed in order, pop in reverse
		pattern := vm.pop().AsStr(vm.convfmt)
		str := vm.pop().AsStr(vm.convfmt)
		rstart, rlength := vm.builtinMatch(str, pattern)
		vm.specials.RSTART = rstart
		vm.specials.RLENGTH = rlength
		vm.push(types.Num(float64(rstart)))

	case compiler.BuiltinRand:
		vm.push(types.Num(vm.randSource.Float64()))

	case compiler.BuiltinSin:
		x := vm.pop().AsNum()
		vm.push(types.Num(math.Sin(x)))

	case compiler.BuiltinSqrt:
		x := vm.pop().AsNum()
		vm.push(types.Num(math.Sqrt(x)))

	case compiler.BuiltinSrand:
		// srand() with no args - use current time
		seed := time.Now().UnixNano()
		vm.randSource = rand.New(rand.NewSource(seed))
		vm.push(types.Num(float64(seed)))

	case compiler.BuiltinSrandSeed:
		seed := int64(vm.pop().AsNum())
		vm.randSource = rand.New(rand.NewSource(seed))
		vm.push(types.Num(float64(seed)))

	case compiler.BuiltinSub:
		target := vm.pop().AsStr(vm.convfmt)
		replacement := vm.pop().AsStr(vm.convfmt)
		pattern := vm.pop().AsStr(vm.convfmt)
		result, count := vm.builtinSub(pattern, replacement, target)
		// Push both count and result (result on top for assignment)
		vm.push(types.Num(float64(count)))
		vm.push(types.Str(result))

	case compiler.BuiltinSubstr:
		// substr(s, start) - from start to end
		start := int(vm.pop().AsNum())
		s := vm.pop().AsStr(vm.convfmt)
		result := vm.builtinSubstr(s, start, len(s))
		vm.push(types.Str(result))

	case compiler.BuiltinSubstrLen:
		// substr(s, start, length)
		length := int(vm.pop().AsNum())
		start := int(vm.pop().AsNum())
		s := vm.pop().AsStr(vm.convfmt)
		result := vm.builtinSubstr(s, start, length)
		vm.push(types.Str(result))

	case compiler.BuiltinSystem:
		cmd := vm.pop().AsStr(vm.convfmt)
		result := vm.builtinSystem(cmd)
		vm.push(types.Num(float64(result)))

	case compiler.BuiltinTolower:
		s := vm.pop().AsStr(vm.convfmt)
		vm.push(types.Str(toLowerASCII(s)))

	case compiler.BuiltinToupper:
		s := vm.pop().AsStr(vm.convfmt)
		vm.push(types.Str(toUpperASCII(s)))

	default:
		return fmt.Errorf("unknown builtin op: %d", op)
	}

	return nil
}

// builtinSplit splits a string into an array.
func (vm *VM) builtinSplit(str string, scope compiler.Scope, arrIdx int, sep string) int {
	arr := vm.getArray(scope, arrIdx)

	// Clear the array first
	for k := range arr {
		delete(arr, k)
	}

	// Empty string returns 0 elements
	if str == "" {
		return 0
	}

	var parts []string
	if sep == " " {
		// Default separator: split on runs of whitespace
		parts = strings.Fields(str)
	} else if len(sep) == 1 {
		// Single character separator
		parts = strings.Split(str, sep)
	} else if sep == "" {
		// Empty separator: split into individual characters
		parts = make([]string, len(str))
		for i, r := range str {
			parts[i] = string(r)
		}
	} else {
		// Regex separator - use coregex via cache
		re, err := vm.regexCache.Get(sep)
		if err != nil {
			parts = []string{str}
		} else {
			parts = re.Split(str, -1)
		}
	}

	for i, part := range parts {
		arr[strconv.Itoa(i+1)] = types.Str(part)
	}

	return len(parts)
}

// builtinSprintf implements sprintf with AWK-compatible formatting.
func (vm *VM) builtinSprintf(args []types.Value) string {
	if len(args) == 0 {
		return ""
	}

	format := args[0].AsStr(vm.convfmt)
	values := args[1:]

	var result strings.Builder
	valueIdx := 0

	// Helper to get next value
	getNextValue := func() types.Value {
		if valueIdx < len(values) {
			v := values[valueIdx]
			valueIdx++
			return v
		}
		return types.Null()
	}

	i := 0
	for i < len(format) {
		if format[i] != '%' {
			result.WriteByte(format[i])
			i++
			continue
		}

		// Found a % - parse format specifier
		i++
		if i >= len(format) {
			result.WriteByte('%')
			break
		}

		// Handle %%
		if format[i] == '%' {
			result.WriteByte('%')
			i++
			continue
		}

		// Parse flags: -+ #0
		var flags strings.Builder
		for i < len(format) && strings.ContainsAny(string(format[i]), "-+ #0") {
			flags.WriteByte(format[i])
			i++
		}

		// Parse width (may be * for dynamic)
		var width string
		if i < len(format) && format[i] == '*' {
			// Dynamic width from argument
			w := int(getNextValue().AsNum())
			if w < 0 {
				flags.WriteByte('-')
				w = -w
			}
			width = strconv.Itoa(w)
			i++
		} else {
			for i < len(format) && format[i] >= '0' && format[i] <= '9' {
				width += string(format[i])
				i++
			}
		}

		// Parse precision
		var precision string
		if i < len(format) && format[i] == '.' {
			precision = "."
			i++
			if i < len(format) && format[i] == '*' {
				// Dynamic precision from argument
				p := int(getNextValue().AsNum())
				if p < 0 {
					precision = "" // negative precision is ignored
				} else {
					precision += strconv.Itoa(p)
				}
				i++
			} else {
				for i < len(format) && format[i] >= '0' && format[i] <= '9' {
					precision += string(format[i])
					i++
				}
			}
		}

		if i >= len(format) {
			result.WriteString("%" + flags.String() + width + precision)
			break
		}

		specifier := format[i]
		i++

		// Get the value for this specifier
		value := getNextValue()

		// Build Go format string and format value
		switch specifier {
		case 'd', 'i':
			// %i is same as %d in AWK
			goFmt := "%" + flags.String() + width + precision + "d"
			result.WriteString(fmt.Sprintf(goFmt, int64(value.AsNum())))
		case 'o':
			goFmt := "%" + flags.String() + width + precision + "o"
			result.WriteString(fmt.Sprintf(goFmt, uint64(value.AsNum())))
		case 'x':
			goFmt := "%" + flags.String() + width + precision + "x"
			result.WriteString(fmt.Sprintf(goFmt, uint64(value.AsNum())))
		case 'X':
			goFmt := "%" + flags.String() + width + precision + "X"
			result.WriteString(fmt.Sprintf(goFmt, uint64(value.AsNum())))
		case 'u':
			// %u is unsigned decimal - use %d with uint64
			goFmt := "%" + flags.String() + width + precision + "d"
			result.WriteString(fmt.Sprintf(goFmt, uint64(value.AsNum())))
		case 'c':
			// %c: if number, use as ASCII code; if string, use first char
			// AWK convention: number takes precedence for %c
			if value.IsNum() || value.IsNull() {
				n := int(value.AsNum())
				// Any byte value is valid (0-255)
				if n >= 0 && n <= 255 {
					result.WriteByte(byte(n))
				}
			} else {
				// String value - use first character
				s := value.AsStr(vm.convfmt)
				if len(s) > 0 {
					result.WriteByte(s[0])
				}
			}
		case 's':
			s := value.AsStr(vm.convfmt)
			goFmt := "%" + flags.String() + width + precision + "s"
			result.WriteString(fmt.Sprintf(goFmt, s))
		case 'e':
			goFmt := "%" + flags.String() + width + precision + "e"
			result.WriteString(fmt.Sprintf(goFmt, value.AsNum()))
		case 'E':
			goFmt := "%" + flags.String() + width + precision + "E"
			result.WriteString(fmt.Sprintf(goFmt, value.AsNum()))
		case 'f', 'F':
			goFmt := "%" + flags.String() + width + precision + "f"
			result.WriteString(fmt.Sprintf(goFmt, value.AsNum()))
		case 'g':
			goFmt := "%" + flags.String() + width + precision + "g"
			result.WriteString(fmt.Sprintf(goFmt, value.AsNum()))
		case 'G':
			goFmt := "%" + flags.String() + width + precision + "G"
			result.WriteString(fmt.Sprintf(goFmt, value.AsNum()))
		default:
			result.WriteByte('%')
			result.WriteByte(specifier)
		}
	}

	return result.String()
}

// builtinSubstr implements substr.
// AWK substr(s, start[, length]) uses 1-based indexing.
// If start < 1, it's treated as 1 (beginning of string).
// If start+length extends beyond string, returns to end of string.
func (vm *VM) builtinSubstr(s string, start, length int) string {
	// AWK uses 1-based indexing
	// If start < 1, treat as 1 (POSIX behavior)
	if start < 1 {
		start = 1
	}

	// Convert to 0-based for Go
	start--

	if start >= len(s) || length <= 0 {
		return ""
	}

	end := start + length
	if end > len(s) {
		end = len(s)
	}

	return s[start:end]
}

// builtinMatch implements match.
func (vm *VM) builtinMatch(str, pattern string) (int, int) {
	re, err := vm.regexCache.Get(pattern)
	if err != nil {
		return 0, -1
	}

	loc := re.FindStringIndex(str)
	if loc == nil {
		return 0, -1
	}

	// AWK uses 1-based indexing
	return loc[0] + 1, loc[1] - loc[0]
}

// builtinSub implements sub (single substitution).
func (vm *VM) builtinSub(pattern, replacement, target string) (string, int) {
	re, err := vm.regexCache.Get(pattern)
	if err != nil {
		return target, 0
	}

	loc := re.FindStringIndex(target)
	if loc == nil {
		return target, 0
	}

	// Handle & in replacement (matched string)
	matched := target[loc[0]:loc[1]]
	repl := handleAwkReplacement(replacement, matched)

	result := target[:loc[0]] + repl + target[loc[1]:]
	return result, 1
}

// builtinGsub implements gsub (global substitution).
func (vm *VM) builtinGsub(pattern, replacement, target string) (string, int) {
	re, err := vm.regexCache.Get(pattern)
	if err != nil {
		return target, 0
	}

	count := 0
	result := re.ReplaceAllStringFunc(target, func(matched string) string {
		count++
		return handleAwkReplacement(replacement, matched)
	})

	return result, count
}

// handleAwkReplacement handles AWK replacement string semantics.
// & is replaced with the matched string, \& is a literal &.
func handleAwkReplacement(replacement, matched string) string {
	var result strings.Builder
	i := 0
	for i < len(replacement) {
		if replacement[i] == '\\' && i+1 < len(replacement) {
			next := replacement[i+1]
			if next == '&' {
				result.WriteByte('&')
				i += 2
				continue
			} else if next == '\\' {
				result.WriteByte('\\')
				i += 2
				continue
			}
		}
		if replacement[i] == '&' {
			result.WriteString(matched)
		} else {
			result.WriteByte(replacement[i])
		}
		i++
	}
	return result.String()
}

// builtinSystem executes a shell command.
func (vm *VM) builtinSystem(cmd string) int {
	c := exec.Command("sh", "-c", cmd)
	c.Stdout = vm.output
	c.Stderr = vm.output

	err := c.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}

// closeFile closes a file or pipe.
func (vm *VM) closeFile(name string) int {
	return vm.ioManager.Close(name)
}

// flushFile flushes a specific file.
func (vm *VM) flushFile(name string) int {
	return vm.ioManager.Flush(name)
}

// flushAll flushes all files and stdout.
func (vm *VM) flushAll() int {
	// Flush stdout if it's a flushable writer
	if f, ok := vm.output.(interface{ Flush() error }); ok {
		f.Flush()
	}
	return vm.ioManager.Flush("")
}

// toLowerASCII converts string to lowercase with ASCII fast path.
// For pure ASCII strings (90%+ of AWK input), uses byte arithmetic
// instead of Unicode table lookups - 2-3x faster.
func toLowerASCII(s string) string {
	// Fast scan: check if all ASCII and find first uppercase
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			// Found uppercase - need to convert
			return toLowerASCIISlow(s, i)
		}
		if c > 127 {
			// Non-ASCII - fallback to stdlib
			return strings.ToLower(s)
		}
	}
	return s // Already lowercase or no letters
}

// toLowerASCIISlow handles the conversion when uppercase is found.
func toLowerASCIISlow(s string, start int) string {
	b := make([]byte, len(s))
	copy(b, s[:start])
	for i := start; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32 // ASCII lowercase offset
		} else if c > 127 {
			// Non-ASCII found mid-string - fallback
			return strings.ToLower(s)
		} else {
			b[i] = c
		}
	}
	return string(b)
}

// toUpperASCII converts string to uppercase with ASCII fast path.
// For pure ASCII strings, uses byte arithmetic instead of Unicode tables.
func toUpperASCII(s string) string {
	// Fast scan: check if all ASCII and find first lowercase
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			// Found lowercase - need to convert
			return toUpperASCIISlow(s, i)
		}
		if c > 127 {
			// Non-ASCII - fallback to stdlib
			return strings.ToUpper(s)
		}
	}
	return s // Already uppercase or no letters
}

// toUpperASCIISlow handles the conversion when lowercase is found.
func toUpperASCIISlow(s string, start int) string {
	b := make([]byte, len(s))
	copy(b, s[:start])
	for i := start; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32 // ASCII uppercase offset
		} else if c > 127 {
			// Non-ASCII found mid-string - fallback
			return strings.ToUpper(s)
		} else {
			b[i] = c
		}
	}
	return string(b)
}
