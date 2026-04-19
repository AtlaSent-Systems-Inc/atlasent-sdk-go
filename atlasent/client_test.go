package atlasent_test

import (
	"context"
	"testing"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
)

func TestMockAllowAll(t *testing.T) {
	m := atlasent.NewMock().AllowAll()
	result, err := m.Evaluate(context.Background(), atlasent.EvaluationPayload{
		Actor:  atlasent.Actor{ID: "user-1", Type: "user"},
		Action: atlasent.Action{ID: "act-1", Type: "data:read"},
		Target: atlasent.Target{ID: "res-1", Type: "database"},
	})
	if err != nil { t.Fatal(err) }
	if result.Outcome != atlasent.OutcomeAllow {
		t.Errorf("expected allow, got %s", result.Outcome)
	}
	if len(m.Calls()) != 1 {
		t.Errorf("expected 1 call, got %d", len(m.Calls()))
	}
}

func TestMockDenyAll(t *testing.T) {
	m := atlasent.NewMock().DenyAll()
	ok, _, err := m.Authorize(context.Background(), atlasent.Actor{ID: "u", Type: "user"}, "data:write", atlasent.Target{ID: "t", Type: "db"})
	if err != nil { t.Fatal(err) }
	if ok { t.Error("expected deny") }
}

func TestMockSetDecision(t *testing.T) {
	m := atlasent.NewMock().AllowAll()
	m.SetDecision(atlasent.MockRule{ActorID: "bad-actor", Outcome: atlasent.OutcomeDeny, RiskLevel: atlasent.RiskHigh})

	r1, _ := m.Evaluate(context.Background(), atlasent.EvaluationPayload{Actor: atlasent.Actor{ID: "good-actor", Type: "user"}, Action: atlasent.Action{ID: "a", Type: "read"}, Target: atlasent.Target{ID: "t", Type: "db"}})
	r2, _ := m.Evaluate(context.Background(), atlasent.EvaluationPayload{Actor: atlasent.Actor{ID: "bad-actor", Type: "user"}, Action: atlasent.Action{ID: "b", Type: "read"}, Target: atlasent.Target{ID: "t", Type: "db"}})
	if r1.Outcome != atlasent.OutcomeAllow {
		t.Errorf("expected allow for good-actor, got %s", r1.Outcome)
	}
	if r2.Outcome != atlasent.OutcomeDeny {
		t.Errorf("expected deny for bad-actor, got %s", r2.Outcome)
	}
}

func TestAuthorizeManyBatch(t *testing.T) {
	c := atlasent.NewMock().AllowAll()
	payloads := []atlasent.EvaluationPayload{
		{Actor: atlasent.Actor{ID: "u1", Type: "user"}, Action: atlasent.Action{ID: "a1", Type: "read"}, Target: atlasent.Target{ID: "t1", Type: "db"}},
		{Actor: atlasent.Actor{ID: "u2", Type: "user"}, Action: atlasent.Action{ID: "a2", Type: "write"}, Target: atlasent.Target{ID: "t2", Type: "db"}},
	}
	results := c.AuthorizeMany(context.Background(), payloads)
	if len(results) != 2 { t.Fatalf("expected 2 results, got %d", len(results)) }
	for _, r := range results {
		if r.Err != nil { t.Errorf("unexpected error: %v", r.Err) }
	}
}
