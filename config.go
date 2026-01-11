package uawk

import "io"

// Config holds configuration options for AWK execution.
type Config struct {
	// FS is the input field separator (default: " ").
	// When set to a single space, runs of whitespace are treated as separators.
	// Otherwise, each occurrence of the string is a separator.
	// Can also be a regular expression pattern.
	FS string

	// RS is the input record separator (default: "\n").
	// When set to empty string, records are separated by blank lines.
	RS string

	// OFS is the output field separator (default: " ").
	// Used when printing multiple values with print statement.
	OFS string

	// ORS is the output record separator (default: "\n").
	// Appended after each print statement.
	ORS string

	// Variables contains pre-defined variables.
	// These are set before BEGIN block execution.
	// Example: map[string]string{"threshold": "100", "prefix": "LOG:"}
	Variables map[string]string

	// Output is the writer for print/printf statements.
	// If nil, output is captured and returned from Run.
	Output io.Writer

	// Stderr is the writer for error output.
	// If nil, errors are discarded.
	Stderr io.Writer

	// Args contains command-line arguments (ARGV).
	// Args[0] is typically the program name.
	Args []string

	// POSIXRegex enables POSIX leftmost-longest regex matching.
	// When true (default), uses AWK/POSIX ERE semantics (slower but compliant).
	// When false, uses leftmost-first matching (faster, Perl-like).
	// Set to false for better performance when POSIX compliance is not required.
	POSIXRegex *bool
}

// applyDefaults fills in default values for unset Config fields.
func (c *Config) applyDefaults() {
	if c.FS == "" {
		c.FS = " "
	}
	if c.RS == "" {
		c.RS = "\n"
	}
	if c.OFS == "" {
		c.OFS = " "
	}
	if c.ORS == "" {
		c.ORS = "\n"
	}
}
