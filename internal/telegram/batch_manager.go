package telegram

import (
	"context"
	"log"
	"maps"
	"strings"
	"sync"
	"time"
)

const (
	defaultMaxMessagesPerMinute = 20
	defaultRateWindow           = time.Minute

	defaultMaxPendingEntriesPerChat = 200

	// Hard Telegram limit for plain text is 4096 chars
	defaultMaxChunkChars = 4000

	defaultMaxEntriesPerChunk = 30
)

type BatchManager struct {
	ctx           context.Context
	flushInterval time.Duration

	sendFunc func(chatID int64, text string) error

	mu      sync.RWMutex
	enabled map[int64]bool
	chats   map[int64]*chatBatch

	maxPendingEntriesPerChat int

	maxChunkChars      int
	maxEntriesPerChunk int

	rateMaxPerWindow int
	rateWindow       time.Duration
	limiterMu        sync.Mutex
	limiters         map[int64]*slidingWindowLimiter
}

type chatBatch struct {
	pending []string
	timer   *time.Timer
}

type slidingWindowLimiter struct {
	mu         sync.Mutex
	timestamps []time.Time
	max        int
	window     time.Duration
}

func newSlidingWindowLimiter(max int, window time.Duration) *slidingWindowLimiter {
	return &slidingWindowLimiter{
		timestamps: make([]time.Time, 0, max),
		max:        max,
		window:     window,
	}
}

// wait blocks until the caller is allowed to send another message.
func (l *slidingWindowLimiter) wait(ctx context.Context) error {
	for {
		now := time.Now()
		cutoff := now.Add(-l.window)

		l.mu.Lock()
		// Drop timestamps outside of the window.
		idx := 0
		for idx < len(l.timestamps) && l.timestamps[idx].Before(cutoff) {
			idx++
		}
		if idx > 0 {
			l.timestamps = append([]time.Time(nil), l.timestamps[idx:]...)
		}

		if len(l.timestamps) < l.max {
			l.timestamps = append(l.timestamps, now)
			l.mu.Unlock()
			return nil
		}

		// Need to wait until the oldest timestamp leaves the window.
		oldest := l.timestamps[0]
		elapsed := now.Sub(oldest)
		wait := max(l.window-elapsed, 0)
		l.mu.Unlock()

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

type Option func(*BatchManager)

func WithRateLimit(maxMessages int, window time.Duration) Option {
	return func(m *BatchManager) {
		m.rateMaxPerWindow = maxMessages
		m.rateWindow = window
	}
}

func WithMaxPendingEntriesPerChat(n int) Option {
	return func(m *BatchManager) {
		m.maxPendingEntriesPerChat = n
	}
}

func WithChunkCaps(maxChars int, maxEntries int) Option {
	return func(m *BatchManager) {
		m.maxChunkChars = maxChars
		m.maxEntriesPerChunk = maxEntries
	}
}

func NewBatchManager(
	ctx context.Context,
	flushInterval time.Duration,
	sendFunc func(chatID int64, text string) error,
	initialEnabled map[int64]bool,
	opts ...Option,
) *BatchManager {
	enabledCopy := make(map[int64]bool, len(initialEnabled))
	maps.Copy(enabledCopy, initialEnabled)

	m := &BatchManager{
		ctx:           ctx,
		flushInterval: flushInterval,
		sendFunc:      sendFunc,
		enabled:       enabledCopy,
		chats:         make(map[int64]*chatBatch),

		maxPendingEntriesPerChat: defaultMaxPendingEntriesPerChat,
		maxChunkChars:            defaultMaxChunkChars,
		maxEntriesPerChunk:       defaultMaxEntriesPerChunk,

		rateMaxPerWindow: defaultMaxMessagesPerMinute,
		rateWindow:       defaultRateWindow,
		limiters:         make(map[int64]*slidingWindowLimiter),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

func (m *BatchManager) IsEnabled(chatID int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled[chatID]
}

func (m *BatchManager) getLimiter(chatID int64) *slidingWindowLimiter {
	m.limiterMu.Lock()
	defer m.limiterMu.Unlock()

	l := m.limiters[chatID]
	if l == nil {
		l = newSlidingWindowLimiter(m.rateMaxPerWindow, m.rateWindow)
		m.limiters[chatID] = l
	}
	return l
}

func (m *BatchManager) sendWithLimit(chatID int64, text string) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	if err := m.getLimiter(chatID).wait(m.ctx); err != nil {
		return err
	}
	return m.sendFunc(chatID, text)
}

type chunk struct {
	entries []string
	text    string
}

func (m *BatchManager) chunkEntries(entries []string) []chunk {
	if len(entries) == 0 {
		return nil
	}

	sep := "\n\n"
	var (
		chunks  []chunk
		curr    []string
		currLen int
	)

	flushCurr := func() {
		if len(curr) == 0 {
			return
		}
		chunks = append(chunks, chunk{
			entries: append([]string(nil), curr...),
			text:    strings.Join(curr, sep),
		})
		curr = nil
		currLen = 0
	}

	for _, e := range entries {
		eLen := len(e)
		if len(curr) == 0 {
			curr = append(curr, e)
			currLen = eLen
			continue
		}

		nextLen := currLen + len(sep) + eLen
		if nextLen > m.maxChunkChars || len(curr)+1 > m.maxEntriesPerChunk {
			flushCurr()
		}

		// Start a new chunk if curr was flushed.
		curr = append(curr, e)
		if len(curr) == 1 {
			currLen = eLen
		} else {
			currLen = nextLen
		}
	}

	flushCurr()
	return chunks
}

func (m *BatchManager) deliverEntries(chatID int64, entries []string) {
	chunks := m.chunkEntries(entries)
	for _, ch := range chunks {
		if err := m.sendWithLimit(chatID, ch.text); err != nil {
			log.Printf(
				"batch chunk send failed for chat %d, fallback to per-entry sends: %v",
				chatID,
				err,
			)
			for _, entry := range ch.entries {
				if err2 := m.sendWithLimit(chatID, entry); err2 != nil {
					log.Printf("failed to send single entry to chat %d: %v", chatID, err2)
				}
			}
		}
	}
}

// SetEnabled turns batching on/off for a chat.
//
// When turning off and there are pending messages, it flushes them immediately
// to avoid losing logs.
func (m *BatchManager) SetEnabled(chatID int64, enabled bool) {
	var pending []string
	var timer *time.Timer

	m.mu.Lock()
	cb := m.chats[chatID]
	if cb == nil {
		cb = &chatBatch{}
		m.chats[chatID] = cb
	}

	m.enabled[chatID] = enabled

	if !enabled {
		timer = cb.timer
		cb.timer = nil
		pending = append([]string(nil), cb.pending...)
		cb.pending = nil

		// Avoid unbounded memory growth.
		if len(pending) == 0 {
			delete(m.chats, chatID)
		}
	}
	m.mu.Unlock()

	if timer != nil {
		_ = timer.Stop()
	}

	if !enabled && len(pending) > 0 {
		select {
		case <-m.ctx.Done():
			return
		default:
		}
		m.deliverEntries(chatID, pending)
	}
}

// Enqueue adds a new message for the chat.
//
// If batching is enabled for the chat, it buffers and flushes later.
// If batching is disabled, it sends immediately (but still rate-limited).
func (m *BatchManager) Enqueue(chatID int64, text string) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	if !m.IsEnabled(chatID) {
		return m.sendWithLimit(chatID, text)
	}

	m.mu.Lock()
	cb := m.chats[chatID]
	if cb == nil {
		cb = &chatBatch{}
		m.chats[chatID] = cb
	}

	cb.pending = append(cb.pending, text)
	if len(cb.pending) > m.maxPendingEntriesPerChat {
		// Drop oldest entries to bound memory.
		cb.pending = cb.pending[len(cb.pending)-m.maxPendingEntriesPerChat:]
	}

	if cb.timer == nil {
		// Start a fixed-window timer once per batch window.
		cb.timer = time.AfterFunc(m.flushInterval, func() {
			m.flush(chatID)
		})
	}
	m.mu.Unlock()
	return nil
}

func (m *BatchManager) flush(chatID int64) {
	var pending []string

	m.mu.Lock()
	cb := m.chats[chatID]
	if cb == nil || len(cb.pending) == 0 {
		if cb != nil {
			cb.timer = nil
		}
		m.mu.Unlock()
		return
	}

	pending = append([]string(nil), cb.pending...)
	cb.pending = nil
	cb.timer = nil
	m.mu.Unlock()

	select {
	case <-m.ctx.Done():
		return
	default:
	}

	m.deliverEntries(chatID, pending)
}
