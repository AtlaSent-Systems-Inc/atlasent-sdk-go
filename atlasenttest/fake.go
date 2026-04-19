// Package atlasenttest is a fake AtlaSent PDP for use in consumer tests.
//
// It spins up an httptest.Server that speaks the AtlaSent wire protocol and
// returns scripted decisions. Your code under test uses the real SDK
// Client pointed at the fake's URL, so retry, cache, and observer paths
// are exercised end-to-end.
//
//	fake := atlasenttest.NewServer(t)
//	defer fake.Close()
//	fake.On("invoice.pay").Allow()
//	fake.OnResource("invoice", "secret_one").Deny("not owner")
//
//	client, _ := atlasent.New("test", atlasent.WithBaseURL(fake.URL))
//	// ... exercise code under test ...
//
// Matching rules:
//   - Rules registered later win; matching is linear.
//   - On() matches any request with that Action.
//   - OnResource(type, id) matches Action-agnostic on the resource.
//   - OnExact matches the full (Principal, Action, Resource) triple.
//   - With no matching rule, the fake returns Deny with reason "no rule".
package atlasenttest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
)

// Server is a running fake PDP.
type Server struct {
	URL    string
	srv    *httptest.Server
	mu     sync.Mutex
	rules  []rule
	calls  []atlasent.CheckRequest
}

type rule struct {
	match    func(atlasent.CheckRequest) bool
	decision atlasent.Decision
}

// NewServer starts a fake PDP bound to t.Cleanup. The Server.URL is
// suitable for atlasent.WithBaseURL.
func NewServer(t *testing.T) *Server {
	t.Helper()
	s := &Server{}
	s.srv = httptest.NewServer(http.HandlerFunc(s.handle))
	s.URL = s.srv.URL
	t.Cleanup(s.Close)
	return s
}

// Close shuts down the fake. Safe to call multiple times.
func (s *Server) Close() {
	if s.srv != nil {
		s.srv.Close()
		s.srv = nil
	}
}

// Calls returns a snapshot of every CheckRequest the fake has received,
// in order. Useful for asserting call counts or argument shapes.
func (s *Server) Calls() []atlasent.CheckRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]atlasent.CheckRequest, len(s.calls))
	copy(out, s.calls)
	return out
}

// Reset clears registered rules and recorded calls.
func (s *Server) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules = nil
	s.calls = nil
}

// Rule is a fluent builder returned by On*; call Allow/Deny/Return to
// install it.
type Rule struct {
	s     *Server
	match func(atlasent.CheckRequest) bool
}

// On registers a rule that matches any request with the given action.
func (s *Server) On(action string) *Rule {
	return &Rule{s: s, match: func(r atlasent.CheckRequest) bool { return r.Action == action }}
}

// OnResource registers a rule that matches any request for (resourceType,
// resourceID). Pass an empty resourceID to match all IDs of that type.
func (s *Server) OnResource(resourceType, resourceID string) *Rule {
	return &Rule{s: s, match: func(r atlasent.CheckRequest) bool {
		if r.Resource.Type != resourceType {
			return false
		}
		return resourceID == "" || r.Resource.ID == resourceID
	}}
}

// OnExact registers a rule that matches the (principalID, action,
// resourceType, resourceID) quadruple exactly.
func (s *Server) OnExact(principalID, action, resourceType, resourceID string) *Rule {
	return &Rule{s: s, match: func(r atlasent.CheckRequest) bool {
		return r.Principal.ID == principalID &&
			r.Action == action &&
			r.Resource.Type == resourceType &&
			r.Resource.ID == resourceID
	}}
}

// Allow registers the rule with an allow decision.
func (r *Rule) Allow() *Server { return r.Return(atlasent.Decision{Allowed: true}) }

// Deny registers the rule with a deny decision carrying reason.
func (r *Rule) Deny(reason string) *Server {
	return r.Return(atlasent.Decision{Allowed: false, Reason: reason})
}

// Return registers the rule with d.
func (r *Rule) Return(d atlasent.Decision) *Server {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	r.s.rules = append(r.s.rules, rule{match: r.match, decision: d})
	return r.s
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/v1/authorize":
		var req atlasent.CheckRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		s.calls = append(s.calls, req)
		s.mu.Unlock()
		_ = json.NewEncoder(w).Encode(s.match(req))
	case "/v1/authorize/batch":
		var br struct {
			Checks []atlasent.CheckRequest `json:"checks"`
		}
		if err := json.NewDecoder(r.Body).Decode(&br); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		decs := make([]atlasent.Decision, len(br.Checks))
		s.mu.Lock()
		for i, req := range br.Checks {
			s.calls = append(s.calls, req)
			decs[i] = s.matchLocked(req)
		}
		s.mu.Unlock()
		_ = json.NewEncoder(w).Encode(struct {
			Decisions []atlasent.Decision `json:"decisions"`
		}{decs})
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) match(req atlasent.CheckRequest) atlasent.Decision {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.matchLocked(req)
}

// matchLocked finds the last matching rule. Caller must hold s.mu.
func (s *Server) matchLocked(req atlasent.CheckRequest) atlasent.Decision {
	for i := len(s.rules) - 1; i >= 0; i-- {
		if s.rules[i].match(req) {
			return s.rules[i].decision
		}
	}
	return atlasent.Decision{Allowed: false, Reason: "no rule"}
}
