package camoufox

import (
	"context"
	"testing"
)

func BenchmarkPoolAcquireRelease(b *testing.B) {
	pool, err := NewPool(context.Background(), 16, &LaunchOptions{})
	if err != nil {
		b.Fatal(err)
	}
	pool.launch = func(ctx context.Context, opts *LaunchOptions) (*Browser, error) {
		return &Browser{}, nil
	}
	pool.closeFn = func(browser *Browser) error { return nil }
	defer pool.Close()

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, release, err := pool.Acquire(context.Background())
			if err != nil {
				b.Fatal(err)
			}
			release()
		}
	})
}
