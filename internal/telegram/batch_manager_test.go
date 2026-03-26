package telegram

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

var errChunkSendFailed = errors.New("chunk send failed")

func TestBatchManager_Off_ImmediateSend(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	calls := make(chan string, 10)

	m := NewBatchManager(
		ctx,
		30*time.Millisecond,
		func(_ int64, text string) error {
			calls <- text
			return nil
		},
		map[int64]bool{}, // batching disabled
	)

	if err := m.Enqueue(1, "a"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := m.Enqueue(1, "b"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	select {
	case got := <-calls:
		if got != "a" {
			t.Fatalf("expected first call %q, got %q", "a", got)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timeout waiting for first send")
	}

	select {
	case got := <-calls:
		if got != "b" {
			t.Fatalf("expected second call %q, got %q", "b", got)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timeout waiting for second send")
	}

	select {
	case got := <-calls:
		t.Fatalf("expected no more sends, got extra send: %q", got)
	default:
	}
}

func TestBatchManager_On_FlushOnceCombined(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	calls := make(chan string, 10)

	flushInterval := 20 * time.Millisecond
	m := NewBatchManager(
		ctx,
		flushInterval,
		func(_ int64, text string) error {
			calls <- text
			return nil
		},
		map[int64]bool{1: true},
	)

	if err := m.Enqueue(1, "a"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := m.Enqueue(1, "b"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	select {
	case got := <-calls:
		want := "a\n\nb"
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timeout waiting for flush")
	}

	// Ensure no second flush happens for this window.
	select {
	case got := <-calls:
		t.Fatalf("expected only one combined send, got extra: %q", got)
	case <-time.After(flushInterval + 50*time.Millisecond):
	}
}

func TestBatchManager_OnThenDisable_FlushImmediately(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	calls := make(chan string, 10)

	flushInterval := 200 * time.Millisecond // large enough to reliably disable first
	m := NewBatchManager(
		ctx,
		flushInterval,
		func(_ int64, text string) error {
			calls <- text
			return nil
		},
		map[int64]bool{1: true},
	)

	if err := m.Enqueue(1, "a"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	m.SetEnabled(1, false) // flush immediately

	select {
	case got := <-calls:
		want := "a"
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timeout waiting for immediate flush on disable")
	}

	// No extra message should arrive from the timer.
	select {
	case got := <-calls:
		t.Fatalf("expected no extra sends after disable, got: %q", got)
	case <-time.After(80 * time.Millisecond):
	}
}

func TestBatchManager_Chunking_SplitsByCaps(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	calls := make(chan string, 10)

	flushInterval := 20 * time.Millisecond
	m := NewBatchManager(
		ctx,
		flushInterval,
		func(_ int64, text string) error {
			calls <- text
			return nil
		},
		map[int64]bool{1: true},
		WithRateLimit(100, 1*time.Second),
		WithChunkCaps(5, 2), // very small chunk caps so "a\n\nb" can't fit
	)

	_ = m.Enqueue(1, "111")
	_ = m.Enqueue(1, "222")
	_ = m.Enqueue(1, "333")

	expect := []string{"111", "222", "333"}
	for i := range len(expect) {
		select {
		case got := <-calls:
			if got != expect[i] {
				t.Fatalf("expected calls[%d]=%q, got %q", i, expect[i], got)
			}
		case <-time.After(300 * time.Millisecond):
			t.Fatalf("timeout waiting for chunk send %d", i)
		}
	}
}

func TestBatchManager_ChunkFailure_FallbackToEntries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	calls := make(chan string, 10)

	flushInterval := 20 * time.Millisecond
	m := NewBatchManager(
		ctx,
		flushInterval,
		func(_ int64, text string) error {
			// Fail only for chunk text (combined entries contain "\n\n").
			if strings.Contains(text, "\n\n") {
				return errChunkSendFailed
			}
			calls <- text
			return nil
		},
		map[int64]bool{1: true},
		WithRateLimit(100, 1*time.Second),
		WithChunkCaps(1000, 100), // force everything into one chunk
	)

	_ = m.Enqueue(1, "a")
	_ = m.Enqueue(1, "b")
	_ = m.Enqueue(1, "c")

	expect := []string{"a", "b", "c"}
	for i := range len(expect) {
		select {
		case got := <-calls:
			if got != expect[i] {
				t.Fatalf("expected calls[%d]=%q, got %q", i, expect[i], got)
			}
		case <-time.After(300 * time.Millisecond):
			t.Fatalf("timeout waiting for fallback send %d", i)
		}
	}
}

func TestBatchManager_RateLimitPerChat(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	window := 80 * time.Millisecond
	m := NewBatchManager(
		ctx,
		time.Second,
		func(_ int64, _ string) error {
			return nil
		},
		map[int64]bool{}, // batching disabled
		WithRateLimit(1, window),
	)

	start := time.Now()
	if err := m.Enqueue(1, "a"); err != nil {
		t.Fatalf("enqueue 1: %v", err)
	}
	if err := m.Enqueue(1, "b"); err != nil {
		t.Fatalf("enqueue 2: %v", err)
	}
	if err := m.Enqueue(1, "c"); err != nil {
		t.Fatalf("enqueue 3: %v", err)
	}

	// With max=1 per window, 3 sends require at least 2 windows.
	minDur := 2*window - 10*time.Millisecond
	if time.Since(start) < minDur {
		t.Fatalf("expected duration >= %v, got %v", minDur, time.Since(start))
	}
}
