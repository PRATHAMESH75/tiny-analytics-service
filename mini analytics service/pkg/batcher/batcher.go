package batcher

import (
	"errors"
	"sync"
	"time"
)

// Batcher collects items and flushes them based on size or time thresholds.
type Batcher[T any] struct {
	mu        sync.Mutex
	buffer    []T
	maxSize   int
	interval  time.Duration
	flushFn   func([]T) error
	stop      chan struct{}
	wg        sync.WaitGroup
	lastError error
}

// New creates a new batcher instance.
func New[T any](maxSize int, interval time.Duration, flushFn func([]T) error) *Batcher[T] {
	b := &Batcher[T]{
		maxSize:  maxSize,
		interval: interval,
		flushFn:  flushFn,
		stop:     make(chan struct{}),
	}
	b.wg.Add(1)
	go b.loop()
	return b
}

// Add queues an item for batching. If the size threshold is met it flushes immediately.
func (b *Batcher[T]) Add(item T) error {
	b.mu.Lock()
	b.buffer = append(b.buffer, item)
	shouldFlush := len(b.buffer) >= b.maxSize
	var batch []T
	if shouldFlush {
		batch = b.detach()
	}
	b.mu.Unlock()
	if shouldFlush {
		return b.runFlush(batch)
	}
	return nil
}

// Flush forces a flush of the accumulated items.
func (b *Batcher[T]) Flush() error {
	b.mu.Lock()
	batch := b.detach()
	b.mu.Unlock()
	return b.runFlush(batch)
}

// Close stops the background ticker and flushes remaining items.
func (b *Batcher[T]) Close() error {
	close(b.stop)
	b.wg.Wait()
	return b.Flush()
}

// LastError returns the last flush error encountered by the background ticker.
func (b *Batcher[T]) LastError() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lastError
}

func (b *Batcher[T]) loop() {
	defer b.wg.Done()
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := b.Flush(); err != nil {
				b.mu.Lock()
				b.lastError = err
				b.mu.Unlock()
			}
		case <-b.stop:
			return
		}
	}
}

func (b *Batcher[T]) detach() []T {
	if len(b.buffer) == 0 {
		return nil
	}
	batch := make([]T, len(b.buffer))
	copy(batch, b.buffer)
	b.buffer = b.buffer[:0]
	return batch
}

func (b *Batcher[T]) runFlush(batch []T) error {
	if len(batch) == 0 {
		return nil
	}
	if b.flushFn == nil {
		return errors.New("batcher: no flush function configured")
	}
	return b.flushFn(batch)
}
