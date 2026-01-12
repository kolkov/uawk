// Package vm provides the AWK virtual machine implementation.
// This file implements parallel execution for uawk (P2-006).
package vm

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"runtime"
	"sync"

	"github.com/kolkov/uawk/internal/compiler"
	"github.com/kolkov/uawk/internal/types"
)

// ParallelConfig holds configuration for parallel execution.
type ParallelConfig struct {
	// NumWorkers is the number of parallel worker goroutines.
	// Default: runtime.NumCPU()
	NumWorkers int

	// ChunkSize is the approximate size in bytes of each input chunk.
	// Default: 4MB (4 * 1024 * 1024)
	ChunkSize int

	// MaxBufferedChunks limits memory usage by blocking when too many
	// chunks are waiting to be processed.
	// Default: NumWorkers * 2
	MaxBufferedChunks int
}

// DefaultParallelConfig returns sensible defaults for parallel execution.
func DefaultParallelConfig() ParallelConfig {
	numCPU := runtime.NumCPU()
	return ParallelConfig{
		NumWorkers:        numCPU,
		ChunkSize:         4 * 1024 * 1024, // 4MB chunks
		MaxBufferedChunks: numCPU * 2,
	}
}

// ParallelExecutor coordinates parallel execution of AWK programs.
// It manages worker goroutines, input chunking, and output aggregation.
type ParallelExecutor struct {
	program  *compiler.Program
	config   ParallelConfig
	vmConfig VMConfig

	// Aggregation state (protected by mutex)
	mu      sync.Mutex
	scalars []types.Value            // Initial scalar values from BEGIN
	arrays  []map[string]types.Value // Initial array values from BEGIN
	totalNR int                      // Total records processed

	// Analysis results for smart aggregation
	analysis *ParallelAnalysis
}

// WorkerResult contains the results from a single worker processing a chunk.
type WorkerResult struct {
	ChunkID int
	Output  []byte
	Scalars []types.Value
	Arrays  []map[string]types.Value
	NR      int // Number of records processed
	StartNR int // Starting NR for this chunk
	Err     error
}

// NewParallelExecutor creates a new parallel executor for the given program.
func NewParallelExecutor(prog *compiler.Program, vmConfig VMConfig, config ParallelConfig) *ParallelExecutor {
	if config.NumWorkers <= 0 {
		config.NumWorkers = runtime.NumCPU()
	}
	if config.ChunkSize <= 0 {
		config.ChunkSize = 4 * 1024 * 1024
	}
	if config.MaxBufferedChunks <= 0 {
		config.MaxBufferedChunks = config.NumWorkers * 2
	}

	// Analyze program for smart aggregation
	analysis := AnalyzeParallelSafety(prog, "\n")

	return &ParallelExecutor{
		program:  prog,
		config:   config,
		vmConfig: vmConfig,
		scalars:  make([]types.Value, prog.NumScalars),
		arrays:   make([]map[string]types.Value, prog.NumArrays),
		analysis: analysis,
	}
}

// Run executes the program in parallel mode.
// BEGIN and END blocks are executed serially; main loop runs in parallel.
func (pe *ParallelExecutor) Run(ctx context.Context, input io.Reader, output io.Writer) error {
	// Phase 1: Execute BEGIN block (single-threaded)
	beginVM := NewWithConfig(pe.program, pe.vmConfig)
	beginVM.SetOutput(output)

	if len(pe.program.Begin) > 0 {
		if err := beginVM.execute(pe.program.Begin); err != nil {
			if exit, ok := err.(*ExitError); ok {
				// Exit in BEGIN - skip main loop but run END
				return pe.runEnd(beginVM, output, exit)
			}
			return err
		}
	}

	// Copy BEGIN state to aggregation state (initial values for workers)
	pe.copyStateFrom(beginVM)

	// Phase 2: Process input in parallel
	if input != nil && len(pe.program.Actions) > 0 {
		if err := pe.processInputParallel(ctx, input, output, beginVM); err != nil {
			if exit, ok := err.(*ExitError); ok {
				return pe.runEnd(beginVM, output, exit)
			}
			return err
		}
	}

	// Phase 3: Execute END block (single-threaded)
	return pe.runEnd(beginVM, output, nil)
}

// runEnd executes the END block with aggregated state.
func (pe *ParallelExecutor) runEnd(vm *VM, output io.Writer, prevExit *ExitError) error {
	if len(pe.program.End) == 0 {
		if prevExit != nil {
			return prevExit
		}
		return nil
	}

	// Copy aggregated state back to VM for END block
	pe.mu.Lock()
	copy(vm.scalars, pe.scalars)
	for i, arr := range pe.arrays {
		if arr != nil {
			vm.arrays[i] = arr
		}
	}
	vm.specials.NR = pe.totalNR
	pe.mu.Unlock()

	vm.SetOutput(output)
	if err := vm.execute(pe.program.End); err != nil {
		if exit, ok := err.(*ExitError); ok {
			return exit
		}
		return err
	}

	if prevExit != nil {
		return prevExit
	}
	return nil
}

// processInputParallel processes input using parallel workers.
func (pe *ParallelExecutor) processInputParallel(
	ctx context.Context,
	input io.Reader,
	output io.Writer,
	templateVM *VM,
) error {
	// Create channels
	chunks := make(chan inputChunk, pe.config.MaxBufferedChunks)
	results := make(chan WorkerResult, pe.config.MaxBufferedChunks)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < pe.config.NumWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			pe.worker(ctx, workerID, chunks, results, templateVM)
		}(i)
	}

	// Start chunk reader
	readerDone := make(chan error, 1)
	go func() {
		readerDone <- pe.readChunks(ctx, input, chunks, templateVM.rs)
		close(chunks)
	}()

	// Start result collector
	collectorDone := make(chan error, 1)
	go func() {
		collectorDone <- pe.collectResults(ctx, results, output)
	}()

	// Wait for workers to finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Wait for all components
	var firstErr error
	if err := <-readerDone; err != nil && firstErr == nil {
		firstErr = err
	}
	if err := <-collectorDone; err != nil && firstErr == nil {
		firstErr = err
	}

	return firstErr
}

// inputChunk represents a chunk of input data.
type inputChunk struct {
	ID      int    // Sequential chunk ID
	Data    []byte // Input data for this chunk
	StartNR int    // Starting NR for this chunk
}

// readChunks reads input and splits it into chunks at record boundaries.
func (pe *ParallelExecutor) readChunks(
	ctx context.Context,
	input io.Reader,
	chunks chan<- inputChunk,
	rs string,
) error {
	reader := bufio.NewReaderSize(input, pe.config.ChunkSize)
	chunkID := 0
	currentNR := 1

	// Determine record separator byte
	rsByte := byte('\n')
	if len(rs) == 1 {
		rsByte = rs[0]
	}

	// Reusable buffers to minimize allocations
	buffer := make([]byte, pe.config.ChunkSize)
	remainder := make([]byte, 0, 4096) // Pre-allocated remainder buffer

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Copy remainder to start of buffer
		copy(buffer, remainder)
		remainderLen := len(remainder)
		remainder = remainder[:0] // Reset without deallocating

		// Read more data after remainder
		n, err := reader.Read(buffer[remainderLen:])
		totalLen := remainderLen + n

		if totalLen == 0 {
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			continue
		}

		data := buffer[:totalLen]

		// If not EOF, find last record boundary and save remainder
		if err != io.EOF {
			lastRS := bytes.LastIndexByte(data, rsByte)
			if lastRS >= 0 && lastRS < len(data)-1 {
				// Grow remainder if needed
				needLen := len(data) - lastRS - 1
				if cap(remainder) < needLen {
					remainder = make([]byte, needLen)
				} else {
					remainder = remainder[:needLen]
				}
				copy(remainder, data[lastRS+1:])
				data = data[:lastRS+1]
			}
		}

		// Count records in this chunk for NR tracking
		recordCount := bytes.Count(data, []byte{rsByte})

		chunk := inputChunk{
			ID:      chunkID,
			Data:    make([]byte, len(data)),
			StartNR: currentNR,
		}
		copy(chunk.Data, data)

		select {
		case chunks <- chunk:
		case <-ctx.Done():
			return ctx.Err()
		}

		chunkID++
		currentNR += recordCount

		if err == io.EOF {
			return nil
		}
	}
}

// worker processes input chunks using its own VM instance.
func (pe *ParallelExecutor) worker(
	ctx context.Context,
	workerID int,
	chunks <-chan inputChunk,
	results chan<- WorkerResult,
	templateVM *VM,
) {
	// Build set of aggregated vars for fast lookup
	aggregatedVars := make(map[int]bool)
	for _, idx := range pe.analysis.AggregatedVars {
		aggregatedVars[idx] = true
	}

	for chunk := range chunks {
		select {
		case <-ctx.Done():
			results <- WorkerResult{
				ChunkID: chunk.ID,
				Err:     ctx.Err(),
			}
			return
		default:
		}

		// Create a fresh VM for each chunk
		vm := NewWithConfig(pe.program, pe.vmConfig)

		// Copy configuration from template
		vm.fs = templateVM.fs
		vm.rs = templateVM.rs
		vm.ofs = templateVM.ofs
		vm.ors = templateVM.ors
		vm.convfmt = templateVM.convfmt
		vm.ofmt = templateVM.ofmt
		vm.subsep = templateVM.subsep

		// Copy scalar state from BEGIN, but NOT aggregated variables
		// Aggregated vars should start at 0 in each worker for proper summing
		pe.mu.Lock()
		for i, v := range pe.scalars {
			if !aggregatedVars[i] {
				vm.scalars[i] = v
			}
			// aggregated vars stay at Null (which converts to 0 for numeric ops)
		}
		pe.mu.Unlock()

		result := pe.processChunk(vm, chunk)
		results <- result
	}
}

// processChunk processes a single input chunk and returns results.
//
//nolint:gocognit,nestif // Complex but necessary - processes AWK program on chunk
func (pe *ParallelExecutor) processChunk(vm *VM, chunk inputChunk) WorkerResult {
	result := WorkerResult{
		ChunkID: chunk.ID,
		StartNR: chunk.StartNR,
	}

	// Capture output
	var outputBuf bytes.Buffer
	vm.SetOutput(&outputBuf)

	// Set up input from chunk data
	scanner := bufio.NewScanner(bytes.NewReader(chunk.Data))

	// Process records
	recordCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		vm.lineNum = chunk.StartNR + recordCount
		vm.specials.NR = vm.lineNum
		vm.fileNum++
		vm.specials.FNR = vm.fileNum

		vm.setLine(line)

		// Execute pattern-action rules
		for i, action := range pe.program.Actions {
			matches := false

			if len(action.Pattern) == 0 {
				matches = true
			} else if len(action.Pattern) == 1 {
				if err := vm.execute(action.Pattern[0]); err != nil {
					result.Err = err
					return result
				}
				matches = vm.pop().AsBool()
			} else if len(action.Pattern) == 2 {
				// Range pattern
				if !vm.rangeActive[i] {
					if err := vm.execute(action.Pattern[0]); err != nil {
						result.Err = err
						return result
					}
					if vm.pop().AsBool() {
						vm.rangeActive[i] = true
						matches = true
					}
				} else {
					matches = true
					if err := vm.execute(action.Pattern[1]); err != nil {
						result.Err = err
						return result
					}
					if vm.pop().AsBool() {
						vm.rangeActive[i] = false
					}
				}
			}

			if matches {
				if action.Body == nil {
					// Default action: print $0
					outputBuf.WriteString(vm.line)
					outputBuf.WriteByte('\n')
				} else if len(action.Body) > 0 {
					if err := vm.execute(action.Body); err != nil {
						if errors.Is(err, ErrNext) {
							break
						}
						if errors.Is(err, ErrNextFile) {
							break
						}
						result.Err = err
						return result
					}
				}
			}
		}
		recordCount++
	}

	if err := scanner.Err(); err != nil {
		result.Err = err
		return result
	}

	result.Output = outputBuf.Bytes()
	result.NR = recordCount

	// Copy state for aggregation
	result.Scalars = make([]types.Value, len(vm.scalars))
	copy(result.Scalars, vm.scalars)

	result.Arrays = make([]map[string]types.Value, len(vm.arrays))
	for i, arr := range vm.arrays {
		if len(arr) > 0 {
			result.Arrays[i] = make(map[string]types.Value, len(arr))
			for k, v := range arr {
				result.Arrays[i][k] = v
			}
		}
	}

	return result
}

// collectResults collects worker results and aggregates them.
func (pe *ParallelExecutor) collectResults(
	ctx context.Context,
	results <-chan WorkerResult,
	output io.Writer,
) error {
	// Collect all results first (for ordering)
	var allResults []WorkerResult
	var firstErr error

	for result := range results {
		if result.Err != nil && firstErr == nil {
			firstErr = result.Err
			continue
		}
		allResults = append(allResults, result)
	}

	if firstErr != nil {
		return firstErr
	}

	// Sort by chunk ID to maintain output order
	// (Note: frawk doesn't guarantee order, but we try to maintain it)
	sortByChunkID(allResults)

	// Aggregate state and write output in order
	for _, result := range allResults {
		// Write output
		if len(result.Output) > 0 {
			if _, err := output.Write(result.Output); err != nil {
				return err
			}
		}

		// Aggregate scalar values (numeric: sum, string: last non-empty)
		pe.aggregateScalars(result.Scalars)

		// Aggregate arrays (union with numeric summing)
		pe.aggregateArrays(result.Arrays)

		// Update total NR
		pe.mu.Lock()
		pe.totalNR += result.NR
		pe.mu.Unlock()
	}

	return nil
}

// aggregateScalars aggregates scalar values from a worker result.
// Numeric values are summed; strings keep the last non-empty value.
func (pe *ParallelExecutor) aggregateScalars(workerScalars []types.Value) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	for i, v := range workerScalars {
		if v.IsNull() {
			continue
		}

		current := pe.scalars[i]
		if current.IsNull() {
			pe.scalars[i] = v
			continue
		}

		// For numeric values, sum them
		if v.IsNum() || current.IsNum() {
			pe.scalars[i] = types.Num(current.AsNum() + v.AsNum())
		} else if v.AsStr("%.6g") != "" {
			// For strings, keep last non-empty
			pe.scalars[i] = v
		}
	}
}

// aggregateArrays aggregates array values from a worker result.
// For each key, numeric values are summed; strings keep last non-empty.
func (pe *ParallelExecutor) aggregateArrays(workerArrays []map[string]types.Value) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	for i, arr := range workerArrays {
		if arr == nil {
			continue
		}
		if pe.arrays[i] == nil {
			pe.arrays[i] = make(map[string]types.Value)
		}

		for k, v := range arr {
			current, exists := pe.arrays[i][k]
			if !exists {
				pe.arrays[i][k] = v
				continue
			}

			// Numeric: sum, String: keep last non-empty
			if v.IsNum() || current.IsNum() {
				pe.arrays[i][k] = types.Num(current.AsNum() + v.AsNum())
			} else if v.AsStr("%.6g") != "" {
				pe.arrays[i][k] = v
			}
		}
	}
}

// copyStateFrom copies scalar and array state from a VM.
func (pe *ParallelExecutor) copyStateFrom(vm *VM) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	copy(pe.scalars, vm.scalars)
	for i, arr := range vm.arrays {
		if len(arr) > 0 {
			pe.arrays[i] = make(map[string]types.Value, len(arr))
			for k, v := range arr {
				pe.arrays[i][k] = v
			}
		}
	}
}

// sortByChunkID sorts results by chunk ID using insertion sort
// (typically few chunks, so O(n^2) is fine).
func sortByChunkID(results []WorkerResult) {
	for i := 1; i < len(results); i++ {
		j := i
		for j > 0 && results[j-1].ChunkID > results[j].ChunkID {
			results[j-1], results[j] = results[j], results[j-1]
			j--
		}
	}
}
