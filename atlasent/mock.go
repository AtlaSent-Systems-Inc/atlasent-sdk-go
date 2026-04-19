package atlasent

import (
	"context"
	"sync"
	"time"
)

// MockRule is a conditional override for MockClient. Empty fields match
// anything; the first matching rule (in insertion order) wins.
type MockRule struct {
	ActorID    string
	ActionType string
	Outcome    Outcome
	RiskLevel  RiskLevel
}

// MockClient is a test double for Client. It performs no network I/O.
// Wire it into code under test to assert what the SDK would send.
type MockClient struct {
	mu             sync.Mutex
	rules          []MockRule
	defaultOutcome Outcome
	calls          []EvaluationPayload
}

// NewMock returns a MockClient that allows all actions by default.
func NewMock() *MockClient {
	return &MockClient{defaultOutcome: OutcomeAllow}
}

// AllowAll sets the default outcome to allow.
func (m *MockClient) AllowAll() *MockClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultOutcome = OutcomeAllow
	return m
}

// DenyAll sets the default outcome to deny.
func (m *MockClient) DenyAll() *MockClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultOutcome = OutcomeDeny
	return m
}

// SetDecision prepends a rule, so the newest rules take precedence.
func (m *MockClient) SetDecision(rule MockRule) *MockClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = append([]MockRule{rule}, m.rules...)
	return m
}

// Calls returns a snapshot of every payload the mock has seen.
func (m *MockClient) Calls() []EvaluationPayload {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]EvaluationPayload(nil), m.calls...)
}

// Reset clears rules and recorded calls.
func (m *MockClient) Reset() *MockClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = nil
	m.calls = nil
	m.defaultOutcome = OutcomeAllow
	return m
}

// Evaluate records the call and returns a synthetic EvaluationResult.
func (m *MockClient) Evaluate(_ context.Context, payload EvaluationPayload) (*EvaluationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, payload)

	outcome := m.defaultOutcome
	level := RiskLow
	for _, r := range m.rules {
		if (r.ActorID == "" || r.ActorID == payload.Actor.ID) &&
			(r.ActionType == "" || r.ActionType == payload.Action.Type) {
			outcome = r.Outcome
			if r.RiskLevel != "" {
				level = r.RiskLevel
			}
			break
		}
	}
	scoreMap := map[RiskLevel]int{RiskLow: 15, RiskMedium: 45, RiskHigh: 75, RiskCritical: 95}
	result := &EvaluationResult{
		ID:           newRequestID(),
		EvaluationID: newRequestID(),
		Outcome:      outcome,
		Risk:         RiskAssessment{Score: scoreMap[level], Level: level},
		EvaluatedAt:  time.Now(),
	}
	if outcome == OutcomeAllow {
		result.PermitID = newRequestID()
	}
	return result, nil
}

// Authorize is a convenience wrapper mirroring Client.Authorize.
func (m *MockClient) Authorize(ctx context.Context, actor Actor, actionType string, target Target) (bool, *EvaluationResult, error) {
	r, err := m.Evaluate(ctx, EvaluationPayload{
		Actor:  actor,
		Action: Action{ID: newRequestID(), Type: actionType},
		Target: target,
	})
	if err != nil {
		return false, nil, err
	}
	return r.Allowed(), r, nil
}

// AuthorizeMany evaluates multiple payloads concurrently and returns
// results in input order.
func (m *MockClient) AuthorizeMany(ctx context.Context, payloads []EvaluationPayload) []BatchResult {
	results := make([]BatchResult, len(payloads))
	var wg sync.WaitGroup
	wg.Add(len(payloads))
	for i, p := range payloads {
		i, p := i, p
		go func() {
			defer wg.Done()
			r, err := m.Evaluate(ctx, p)
			results[i] = BatchResult{Payload: p, Result: r, Err: err}
		}()
	}
	wg.Wait()
	return results
}
