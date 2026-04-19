package bundle

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
)

// Manager drives a Syncer on an interval and keeps the current
// EngineState loaded. It implements atlasent.LocalEvaluator: the SDK
// Client consults the manager BEFORE the remote PDP, and the manager
// answers locally when the bundle covers the request.
//
// Manager is safe for concurrent use.
type Manager struct {
	syncer  Syncer
	engine  PolicyEngine
	current atomic.Pointer[EngineState]

	mu          sync.Mutex
	lastBundle  *Bundle
	lastSyncErr error

	interval time.Duration
	stop     chan struct{}
	done     chan struct{}
	onChange func(old, new *Bundle)
}

// ManagerOption configures a Manager.
type ManagerOption func(*Manager)

// WithSyncInterval overrides the default sync interval (30s). Shorter
// intervals reduce policy-change propagation time; longer intervals
// reduce PDP load.
func WithSyncInterval(d time.Duration) ManagerOption {
	return func(m *Manager) { m.interval = d }
}

// WithOnBundleChange fires whenever a new bundle is loaded (including
// the initial load). The callback receives the previous and new bundle;
// old is nil on first load.
func WithOnBundleChange(fn func(old, new *Bundle)) ManagerOption {
	return func(m *Manager) { m.onChange = fn }
}

// NewManager constructs a Manager and performs the initial sync
// synchronously. The first sync must succeed; otherwise there's no
// bundle to evaluate against and Evaluate would always return ok=false
// silently. If the initial fetch fails, NewManager returns the error
// and the manager is not started.
//
// After construction the Manager refreshes in a background goroutine
// until Close is called.
func NewManager(s Syncer, engine PolicyEngine, opts ...ManagerOption) (*Manager, error) {
	if s == nil {
		return nil, errors.New("bundle: Syncer is required")
	}
	if engine == nil {
		return nil, errors.New("bundle: PolicyEngine is required")
	}
	m := &Manager{
		syncer:   s,
		engine:   engine,
		interval: 30 * time.Second,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	for _, opt := range opts {
		opt(m)
	}

	// Initial sync — fail loudly.
	b, fresh, err := s.Fetch(context.Background())
	if err != nil {
		return nil, err
	}
	if !fresh || b == nil {
		return nil, errors.New("bundle: initial sync returned 304; server must ship a bundle on first load")
	}
	if err := m.load(b); err != nil {
		return nil, err
	}

	go m.run()
	return m, nil
}

// Evaluate implements atlasent.LocalEvaluator. Delegates to the current
// EngineState; returns ok=false when no bundle is loaded.
func (m *Manager) Evaluate(ctx context.Context, req atlasent.CheckRequest) (atlasent.Decision, bool, error) {
	p := m.current.Load()
	if p == nil {
		return atlasent.Decision{}, false, nil
	}
	return (*p).Evaluate(ctx, req)
}

// LastBundle returns the most recently loaded bundle (or nil). Intended
// for diagnostics and health endpoints, not the hot path.
func (m *Manager) LastBundle() *Bundle {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastBundle
}

// LastError returns the most recent sync failure (or nil). Cleared on
// the next successful sync.
func (m *Manager) LastError() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastSyncErr
}

// Close stops the background sync loop and blocks until it exits.
func (m *Manager) Close() {
	select {
	case <-m.stop:
		// already closed
	default:
		close(m.stop)
	}
	<-m.done
}

func (m *Manager) run() {
	defer close(m.done)
	t := time.NewTicker(m.interval)
	defer t.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(context.Background(), m.interval)
			m.sync(ctx)
			cancel()
		}
	}
}

func (m *Manager) sync(ctx context.Context) {
	b, fresh, err := m.syncer.Fetch(ctx)
	m.mu.Lock()
	m.lastSyncErr = err
	m.mu.Unlock()
	if err != nil || !fresh || b == nil {
		return
	}
	_ = m.load(b)
}

func (m *Manager) load(b *Bundle) error {
	state, err := m.engine.Load(b.Payload)
	if err != nil {
		m.mu.Lock()
		m.lastSyncErr = err
		m.mu.Unlock()
		return err
	}
	m.current.Store(&state)

	m.mu.Lock()
	old := m.lastBundle
	m.lastBundle = b
	m.lastSyncErr = nil
	cb := m.onChange
	m.mu.Unlock()

	if cb != nil {
		defer func() { _ = recover() }()
		cb(old, b)
	}
	return nil
}
