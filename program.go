package uawk

import (
	"bytes"
	"io"

	"github.com/kolkov/uawk/internal/compiler"
	"github.com/kolkov/uawk/internal/vm"
)

// Program represents a compiled AWK program ready for execution.
// It is safe for concurrent use; each call to Run creates an
// independent execution context.
type Program struct {
	compiled *compiler.Program
	source   string // Original source for debugging
}

// Run executes the compiled program with the given input and configuration.
// Returns the output as a string, or an error if execution fails.
//
// If config is nil, default configuration is used.
// If config.Output is set, output is written there and the returned
// string will be empty.
func (p *Program) Run(input io.Reader, config *Config) (string, error) {
	if config == nil {
		config = &Config{}
	}
	config.applyDefaults()

	// Create VM with regex configuration
	v := p.createVM(config)
	defer p.putVM(v)

	// Configure VM
	configureVM(v, config)

	// Set input
	v.SetInput(input)

	// Set output capture if not provided
	var outputBuf *bytes.Buffer
	if config.Output == nil {
		outputBuf = &bytes.Buffer{}
		v.SetOutput(outputBuf)
	} else {
		v.SetOutput(config.Output)
	}

	// Execute
	err := v.Run()

	// Handle exit error (normal program termination)
	if err != nil {
		if exitErr, ok := err.(*vm.ExitError); ok {
			if exitErr.Code != 0 {
				return outputBuf.String(), &ExitError{Code: exitErr.Code}
			}
			// exit 0 is success, not an error
			err = nil
		}
	}

	if err != nil {
		return "", &RuntimeError{Message: err.Error()}
	}

	if outputBuf != nil {
		return outputBuf.String(), nil
	}
	return "", nil
}

// Disassemble returns a human-readable representation of the compiled bytecode.
// Useful for debugging and understanding program structure.
func (p *Program) Disassemble() string {
	return p.compiled.Disassemble()
}

// Source returns the original AWK source code.
func (p *Program) Source() string {
	return p.source
}

// createVM creates a new VM with the specified configuration.
func (p *Program) createVM(config *Config) *vm.VM {
	// Determine POSIX regex mode (default: true for AWK compatibility)
	posixRegex := true
	if config.POSIXRegex != nil {
		posixRegex = *config.POSIXRegex
	}

	vmConfig := vm.VMConfig{POSIXRegex: posixRegex}
	return vm.NewWithConfig(p.compiled, vmConfig)
}

// putVM returns a VM to the pool for reuse.
func (p *Program) putVM(v *vm.VM) {
	// Note: VM would need a Reset() method for proper reuse
	// For now, we don't reuse VMs to ensure clean state
	// p.vmPool.Put(v)
}

// configureVM applies Config settings to a VM.
func configureVM(v *vm.VM, config *Config) {
	// Set args
	if len(config.Args) > 0 {
		v.SetArgs(config.Args)
	}

	// Apply field/record separators
	if config.FS != "" && config.FS != " " {
		v.SetFS(config.FS)
	}
	if config.RS != "" && config.RS != "\n" {
		v.SetRS(config.RS)
	}
	if config.OFS != "" && config.OFS != " " {
		v.SetOFS(config.OFS)
	}
	if config.ORS != "" && config.ORS != "\n" {
		v.SetORS(config.ORS)
	}

	// Apply custom variables
	for name, value := range config.Variables {
		v.SetVar(name, value)
	}
}
