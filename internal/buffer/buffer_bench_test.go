package buffer

import (
	"context"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

func BenchmarkBuffer_Throughput_BlockOnFull(b *testing.B) {
	ctx := context.Background()
	buf := New(ctx, b.N, 1*time.Millisecond, WithPolicy(BlockOnFull))
	buf.Start()

	go func() {
		for range buf.Output() {
		}
	}()

	b.ResetTimer()
	for range b.N {
		buf.Input() <- []byte("benchmark-message")
	}
	buf.Stop()
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ops/sec")
}

func BenchmarkBuffer_Throughput_DropNew(b *testing.B) {
	ctx := context.Background()
	buf := New(ctx, 1000, time.Hour, WithPolicy(DropNew))
	buf.Start()
	defer buf.Stop()

	b.ResetTimer()
	for range b.N {
		buf.Input() <- []byte("benchmark-message")
	}
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ops/sec")
}

func BenchmarkBuffer_Throughput_DropOldest(b *testing.B) {
	ctx := context.Background()
	buf := New(ctx, 1000, time.Hour, WithPolicy(DropOldest))
	buf.Start()
	defer buf.Stop()

	b.ResetTimer()
	for range b.N {
		buf.Input() <- []byte("benchmark-message")
	}
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ops/sec")
}

func BenchmarkBuffer_Throughput_FlushInterval(b *testing.B) {
	ctx := context.Background()
	buf := New(ctx, 10000, 50*time.Millisecond)
	buf.Start()
	defer buf.Stop()

	go func() {
		for range buf.Output() {
		}
	}()

	b.ResetTimer()
	for range b.N {
		buf.Input() <- []byte("benchmark-message")
	}
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ops/sec")
}

func BenchmarkBuffer_Latency_Single(b *testing.B) {
	ctx := context.Background()
	buf := New(ctx, 1, time.Hour, WithPolicy(BlockOnFull))
	buf.Start()
	defer buf.Stop()

	for b.Loop() {
		buf.Input() <- []byte("x")
		<-buf.Output()
	}
}

func BenchmarkBuffer_Latency_Batched(b *testing.B) {
	ctx := context.Background()
	buf := New(ctx, b.N, 1*time.Millisecond, WithPolicy(BlockOnFull))
	buf.Start()
	defer buf.Stop()

	b.ResetTimer()
	for range b.N {
		buf.Input() <- []byte("x")
	}
	for range b.N {
		<-buf.Output()
	}
}

func BenchmarkBuffer_Concurrent_Writers(b *testing.B) {
	ctx := context.Background()
	buf := New(ctx, b.N, 1*time.Millisecond, WithPolicy(BlockOnFull))
	buf.Start()

	var wg sync.WaitGroup

	go func() {
		for range b.N {
			<-buf.Output()
		}
	}()

	numWriters := runtime.NumCPU()

	b.ResetTimer()
	for range numWriters {
		wg.Go(func() {
			msg := []byte("concurrent-message")
			for range b.N / numWriters {
				buf.Input() <- msg
			}
		})
	}
	wg.Wait()
	buf.Stop()
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ops/sec")
}

func BenchmarkBuffer_Memory_Allocations(b *testing.B) {
	ctx := context.Background()

	b.Run("BlockOnFull", func(b *testing.B) {
		buf := New(ctx, b.N, 1*time.Millisecond, WithPolicy(BlockOnFull))
		buf.Start()
		defer buf.Stop()

		var mStatsBefore, mStatsAfter runtime.MemStats
		runtime.ReadMemStats(&mStatsBefore)

		for range b.N {
			buf.Input() <- []byte("memory-test")
			<-buf.Output()
		}

		runtime.ReadMemStats(&mStatsAfter)
		b.ReportMetric(float64(mStatsAfter.Mallocs-mStatsBefore.Mallocs)/float64(b.N), "allocs/op")
		b.ReportMetric(float64(mStatsAfter.TotalAlloc-mStatsBefore.TotalAlloc)/float64(b.N), "B/op")
	})

	b.Run("DropNew", func(b *testing.B) {
		buf := New(ctx, 1000, 1*time.Millisecond, WithPolicy(DropNew))
		buf.Start()
		defer buf.Stop()

		go func() {
			for range buf.Output() {
			}
		}()

		var mStatsBefore, mStatsAfter runtime.MemStats
		runtime.ReadMemStats(&mStatsBefore)

		for range b.N {
			buf.Input() <- []byte("memory-test")
		}

		runtime.ReadMemStats(&mStatsAfter)
		b.ReportMetric(float64(mStatsAfter.Mallocs-mStatsBefore.Mallocs)/float64(b.N), "allocs/op")
		b.ReportMetric(float64(mStatsAfter.TotalAlloc-mStatsBefore.TotalAlloc)/float64(b.N), "B/op")
	})

	b.Run("DropOldest", func(b *testing.B) {
		buf := New(ctx, 1000, 1*time.Millisecond, WithPolicy(DropOldest))
		buf.Start()
		defer buf.Stop()

		go func() {
			for range buf.Output() {
			}
		}()

		var mStatsBefore, mStatsAfter runtime.MemStats
		runtime.ReadMemStats(&mStatsBefore)

		for range b.N {
			buf.Input() <- []byte("memory-test")
		}

		runtime.ReadMemStats(&mStatsAfter)
		b.ReportMetric(float64(mStatsAfter.Mallocs-mStatsBefore.Mallocs)/float64(b.N), "allocs/op")
		b.ReportMetric(float64(mStatsAfter.TotalAlloc-mStatsBefore.TotalAlloc)/float64(b.N), "B/op")
	})
}

func BenchmarkBuffer_Memory_GCPressure(b *testing.B) {
	ctx := context.Background()
	buf := New(ctx, 100, 50*time.Millisecond)
	buf.Start()
	defer buf.Stop()

	go func() {
		for range buf.Output() {
		}
	}()

	var mStatsBefore, mStatsAfter runtime.MemStats

	for range 5 {
		runtime.GC()
	}
	runtime.ReadMemStats(&mStatsBefore)

	for b.Loop() {
		buf.Input() <- make([]byte, 1024)
	}
	b.StopTimer()

	runtime.GC()
	runtime.ReadMemStats(&mStatsAfter)

	b.ReportMetric(float64(mStatsAfter.Mallocs-mStatsBefore.Mallocs), "total_allocs")
	b.ReportMetric(float64(mStatsAfter.GCSys), "gc_heap_objects")
}

func BenchmarkBuffer_PayloadSizes(b *testing.B) {
	ctx := context.Background()

	payloads := []int{64, 256, 1024, 4096, 16384}

	for _, size := range payloads {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			buf := New(ctx, b.N, 1*time.Millisecond, WithPolicy(BlockOnFull))
			buf.Start()
			defer buf.Stop()

			msg := make([]byte, size)
			b.ResetTimer()
			for range b.N {
				buf.Input() <- msg
				<-buf.Output()
			}
			b.ReportMetric(float64(size), "payload_B")
		})
	}
}

func BenchmarkBuffer_BufferSizes(b *testing.B) {
	ctx := context.Background()
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			buf := New(ctx, size, 1*time.Millisecond, WithPolicy(BlockOnFull))
			buf.Start()
			defer buf.Stop()

			go func() {
				for range buf.Output() {
				}
			}()

			b.ResetTimer()
			for range b.N {
				buf.Input() <- []byte("msg")
			}
			b.ReportMetric(float64(size), "buffer_size")
		})
	}
}

func BenchmarkBuffer_PoolUsage(b *testing.B) {
	ctx := context.Background()
	buf := New(ctx, 1000, 50*time.Millisecond)
	buf.Start()
	defer buf.Stop()

	go func() {
		for range buf.Output() {
		}
	}()

	var statsBefore, statsAfter runtime.MemStats
	runtime.ReadMemStats(&statsBefore)

	for range b.N {
		buf.Input() <- []byte("pool-test")
	}
	runtime.GC()
	runtime.ReadMemStats(&statsAfter)

	b.ReportMetric(float64(statsAfter.Mallocs-statsBefore.Mallocs)/float64(b.N), "pool_allocs/op")
}

func BenchmarkBuffer_StopOverhead(b *testing.B) {
	b.Run("Stop", func(b *testing.B) {
		b.ResetTimer()
		for range b.N {
			ctx := context.Background()
			buf := New(ctx, 1000, time.Hour)
			buf.Start()
			buf.Input() <- []byte("cleanup")
			buf.Stop()
		}
	})
}
