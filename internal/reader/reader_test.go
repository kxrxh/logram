package reader

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const waitInterval = 100 * time.Millisecond

func collectWithTimeout(ch <-chan []byte, count int, timeout time.Duration) []string {
	var lines []string
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for len(lines) < count {
		select {
		case line, ok := <-ch:
			if !ok {
				return lines
			}
			lines = append(lines, string(line))
		case <-ctx.Done():
			return lines
		}
	}
	return lines
}

func TestReadFileTail_BasicAppend(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "test.log")

	_ = os.WriteFile(tmpPath, []byte("old1\nold2\n"), 0o644)

	ctx := t.Context()
	ch := ReadFileTail(ctx, tmpPath)

	time.Sleep(waitInterval)

	f, _ := os.OpenFile(tmpPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if _, err := f.WriteString("new1\nnew2\n"); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	lines := collectWithTimeout(ch, 2, 1*time.Second)

	if len(lines) != 2 {
		t.Fatalf("expected 2 new lines, got %v", lines)
	}
	if lines[0] != "new1" || lines[1] != "new2" {
		t.Errorf("unexpected lines: %v", lines)
	}
}

func TestReadFileTail_MainGoOverwriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "test.log")

	initial := "line1\nline2\nline3\n"
	_ = os.WriteFile(tmpPath, []byte(initial), 0o644)

	ctx := t.Context()
	ch := ReadFileTail(ctx, tmpPath)
	time.Sleep(waitInterval)

	updated := "line2\nline3\nline4\n"
	_ = os.WriteFile(tmpPath, []byte(updated), 0o644)

	lines := collectWithTimeout(ch, 1, 1*time.Second)

	if len(lines) != 1 {
		t.Fatalf("expected 1 new line (line4), got %d lines: %v", len(lines), lines)
	}
	if lines[0] != "line4" {
		t.Errorf("expected 'line4', got %q", lines[0])
	}
}

func TestReadFileTail_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "ghost.log")

	ctx := t.Context()

	ch := ReadFileTail(ctx, tmpPath)

	time.Sleep(waitInterval)
	_ = os.WriteFile(tmpPath, []byte("discovered\n"), 0o644)

	lines := collectWithTimeout(ch, 1, 1*time.Second)
	t.Logf("Found: %v", lines)
}

func TestReadFileTail_ContextCancel(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "cancel.log")
	_ = os.WriteFile(tmpPath, []byte("init\n"), 0o644)

	ctx, cancel := context.WithCancel(context.Background())
	ch := ReadFileTail(ctx, tmpPath)

	time.Sleep(waitInterval)
	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should have been closed")
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for channel close")
	}
}

func TestReadFileTail_SignatureRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "recovery.log")

	_ = os.WriteFile(tmpPath, []byte("A\nB\nC\n"), 0o644)

	ctx := t.Context()
	ch := ReadFileTail(ctx, tmpPath)
	time.Sleep(waitInterval)

	_ = os.WriteFile(tmpPath, []byte("X\nY\nZ\n"), 0o644)

	lines := collectWithTimeout(ch, 1, 1*time.Second)

	if len(lines) == 0 {
		t.Error("expected to recover and read at least the latest line")
	} else if lines[len(lines)-1] != "Z" {
		t.Errorf("expected latest line 'Z', got %q", lines[len(lines)-1])
	}
}
