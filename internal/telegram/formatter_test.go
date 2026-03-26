package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/kxrxh/logram/internal/parser"
)

func TestMessageFormatter_EscapesHTMLInLogMessage(t *testing.T) {
	f := NewMessageFormatter()

	entry := parser.LogEntry{
		Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Level:     parser.LevelInfo,
		Message:   []byte("hello <b>world</b> & x"),
	}

	out := f.FormatLogEntry(entry)

	if strings.Contains(out, "hello <b>world</b> & x") {
		t.Fatalf("expected HTML to be escaped, got: %s", out)
	}

	if !strings.Contains(out, "hello &lt;b&gt;world&lt;/b&gt; &amp; x") {
		t.Fatalf("expected escaped HTML, got: %s", out)
	}
}
