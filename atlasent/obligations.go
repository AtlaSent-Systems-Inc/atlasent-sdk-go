package atlasent

import (
	"context"
	"fmt"
	"sync"
)

// ObligationHandler processes one obligation string from a Decision. Handlers
// can mutate request-scoped state (e.g. set a "redact" flag on a pointer
// carried through ctx) or short-circuit with an error.
type ObligationHandler func(ctx context.Context, obligation string) error

// ObligationRegistry maps obligation strings to their handlers. It is safe
// for concurrent Apply; Register must happen before first Apply.
//
// The recommended pattern is one registry per process, registered at
// startup, then passed alongside every Decision that crosses an
// enforcement boundary.
type ObligationRegistry struct {
	mu            sync.RWMutex
	handlers      map[string]ObligationHandler
	allowsUnknown bool
}

// NewObligationRegistry returns an empty registry. By default, Apply
// returns an error when it encounters an unregistered obligation; see
// AllowUnknown to change that.
func NewObligationRegistry() *ObligationRegistry {
	return &ObligationRegistry{handlers: make(map[string]ObligationHandler)}
}

// Register adds a handler for one obligation string. The obligation is
// matched exactly (case-sensitive).
func (r *ObligationRegistry) Register(obligation string, h ObligationHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[obligation] = h
}

// AllowUnknown controls whether Apply tolerates obligations with no
// registered handler. Defaults to false: unknown obligations are a hard
// error, because silently ignoring "redact:ssn" is worse than failing.
func (r *ObligationRegistry) AllowUnknown(ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.allowsUnknown = ok
}

// Apply runs every obligation on d in order. The first handler error stops
// iteration and propagates. Unknown obligations return an error unless
// AllowUnknown(true) was set.
func (r *ObligationRegistry) Apply(ctx context.Context, d Decision) error {
	r.mu.RLock()
	handlers := r.handlers
	allowUnknown := r.allowsUnknown
	r.mu.RUnlock()

	for _, o := range d.Obligations {
		h, ok := handlers[o]
		if !ok {
			if allowUnknown {
				continue
			}
			return fmt.Errorf("atlasent: unhandled obligation %q", o)
		}
		if err := h(ctx, o); err != nil {
			return fmt.Errorf("atlasent: obligation %q: %w", o, err)
		}
	}
	return nil
}
