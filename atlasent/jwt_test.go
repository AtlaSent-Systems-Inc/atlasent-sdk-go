package atlasent

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestPrincipalFromClaims_Defaults(t *testing.T) {
	p, err := PrincipalFromClaims(map[string]any{
		"sub":    "user_alice",
		"groups": []any{"finance", "admin"},
		"roles":  []any{"billing-writer"},
	})
	if err != nil {
		t.Fatalf("PrincipalFromClaims: %v", err)
	}
	if p.ID != "user_alice" {
		t.Fatalf("want ID=user_alice, got %q", p.ID)
	}
	if p.Type != "user" {
		t.Fatalf("want Type=user (default), got %q", p.Type)
	}
	want := map[string]bool{"finance": true, "admin": true, "role/billing-writer": true}
	got := map[string]bool{}
	for _, g := range p.Groups {
		got[g] = true
	}
	for k := range want {
		if !got[k] {
			t.Errorf("missing group %q in %v", k, p.Groups)
		}
	}
}

func TestPrincipalFromClaims_MissingSub(t *testing.T) {
	_, err := PrincipalFromClaims(map[string]any{"name": "alice"})
	if err == nil {
		t.Fatal("want error when sub missing")
	}
}

func TestPrincipalFromClaims_CustomGroupsClaim(t *testing.T) {
	p, err := PrincipalFromClaims(
		map[string]any{
			"sub":            "u1",
			"cognito:groups": []any{"admin"},
		},
		WithGroupsClaim("cognito:groups"),
	)
	if err != nil {
		t.Fatalf("PrincipalFromClaims: %v", err)
	}
	if len(p.Groups) != 1 || p.Groups[0] != "admin" {
		t.Fatalf("want [admin], got %v", p.Groups)
	}
}

func TestPrincipalFromClaims_CustomIDClaim(t *testing.T) {
	p, err := PrincipalFromClaims(
		map[string]any{"user_id": "u42"},
		WithIDClaim("user_id"),
	)
	if err != nil {
		t.Fatalf("PrincipalFromClaims: %v", err)
	}
	if p.ID != "u42" {
		t.Fatalf("want ID=u42, got %q", p.ID)
	}
}

func TestPrincipalFromJWT(t *testing.T) {
	payload := map[string]any{
		"sub":    "user_bob",
		"groups": []any{"ops"},
	}
	buf, _ := json.Marshal(payload)
	b := base64.RawURLEncoding.EncodeToString(buf)
	token := strings.Join([]string{"hdr", b, "sig"}, ".")

	p, err := PrincipalFromJWT(token)
	if err != nil {
		t.Fatalf("PrincipalFromJWT: %v", err)
	}
	if p.ID != "user_bob" {
		t.Fatalf("want ID=user_bob, got %q", p.ID)
	}
	if len(p.Groups) != 1 || p.Groups[0] != "ops" {
		t.Fatalf("want [ops], got %v", p.Groups)
	}
}

func TestPrincipalFromJWT_Malformed(t *testing.T) {
	for _, bad := range []string{"", "onepart", "two.parts", "a.%%.c"} {
		if _, err := PrincipalFromJWT(bad); err == nil {
			t.Errorf("want error for %q", bad)
		}
	}
}
