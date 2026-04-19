package atlasent

import (
	"context"
	"sync"
	"sync/atomic"
)

// AsyncObserver wraps an Observer so events are handed off to a background
// goroutine and never block the Check caller. Useful when the underlying
// Observer talks to a slow metrics exporter or remote log endpoint.
//
// Back-pressure policy is drop-newest: when the internal queue is full, new
// events are dropped and counted on Dropped, rather than stalling the hot
// path. Observability correctness is worth less than the request that
// triggered it.
type AsyncObserver struct {
	inner   Observer
	ch      chan asyncEvent
	done    chan struct{}
	stopped atomic.Bool
	dropped atomic.Uint64
	stopOnce sync.Once
}

type asyncEvent struct {
	ctx context.Context
	ev  CheckEvent
}

// NewAsyncObserver starts a background worker that drains events into
// inner. queueSize bounds the burst tolerance; 1024 is a reasonable default.
// Callers must invoke Close when done to drain pending events.
func NewAsyncObserver(inner Observer, queueSize int) *AsyncObserver {
	if queueSize <= 0 {
		queueSize = 1024
	}
	a := &AsyncObserver{
		inner: inner,
		ch:    make(chan asyncEvent, queueSize),
		done:  make(chan struct{}),
	}
	go a.run()
	return a
}

// OnCheck implements Observer with non-blocking send semantics.
func (a *AsyncObserver) OnCheck(ctx context.Context, ev CheckEvent) {
	if a.stopped.Load() {
		return
	}
	select {
	case a.ch <- asyncEvent{ctx: ctx, ev: ev}:
	default:
		a.dropped.Add(1)
	}
}

// Dropped is the count of events dropped due to queue pressure. Poll for
// monitoring.
func (a *AsyncObserver) Dropped() uint64 { return a.dropped.Load() }

// Close signals the worker to drain the queue and exit, then blocks until
// it does. Safe to call multiple times.
func (a *AsyncObserver) Close() {
	a.stopOnce.Do(func() {
		a.stopped.Store(true)
		close(a.ch)
	})
	<-a.done
}

func (a *AsyncObserver) run() {
	defer close(a.done)
	for e := range a.ch {
		a.safeInvoke(e)
	}
}

func (a *AsyncObserver) safeInvoke(e asyncEvent) {
	defer func() { _ = recover() }()
	a.inner.OnCheck(e.ctx, e.ev)
}
