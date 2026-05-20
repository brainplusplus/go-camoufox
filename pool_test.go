package camoufox

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPoolBoundedAcquireReleaseAndClose(t *testing.T) {
	pool, err := NewPool(context.Background(), 2, &LaunchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var active int32
	var maxActive int32
	var closed int32
	pool.launch = func(ctx context.Context, opts *LaunchOptions) (*Browser, error) {
		now := atomic.AddInt32(&active, 1)
		for {
			old := atomic.LoadInt32(&maxActive)
			if now <= old || atomic.CompareAndSwapInt32(&maxActive, old, now) {
				break
			}
		}
		return &Browser{}, nil
	}
	pool.closeFn = func(browser *Browser) error {
		atomic.AddInt32(&closed, 1)
		atomic.AddInt32(&active, -1)
		return nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			browser, release, err := pool.Acquire(context.Background())
			if err != nil {
				t.Errorf("acquire failed: %v", err)
				return
			}
			if browser == nil {
				t.Error("nil browser")
			}
			time.Sleep(time.Millisecond)
			release()
			release()
		}()
	}
	wg.Wait()
	if maxActive > 2 {
		t.Fatalf("pool exceeded size: %d", maxActive)
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&closed) != atomic.LoadInt32(&maxActive) {
		t.Fatalf("expected all launched browsers to close, closed=%d launched=%d", closed, maxActive)
	}
	_, _, err = pool.Acquire(context.Background())
	if !errors.Is(err, ErrPoolClosed) {
		t.Fatalf("expected ErrPoolClosed, got %v", err)
	}
}

func TestPoolAcquireRespectsCancellation(t *testing.T) {
	pool, err := NewPool(context.Background(), 1, &LaunchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pool.launch = func(ctx context.Context, opts *LaunchOptions) (*Browser, error) {
		return &Browser{}, nil
	}
	browser, release, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if browser == nil {
		t.Fatal("nil browser")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = pool.Acquire(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	release()
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
}
