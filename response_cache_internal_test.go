package ezclassify

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestResponseCachePendingRequestOutlivesTTL(t *testing.T) {
	cache := newResponseCache(2)
	var calls atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	call := func(context.Context) (any, bool, error) {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		return "answer", true, nil
	}
	first := make(chan error, 1)
	go func() {
		_, err := cache.load(context.Background(), "same", 100*time.Millisecond, false, call)
		first <- err
	}()
	<-entered
	time.Sleep(120 * time.Millisecond)
	second := make(chan error, 1)
	go func() {
		_, err := cache.load(context.Background(), "same", 100*time.Millisecond, false, call)
		second <- err
	}()
	time.Sleep(10 * time.Millisecond)
	if got := calls.Load(); got != 1 {
		t.Fatalf("pending request past TTL started %d calls, want 1", got)
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	if _, err := cache.load(context.Background(), "same", 100*time.Millisecond, false, call); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("TTL did not start when response arrived: %d calls, want 1", got)
	}
	time.Sleep(120 * time.Millisecond)
	if _, err := cache.load(context.Background(), "same", 100*time.Millisecond, false, call); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expired response used %d calls, want 2", got)
	}
}

func TestResponseCacheLastCallerCancelsWireRequest(t *testing.T) {
	cache := newResponseCache(2)
	ctx, cancel := context.WithCancel(context.Background())
	entered := make(chan struct{})
	wireCanceled := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		_, err := cache.load(ctx, "same", time.Minute, true, func(shared context.Context) (any, bool, error) {
			close(entered)
			<-shared.Done()
			close(wireCanceled)
			return nil, false, shared.Err()
		})
		result <- err
	}()
	<-entered
	cancel()
	if err := <-result; err != context.Canceled {
		t.Fatalf("caller received %v, want context.Canceled", err)
	}
	select {
	case <-wireCanceled:
	case <-time.After(time.Second):
		t.Fatal("last canceled caller left wire request running")
	}
}
