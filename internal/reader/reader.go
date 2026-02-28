package reader

import (
	"bufio"
	"context"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	debounceDuration = 30 * time.Millisecond
	signatureSize    = 3
)

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
	defer func() { _ = watcher.Close() }()

	if err := watcher.Add(t.path); err != nil {
		return
	}

	if lines := t.readAllLines(); len(lines) > 0 {
		t.updateSignature(lines)
	}

	var timer *time.Timer
	var timerCh <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				timer, timerCh = t.resetTimer(timer)
			}

		case <-timerCh:
			timer = nil
			timerCh = nil
			t.processUpdate(ctx)

		case <-watcher.Errors:
		}
	}
}

func (t *fileTailer) processUpdate(ctx context.Context) {
	lines := t.readAllLines()
	if len(lines) == 0 {
		return
	}

	if len(t.signature) == 0 {
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
		if !t.emit(ctx, []byte(lines[len(lines)-1])) {
			return
		}
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
	defer func() { _ = file.Close() }()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func (t *fileTailer) updateSignature(lines []string) {
	if len(lines) <= t.sigSize {
		t.signature = append([]string(nil), lines...)
		return
	}
	t.signature = append([]string(nil), lines[len(lines)-t.sigSize:]...)
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

func (t *fileTailer) resetTimer(timer *time.Timer) (*time.Timer, <-chan time.Time) {
	if timer == nil {
		timer = time.NewTimer(debounceDuration)
		return timer, timer.C
	}

	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(debounceDuration)
	return timer, timer.C
}
