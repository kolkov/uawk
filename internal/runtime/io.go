package runtime

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"sync"
)

// IOManager manages file and pipe I/O for AWK operations.
// It handles file caching (files stay open until explicitly closed)
// and provides thread-safe access to I/O resources.
type IOManager struct {
	mu sync.Mutex

	// Output files (> and >>)
	outFiles map[string]*OutputFile

	// Input files (<)
	inFiles map[string]*InputFile

	// Output pipes (| cmd)
	outPipes map[string]*OutputPipe

	// Input pipes (cmd |)
	inPipes map[string]*InputPipe
}

// OutputFile wraps an os.File for output operations.
type OutputFile struct {
	file   *os.File
	writer *bufio.Writer
}

// InputFile wraps an os.File for input operations.
type InputFile struct {
	file    *os.File
	scanner *bufio.Scanner
}

// OutputPipe wraps an exec.Cmd for pipe output.
type OutputPipe struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	writer *bufio.Writer
}

// InputPipe wraps an exec.Cmd for pipe input.
type InputPipe struct {
	cmd     *exec.Cmd
	stdout  io.ReadCloser
	scanner *bufio.Scanner
}

// NewIOManager creates a new I/O manager.
func NewIOManager() *IOManager {
	return &IOManager{
		outFiles: make(map[string]*OutputFile),
		inFiles:  make(map[string]*InputFile),
		outPipes: make(map[string]*OutputPipe),
		inPipes:  make(map[string]*InputPipe),
	}
}

// GetOutputFile returns an output file for writing, creating it if needed.
// If append is true, opens in append mode.
func (m *IOManager) GetOutputFile(name string, append bool) (*bufio.Writer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already open
	if of, ok := m.outFiles[name]; ok {
		return of.writer, nil
	}

	// Open file
	var flag int
	if append {
		flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	} else {
		flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}

	file, err := os.OpenFile(name, flag, 0644)
	if err != nil {
		return nil, err
	}

	of := &OutputFile{
		file:   file,
		writer: bufio.NewWriter(file),
	}
	m.outFiles[name] = of

	return of.writer, nil
}

// GetInputFile returns an input file for reading, opening it if needed.
func (m *IOManager) GetInputFile(name string) (*bufio.Scanner, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already open
	if inf, ok := m.inFiles[name]; ok {
		return inf.scanner, nil
	}

	// Open file
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	inf := &InputFile{
		file:    file,
		scanner: bufio.NewScanner(file),
	}
	m.inFiles[name] = inf

	return inf.scanner, nil
}

// GetOutputPipe returns an output pipe, creating the command if needed.
func (m *IOManager) GetOutputPipe(cmdStr string) (*bufio.Writer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already running
	if op, ok := m.outPipes[cmdStr]; ok {
		return op.writer, nil
	}

	// Start command
	cmd := exec.Command(getShell(), getShellArg(), cmdStr)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		return nil, err
	}

	op := &OutputPipe{
		cmd:    cmd,
		stdin:  stdin,
		writer: bufio.NewWriter(stdin),
	}
	m.outPipes[cmdStr] = op

	return op.writer, nil
}

// GetInputPipe returns an input pipe, starting the command if needed.
func (m *IOManager) GetInputPipe(cmdStr string) (*bufio.Scanner, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already running
	if ip, ok := m.inPipes[cmdStr]; ok {
		return ip.scanner, nil
	}

	// Start command
	cmd := exec.Command(getShell(), getShellArg(), cmdStr)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		stdout.Close()
		return nil, err
	}

	ip := &InputPipe{
		cmd:     cmd,
		stdout:  stdout,
		scanner: bufio.NewScanner(stdout),
	}
	m.inPipes[cmdStr] = ip

	return ip.scanner, nil
}

// Close closes a file or pipe by name.
// Returns 0 on success, -1 on error or if not found.
func (m *IOManager) Close(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Try output files
	if of, ok := m.outFiles[name]; ok {
		of.writer.Flush()
		err := of.file.Close()
		delete(m.outFiles, name)
		if err != nil {
			return -1
		}
		return 0
	}

	// Try input files
	if inf, ok := m.inFiles[name]; ok {
		err := inf.file.Close()
		delete(m.inFiles, name)
		if err != nil {
			return -1
		}
		return 0
	}

	// Try output pipes
	if op, ok := m.outPipes[name]; ok {
		op.writer.Flush()
		op.stdin.Close()
		err := op.cmd.Wait()
		delete(m.outPipes, name)
		if err != nil {
			return -1
		}
		return 0
	}

	// Try input pipes
	if ip, ok := m.inPipes[name]; ok {
		ip.stdout.Close()
		err := ip.cmd.Wait()
		delete(m.inPipes, name)
		if err != nil {
			return -1
		}
		return 0
	}

	return -1 // Not found
}

// Flush flushes a specific file or all files.
// If name is empty, flushes all output files.
// Returns 0 on success, -1 on error.
func (m *IOManager) Flush(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	if name == "" {
		// Flush all
		for _, of := range m.outFiles {
			of.writer.Flush()
			of.file.Sync()
		}
		for _, op := range m.outPipes {
			op.writer.Flush()
		}
		return 0
	}

	// Flush specific file
	if of, ok := m.outFiles[name]; ok {
		if err := of.writer.Flush(); err != nil {
			return -1
		}
		if err := of.file.Sync(); err != nil {
			return -1
		}
		return 0
	}

	// Flush specific pipe
	if op, ok := m.outPipes[name]; ok {
		if err := op.writer.Flush(); err != nil {
			return -1
		}
		return 0
	}

	return -1 // Not found
}

// CloseAll closes all files and pipes.
func (m *IOManager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, of := range m.outFiles {
		of.writer.Flush()
		of.file.Close()
	}
	m.outFiles = make(map[string]*OutputFile)

	for _, inf := range m.inFiles {
		inf.file.Close()
	}
	m.inFiles = make(map[string]*InputFile)

	for _, op := range m.outPipes {
		op.writer.Flush()
		op.stdin.Close()
		op.cmd.Wait()
	}
	m.outPipes = make(map[string]*OutputPipe)

	for _, ip := range m.inPipes {
		ip.stdout.Close()
		ip.cmd.Wait()
	}
	m.inPipes = make(map[string]*InputPipe)
}

// getShell returns the shell to use for command execution.
func getShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	// Windows
	if comspec := os.Getenv("COMSPEC"); comspec != "" {
		return comspec
	}
	return "sh"
}

// getShellArg returns the argument to pass to the shell for command execution.
func getShellArg() string {
	shell := getShell()
	// Windows cmd.exe uses /c
	if shell == os.Getenv("COMSPEC") || shell == "cmd.exe" || shell == "cmd" {
		return "/c"
	}
	return "-c"
}
