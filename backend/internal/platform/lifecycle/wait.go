package lifecycle

import (
	"context"
	"sync"
)

// WaitGroup waits for all tracked work or returns when the shutdown deadline expires.
func WaitGroup(ctx context.Context, workers *sync.WaitGroup) error {
	done := make(chan struct{})
	go func() {
		workers.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
