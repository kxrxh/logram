package reader

import (
	"bufio"
	"context"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

const signatureSize = 3

func ReadFileTail(ctx context.Context, path string) <-chan []byte {
	out := make(chan []byte, 200)
	t := &fileTailer{
		path:    path,
		out:     out,
		sigSize: signatureSize,
	}
	go t.run(ctx)
	return out
}

type fileTailer struct {
	path      string
	out       chan<- []byte
	signature []string
	sigSize   int
}

func (t *fileTailer) run(ctx context.Context) {
	defer close(t.out)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	defer watcher.Close()

	// Watch the directory, not the file, to handle missing files
	dir := filepath.Dir(t.path)
	if err := watcher.Add(dir); err != nil {
		return
	}

	// Initial check if file exists
	if _, err := os.Stat(t.path); err == nil {
		lines := t.readAllLines()
		t.updateSignature(lines)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// Only process events related to our specific file
			if filepath.Clean(event.Name) == filepath.Clean(t.path) {
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					t.processUpdate(ctx)
				}
			}
		case <-watcher.Errors:
			return
		}
	}
}

func (t *fileTailer) processUpdate(ctx context.Context) {
	lines := t.readAllLines()
	if len(lines) == 0 {
		return
	}

	if len(t.signature) == 0 {
		// New file or first time reading
		t.emit(ctx, []byte(lines[len(lines)-1]))
		t.updateSignature(lines)
		return
	}

	idx := t.findLastReadIndex(lines)
	if idx != -1 {
		// Append new lines
		for i := idx + 1; i < len(lines); i++ {
			if !t.emit(ctx, []byte(lines[i])) {
				return
			}
		}
	} else {
		// File was truncated or rotated, send last line to recover
		t.emit(ctx, []byte(lines[len(lines)-1]))
	}
	t.updateSignature(lines)
}

func (t *fileTailer) emit(ctx context.Context, data []byte) bool {
	select {
	case t.out <- data:
		return true
	case <-ctx.Done():
		return false
	}
}

func (t *fileTailer) readAllLines() []string {
	file, err := os.Open(t.path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func (t *fileTailer) updateSignature(lines []string) {
	if len(lines) == 0 {
		t.signature = nil
		return
	}
	start := len(lines) - t.sigSize
	if start < 0 {
		start = 0
	}
	t.signature = append([]string(nil), lines[start:]...)
}

func (t *fileTailer) findLastReadIndex(lines []string) int {
	if len(t.signature) == 0 {
		return -1
	}
	for i := len(lines) - len(t.signature); i >= 0; i-- {
		match := true
		for j := range t.signature {
			if lines[i+j] != t.signature[j] {
				match = false
				break
			}
		}
		if match {
			return i + len(t.signature) - 1
		}
	}
	return -1
}
