package vm

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kolkov/uawk/internal/compiler"
	"github.com/kolkov/uawk/internal/runtime"
	"github.com/kolkov/uawk/internal/types"
)

// Error types
var (
	ErrNext     = errors.New("next")
	ErrNextFile = errors.New("nextfile")
	ErrBreak    = errors.New("break")
	ErrReturn   = errors.New("return")
)

// Stack size constant.
const (
	// DefaultStackSize is the initial stack capacity.
	DefaultStackSize = 256
)

// Capacity thresholds for buffer pooling.
// Buffers exceeding max capacity are reallocated to base capacity
// to prevent holding peak allocations indefinitely.
// Thresholds are set high to avoid reallocations during normal operation
// while still preventing unbounded growth from pathological inputs.
const (
	baseFieldCapacity = 32   // Initial field buffer capacity
	maxFieldCapacity  = 1024 // Reset to base if exceeds this (was 256)
	basePrintCapacity = 16   // Initial print args capacity
	maxPrintCapacity  = 256  // Reset to base if exceeds this (was 128)
	basePrintBuf      = 256  // Initial print buffer capacity
	maxPrintBuf       = 8192 // Reset to base if exceeds this (was 4096)
)

// asciiSpace is a lookup table for ASCII whitespace characters.
// Using a lookup table provides O(1) constant-time checks vs 4+ comparisons.
// Includes all standard ASCII whitespace: space, tab, newline, carriage return,
// vertical tab, and form feed.
var asciiSpace = [256]bool{
	' ':  true,
	'\t': true,
	'\n': true,
	'\r': true,
	'\v': true,
	'\f': true,
}

// ExitError represents an exit with a status code.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit %d", e.Code)
}

// VM is the AWK virtual machine.
type VM struct {
	program *compiler.Program

	// Value stack (inline for performance - no pointer indirection)
	stackData []types.Value
	sp        int // Stack pointer (index of next free slot)

	// Call stack
	frames []CallFrame

	// Global variables
	scalars []types.Value
	arrays  []map[string]types.Value

	// Special variables
	specials *SpecialVars

	// I/O
	inputReader io.Reader
	input       *bufio.Scanner
	output      io.Writer
	ioManager   *runtime.IOManager

	// Record state - string-based field storage for zero-copy performance
	line         string   // Raw line ($0)
	fieldsStr    []string // Parsed field strings (0-indexed: [0]=$1, [1]=$2, etc.)
	fieldsStrGen []uint32 // Generation when field was explicitly assigned as string
	numFields    int      // NF value
	haveFields   bool     // True if fields were parsed (lazy splitting)
	haveNF       bool     // True if NF was counted (without full split)
	lineIsStr    bool     // True if $0 was explicitly assigned
	lineNum      int      // NR
	fileNum      int      // FNR

	// Generation counter for O(1) state invalidation (vs O(n) memset)
	// Incremented each line - fields from previous lines become "stale"
	generation uint32

	// Compiled regexes (lazily compiled)
	regexes []*runtime.Regex
	// Regex cache for dynamic patterns
	regexCache *runtime.RegexCache

	// Range pattern state
	rangeActive []bool

	// Configuration
	convfmt string // Number to string conversion format
	ofmt    string // Output format for numbers
	ofs     string // Output field separator
	ors     string // Output record separator
	fs      string // Input field separator
	rs      string // Input record separator
	subsep  string // Subscript separator

	// Random number generator (for reproducible srand)
	randSource *rand.Rand

	// Reusable buffers for performance (reduce allocations)
	printArgs []types.Value // Reusable args slice for print
	printBuf  []byte        // Reusable buffer for print output
}

// CallFrame represents a function call on the call stack.
type CallFrame struct {
	fn        *compiler.Function
	ip        int                      // Saved instruction pointer
	bp        int                      // Base pointer (stack position before call)
	locals    []types.Value            // Local scalar variables
	localArrs []map[string]types.Value // Local array variables
	code      []compiler.Opcode        // Code being executed
}

// VMConfig holds VM configuration options.
type VMConfig struct {
	// POSIXRegex enables POSIX leftmost-longest regex matching.
	// When true (default), uses AWK/POSIX ERE semantics (slower but compliant).
	// When false, uses leftmost-first matching (faster, Perl-like).
	POSIXRegex bool
}

// DefaultVMConfig returns the default configuration (POSIX compliant).
func DefaultVMConfig() VMConfig {
	return VMConfig{POSIXRegex: true}
}

// FastVMConfig returns a performance-optimized configuration.
// Disables POSIX regex matching for faster execution.
func FastVMConfig() VMConfig {
	return VMConfig{POSIXRegex: false}
}

// SpecialVars holds AWK special variables.
type SpecialVars struct {
	ARGC     int
	ARGV     map[string]types.Value
	CONVFMT  string
	ENVIRON  map[string]types.Value
	FILENAME string
	FNR      int
	FS       string
	NF       int
	NR       int
	OFMT     string
	OFS      string
	ORS      string
	RLENGTH  int
	RS       string
	RSTART   int
	SUBSEP   string
}

// New creates a new VM for the given compiled program with default POSIX config.
func New(prog *compiler.Program) *VM {
	return NewWithConfig(prog, DefaultVMConfig())
}

// NewWithConfig creates a new VM with the specified configuration.
func NewWithConfig(prog *compiler.Program, config VMConfig) *VM {
	// Create regex config from VM config
	regexConfig := runtime.RegexConfig{POSIX: config.POSIXRegex}

	vm := &VM{
		program:    prog,
		stackData:  make([]types.Value, DefaultStackSize),
		sp:         0,
		frames:     make([]CallFrame, 0, 16),
		scalars:    make([]types.Value, prog.NumScalars),
		arrays:     make([]map[string]types.Value, prog.NumArrays),
		output:     os.Stdout,
		ioManager:  runtime.NewIOManager(),
		regexes:    make([]*runtime.Regex, len(prog.Regexes)),
		regexCache: runtime.NewRegexCacheWithConfig(1000, regexConfig),
		specials:   newSpecialVars(),
		randSource: rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	// Initialize arrays
	for i := range vm.arrays {
		vm.arrays[i] = make(map[string]types.Value)
	}

	// Initialize string-based fields with pre-allocated capacity
	vm.fieldsStr = make([]string, 0, baseFieldCapacity)    // 0-indexed: [0]=$1, [1]=$2, etc.
	vm.fieldsStrGen = make([]uint32, 0, baseFieldCapacity) // Generation tracking for explicit string assignment
	vm.line = ""
	vm.numFields = 0
	vm.haveFields = true // Empty record is "fully parsed"
	vm.lineIsStr = false
	vm.generation = 1 // Start at 1 (0 means "never assigned")

	// Initialize reusable buffers for print
	vm.printArgs = make([]types.Value, 0, basePrintCapacity)
	vm.printBuf = make([]byte, 0, basePrintBuf)

	// Initialize range pattern state
	vm.rangeActive = make([]bool, len(prog.Actions))

	// Sync special vars to VM config
	vm.syncFromSpecials()

	return vm
}

// -----------------------------------------------------------------------------
// Inline Stack Operations
// These methods provide direct stack access without pointer indirection.
// The stack grows automatically when needed.
// -----------------------------------------------------------------------------

// push pushes a value onto the stack.
// Inlined for performance in hot paths.
func (vm *VM) push(v types.Value) {
	if vm.sp >= len(vm.stackData) {
		vm.growStack()
	}
	vm.stackData[vm.sp] = v
	vm.sp++
}

// pop removes and returns the top value from the stack.
// Panics if the stack is empty.
func (vm *VM) pop() types.Value {
	vm.sp--
	return vm.stackData[vm.sp]
}

// peek returns the top value without removing it.
func (vm *VM) peek() types.Value {
	return vm.stackData[vm.sp-1]
}

// peekN returns the value N positions from the top (0 = top).
func (vm *VM) peekN(n int) types.Value {
	return vm.stackData[vm.sp-1-n]
}

// peekPop returns the second-from-top value and pops the top value.
// Useful for binary operations: left = peek, right = pop.
// Returns (second-from-top, top).
func (vm *VM) peekPop() (types.Value, types.Value) {
	vm.sp--
	return vm.stackData[vm.sp-1], vm.stackData[vm.sp]
}

// replaceTop replaces the top value without pop/push overhead.
// Useful for unary operations and binary ops that reuse one operand's slot.
func (vm *VM) replaceTop(v types.Value) {
	vm.stackData[vm.sp-1] = v
}

// drop removes the top value without returning it.
func (vm *VM) drop() {
	vm.sp--
}

// dup duplicates the top value.
func (vm *VM) dup() {
	vm.push(vm.stackData[vm.sp-1])
}

// swap swaps the top two values.
func (vm *VM) swap() {
	vm.stackData[vm.sp-1], vm.stackData[vm.sp-2] = vm.stackData[vm.sp-2], vm.stackData[vm.sp-1]
}

// rote rotates the top three values: [a, b, c] -> [b, c, a]
// where c is on top, a becomes on top after rotation.
func (vm *VM) rote() {
	a := vm.stackData[vm.sp-3]
	b := vm.stackData[vm.sp-2]
	c := vm.stackData[vm.sp-1]
	vm.stackData[vm.sp-3] = b
	vm.stackData[vm.sp-2] = c
	vm.stackData[vm.sp-1] = a
}

// stackPosition returns the current stack pointer.
// Used for saving state before function calls.
func (vm *VM) stackPosition() int {
	return vm.sp
}

// =============================================================================
// Typed Stack Operations (uawk-specific optimization)
// These avoid boxing/unboxing overhead for numeric-heavy workloads.
// Not present in GoAWK - unique to uawk for performance.
// =============================================================================

// popFloat pops the top value and returns it as float64.
// Avoids creating intermediate Value for numeric operations.
func (vm *VM) popFloat() float64 {
	vm.sp--
	return vm.stackData[vm.sp].AsNum()
}

// peekFloat returns the top value as float64 without removing it.
func (vm *VM) peekFloat() float64 {
	return vm.stackData[vm.sp-1].AsNum()
}

// pushFloat pushes a float64 directly as a Num Value.
func (vm *VM) pushFloat(f float64) {
	if vm.sp >= len(vm.stackData) {
		vm.growStack()
	}
	vm.stackData[vm.sp] = types.Num(f)
	vm.sp++
}

// replaceTopFloat replaces the top value with a float64.
func (vm *VM) replaceTopFloat(f float64) {
	vm.stackData[vm.sp-1] = types.Num(f)
}

// peekPopFloat returns (second-from-top as float, top as float) and pops top.
// Optimized for binary numeric operations.
func (vm *VM) peekPopFloat() (float64, float64) {
	vm.sp--
	return vm.stackData[vm.sp-1].AsNum(), vm.stackData[vm.sp].AsNum()
}

// popBool pops the top value and returns it as bool.
func (vm *VM) popBool() bool {
	vm.sp--
	return vm.stackData[vm.sp].AsBool()
}

// replaceTopBool replaces the top value with a bool.
func (vm *VM) replaceTopBool(b bool) {
	vm.stackData[vm.sp-1] = types.Bool(b)
}

// popN returns a view of the top N values and decrements sp.
// WARNING: The returned slice is a view into the stack data.
// The caller must not hold the reference after pushing new values.
// This is an optimization for variadic functions like printf.
func (vm *VM) popN(n int) []types.Value {
	vm.sp -= n
	return vm.stackData[vm.sp : vm.sp+n]
}

// growStack doubles the stack capacity.
func (vm *VM) growStack() {
	newData := make([]types.Value, len(vm.stackData)*2)
	copy(newData, vm.stackData)
	vm.stackData = newData
}

// newSpecialVars creates default special variables.
func newSpecialVars() *SpecialVars {
	env := make(map[string]types.Value)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = types.Str(parts[1])
		}
	}

	return &SpecialVars{
		ARGC:    0,
		ARGV:    make(map[string]types.Value),
		CONVFMT: "%.6g",
		ENVIRON: env,
		FS:      " ",
		OFMT:    "%.6g",
		OFS:     " ",
		ORS:     "\n",
		RS:      "\n",
		SUBSEP:  "\034",
	}
}

// syncFromSpecials syncs special vars to VM config.
func (vm *VM) syncFromSpecials() {
	vm.convfmt = vm.specials.CONVFMT
	vm.ofmt = vm.specials.OFMT
	vm.ofs = vm.specials.OFS
	vm.ors = vm.specials.ORS
	vm.fs = vm.specials.FS
	vm.rs = vm.specials.RS
	vm.subsep = vm.specials.SUBSEP
}

// SetInput sets the input reader.
func (vm *VM) SetInput(r io.Reader) {
	vm.inputReader = r
	// Scanner is set up lazily in processInput to allow BEGIN to set RS
}

// setupScanner creates a scanner with the current RS setting.
func (vm *VM) setupScanner() {
	if vm.inputReader == nil {
		return
	}
	vm.input = bufio.NewScanner(vm.inputReader)

	// Configure split function based on RS
	if vm.rs == "\n" {
		// Default: split on newlines (default scanner behavior)
		return
	}

	if vm.rs == "" {
		// Paragraph mode: split on blank lines
		vm.input.Split(vm.paragraphSplit)
	} else if len(vm.rs) == 1 {
		// Single character RS
		sep := vm.rs[0]
		vm.input.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
			if atEOF && len(data) == 0 {
				return 0, nil, nil
			}
			if i := indexOf(data, sep); i >= 0 {
				return i + 1, data[0:i], nil
			}
			if atEOF {
				return len(data), data, nil
			}
			return 0, nil, nil
		})
	}
	// For multi-char RS, would need regex matching (not implemented)
}

// indexOf finds the first occurrence of byte b in data.
func indexOf(data []byte, b byte) int {
	for i, c := range data {
		if c == b {
			return i
		}
	}
	return -1
}

// paragraphSplit is a split function for paragraph mode (RS="").
func (vm *VM) paragraphSplit(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// Skip leading blank lines
	start := 0
	for start < len(data) {
		if data[start] == '\n' {
			start++
		} else {
			break
		}
	}
	if start >= len(data) {
		if atEOF {
			return len(data), nil, nil
		}
		return 0, nil, nil
	}

	// Find end of paragraph (blank line or double newline)
	for i := start; i < len(data); i++ {
		if i > 0 && data[i] == '\n' && data[i-1] == '\n' {
			// Found blank line
			return i + 1, data[start : i-1], nil
		}
	}

	if atEOF {
		// Return remaining data as last paragraph
		end := len(data)
		for end > start && data[end-1] == '\n' {
			end--
		}
		return len(data), data[start:end], nil
	}

	// Need more data
	return 0, nil, nil
}

// SetOutput sets the output writer.
func (vm *VM) SetOutput(w io.Writer) {
	vm.output = w
}

// SetArgs sets ARGC and ARGV.
func (vm *VM) SetArgs(args []string) {
	vm.specials.ARGC = len(args)
	for i, arg := range args {
		vm.specials.ARGV[strconv.Itoa(i)] = types.Str(arg)
	}
}

// SetFS sets the input field separator.
func (vm *VM) SetFS(fs string) {
	vm.specials.FS = fs
	vm.fs = fs
}

// SetRS sets the input record separator.
func (vm *VM) SetRS(rs string) {
	vm.specials.RS = rs
	vm.rs = rs
}

// SetOFS sets the output field separator.
func (vm *VM) SetOFS(ofs string) {
	vm.specials.OFS = ofs
	vm.ofs = ofs
}

// SetORS sets the output record separator.
func (vm *VM) SetORS(ors string) {
	vm.specials.ORS = ors
	vm.ors = ors
}

// SetVar sets a global variable by name.
// Returns false if the variable is not found.
func (vm *VM) SetVar(name, value string) bool {
	// Check for special variables
	switch name {
	case "FS":
		vm.SetFS(value)
		return true
	case "RS":
		vm.SetRS(value)
		return true
	case "OFS":
		vm.SetOFS(value)
		return true
	case "ORS":
		vm.SetORS(value)
		return true
	case "CONVFMT":
		vm.specials.CONVFMT = value
		vm.convfmt = value
		return true
	case "OFMT":
		vm.specials.OFMT = value
		vm.ofmt = value
		return true
	case "SUBSEP":
		vm.specials.SUBSEP = value
		vm.subsep = value
		return true
	}

	// Check for global scalars
	for i, name2 := range vm.program.ScalarNames {
		if name2 == name {
			vm.scalars[i] = types.Str(value)
			return true
		}
	}

	return false
}

// Run executes the compiled program.
func (vm *VM) Run() error {
	var exitErr *ExitError

	// Execute BEGIN blocks
	if len(vm.program.Begin) > 0 {
		if err := vm.execute(vm.program.Begin); err != nil {
			if exit, ok := err.(*ExitError); ok {
				exitErr = exit
			} else {
				return err
			}
		}
	}

	// Process input (if no exit from BEGIN)
	if exitErr == nil && vm.inputReader != nil {
		if err := vm.processInput(); err != nil {
			if exit, ok := err.(*ExitError); ok {
				exitErr = exit
			} else {
				return err
			}
		}
	}

	// Execute END blocks (always, even after exit)
	if err := vm.executeEnd(); err != nil {
		if exit, ok := err.(*ExitError); ok {
			return exit // Exit in END overrides previous exit
		}
		return err
	}

	// Close all files and pipes
	vm.ioManager.CloseAll()

	// Return the saved exit error if any
	if exitErr != nil {
		return exitErr
	}
	return nil
}

// executeEnd runs END blocks.
func (vm *VM) executeEnd() error {
	if len(vm.program.End) > 0 {
		if err := vm.execute(vm.program.End); err != nil {
			if exit, ok := err.(*ExitError); ok {
				return exit
			}
			return err
		}
	}
	return nil
}

// processInput reads and processes input records.
func (vm *VM) processInput() error {
	// Set up scanner now that BEGIN has run (RS may have been set)
	vm.setupScanner()
	if vm.input == nil {
		return nil
	}

	for vm.input.Scan() {
		line := vm.input.Text()
		vm.lineNum++
		vm.specials.NR = vm.lineNum
		vm.fileNum++
		vm.specials.FNR = vm.fileNum

		// Use lazy field splitting - fields are only parsed when accessed
		vm.setLine(line)

		// Execute each pattern-action rule
		for i, action := range vm.program.Actions {
			matches := false

			if len(action.Pattern) == 0 {
				// No pattern - always matches
				matches = true
			} else if len(action.Pattern) == 1 {
				// Single pattern
				if err := vm.execute(action.Pattern[0]); err != nil {
					return err
				}
				matches = vm.pop().AsBool()
			} else if len(action.Pattern) == 2 {
				// Range pattern
				if !vm.rangeActive[i] {
					// Check start pattern
					if err := vm.execute(action.Pattern[0]); err != nil {
						return err
					}
					if vm.pop().AsBool() {
						vm.rangeActive[i] = true
						matches = true
					}
				} else {
					// Already in range, check end pattern
					matches = true
					if err := vm.execute(action.Pattern[1]); err != nil {
						return err
					}
					if vm.pop().AsBool() {
						vm.rangeActive[i] = false
					}
				}
			}

			if matches {
				if action.Body == nil {
					// Default action: print $0
					fmt.Fprintln(vm.output, vm.line)
				} else if len(action.Body) > 0 {
					if err := vm.execute(action.Body); err != nil {
						if errors.Is(err, ErrNext) {
							break // Skip to next record
						}
						if errors.Is(err, ErrNextFile) {
							// Skip remaining records in this file
							// For single input, this is same as next
							break
						}
						return err
					}
				}
			}
		}
	}

	return vm.input.Err()
}

// setLine sets the current line ($0) without parsing fields.
// This enables lazy field splitting - fields are only parsed when accessed.
// This is a key optimization: programs that don't access fields skip parsing entirely.
// Uses generation-based invalidation: O(1) instead of O(n) memset.
func (vm *VM) setLine(line string) {
	vm.line = line
	vm.lineIsStr = false // From input, not explicit assignment
	vm.haveFields = false
	vm.haveNF = false
	vm.numFields = 0

	// O(1) invalidation: increment generation instead of clearing fieldsStrGen array
	// All fields from previous lines become "stale" (their generation != current)
	vm.generation++
	if vm.generation == 0 {
		// Handle overflow (once per 4B lines) - extremely rare
		vm.generation = 1
		// Clear the generation array only on overflow
		clear(vm.fieldsStrGen)
	}
	// NF will be set when ensureFields or countNF is called
}

// ensureFields ensures that fields are parsed from the current line.
// Uses 0-indexed string storage for zero-copy performance.
// Called lazily when fields are accessed.
// Generation-based tracking eliminates O(n) fieldsIsStr reset per line.
func (vm *VM) ensureFields() {
	if vm.haveFields {
		return
	}
	vm.haveFields = true
	vm.haveNF = true // After full split, NF is known

	// Capacity-aware reset: shrink if buffer grew too large (prevents memory leaks)
	if cap(vm.fieldsStr) > maxFieldCapacity {
		vm.fieldsStr = make([]string, 0, baseFieldCapacity)
		vm.fieldsStrGen = make([]uint32, 0, baseFieldCapacity)
	} else {
		vm.fieldsStr = vm.fieldsStr[:0]
	}
	// Note: fieldsStrGen is NOT reset - generation tracking handles staleness O(1)

	if vm.line == "" {
		vm.numFields = 0
		vm.specials.NF = 0
		return
	}

	if vm.fs == " " {
		// Default FS: split on runs of whitespace (zero-copy, reuses slice)
		vm.splitWhitespace()
	} else if len(vm.fs) == 1 {
		// Single character FS (zero-copy, reuses slice)
		vm.splitSingleChar(vm.fs[0])
	} else if vm.fs != "" {
		// Regex FS - use coregex via cache
		re, err := vm.regexCache.Get(vm.fs)
		if err == nil {
			parts := re.Split(vm.line, -1)
			for _, p := range parts {
				vm.fieldsStr = append(vm.fieldsStr, p)
			}
		}
	}

	// Ensure fieldsStrGen has capacity for all fields
	// O(1) amortized - only extends when needed, not every line
	for len(vm.fieldsStrGen) < len(vm.fieldsStr) {
		vm.fieldsStrGen = append(vm.fieldsStrGen, 0) // 0 = never assigned
	}

	vm.numFields = len(vm.fieldsStr)
	vm.specials.NF = vm.numFields
}

// countNF counts the number of fields without creating substrings.
// This is much faster than ensureFields when only NF is needed.
// Only works for default FS (whitespace) and single-char FS.
func (vm *VM) countNF() {
	if vm.haveNF {
		return
	}
	vm.haveNF = true

	if vm.line == "" {
		vm.numFields = 0
		vm.specials.NF = 0
		return
	}

	if vm.fs == " " {
		// Count whitespace-separated fields
		vm.numFields = vm.countFieldsWhitespace()
	} else if len(vm.fs) == 1 {
		// Count single-char separated fields
		vm.numFields = vm.countFieldsSingleChar(vm.fs[0])
	} else {
		// Regex FS - need full split
		vm.ensureFields()
		return
	}
	vm.specials.NF = vm.numFields
}

// countFieldsWhitespace counts fields without creating substrings.
// Much faster than splitWhitespace when only NF is needed.
func (vm *VM) countFieldsWhitespace() int {
	line := vm.line
	n := len(line)
	i := 0
	count := 0

	// Skip leading whitespace
	for i < n && asciiSpace[line[i]] {
		i++
	}

	for i < n {
		count++
		// Skip non-whitespace (field content)
		for i < n && !asciiSpace[line[i]] {
			i++
		}
		// Skip whitespace between fields
		for i < n && asciiSpace[line[i]] {
			i++
		}
	}
	return count
}

// countFieldsSingleChar counts fields separated by a single character.
// Uses strings.Count which is SIMD-optimized.
func (vm *VM) countFieldsSingleChar(sep byte) int {
	if vm.line == "" {
		return 1 // Empty line still has 1 empty field
	}
	return strings.Count(vm.line, string(sep)) + 1
}

// splitWhitespace splits vm.line on runs of whitespace into vm.fieldsStr.
// Zero-copy: stores substrings directly without Value conversion.
func (vm *VM) splitWhitespace() {
	line := vm.line
	n := len(line)
	i := 0

	// Skip leading whitespace
	for i < n && asciiSpace[line[i]] {
		i++
	}

	for i < n {
		// Find end of field (next whitespace)
		start := i
		for i < n && !asciiSpace[line[i]] {
			i++
		}
		// Add field as substring (zero-copy)
		vm.fieldsStr = append(vm.fieldsStr, line[start:i])
		// Skip whitespace between fields
		for i < n && asciiSpace[line[i]] {
			i++
		}
	}
}

// splitSingleChar splits vm.line on a single character into vm.fieldsStr.
// Uses strings.IndexByte which is SIMD-optimized on modern CPUs.
func (vm *VM) splitSingleChar(sep byte) {
	line := vm.line

	for {
		idx := strings.IndexByte(line, sep)
		if idx < 0 {
			break
		}
		vm.fieldsStr = append(vm.fieldsStr, line[:idx])
		line = line[idx+1:]
	}
	// Handle last field (or entire line if no separator found)
	vm.fieldsStr = append(vm.fieldsStr, line)
}

// splitRecord splits a line into fields immediately.
// Uses setLine + ensureFields internally.
func (vm *VM) splitRecord(line string) {
	vm.setLine(line)
	vm.ensureFields()
}

// execute runs bytecode and returns any error.
func (vm *VM) execute(code []compiler.Opcode) error {
	ip := 0
	for ip < len(code) {
		op := code[ip]
		ip++

		switch op {
		case compiler.Nop:
			// Do nothing

		case compiler.Num:
			idx := int(code[ip])
			ip++
			vm.push(types.Num(vm.program.Nums[idx]))

		case compiler.Str:
			idx := int(code[ip])
			ip++
			vm.push(types.Str(vm.program.Strs[idx]))

		case compiler.Dupe:
			vm.dup()

		case compiler.Drop:
			vm.drop()

		case compiler.Swap:
			vm.swap()

		case compiler.Rote:
			vm.rote()

		case compiler.LoadGlobal:
			idx := int(code[ip])
			ip++
			vm.push(vm.scalars[idx])

		case compiler.LoadLocal:
			idx := int(code[ip])
			ip++
			frame := &vm.frames[len(vm.frames)-1]
			vm.push(frame.locals[idx])

		case compiler.LoadSpecial:
			idx := int(code[ip])
			ip++
			vm.push(vm.getSpecial(idx))

		case compiler.StoreGlobal:
			idx := int(code[ip])
			ip++
			vm.scalars[idx] = vm.pop()

		case compiler.StoreLocal:
			idx := int(code[ip])
			ip++
			frame := &vm.frames[len(vm.frames)-1]
			frame.locals[idx] = vm.pop()

		case compiler.StoreSpecial:
			idx := int(code[ip])
			ip++
			vm.setSpecial(idx, vm.pop())

		case compiler.Field:
			index := int(vm.peek().AsNum())
			vm.replaceTop(vm.getField(index))

		case compiler.FieldInt:
			index := int(code[ip])
			ip++
			vm.push(vm.getField(index))

		case compiler.StoreField:
			index := int(vm.pop().AsNum())
			value := vm.pop()
			vm.setField(index, value)

		case compiler.ArrayGet:
			scope := compiler.Scope(code[ip])
			ip++
			idx := int(code[ip])
			ip++
			key := vm.pop().AsStr(vm.convfmt)
			arr := vm.getArray(scope, idx)
			if v, ok := arr[key]; ok {
				vm.push(v)
			} else {
				// AWK creates the element on access (with empty string value)
				arr[key] = types.Str("")
				vm.push(types.Str(""))
			}

		case compiler.ArraySet:
			scope := compiler.Scope(code[ip])
			ip++
			idx := int(code[ip])
			ip++
			key := vm.pop().AsStr(vm.convfmt)
			value := vm.pop()
			arr := vm.getArray(scope, idx)
			arr[key] = value

		case compiler.ArrayDelete:
			scope := compiler.Scope(code[ip])
			ip++
			idx := int(code[ip])
			ip++
			key := vm.pop().AsStr(vm.convfmt)
			arr := vm.getArray(scope, idx)
			delete(arr, key)

		case compiler.ArrayClear:
			scope := compiler.Scope(code[ip])
			ip++
			idx := int(code[ip])
			ip++
			vm.clearArray(scope, idx)

		case compiler.ArrayIn:
			scope := compiler.Scope(code[ip])
			ip++
			idx := int(code[ip])
			ip++
			key := vm.pop().AsStr(vm.convfmt)
			arr := vm.getArray(scope, idx)
			_, ok := arr[key]
			vm.push(types.Bool(ok))

		case compiler.IncrGlobal:
			amount := float64(code[ip])
			ip++
			idx := int(code[ip])
			ip++
			vm.scalars[idx] = types.Num(vm.scalars[idx].AsNum() + amount)

		case compiler.IncrLocal:
			amount := float64(code[ip])
			ip++
			idx := int(code[ip])
			ip++
			frame := &vm.frames[len(vm.frames)-1]
			frame.locals[idx] = types.Num(frame.locals[idx].AsNum() + amount)

		case compiler.IncrSpecial:
			amount := float64(code[ip])
			ip++
			idx := int(code[ip])
			ip++
			v := vm.getSpecial(idx)
			vm.setSpecial(idx, types.Num(v.AsNum()+amount))

		case compiler.IncrField:
			amount := float64(code[ip])
			ip++
			index := int(vm.pop().AsNum())
			v := vm.getField(index)
			vm.setField(index, types.Num(v.AsNum()+amount))

		case compiler.IncrArray:
			amount := float64(code[ip])
			ip++
			scope := compiler.Scope(code[ip])
			ip++
			idx := int(code[ip])
			ip++
			key := vm.pop().AsStr(vm.convfmt)
			arr := vm.getArray(scope, idx)
			v := arr[key]
			arr[key] = types.Num(v.AsNum() + amount)

		case compiler.AugGlobal:
			augOp := compiler.AugOp(code[ip])
			ip++
			idx := int(code[ip])
			ip++
			rhs := vm.pop().AsNum()
			lhs := vm.scalars[idx].AsNum()
			vm.scalars[idx] = types.Num(vm.applyAugOp(augOp, lhs, rhs))

		case compiler.AugLocal:
			augOp := compiler.AugOp(code[ip])
			ip++
			idx := int(code[ip])
			ip++
			rhs := vm.pop().AsNum()
			frame := &vm.frames[len(vm.frames)-1]
			lhs := frame.locals[idx].AsNum()
			frame.locals[idx] = types.Num(vm.applyAugOp(augOp, lhs, rhs))

		case compiler.AugSpecial:
			augOp := compiler.AugOp(code[ip])
			ip++
			idx := int(code[ip])
			ip++
			rhs := vm.pop().AsNum()
			lhs := vm.getSpecial(idx).AsNum()
			vm.setSpecial(idx, types.Num(vm.applyAugOp(augOp, lhs, rhs)))

		case compiler.AugField:
			augOp := compiler.AugOp(code[ip])
			ip++
			index := int(vm.pop().AsNum())
			rhs := vm.pop().AsNum()
			lhs := vm.getField(index).AsNum()
			vm.setField(index, types.Num(vm.applyAugOp(augOp, lhs, rhs)))

		case compiler.AugArray:
			augOp := compiler.AugOp(code[ip])
			ip++
			scope := compiler.Scope(code[ip])
			ip++
			idx := int(code[ip])
			ip++
			key := vm.pop().AsStr(vm.convfmt)
			rhs := vm.pop().AsNum()
			arr := vm.getArray(scope, idx)
			lhs := arr[key].AsNum()
			arr[key] = types.Num(vm.applyAugOp(augOp, lhs, rhs))

		case compiler.Regex:
			idx := int(code[ip])
			ip++
			re := vm.getRegex(idx)
			matched := re.MatchString(vm.line)
			vm.push(types.Bool(matched))

		case compiler.IndexMulti:
			count := int(code[ip])
			ip++
			parts := make([]string, count)
			for i := count - 1; i >= 0; i-- {
				parts[i] = vm.pop().AsStr(vm.convfmt)
			}
			key := strings.Join(parts, vm.subsep)
			vm.push(types.Str(key))

		case compiler.ConcatMulti:
			count := int(code[ip])
			ip++
			parts := make([]string, count)
			for i := count - 1; i >= 0; i-- {
				parts[i] = vm.pop().AsStr(vm.convfmt)
			}
			vm.push(types.Str(strings.Join(parts, "")))

		case compiler.Add:
			// Optimized: use typed stack ops to avoid boxing/unboxing overhead
			a, b := vm.peekPopFloat()
			vm.replaceTopFloat(a + b)

		case compiler.Subtract:
			a, b := vm.peekPopFloat()
			vm.replaceTopFloat(a - b)

		case compiler.Multiply:
			a, b := vm.peekPopFloat()
			vm.replaceTopFloat(a * b)

		case compiler.Divide:
			a, b := vm.peekPopFloat()
			if b == 0 {
				return fmt.Errorf("division by zero")
			}
			vm.replaceTopFloat(a / b)

		case compiler.Power:
			a, b := vm.peekPopFloat()
			vm.replaceTopFloat(math.Pow(a, b))

		case compiler.Modulo:
			a, b := vm.peekPopFloat()
			if b == 0 {
				return fmt.Errorf("division by zero")
			}
			vm.replaceTopFloat(math.Mod(a, b))

		case compiler.Equal:
			a, b := vm.peekPop()
			an, aIsStr := a.IsTrueStr()
			bn, bIsStr := b.IsTrueStr()
			var result bool
			if aIsStr || bIsStr {
				result = types.Compare(a, b) == 0
			} else {
				result = an == bn
			}
			vm.replaceTop(types.Bool(result))

		case compiler.NotEqual:
			a, b := vm.peekPop()
			an, aIsStr := a.IsTrueStr()
			bn, bIsStr := b.IsTrueStr()
			var result bool
			if aIsStr || bIsStr {
				result = types.Compare(a, b) != 0
			} else {
				result = an != bn
			}
			vm.replaceTop(types.Bool(result))

		case compiler.Less:
			a, b := vm.peekPop()
			an, aIsStr := a.IsTrueStr()
			bn, bIsStr := b.IsTrueStr()
			var result bool
			if aIsStr || bIsStr {
				result = types.Compare(a, b) < 0
			} else {
				result = an < bn
			}
			vm.replaceTop(types.Bool(result))

		case compiler.LessEqual:
			a, b := vm.peekPop()
			an, aIsStr := a.IsTrueStr()
			bn, bIsStr := b.IsTrueStr()
			var result bool
			if aIsStr || bIsStr {
				result = types.Compare(a, b) <= 0
			} else {
				result = an <= bn
			}
			vm.replaceTop(types.Bool(result))

		case compiler.Greater:
			a, b := vm.peekPop()
			an, aIsStr := a.IsTrueStr()
			bn, bIsStr := b.IsTrueStr()
			var result bool
			if aIsStr || bIsStr {
				result = types.Compare(a, b) > 0
			} else {
				result = an > bn
			}
			vm.replaceTop(types.Bool(result))

		case compiler.GreaterEqual:
			a, b := vm.peekPop()
			an, aIsStr := a.IsTrueStr()
			bn, bIsStr := b.IsTrueStr()
			var result bool
			if aIsStr || bIsStr {
				result = types.Compare(a, b) >= 0
			} else {
				result = an >= bn
			}
			vm.replaceTop(types.Bool(result))

		case compiler.Concat:
			a, b := vm.peekPop()
			vm.replaceTop(types.Str(a.AsStr(vm.convfmt) + b.AsStr(vm.convfmt)))

		case compiler.Match:
			str, pattern := vm.peekPop()
			re, err := vm.regexCache.Get(pattern.AsStr(vm.convfmt))
			if err != nil {
				vm.replaceTop(types.Num(0))
			} else {
				vm.replaceTop(types.Bool(re.MatchString(str.AsStr(vm.convfmt))))
			}

		case compiler.NotMatch:
			str, pattern := vm.peekPop()
			re, err := vm.regexCache.Get(pattern.AsStr(vm.convfmt))
			if err != nil {
				vm.replaceTop(types.Num(1))
			} else {
				vm.replaceTop(types.Bool(!re.MatchString(str.AsStr(vm.convfmt))))
			}

		case compiler.UnaryMinus:
			// Optimized: use typed stack ops to avoid boxing/unboxing
			vm.replaceTopFloat(-vm.peekFloat())

		case compiler.UnaryPlus:
			// Optimized: forces numeric conversion using typed ops
			vm.replaceTopFloat(vm.peekFloat())

		case compiler.Not:
			// Optimized: use typed stack ops
			vm.replaceTopBool(!vm.stackData[vm.sp-1].AsBool())

		case compiler.Boolean:
			// Optimized: use typed stack ops
			vm.replaceTopBool(vm.stackData[vm.sp-1].AsBool())

		case compiler.Jump:
			offset := int(code[ip])
			ip++
			ip += offset

		case compiler.JumpTrue:
			offset := int(code[ip])
			ip++
			if vm.pop().AsBool() {
				ip += offset
			}

		case compiler.JumpFalse:
			offset := int(code[ip])
			ip++
			if !vm.pop().AsBool() {
				ip += offset
			}

		case compiler.JumpEqual:
			offset := int(code[ip])
			ip++
			b := vm.pop()
			a := vm.pop()
			an, aIsStr := a.IsTrueStr()
			bn, bIsStr := b.IsTrueStr()
			var cond bool
			if aIsStr || bIsStr {
				cond = types.Compare(a, b) == 0
			} else {
				cond = an == bn
			}
			if cond {
				ip += offset
			}

		case compiler.JumpNotEq:
			offset := int(code[ip])
			ip++
			b := vm.pop()
			a := vm.pop()
			an, aIsStr := a.IsTrueStr()
			bn, bIsStr := b.IsTrueStr()
			var cond bool
			if aIsStr || bIsStr {
				cond = types.Compare(a, b) != 0
			} else {
				cond = an != bn
			}
			if cond {
				ip += offset
			}

		case compiler.JumpLess:
			offset := int(code[ip])
			ip++
			b := vm.pop()
			a := vm.pop()
			an, aIsStr := a.IsTrueStr()
			bn, bIsStr := b.IsTrueStr()
			var cond bool
			if aIsStr || bIsStr {
				cond = types.Compare(a, b) < 0
			} else {
				cond = an < bn
			}
			if cond {
				ip += offset
			}

		case compiler.JumpLessEq:
			offset := int(code[ip])
			ip++
			b := vm.pop()
			a := vm.pop()
			an, aIsStr := a.IsTrueStr()
			bn, bIsStr := b.IsTrueStr()
			var cond bool
			if aIsStr || bIsStr {
				cond = types.Compare(a, b) <= 0
			} else {
				cond = an <= bn
			}
			if cond {
				ip += offset
			}

		case compiler.JumpGreater:
			offset := int(code[ip])
			ip++
			b := vm.pop()
			a := vm.pop()
			an, aIsStr := a.IsTrueStr()
			bn, bIsStr := b.IsTrueStr()
			var cond bool
			if aIsStr || bIsStr {
				cond = types.Compare(a, b) > 0
			} else {
				cond = an > bn
			}
			if cond {
				ip += offset
			}

		case compiler.JumpGrEq:
			offset := int(code[ip])
			ip++
			b := vm.pop()
			a := vm.pop()
			an, aIsStr := a.IsTrueStr()
			bn, bIsStr := b.IsTrueStr()
			var cond bool
			if aIsStr || bIsStr {
				cond = types.Compare(a, b) >= 0
			} else {
				cond = an >= bn
			}
			if cond {
				ip += offset
			}

		case compiler.Next:
			return ErrNext

		case compiler.Nextfile:
			return ErrNextFile

		case compiler.Exit:
			return &ExitError{Code: 0}

		case compiler.ExitCode:
			code := int(vm.pop().AsNum())
			return &ExitError{Code: code}

		case compiler.ForIn:
			varScope := compiler.Scope(code[ip])
			ip++
			varIdx := int(code[ip])
			ip++
			arrScope := compiler.Scope(code[ip])
			ip++
			arrIdx := int(code[ip])
			ip++
			offset := int(code[ip])
			ip++

			arr := vm.getArray(arrScope, arrIdx)
			for key := range arr {
				vm.setScalar(varScope, varIdx, types.Str(key))
				// Execute loop body (code after ForIn until offset)
				bodyEnd := ip + offset
				if err := vm.execute(code[ip:bodyEnd]); err != nil {
					if errors.Is(err, ErrBreak) {
						break
					}
					return err
				}
			}
			ip += offset

		case compiler.BreakForIn:
			return ErrBreak

		case compiler.CallBuiltin:
			builtinOp := compiler.BuiltinOp(code[ip])
			ip++
			if err := vm.callBuiltin(builtinOp); err != nil {
				return err
			}

		case compiler.CallUser:
			funcIdx := int(code[ip])
			ip++
			numArrayArgs := int(code[ip])
			ip++

			fn := &vm.program.Functions[funcIdx]

			// Create new frame
			frame := CallFrame{
				fn:        fn,
				ip:        ip,
				bp:        vm.stackPosition(),
				locals:    make([]types.Value, fn.NumScalars),
				localArrs: make([]map[string]types.Value, fn.NumArrays),
				code:      code,
			}

			// Pop scalar arguments from stack
			for i := fn.NumScalars - 1; i >= 0; i-- {
				frame.locals[i] = vm.pop()
			}

			// Handle array arguments
			for i := 0; i < numArrayArgs; i++ {
				scope := compiler.Scope(code[ip])
				ip++
				arrIdx := int(code[ip])
				ip++
				frame.localArrs[i] = vm.getArray(scope, arrIdx)
			}

			// Initialize remaining local arrays
			for i := numArrayArgs; i < fn.NumArrays; i++ {
				frame.localArrs[i] = make(map[string]types.Value)
			}

			vm.frames = append(vm.frames, frame)

			// Execute function body
			if err := vm.execute(fn.Body); err != nil {
				if errors.Is(err, ErrReturn) {
					// Return value is on stack
				} else {
					vm.frames = vm.frames[:len(vm.frames)-1]
					return err
				}
			} else {
				// No explicit return - push null
				vm.push(types.Null())
			}

			vm.frames = vm.frames[:len(vm.frames)-1]

		case compiler.Return:
			return ErrReturn

		case compiler.ReturnNull:
			vm.push(types.Null())
			return ErrReturn

		case compiler.Nulls:
			count := int(code[ip])
			ip++
			for i := 0; i < count; i++ {
				vm.push(types.Null())
			}

		case compiler.CallSplit:
			scope := compiler.Scope(code[ip])
			ip++
			arrIdx := int(code[ip])
			ip++
			str := vm.pop().AsStr(vm.convfmt)
			n := vm.builtinSplit(str, scope, arrIdx, vm.fs)
			vm.push(types.Num(float64(n)))

		case compiler.CallSplitSep:
			scope := compiler.Scope(code[ip])
			ip++
			arrIdx := int(code[ip])
			ip++
			sep := vm.pop().AsStr(vm.convfmt)
			str := vm.pop().AsStr(vm.convfmt)
			n := vm.builtinSplit(str, scope, arrIdx, sep)
			vm.push(types.Num(float64(n)))

		case compiler.CallSprintf:
			numArgs := int(code[ip])
			ip++
			args := make([]types.Value, numArgs)
			for i := numArgs - 1; i >= 0; i-- {
				args[i] = vm.pop()
			}
			result := vm.builtinSprintf(args)
			vm.push(types.Str(result))

		case compiler.CallLength:
			scope := compiler.Scope(code[ip])
			ip++
			arrIdx := int(code[ip])
			ip++
			arr := vm.getArray(scope, arrIdx)
			vm.push(types.Num(float64(len(arr))))

		case compiler.Print:
			numArgs := int(code[ip])
			ip++
			redirect := compiler.Redirect(code[ip])
			ip++
			vm.executePrint(numArgs, redirect, false)

		case compiler.Printf:
			numArgs := int(code[ip])
			ip++
			redirect := compiler.Redirect(code[ip])
			ip++
			vm.executePrint(numArgs, redirect, true)

		case compiler.Getline:
			redirect := compiler.Redirect(code[ip])
			ip++
			result := vm.executeGetline(redirect, nil)
			vm.push(types.Num(float64(result)))

		case compiler.GetlineVar:
			redirect := compiler.Redirect(code[ip])
			ip++
			scope := compiler.Scope(code[ip])
			ip++
			idx := int(code[ip])
			ip++
			result := vm.executeGetlineVar(redirect, scope, idx)
			vm.push(types.Num(float64(result)))

		case compiler.GetlineField:
			redirect := compiler.Redirect(code[ip])
			ip++
			fieldIdx := int(vm.pop().AsNum())
			result := vm.executeGetlineField(redirect, fieldIdx)
			vm.push(types.Num(float64(result)))

		case compiler.Halt:
			return nil

		default:
			return fmt.Errorf("unknown opcode: %d", op)
		}
	}

	return nil
}

// applyAugOp applies an augmented assignment operation.
func (vm *VM) applyAugOp(op compiler.AugOp, lhs, rhs float64) float64 {
	switch op {
	case compiler.AugAdd:
		return lhs + rhs
	case compiler.AugSub:
		return lhs - rhs
	case compiler.AugMul:
		return lhs * rhs
	case compiler.AugDiv:
		if rhs == 0 {
			return math.Inf(1)
		}
		return lhs / rhs
	case compiler.AugPow:
		return math.Pow(lhs, rhs)
	case compiler.AugMod:
		if rhs == 0 {
			return math.NaN()
		}
		return math.Mod(lhs, rhs)
	default:
		return lhs
	}
}

// getField returns a field value.
// Returns Str for explicitly assigned fields, NumStr for fields from input.
// Uses 0-indexed internal storage: $1 is fieldsStr[0], $2 is fieldsStr[1], etc.
// Generation-based tracking: O(1) check instead of O(n) reset per line.
func (vm *VM) getField(index int) types.Value {
	if index < 0 {
		return types.Str("")
	}
	// $0 is the raw line
	if index == 0 {
		if vm.lineIsStr {
			return types.Str(vm.line) // Explicitly assigned
		}
		return types.NumStr(vm.line) // From input
	}
	// Lazy split: ensure fields are parsed before accessing $1, $2, etc.
	vm.ensureFields()
	idx := index - 1 // Convert to 0-indexed
	if idx < vm.numFields {
		// Check if field was explicitly assigned in current generation
		if idx < len(vm.fieldsStrGen) && vm.fieldsStrGen[idx] == vm.generation {
			return types.Str(vm.fieldsStr[idx]) // Explicitly assigned this line
		}
		return types.NumStr(vm.fieldsStr[idx]) // From input
	}
	return types.Str("")
}

// setField sets a field value.
// Uses 0-indexed string storage with generation-based type tracking.
func (vm *VM) setField(index int, value types.Value) {
	if index < 0 {
		return
	}

	// Track if value is a pure string (not numeric)
	isStr := value.IsStr()

	if index > 0 {
		// Setting $1, $2, etc. - ensure fields are parsed first
		vm.ensureFields()

		idx := index - 1 // Convert to 0-indexed

		// Extend fieldsStr and fieldsStrGen if necessary
		for idx >= vm.numFields {
			vm.fieldsStr = append(vm.fieldsStr, "")
			vm.fieldsStrGen = append(vm.fieldsStrGen, 0) // 0 = not assigned
			vm.numFields++
		}

		// Store as string
		vm.fieldsStr[idx] = value.AsStr(vm.convfmt)
		// Mark as explicit string assignment using generation tracking
		if isStr {
			vm.fieldsStrGen[idx] = vm.generation // Mark as assigned this line
		} else {
			vm.fieldsStrGen[idx] = 0 // Not a string assignment
		}
		vm.specials.NF = vm.numFields

		// Rebuild $0 from fieldsStr using OFS
		vm.rebuildLine()
		vm.lineIsStr = false // Rebuilt $0 is not a "string assignment"
	} else {
		// Setting $0 - re-split into fields
		vm.line = value.AsStr(vm.convfmt)
		vm.lineIsStr = isStr // Track if $0 was assigned as string
		vm.haveFields = false
		vm.ensureFields()
	}
}

// rebuildLine rebuilds vm.line ($0) from fieldsStr using OFS.
// Uses 0-indexed fieldsStr: fieldsStr[0] is $1, fieldsStr[1] is $2, etc.
func (vm *VM) rebuildLine() {
	if vm.numFields == 0 {
		vm.line = ""
		return
	}
	// Build line using OFS
	var buf strings.Builder
	buf.Grow(len(vm.line)) // Pre-allocate roughly same size
	for i := 0; i < vm.numFields; i++ {
		if i > 0 {
			buf.WriteString(vm.ofs)
		}
		buf.WriteString(vm.fieldsStr[i])
	}
	vm.line = buf.String()
}

// getArray returns an array by scope and index.
func (vm *VM) getArray(scope compiler.Scope, idx int) map[string]types.Value {
	switch scope {
	case compiler.ScopeGlobal:
		return vm.arrays[idx]
	case compiler.ScopeLocal:
		frame := &vm.frames[len(vm.frames)-1]
		return frame.localArrs[idx]
	case compiler.ScopeSpecial:
		// ARGV or ENVIRON
		if idx == 2 {
			return vm.specials.ARGV
		}
		return vm.specials.ENVIRON
	default:
		return vm.arrays[idx]
	}
}

// clearArray clears an array.
func (vm *VM) clearArray(scope compiler.Scope, idx int) {
	arr := vm.getArray(scope, idx)
	for k := range arr {
		delete(arr, k)
	}
}

// setScalar sets a scalar variable.
func (vm *VM) setScalar(scope compiler.Scope, idx int, value types.Value) {
	switch scope {
	case compiler.ScopeGlobal:
		vm.scalars[idx] = value
	case compiler.ScopeLocal:
		frame := &vm.frames[len(vm.frames)-1]
		frame.locals[idx] = value
	case compiler.ScopeSpecial:
		vm.setSpecial(idx, value)
	}
}

// getSpecial returns a special variable value.
func (vm *VM) getSpecial(idx int) types.Value {
	switch idx {
	case 1: // ARGC
		return types.Num(float64(vm.specials.ARGC))
	case 3: // CONVFMT
		return types.Str(vm.specials.CONVFMT)
	case 5: // FILENAME
		return types.Str(vm.specials.FILENAME)
	case 6: // FNR
		return types.Num(float64(vm.specials.FNR))
	case 7: // FS
		return types.Str(vm.specials.FS)
	case 8: // NF
		// Use fast countNF instead of full field splitting when possible
		vm.countNF()
		return types.Num(float64(vm.specials.NF))
	case 9: // NR
		return types.Num(float64(vm.specials.NR))
	case 10: // OFMT
		return types.Str(vm.specials.OFMT)
	case 11: // OFS
		return types.Str(vm.specials.OFS)
	case 12: // ORS
		return types.Str(vm.specials.ORS)
	case 13: // RLENGTH
		return types.Num(float64(vm.specials.RLENGTH))
	case 14: // RS
		return types.Str(vm.specials.RS)
	case 15: // RSTART
		return types.Num(float64(vm.specials.RSTART))
	case 16: // SUBSEP
		return types.Str(vm.specials.SUBSEP)
	default:
		return types.Null()
	}
}

// setSpecial sets a special variable value.
func (vm *VM) setSpecial(idx int, value types.Value) {
	switch idx {
	case 1: // ARGC
		vm.specials.ARGC = int(value.AsNum())
	case 3: // CONVFMT
		vm.specials.CONVFMT = value.AsStr(vm.convfmt)
		vm.convfmt = vm.specials.CONVFMT
	case 5: // FILENAME
		vm.specials.FILENAME = value.AsStr(vm.convfmt)
	case 6: // FNR
		vm.specials.FNR = int(value.AsNum())
		vm.fileNum = vm.specials.FNR
	case 7: // FS
		vm.specials.FS = value.AsStr(vm.convfmt)
		vm.fs = vm.specials.FS
	case 8: // NF
		// Ensure fields are parsed before modifying (lazy splitting)
		vm.ensureFields()
		nf := int(value.AsNum())
		vm.specials.NF = nf
		// Adjust fieldsStr and fieldsStrGen arrays (0-indexed: need nf elements)
		for vm.numFields < nf {
			vm.fieldsStr = append(vm.fieldsStr, "")
			vm.fieldsStrGen = append(vm.fieldsStrGen, 0) // 0 = not assigned
			vm.numFields++
		}
		if nf < vm.numFields {
			vm.fieldsStr = vm.fieldsStr[:nf]
			vm.fieldsStrGen = vm.fieldsStrGen[:nf]
			vm.numFields = nf
		}
		// Rebuild $0 from fieldsStr
		vm.rebuildLine()
	case 9: // NR
		vm.specials.NR = int(value.AsNum())
		vm.lineNum = vm.specials.NR
	case 10: // OFMT
		vm.specials.OFMT = value.AsStr(vm.convfmt)
		vm.ofmt = vm.specials.OFMT
	case 11: // OFS
		vm.specials.OFS = value.AsStr(vm.convfmt)
		vm.ofs = vm.specials.OFS
	case 12: // ORS
		vm.specials.ORS = value.AsStr(vm.convfmt)
		vm.ors = vm.specials.ORS
	case 13: // RLENGTH
		vm.specials.RLENGTH = int(value.AsNum())
	case 14: // RS
		vm.specials.RS = value.AsStr(vm.convfmt)
		vm.rs = vm.specials.RS
	case 15: // RSTART
		vm.specials.RSTART = int(value.AsNum())
	case 16: // SUBSEP
		vm.specials.SUBSEP = value.AsStr(vm.convfmt)
		vm.subsep = vm.specials.SUBSEP
	}
}

// getRegex returns a compiled regex, compiling it lazily.
func (vm *VM) getRegex(idx int) *runtime.Regex {
	if vm.regexes[idx] == nil {
		pattern := vm.program.Regexes[idx]
		re, err := runtime.Compile(pattern)
		if err != nil {
			// Return a regex that never matches
			re = runtime.MustCompile(`\A\z`)
		}
		vm.regexes[idx] = re
	}
	return vm.regexes[idx]
}

// executePrint executes a print/printf statement.
// Optimized: uses reusable buffers to minimize allocations.
func (vm *VM) executePrint(numArgs int, redirect compiler.Redirect, isPrintf bool) {
	// Get output destination
	var out io.Writer = vm.output

	if redirect != compiler.RedirectNone {
		// Pop destination (it was pushed first)
		dest := vm.peekN(numArgs).AsStr(vm.convfmt)

		// Get appropriate writer based on redirect type
		var err error
		switch redirect {
		case compiler.RedirectWrite:
			out, err = vm.ioManager.GetOutputFile(dest, false)
		case compiler.RedirectAppend:
			out, err = vm.ioManager.GetOutputFile(dest, true)
		case compiler.RedirectPipe:
			out, err = vm.ioManager.GetOutputPipe(dest)
		}
		if err != nil {
			// On error, silently use stdout
			out = vm.output
		}
	}

	// Capacity-aware reuse: shrink if buffer grew too large
	args := vm.printArgs
	if cap(args) > maxPrintCapacity {
		args = make([]types.Value, 0, basePrintCapacity)
	}
	args = args[:0]
	if cap(args) < numArgs {
		args = make([]types.Value, numArgs)
	} else {
		args = args[:numArgs]
	}
	for i := numArgs - 1; i >= 0; i-- {
		args[i] = vm.pop()
	}
	vm.printArgs = args[:0] // Save for next call

	// Remove destination from stack if redirected
	if redirect != compiler.RedirectNone {
		vm.drop()
	}

	if isPrintf {
		if len(args) > 0 {
			result := vm.builtinSprintf(args)
			io.WriteString(out, result)
		}
	} else {
		// Build output in reusable buffer to minimize io.Writer calls
		// Capacity-aware: shrink if buffer grew too large
		buf := vm.printBuf
		if cap(buf) > maxPrintBuf {
			buf = make([]byte, 0, basePrintBuf)
		}
		buf = buf[:0]

		if len(args) == 0 {
			// print with no args prints $0
			buf = append(buf, vm.line...)
		} else {
			for i, arg := range args {
				if i > 0 {
					buf = append(buf, vm.ofs...)
				}
				buf = append(buf, arg.AsStr(vm.ofmt)...)
			}
		}
		buf = append(buf, vm.ors...)

		out.Write(buf)
		vm.printBuf = buf[:0] // Save for next call
	}
}

// executeGetline executes getline without a target.
func (vm *VM) executeGetline(redirect compiler.Redirect, _ interface{}) int {
	var scanner *bufio.Scanner
	var err error

	switch redirect {
	case compiler.RedirectInput:
		// getline < file
		source := vm.pop().AsStr(vm.convfmt)
		scanner, err = vm.ioManager.GetInputFile(source)
		if err != nil {
			return -1
		}
	case compiler.RedirectPipe:
		// cmd | getline
		source := vm.pop().AsStr(vm.convfmt)
		scanner, err = vm.ioManager.GetInputPipe(source)
		if err != nil {
			return -1
		}
	default:
		// Regular getline from stdin
		scanner = vm.input
	}

	if scanner != nil && scanner.Scan() {
		line := scanner.Text()
		vm.splitRecord(line)
		vm.lineNum++
		vm.specials.NR = vm.lineNum
		vm.fileNum++
		vm.specials.FNR = vm.fileNum
		return 1
	}
	return 0
}

// executeGetlineVar executes getline into a variable.
func (vm *VM) executeGetlineVar(redirect compiler.Redirect, scope compiler.Scope, idx int) int {
	var scanner *bufio.Scanner
	var err error

	switch redirect {
	case compiler.RedirectInput:
		source := vm.pop().AsStr(vm.convfmt)
		scanner, err = vm.ioManager.GetInputFile(source)
		if err != nil {
			return -1
		}
	case compiler.RedirectPipe:
		source := vm.pop().AsStr(vm.convfmt)
		scanner, err = vm.ioManager.GetInputPipe(source)
		if err != nil {
			return -1
		}
	default:
		scanner = vm.input
	}

	if scanner != nil && scanner.Scan() {
		line := scanner.Text()
		vm.setScalar(scope, idx, types.Str(line))
		vm.lineNum++
		vm.specials.NR = vm.lineNum
		vm.fileNum++
		vm.specials.FNR = vm.fileNum
		return 1
	}
	return 0
}

// executeGetlineField executes getline into a field.
func (vm *VM) executeGetlineField(redirect compiler.Redirect, fieldIdx int) int {
	var scanner *bufio.Scanner
	var err error

	switch redirect {
	case compiler.RedirectInput:
		source := vm.pop().AsStr(vm.convfmt)
		scanner, err = vm.ioManager.GetInputFile(source)
		if err != nil {
			return -1
		}
	case compiler.RedirectPipe:
		source := vm.pop().AsStr(vm.convfmt)
		scanner, err = vm.ioManager.GetInputPipe(source)
		if err != nil {
			return -1
		}
	default:
		scanner = vm.input
	}

	if scanner != nil && scanner.Scan() {
		line := scanner.Text()
		vm.setField(fieldIdx, types.Str(line))
		vm.lineNum++
		vm.specials.NR = vm.lineNum
		vm.fileNum++
		vm.specials.FNR = vm.fileNum
		return 1
	}
	return 0
}
