package system

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadLastLines(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test_logs_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "test.log")
	content := "line 1\nline 2\nline 3\nline 4\nline 5\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test log: %v", err)
	}

	// Request last 3 lines
	lines, offset, err := ReadLastLines(logFile, 3)
	if err != nil {
		t.Fatalf("ReadLastLines failed: %v", err)
	}

	expected := []string{"line 3", "line 4", "line 5"}
	if !reflect.DeepEqual(lines, expected) {
		t.Errorf("expected lines %v, got %v", expected, lines)
	}

	if offset != int64(len(content)) {
		t.Errorf("expected offset %d, got %d", len(content), offset)
	}

	// Request more lines than exist
	allLines, _, err := ReadLastLines(logFile, 10)
	if err != nil {
		t.Fatalf("ReadLastLines failed: %v", err)
	}
	if len(allLines) != 5 {
		t.Errorf("expected 5 lines, got %d", len(allLines))
	}

	// Non-existent file
	noLines, _, err := ReadLastLines(filepath.Join(tempDir, "missing.log"), 5)
	if err != nil {
		t.Errorf("expected no error for missing file, got %v", err)
	}
	if len(noLines) != 0 {
		t.Errorf("expected 0 lines for missing file, got %d", len(noLines))
	}
}
