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

var batchPool = sync.Pool{
	New: func() any {
		s := make([][]byte, 0)
		return &s
	},
}

type Buffer struct {
	maxSize       int
	flushInterval time.Duration
	policy        BufferPolicy
	input         chan []byte
	output        chan []byte
	ctx           context.Context
	done          chan struct{}
	wg            sync.WaitGroup
	stopOnce      sync.Once
}

type Option func(*Buffer)

func WithPolicy(p BufferPolicy) Option   { return func(b *Buffer) { b.policy = p } }
func WithInputCh(ch chan []byte) Option  { return func(b *Buffer) { b.input = ch } }
func WithOutputCh(ch chan []byte) Option { return func(b *Buffer) { b.output = ch } }

func New(ctx context.Context, maxSize int, flushInterval time.Duration, opts ...Option) *Buffer {
	b := &Buffer{
		maxSize:       maxSize,
		flushInterval: flushInterval,
		policy:        BlockOnFull,
		input:         make(chan []byte, maxSize),
		output:        make(chan []byte, maxSize),
		ctx:           ctx,
		done:          make(chan struct{}),
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
		close(b.done)
		b.wg.Wait()
		close(b.output)
	})
}

func (b *Buffer) Input() chan<- []byte  { return b.input }
func (b *Buffer) Output() <-chan []byte { return b.output }

func (b *Buffer) run() {
	defer b.wg.Done()
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	batch := batchPool.Get().(*[][]byte)
	*batch = (*batch)[:0]

	for {
		select {
		case <-b.ctx.Done():
			for {
				select {
				case msg, ok := <-b.input:
					if !ok {
						break
					}
					if len(*batch) < b.maxSize {
						*batch = append(*batch, msg)
					}
					continue
				default:
				}
				break
			}
			b.emit(*batch, true)
			batchPool.Put(batch)
			return
		case <-b.done:
			for {
				select {
				case msg, ok := <-b.input:
					if !ok {
						break
					}
					if len(*batch) < b.maxSize {
						*batch = append(*batch, msg)
					}
					continue
				default:
				}
				break
			}
			b.emit(*batch, true)
			batchPool.Put(batch)
			return

		case msg, ok := <-b.input:
			if !ok {
				b.emit(*batch, true)
				batchPool.Put(batch)
				return
			}

			if len(*batch) >= b.maxSize {
				switch b.policy {
				case BlockOnFull:
					b.emit(*batch, false)
					*batch = (*batch)[:0]
					ticker.Reset(b.flushInterval)
				case DropNew:
					continue
				case DropOldest:
					if len(*batch) > 0 {
						*batch = append((*batch)[:0], (*batch)[1:]...)
					}
				}
			}

			*batch = append(*batch, msg)

			if b.policy == BlockOnFull && len(*batch) >= b.maxSize {
				b.emit(*batch, false)
				*batch = (*batch)[:0]
				ticker.Reset(b.flushInterval)
			}

		case <-ticker.C:
			b.flushBatch(*batch)
			*batch = (*batch)[:0]
		}
	}
}

// emit sends batch messages to output channel.
// With force=true, sends all messages immediately.
// Without force, sends with timeout (emitTimeout) and returns on timeout.
func (b *Buffer) emit(batch [][]byte, force bool) {
	for _, msg := range batch {
		if force {
			b.output <- msg
		} else {
			select {
			case b.output <- msg:
			default:
			}
		}
	}
}

func (b *Buffer) flushBatch(batch [][]byte) {
	if len(batch) > 0 {
		b.emit(batch, false)
	}
}
