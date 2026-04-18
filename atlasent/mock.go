package atlasent

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MockRule defines a conditional override for the mock client.
type MockRule struct {
	ActorID    string
	ActionType string
	Decision   Decision
	RiskLevel  RiskLevel
}

// MockClient is a test double for Client. It does not make HTTP calls.
type MockClient struct {
	mu              sync.Mutex
	rules           []MockRule
	defaultDecision Decision
	calls           []EvaluationPayload
}

// NewMock returns a MockClient that allows all actions by default.
func NewMock() *MockClient {
	return &MockClient{defaultDecision: DecisionAllow}
}

func (m *MockClient) AllowAll() *MockClient {
	m.mu.Lock(); defer m.mu.Unlock()
	m.defaultDecision = DecisionAllow; return m
}

func (m *MockClient) DenyAll() *MockClient {
	m.mu.Lock(); defer m.mu.Unlock()
	m.defaultDecision = DecisionDeny; return m
}

func (m *MockClient) SetDecision(rule MockRule) *MockClient {
	m.mu.Lock(); defer m.mu.Unlock()
	m.rules = append([]MockRule{rule}, m.rules...); return m
}

func (m *MockClient) Calls() []EvaluationPayload {
	m.mu.Lock(); defer m.mu.Unlock()
	return append([]EvaluationPayload(nil), m.calls...)
}

func (m *MockClient) Reset() *MockClient {
	m.mu.Lock(); defer m.mu.Unlock()
	m.rules = nil; m.calls = nil; m.defaultDecision = DecisionAllow; return m
}

func (m *MockClient) Evaluate(_ context.Context, payload EvaluationPayload) (*EvaluationResult, error) {
	m.mu.Lock(); defer m.mu.Unlock()
	m.calls = append(m.calls, payload)

	decision := m.defaultDecision
	level := RiskLow
	for _, r := range m.rules {
		if (r.ActorID == "" || r.ActorID == payload.Actor.ID) &&
			(r.ActionType == "" || r.ActionType == payload.Action.Type) {
			decision = r.Decision
			if r.RiskLevel != "" { level = r.RiskLevel }
			break
		}
	}
	scoreMap := map[RiskLevel]int{RiskLow: 15, RiskMedium: 45, RiskHigh: 75, RiskCritical: 95}
	result := &EvaluationResult{
		ID:          uuid.NewString(),
		Decision:    decision,
		Risk:        RiskAssessment{Score: scoreMap[level], Level: level, Factors: []string{}},
		EvaluatedAt: time.Now(),
	}
	if decision == DecisionAllow {
		result.PermitID = uuid.NewString()
	}
	return result, nil
}

func (m *MockClient) Authorize(ctx context.Context, actor Actor, actionType string, target Target) (bool, *EvaluationResult, error) {
	r, err := m.Evaluate(ctx, EvaluationPayload{Actor: actor, Action: Action{ID: uuid.NewString(), Type: actionType}, Target: target})
	if err != nil { return false, nil, err }
	return r.Decision == DecisionAllow, r, nil
}

// AuthorizeMany evaluates multiple payloads concurrently and returns results in order.
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
