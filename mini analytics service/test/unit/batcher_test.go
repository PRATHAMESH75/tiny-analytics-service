package unit

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"tiny-analytics/pkg/batcher"
)

func TestBatcherFlushBySize(t *testing.T) {
	var (
		mu      sync.Mutex
		flushed [][]int
	)
	b := batcher.New[int](3, time.Second, func(items []int) error {
		mu.Lock()
		defer mu.Unlock()
		cp := append([]int(nil), items...)
		flushed = append(flushed, cp)
		return nil
	})
	defer b.Close()

	require.NoError(t, b.Add(1))
	require.NoError(t, b.Add(2))
	require.NoError(t, b.Add(3))

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(flushed) == 1 && len(flushed[0]) == 3
	}, time.Second, 50*time.Millisecond)
}

func TestBatcherFlushByInterval(t *testing.T) {
	var (
		mu      sync.Mutex
		flushed int
	)
	b := batcher.New[int](10, 50*time.Millisecond, func(items []int) error {
		mu.Lock()
		defer mu.Unlock()
		flushed += len(items)
		return nil
	})
	defer b.Close()

	require.NoError(t, b.Add(42))

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return flushed == 1
	}, time.Second, 20*time.Millisecond)
}
