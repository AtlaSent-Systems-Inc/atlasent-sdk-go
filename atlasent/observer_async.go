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
//
// The channel is never closed; shutdown is signaled via a separate stop
// channel. This avoids the "send on closed channel" race that occurs when
// Close runs concurrently with OnCheck.
type AsyncObserver struct {
	inner    Observer
	ch       chan asyncEvent
	stop     chan struct{}
	done     chan struct{}
	dropped  atomic.Uint64
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
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
	go a.run()
	return a
}

// OnCheck implements Observer with non-blocking send semantics. After Close
// has been called the event is dropped (counted on Dropped) rather than
// sent.
func (a *AsyncObserver) OnCheck(ctx context.Context, ev CheckEvent) {
	// stop wins over channel send so post-shutdown calls drop instead of
	// piling up in the buffer.
	select {
	case <-a.stop:
		a.dropped.Add(1)
		return
	default:
	}
	select {
	case a.ch <- asyncEvent{ctx: ctx, ev: ev}:
	case <-a.stop:
		a.dropped.Add(1)
	default:
		a.dropped.Add(1)
	}
}

// Dropped is the count of events dropped due to queue pressure or
// post-shutdown calls. Poll for monitoring.
func (a *AsyncObserver) Dropped() uint64 { return a.dropped.Load() }

// Close signals the worker to drain pending events and exit, then blocks
// until it does. Safe to call multiple times.
func (a *AsyncObserver) Close() {
	a.stopOnce.Do(func() { close(a.stop) })
	<-a.done
}

// run drains a.ch until stop fires, then makes a final non-blocking pass
// to flush any in-flight events that landed in the buffer before stop.
func (a *AsyncObserver) run() {
	defer close(a.done)
	for {
		select {
		case e := <-a.ch:
			a.safeInvoke(e)
		case <-a.stop:
			a.drain()
			return
		}
	}
}

// drain pulls every event currently in the buffer in a non-blocking
// fashion. Sends that race with stop may still arrive after this returns;
// they sit in the buffer and are GC'd with the AsyncObserver.
func (a *AsyncObserver) drain() {
	for {
		select {
		case e := <-a.ch:
			a.safeInvoke(e)
		default:
			return
		}
	}
}

func (a *AsyncObserver) safeInvoke(e asyncEvent) {
	defer func() { _ = recover() }()
	a.inner.OnCheck(e.ctx, e.ev)
}
