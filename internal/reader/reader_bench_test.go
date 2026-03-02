package reader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func BenchmarkReadAllLines_100Lines(b *testing.B) {
	tmpDir := b.TempDir()
	tmpPath := filepath.Join(tmpDir, "bench.log")

	lines := make([]string, 100)
	for i := range 100 {
		lines[i] = fmt.Sprintf("log line %d with some data", i)
	}
	content := []byte(joinLines(lines))
	_ = os.WriteFile(tmpPath, content, 0o644)

	t := &fileTailer{
		path:    tmpPath,
		sigSize: signatureSize,
	}

	b.ResetTimer()
	for b.Loop() {
		_ = t.readAllLines()
	}
}

func BenchmarkReadAllLines_10000Lines(b *testing.B) {
	tmpDir := b.TempDir()
	tmpPath := filepath.Join(tmpDir, "bench.log")

	lines := make([]string, 10000)
	for i := range 10000 {
		lines[i] = fmt.Sprintf("log line %d with some data", i)
	}
	content := []byte(joinLines(lines))
	_ = os.WriteFile(tmpPath, content, 0o644)

	t := &fileTailer{
		path:    tmpPath,
		sigSize: signatureSize,
	}

	b.ResetTimer()
	for b.Loop() {
		_ = t.readAllLines()
	}
}

func BenchmarkReadAllLines_100000Lines(b *testing.B) {
	tmpDir := b.TempDir()
	tmpPath := filepath.Join(tmpDir, "bench.log")

	lines := make([]string, 100000)
	for i := range 100000 {
		lines[i] = fmt.Sprintf("log line %d with some data", i)
	}
	content := []byte(joinLines(lines))
	_ = os.WriteFile(tmpPath, content, 0o644)

	t := &fileTailer{
		path:    tmpPath,
		sigSize: signatureSize,
	}

	b.ResetTimer()
	for b.Loop() {
		_ = t.readAllLines()
	}
}

func BenchmarkFindLastReadIndex_100Lines(b *testing.B) {
	lines := make([]string, 100)
	for i := range 100 {
		lines[i] = fmt.Sprintf("log line %d with some data", i)
	}

	t := &fileTailer{
		signature: lines[97:],
		sigSize:   signatureSize,
	}

	b.ResetTimer()
	for b.Loop() {
		_ = t.findLastReadIndex(lines)
	}
}

func BenchmarkFindLastReadIndex_10000Lines(b *testing.B) {
	lines := make([]string, 10000)
	for i := range 10000 {
		lines[i] = fmt.Sprintf("log line %d with some data", i)
	}

	t := &fileTailer{
		signature: lines[9997:],
		sigSize:   signatureSize,
	}

	b.ResetTimer()
	for b.Loop() {
		_ = t.findLastReadIndex(lines)
	}
}

func joinLines(lines []string) string {
	var sb strings.Builder
	sb.Grow(len(lines) * 50)
	for i, line := range lines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(line)
	}
	return sb.String()
}
