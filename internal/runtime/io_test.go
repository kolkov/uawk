package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIOManagerOutputFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	m := NewIOManager()
	defer m.CloseAll()

	// Test write mode
	w, err := m.GetOutputFile(testFile, false)
	if err != nil {
		t.Fatalf("GetOutputFile failed: %v", err)
	}

	_, err = w.WriteString("hello\n")
	if err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}
	w.Flush()

	// Verify content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(content) != "hello\n" {
		t.Errorf("Expected 'hello\\n', got %q", string(content))
	}
}

func TestIOManagerOutputFileAppend(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "append.txt")

	// Create initial content
	err := os.WriteFile(testFile, []byte("first\n"), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	m := NewIOManager()
	defer m.CloseAll()

	// Test append mode
	w, err := m.GetOutputFile(testFile, true)
	if err != nil {
		t.Fatalf("GetOutputFile (append) failed: %v", err)
	}

	_, err = w.WriteString("second\n")
	if err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}
	w.Flush()

	// Verify content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(content) != "first\nsecond\n" {
		t.Errorf("Expected 'first\\nsecond\\n', got %q", string(content))
	}
}

func TestIOManagerInputFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "input.txt")

	// Create test file
	err := os.WriteFile(testFile, []byte("line1\nline2\nline3\n"), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	m := NewIOManager()
	defer m.CloseAll()

	// Read file
	scanner, err := m.GetInputFile(testFile)
	if err != nil {
		t.Fatalf("GetInputFile failed: %v", err)
	}

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "line1" || lines[1] != "line2" || lines[2] != "line3" {
		t.Errorf("Unexpected lines: %v", lines)
	}
}

func TestIOManagerClose(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "close.txt")

	m := NewIOManager()
	defer m.CloseAll()

	// Open file
	_, err := m.GetOutputFile(testFile, false)
	if err != nil {
		t.Fatalf("GetOutputFile failed: %v", err)
	}

	// Close should succeed
	result := m.Close(testFile)
	if result != 0 {
		t.Errorf("Close returned %d, expected 0", result)
	}

	// Close again should fail (not found)
	result = m.Close(testFile)
	if result != -1 {
		t.Errorf("Second Close returned %d, expected -1", result)
	}
}

func TestIOManagerFlush(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "flush.txt")

	m := NewIOManager()
	defer m.CloseAll()

	// Open file and write
	w, err := m.GetOutputFile(testFile, false)
	if err != nil {
		t.Fatalf("GetOutputFile failed: %v", err)
	}

	_, err = w.WriteString("buffered")
	if err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}

	// Flush specific file
	result := m.Flush(testFile)
	if result != 0 {
		t.Errorf("Flush returned %d, expected 0", result)
	}

	// Verify content is flushed
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(content) != "buffered" {
		t.Errorf("Expected 'buffered', got %q", string(content))
	}
}

func TestIOManagerFlushAll(t *testing.T) {
	tmpDir := t.TempDir()

	m := NewIOManager()
	defer m.CloseAll()

	// Open multiple files
	for i := 1; i <= 3; i++ {
		testFile := filepath.Join(tmpDir, "file"+string('0'+byte(i))+".txt")
		w, err := m.GetOutputFile(testFile, false)
		if err != nil {
			t.Fatalf("GetOutputFile failed: %v", err)
		}
		w.WriteString("content")
	}

	// Flush all
	result := m.Flush("")
	if result != 0 {
		t.Errorf("Flush all returned %d, expected 0", result)
	}
}

func TestIOManagerFileCaching(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "cache.txt")

	m := NewIOManager()
	defer m.CloseAll()

	// Get same file twice - should return cached writer
	w1, err := m.GetOutputFile(testFile, false)
	if err != nil {
		t.Fatalf("First GetOutputFile failed: %v", err)
	}

	w2, err := m.GetOutputFile(testFile, false)
	if err != nil {
		t.Fatalf("Second GetOutputFile failed: %v", err)
	}

	// Should be the same writer
	if w1 != w2 {
		t.Error("Expected same writer for same file")
	}
}

func TestIOManagerInputPipe(t *testing.T) {
	// Skip on Windows in CI environments where shell may not be available
	if os.Getenv("CI") != "" && strings.Contains(os.Getenv("OS"), "Windows") {
		t.Skip("Skipping pipe test in Windows CI")
	}

	m := NewIOManager()
	defer m.CloseAll()

	// Test echo command
	scanner, err := m.GetInputPipe("echo hello")
	if err != nil {
		t.Skipf("Pipe test skipped (shell not available): %v", err)
	}

	if scanner.Scan() {
		text := scanner.Text()
		if !strings.Contains(text, "hello") {
			t.Errorf("Expected 'hello', got %q", text)
		}
	} else {
		t.Error("Expected to read line from pipe")
	}
}

func TestIOManagerOutputPipe(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "pipe_out.txt")

	// Convert Windows path to Unix-style for shell compatibility
	// C:\Users\... -> /c/Users/... (Git Bash / MSYS2 format)
	shellPath := testFile
	if len(testFile) > 2 && testFile[1] == ':' {
		// Windows drive letter path - convert to /x/... format
		drive := strings.ToLower(string(testFile[0]))
		shellPath = "/" + drive + strings.ReplaceAll(testFile[2:], "\\", "/")
	}

	m := NewIOManager()
	defer m.CloseAll()

	// Test cat command to write to file
	cmd := "cat > " + shellPath
	w, err := m.GetOutputPipe(cmd)
	if err != nil {
		t.Skipf("Pipe test skipped (shell not available): %v", err)
	}

	_, err = w.WriteString("piped content\n")
	if err != nil {
		t.Fatalf("WriteString to pipe failed: %v", err)
	}

	// Close pipe to finish command
	m.Close(cmd)

	// Verify content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Skipf("Could not verify pipe output: %v", err)
	}
	if !strings.Contains(string(content), "piped content") {
		t.Errorf("Expected 'piped content', got %q", string(content))
	}
}

func TestIOManagerErrorHandling(t *testing.T) {
	m := NewIOManager()
	defer m.CloseAll()

	// Try to read non-existent file
	_, err := m.GetInputFile("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}

	// Flush non-existent file
	result := m.Flush("/nonexistent/file.txt")
	if result != -1 {
		t.Errorf("Flush non-existent returned %d, expected -1", result)
	}

	// Close non-existent file
	result = m.Close("/nonexistent/file.txt")
	if result != -1 {
		t.Errorf("Close non-existent returned %d, expected -1", result)
	}
}

func BenchmarkIOManagerWrite(b *testing.B) {
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "bench.txt")

	m := NewIOManager()
	defer m.CloseAll()

	w, _ := m.GetOutputFile(testFile, false)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.WriteString("benchmark line\n")
	}
	w.Flush()
}

func BenchmarkIOManagerCacheHit(b *testing.B) {
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "cache_bench.txt")

	m := NewIOManager()
	defer m.CloseAll()

	// Prime the cache
	m.GetOutputFile(testFile, false)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.GetOutputFile(testFile, false)
	}
}
