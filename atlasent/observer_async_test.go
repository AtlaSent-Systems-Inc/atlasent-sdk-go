package atlasent

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

func TestAsyncObserverDeliversEvents(t *testing.T) {
	var got atomic.Int32
	var mu sync.Mutex
	var seen []CheckEvent
	inner := ObserverFunc(func(_ context.Context, ev CheckEvent) {
		mu.Lock()
		seen = append(seen, ev)
		mu.Unlock()
		got.Add(1)
	})
	a := NewAsyncObserver(inner, 16)
	for i := 0; i < 10; i++ {
		a.OnCheck(context.Background(), CheckEvent{Attempts: i})
	}
	a.Close()

	if got.Load() != 10 {
		t.Fatalf("want 10 events delivered, got %d (dropped=%d)", got.Load(), a.Dropped())
	}
	mu.Lock()
	n := len(seen)
	mu.Unlock()
	if n != 10 {
		t.Fatalf("want 10 seen, got %d", n)
	}
}

func TestAsyncObserverDropsWhenFull(t *testing.T) {
	start := make(chan struct{})
	inner := ObserverFunc(func(_ context.Context, _ CheckEvent) {
		<-start // block the worker so the queue fills
	})
	a := NewAsyncObserver(inner, 2)

	// First event is consumed and blocks in inner; queue holds 2 more.
	// Everything beyond that is dropped.
	for i := 0; i < 10; i++ {
		a.OnCheck(context.Background(), CheckEvent{})
	}
	close(start)
	a.Close()

	if a.Dropped() < 5 {
		t.Fatalf("expected several dropped events, got %d", a.Dropped())
	}
}

func TestAsyncObserverNoPanicOnConcurrentClose(t *testing.T) {
	// Hammer OnCheck from many goroutines while a separate goroutine
	// calls Close. The pre-fix version would panic on "send on closed
	// channel" under this load.
	a := NewAsyncObserver(ObserverFunc(func(_ context.Context, _ CheckEvent) {}), 4)

	const senders = 32
	var wg sync.WaitGroup
	wg.Add(senders)
	for i := 0; i < senders; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				a.OnCheck(context.Background(), CheckEvent{})
			}
		}()
	}
	// Close while sends are mid-flight.
	a.Close()
	wg.Wait()
	// And the OnCheck calls keep being safe after Close.
	a.OnCheck(context.Background(), CheckEvent{})
}

func TestAsyncObserverSurvivesPanic(t *testing.T) {
	var ran atomic.Bool
	inner := ObserverFunc(func(_ context.Context, ev CheckEvent) {
		if ev.Attempts == 0 {
			panic("boom")
		}
		ran.Store(true)
	})
	a := NewAsyncObserver(inner, 8)
	a.OnCheck(context.Background(), CheckEvent{Attempts: 0})
	a.OnCheck(context.Background(), CheckEvent{Attempts: 1})
	a.Close()
	if !ran.Load() {
		t.Fatal("second event should have run despite earlier panic")
	}
}
