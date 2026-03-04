package reader

import (
	"bufio"
	"context"
	"log"
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
	lastSize  int64
}

func (t *fileTailer) run(ctx context.Context) {
	defer close(t.out)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	defer func() {
		if err := watcher.Close(); err != nil {
			log.Printf("close watcher: %v", err)
		}
	}()

	// Watch the directory, not the file, to handle missing files
	dir := filepath.Dir(t.path)
	if err := watcher.Add(dir); err != nil {
		return
	}

	// Initial check if file exists
	if stat, err := os.Stat(t.path); err == nil {
		t.lastSize = stat.Size()
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
	stat, err := os.Stat(t.path)
	if err != nil {
		return
	}

	// File was truncated or rotated
	if stat.Size() < t.lastSize {
		t.signature = nil
		t.lastSize = stat.Size()
		lines := t.readAllLines()
		if len(lines) > 0 {
			t.emit(ctx, []byte(lines[len(lines)-1]))
			t.updateSignature(lines)
		}
		return
	}

	lines := t.readAllLines()
	if len(lines) == 0 {
		return
	}

	t.lastSize = stat.Size()

	if len(t.signature) == 0 {
		t.emit(ctx, []byte(lines[len(lines)-1]))
		t.updateSignature(lines)
		return
	}

	idx := t.findLastReadIndex(lines)
	if idx != -1 {
		for i := idx + 1; i < len(lines); i++ {
			if !t.emit(ctx, []byte(lines[i])) {
				return
			}
		}
	} else {
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
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("close file: %v", err)
		}
	}()

	var lines []string
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
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
	start := max(len(lines)-t.sigSize, 0)
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
