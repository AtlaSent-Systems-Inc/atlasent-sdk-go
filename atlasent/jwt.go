package atlasent

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// PrincipalFromClaims builds a Principal from a JWT claims map (already
// decoded by your auth layer). This function does NOT verify the token;
// verification is the auth layer's job. Call this after your JWT library
// has validated signature, expiry, and issuer.
//
// Mapping rules (overridable via FromClaimsOption):
//
//	sub           -> Principal.ID           (required)
//	iss or aud[0] -> Principal.Type         ("user" if missing)
//	groups[]      -> Principal.Groups
//	roles[]       -> appended to Principal.Groups (role/<name>)
//	everything    -> Principal.Attributes (for policy reference)
//
// Standard JWTs use "sub"; AWS Cognito uses "cognito:groups"; Auth0 uses
// "https://your.tld/groups"; use WithGroupsClaim to point at whichever
// claim your IDP emits.
func PrincipalFromClaims(claims map[string]any, opts ...FromClaimsOption) (Principal, error) {
	cfg := fromClaimsConfig{
		idClaim:     "sub",
		typeClaim:   "",
		groupsClaim: "groups",
		rolesClaim:  "roles",
	}
	for _, o := range opts {
		o(&cfg)
	}

	p := Principal{Attributes: map[string]any{}}
	for k, v := range claims {
		p.Attributes[k] = v
	}

	id, _ := claims[cfg.idClaim].(string)
	if id == "" {
		return Principal{}, fmt.Errorf("atlasent: JWT claim %q missing or not a string", cfg.idClaim)
	}
	p.ID = id

	if cfg.typeClaim != "" {
		if t, ok := claims[cfg.typeClaim].(string); ok {
			p.Type = t
		}
	}
	if p.Type == "" {
		p.Type = "user"
	}

	p.Groups = append(p.Groups, stringsAt(claims, cfg.groupsClaim)...)
	for _, r := range stringsAt(claims, cfg.rolesClaim) {
		p.Groups = append(p.Groups, "role/"+r)
	}
	return p, nil
}

// PrincipalFromJWT is a convenience wrapper that decodes the payload of an
// unverified JWT and builds a Principal from its claims. It does NOT verify
// signature, expiry, or audience — use your JWT library for that, then
// pass the validated claims to PrincipalFromClaims.
//
// This helper exists for prototypes and tests; production code should verify
// the token and call PrincipalFromClaims directly.
func PrincipalFromJWT(token string, opts ...FromClaimsOption) (Principal, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Principal{}, fmt.Errorf("atlasent: JWT must have 3 parts, got %d", len(parts))
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Principal{}, fmt.Errorf("atlasent: decode JWT payload: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(raw, &claims); err != nil {
		return Principal{}, fmt.Errorf("atlasent: parse JWT claims: %w", err)
	}
	return PrincipalFromClaims(claims, opts...)
}

// stringsAt extracts a []string at key, tolerating []any of strings.
func stringsAt(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	switch vv := v.(type) {
	case []string:
		return vv
	case []any:
		out := make([]string, 0, len(vv))
		for _, x := range vv {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// FromClaimsOption configures PrincipalFromClaims.
type FromClaimsOption func(*fromClaimsConfig)

type fromClaimsConfig struct {
	idClaim     string
	typeClaim   string
	groupsClaim string
	rolesClaim  string
}

// WithIDClaim overrides the claim used for Principal.ID. Default: "sub".
func WithIDClaim(c string) FromClaimsOption { return func(f *fromClaimsConfig) { f.idClaim = c } }

// WithTypeClaim sets the claim used for Principal.Type. Default: unset
// (falls back to "user").
func WithTypeClaim(c string) FromClaimsOption { return func(f *fromClaimsConfig) { f.typeClaim = c } }

// WithGroupsClaim overrides the claim used for Principal.Groups. Default:
// "groups". Use this for Cognito ("cognito:groups"), Auth0 namespaced
// claims, etc.
func WithGroupsClaim(c string) FromClaimsOption {
	return func(f *fromClaimsConfig) { f.groupsClaim = c }
}

// WithRolesClaim overrides the claim used for Principal roles, which are
// folded into Groups as "role/<name>". Default: "roles".
func WithRolesClaim(c string) FromClaimsOption { return func(f *fromClaimsConfig) { f.rolesClaim = c } }
