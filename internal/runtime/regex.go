// Package runtime provides AWK runtime support including regex operations.
package runtime

import (
	"sync"

	"github.com/coregx/coregex"
)

// dotallPrefix is prepended to patterns for AWK semantics (dot matches newline).
const dotallPrefix = "(?s)"

// RegexConfig controls regex behavior.
type RegexConfig struct {
	// POSIX enables leftmost-longest matching (AWK/POSIX ERE semantics).
	// When false, uses leftmost-first matching (faster, Perl-like).
	// Default: true for AWK compatibility.
	POSIX bool
}

// DefaultConfig returns the default POSIX-compliant configuration.
func DefaultConfig() RegexConfig {
	return RegexConfig{POSIX: true}
}

// FastConfig returns a performance-optimized configuration.
// Disables POSIX leftmost-longest matching for faster execution.
func FastConfig() RegexConfig {
	return RegexConfig{POSIX: false}
}

// Regex wraps coregex for AWK regex operations.
// Provides thread-safe cached compilation for performance.
// Simple character class patterns use fast path (14-22x speedup).
// Composite patterns like [a-zA-Z]+[0-9]+ use sequential fast path.
// Literal prefiltering provides 5x+ speedup for patterns with literal substrings.
type Regex struct {
	pattern   string
	re        *coregex.Regexp
	charClass *CharClassSearcher // Fast path for simple patterns like \d+, \s+, \w+
	composite *CompositeSearcher // Fast path for composite patterns like [a-zA-Z]+[0-9]+
	literals  *LiteralInfo       // Fast rejection for patterns with literal substrings
	posix     bool               // POSIX leftmost-longest matching enabled
}

// Compile creates a new Regex from pattern with default POSIX config.
// AWK semantics: dot matches any character including newlines.
// Automatically uses fast path for simple character class patterns.
// Extracts literal substrings for prefiltering when applicable.
func Compile(pattern string) (*Regex, error) {
	return CompileWithConfig(pattern, DefaultConfig())
}

// CompileWithConfig creates a new Regex with specified configuration.
// AWK semantics: dot matches any character including newlines.
// When config.POSIX is true, uses leftmost-longest matching (slower but POSIX compliant).
// When config.POSIX is false, uses leftmost-first matching (faster, Perl-like).
func CompileWithConfig(pattern string, config RegexConfig) (*Regex, error) {
	// Prepend dotallPrefix for AWK dotall semantics: . matches \n
	awkPattern := dotallPrefix + pattern

	// Try fast path for simple character class patterns
	charClass := analyzeCharClass(awkPattern)

	// Try composite fast path (e.g., [a-zA-Z]+[0-9]+)
	var composite *CompositeSearcher
	if charClass == nil {
		composite = analyzeComposite(awkPattern)
	}

	// Extract literals for prefiltering (only if no other fast path)
	var literals *LiteralInfo
	if charClass == nil && composite == nil {
		literals = extractLiterals(awkPattern)
	}

	// Still compile the full regex as fallback and for complex operations
	re, err := coregex.Compile(awkPattern)
	if err != nil {
		return nil, err
	}

	// POSIX mode: use leftmost-longest matching (AWK/ERE semantics)
	// Non-POSIX mode: use leftmost-first matching (faster, Perl-like)
	if config.POSIX {
		re.Longest()
	}

	return &Regex{
		pattern:   pattern,
		re:        re,
		charClass: charClass,
		composite: composite,
		literals:  literals,
		posix:     config.POSIX,
	}, nil
}

// MustCompile creates a Regex, panicking on error.
func MustCompile(pattern string) *Regex {
	re, err := Compile(pattern)
	if err != nil {
		panic(err)
	}
	return re
}

// Pattern returns the original pattern string.
func (r *Regex) Pattern() string {
	return r.pattern
}

// IsPOSIX returns true if this regex uses POSIX leftmost-longest matching.
func (r *Regex) IsPOSIX() bool {
	return r.posix
}

// MatchString reports whether s contains any match.
// Uses fast path for simple character class patterns.
// Uses composite fast path for patterns like [a-zA-Z]+[0-9]+.
// Uses literal prefiltering for patterns with literal substrings.
func (r *Regex) MatchString(s string) bool {
	// Fast path 1: CharClass (14-22x speedup for \d+, \s+, \w+, etc.)
	if r.charClass != nil {
		return r.charClass.MatchString(s)
	}

	// Fast path 2: Composite (e.g., [a-zA-Z]+[0-9]+)
	if r.composite != nil {
		return r.composite.MatchString(s)
	}

	// Fast path 3: Literal prefiltering (5x+ speedup for patterns with literals)
	// CanReject is allocation-free and uses SIMD-optimized string functions
	if r.literals != nil && r.literals.CanReject(s) {
		return false
	}

	// Full regex as fallback
	return r.re.MatchString(s)
}

// FindStringIndex returns the start and end of the first match, or nil.
// Uses fast path for simple character class patterns.
// Uses composite fast path for patterns like [a-zA-Z]+[0-9]+.
// Uses literal prefiltering for patterns with literal substrings.
func (r *Regex) FindStringIndex(s string) []int {
	// Fast path 1: CharClass
	if r.charClass != nil {
		return r.charClass.FindStringIndex(s)
	}

	// Fast path 2: Composite
	if r.composite != nil {
		return r.composite.FindStringIndex(s)
	}

	// Fast path 3: Literal prefiltering
	if r.literals != nil && r.literals.CanReject(s) {
		return nil
	}

	// Full regex as fallback
	return r.re.FindStringIndex(s)
}

// FindAllStringIndex returns all non-overlapping matches.
func (r *Regex) FindAllStringIndex(s string, n int) [][]int {
	return r.re.FindAllStringIndex(s, n)
}

// ReplaceAllString replaces all matches with repl.
func (r *Regex) ReplaceAllString(s, repl string) string {
	return r.re.ReplaceAllString(s, repl)
}

// ReplaceAllStringFunc replaces all matches using the function.
func (r *Regex) ReplaceAllStringFunc(s string, f func(string) string) string {
	return r.re.ReplaceAllStringFunc(s, f)
}

// Split slices s into substrings separated by matches.
func (r *Regex) Split(s string, n int) []string {
	return r.re.Split(s, n)
}

// RegexCache provides thread-safe compiled regex caching with FIFO eviction.
// Optimized for AWK workloads: lock-free reads via sync.Map, no LRU overhead.
type RegexCache struct {
	cache   sync.Map    // map[string]*Regex - lock-free reads
	orderMu sync.Mutex  // Protects order slice for eviction
	order   []string    // FIFO order for eviction
	size    int32       // Approximate size (not atomic - orderMu protects it)
	maxSize int
	config  RegexConfig // Configuration for compiled regexes
}

// NewRegexCache creates a cache with specified max size and default POSIX config.
func NewRegexCache(maxSize int) *RegexCache {
	return NewRegexCacheWithConfig(maxSize, DefaultConfig())
}

// NewRegexCacheWithConfig creates a cache with specified max size and config.
func NewRegexCacheWithConfig(maxSize int, config RegexConfig) *RegexCache {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &RegexCache{
		order:   make([]string, 0, maxSize),
		maxSize: maxSize,
		config:  config,
	}
}

// Get returns a compiled regex, compiling and caching if needed.
// Lock-free on cache hit for maximum performance in hot loops.
func (c *RegexCache) Get(pattern string) (*Regex, error) {
	// Fast path: lock-free cache lookup via sync.Map
	if re, ok := c.cache.Load(pattern); ok {
		return re.(*Regex), nil
	}

	// Slow path: compile and cache with configured settings
	re, err := CompileWithConfig(pattern, c.config)
	if err != nil {
		return nil, err
	}

	// Try to store (another goroutine might have stored it already)
	if existing, loaded := c.cache.LoadOrStore(pattern, re); loaded {
		return existing.(*Regex), nil
	}

	// Successfully stored - update eviction order
	c.orderMu.Lock()
	c.order = append(c.order, pattern)
	c.size++

	// Evict oldest if at capacity (FIFO - simpler and good enough for AWK)
	for int(c.size) > c.maxSize && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		c.cache.Delete(oldest)
		c.size--
	}
	c.orderMu.Unlock()

	return re, nil
}

// MustGet returns a compiled regex, panicking on error.
func (c *RegexCache) MustGet(pattern string) *Regex {
	re, err := c.Get(pattern)
	if err != nil {
		panic(err)
	}
	return re
}

// Len returns the approximate number of cached regexes.
func (c *RegexCache) Len() int {
	c.orderMu.Lock()
	n := int(c.size)
	c.orderMu.Unlock()
	return n
}

// Clear removes all cached regexes.
func (c *RegexCache) Clear() {
	c.orderMu.Lock()
	defer c.orderMu.Unlock()
	for _, p := range c.order {
		c.cache.Delete(p)
	}
	c.order = c.order[:0]
	c.size = 0
}

// Config returns the cache's regex configuration.
func (c *RegexCache) Config() RegexConfig {
	return c.config
}
