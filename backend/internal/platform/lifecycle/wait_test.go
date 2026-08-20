package lifecycle

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestWaitGroupReturnsAfterWorkersFinish(t *testing.T) {
	workers := &sync.WaitGroup{}
	workers.Add(1)
	go workers.Done()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := WaitGroup(ctx, workers); err != nil {
		t.Fatalf("WaitGroup() error = %v", err)
	}
}

func TestWaitGroupHonorsContextDeadline(t *testing.T) {
	workers := &sync.WaitGroup{}
	workers.Add(1)
	defer workers.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := WaitGroup(ctx, workers)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitGroup() error = %v, want %v", err, context.DeadlineExceeded)
	}
}
