package buffer

import (
	"context"
	"sync"
	"time"
)

type BufferPolicy int

const (
	BlockOnFull BufferPolicy = iota
	DropNew
	DropOldest
)

type Buffer struct {
	maxSize       int
	flushInterval time.Duration
	policy        BufferPolicy
	input         chan string
	output        chan string
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	stopOnce      sync.Once
}

type Option func(*Buffer)

func WithPolicy(p BufferPolicy) Option   { return func(b *Buffer) { b.policy = p } }
func WithInputCh(ch chan string) Option  { return func(b *Buffer) { b.input = ch } }
func WithOutputCh(ch chan string) Option { return func(b *Buffer) { b.output = ch } }

func New(ctx context.Context, maxSize int, flushInterval time.Duration, opts ...Option) *Buffer {
	ctx, cancel := context.WithCancel(ctx)
	b := &Buffer{
		maxSize:       maxSize,
		flushInterval: flushInterval,
		policy:        BlockOnFull,
		input:         make(chan string, maxSize),
		output:        make(chan string, maxSize),
		ctx:           ctx,
		cancel:        cancel,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

func (b *Buffer) Start() {
	b.wg.Add(1)
	go b.run()
}

func (b *Buffer) Stop() {
	b.stopOnce.Do(func() {
		b.cancel()
		b.wg.Wait()
		close(b.output)
	})
}

func (b *Buffer) Input() chan<- string  { return b.input }
func (b *Buffer) Output() <-chan string { return b.output }

func (b *Buffer) run() {
	defer b.wg.Done()
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	batch := make([]string, 0, b.maxSize)

	for {
		select {
		case <-b.ctx.Done():
			// Final drain of the input channel before exiting
			for {
				select {
				case msg, ok := <-b.input:
					if !ok {
						break
					}
					if len(batch) < b.maxSize {
						batch = append(batch, msg)
					}
					continue
				default:
				}
				break
			}
			b.emit(batch, true)
			return

		case msg, ok := <-b.input:
			if !ok {
				b.emit(batch, true)
				return
			}

			if len(batch) >= b.maxSize {
				switch b.policy {
				case BlockOnFull:
					b.emit(batch, false)
					batch = batch[:0]
					ticker.Reset(b.flushInterval)
				case DropNew:
					continue // Skip appending this message
				case DropOldest:
					if len(batch) > 0 {
						batch = batch[1:]
					}
				}
			}

			batch = append(batch, msg)

			// Only auto-flush on hit if policy is BlockOnFull
			if b.policy == BlockOnFull && len(batch) >= b.maxSize {
				b.emit(batch, false)
				batch = batch[:0]
				ticker.Reset(b.flushInterval)
			}

		case <-ticker.C:
			if len(batch) > 0 {
				b.emit(batch, false)
				batch = batch[:0]
			}
		}
	}
}

func (b *Buffer) emit(batch []string, force bool) {
	for _, msg := range batch {
		if force {
			b.output <- msg
		} else {
			select {
			case b.output <- msg:
			case <-time.After(50 * time.Millisecond):
				return
			}
		}
	}
}
