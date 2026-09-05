package clipboard

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type overlappingBackend struct {
	active atomic.Int32
	max    atomic.Int32
}

func (b *overlappingBackend) use() {
	n := b.active.Add(1)
	for old := b.max.Load(); n > old; old = b.max.Load() {
		if b.max.CompareAndSwap(old, n) {
			break
		}
	}
	time.Sleep(time.Millisecond)
	b.active.Add(-1)
}
func (b *overlappingBackend) Available(context.Context) error          { b.use(); return nil }
func (b *overlappingBackend) ReadText(context.Context) (string, error) { b.use(); return "text", nil }
func (b *overlappingBackend) WriteText(context.Context, string) error  { b.use(); return nil }

func TestSynchronizedIncludesAvailability(t *testing.T) {
	probe := &overlappingBackend{}
	backend := NewSynchronized(probe)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range 60 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			switch i % 3 {
			case 0:
				_ = backend.Available(context.Background())
			case 1:
				_, _ = backend.ReadText(context.Background())
			case 2:
				_ = backend.WriteText(context.Background(), "text")
			}
		}()
	}
	close(start)
	wg.Wait()
	if got := probe.max.Load(); got != 1 {
		t.Fatalf("狀態與讀寫互相搶用：同時進入 %d 個操作", got)
	}
}

func TestSynchronizedCancelledRequestDoesNotTouchBackend(t *testing.T) {
	probe := &overlappingBackend{}
	backend := NewSynchronized(probe)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := backend.Available(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := backend.ReadText(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := backend.WriteText(ctx, "text"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if probe.max.Load() != 0 {
		t.Fatal("已取消的請求仍接觸剪貼簿")
	}
}
