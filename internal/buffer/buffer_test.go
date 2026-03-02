package buffer

import (
	"context"
	"testing"
	"time"
)

func TestNew_Defaults(t *testing.T) {
	ctx := t.Context()
	buf := New(ctx, 5, time.Second)

	if buf.maxSize != 5 {
		t.Errorf("expected maxSize 5, got %d", buf.maxSize)
	}
	if buf.flushInterval != time.Second {
		t.Errorf("expected flushInterval 1s, got %v", buf.flushInterval)
	}
	if buf.policy != BlockOnFull {
		t.Errorf("expected default policy BlockOnFull, got %v", buf.policy)
	}
}

func TestWithPolicy(t *testing.T) {
	buf := New(t.Context(), 10, time.Second, WithPolicy(DropOldest))
	if buf.policy != DropOldest {
		t.Errorf("expected policy DropOldest, got %v", buf.policy)
	}
}

func TestBuffer_Policy_DropNew(t *testing.T) {
	buf := New(t.Context(), 2, time.Hour, WithPolicy(DropNew))
	buf.Start()

	buf.Input() <- []byte("1")
	buf.Input() <- []byte("2")
	buf.Input() <- []byte("3") // Should be dropped

	buf.Stop()

	if string(<-buf.Output()) != "1" {
		t.Error("expected 1")
	}
	if string(<-buf.Output()) != "2" {
		t.Error("expected 2")
	}

	_, ok := <-buf.Output()
	if ok {
		t.Error("expected channel to be closed")
	}
}

func TestBuffer_Policy_DropOldest(t *testing.T) {
	buf := New(t.Context(), 2, time.Hour, WithPolicy(DropOldest))
	buf.Start()

	buf.Input() <- []byte("1")
	buf.Input() <- []byte("2")
	buf.Input() <- []byte("3") // Should drop "1"

	buf.Stop()

	if string(<-buf.Output()) != "2" {
		t.Error("expected 2")
	}
	if string(<-buf.Output()) != "3" {
		t.Error("expected 3")
	}
}

func TestBuffer_Policy_BlockOnFull(t *testing.T) {
	buf := New(t.Context(), 2, time.Hour, WithPolicy(BlockOnFull))
	buf.Start()
	defer buf.Stop()

	buf.Input() <- []byte("1")
	buf.Input() <- []byte("2") // Should trigger automatic flush

	select {
	case m := <-buf.Output():
		if string(m) != "1" {
			t.Errorf("expected 1, got %s", m)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("BlockOnFull failed to flush when full")
	}
}

func TestBuffer_FlushOnInterval(t *testing.T) {
	interval := 50 * time.Millisecond
	buf := New(t.Context(), 100, interval)
	buf.Start()
	defer buf.Stop()

	buf.Input() <- []byte("timed-msg")

	select {
	case msg := <-buf.Output():
		if string(msg) != "timed-msg" {
			t.Errorf("got %s", msg)
		}
	case <-time.After(interval * 3):
		t.Error("flush interval exceeded")
	}
}

func TestBuffer_StopDrainsRemaining(t *testing.T) {
	buf := New(t.Context(), 10, time.Hour)
	buf.Start()

	buf.Input() <- []byte("drain-me")
	buf.Stop()

	select {
	case msg := <-buf.Output():
		if string(msg) != "drain-me" {
			t.Errorf("expected drain-me, got %q", msg)
		}
	default:
		t.Error("message was lost during Stop")
	}
}

func TestBuffer_DoubleStop(t *testing.T) {
	buf := New(t.Context(), 10, time.Hour)
	buf.Start()
	buf.Stop()
	buf.Stop() // Should not panic
}

func TestBuffer_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	buf := New(ctx, 10, time.Hour)
	buf.Start()

	cancel()
	buf.Stop()

	_, ok := <-buf.Output()
	if ok {
		t.Error("Output channel should be closed")
	}
}
