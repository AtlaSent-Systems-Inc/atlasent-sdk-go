package atlasent

import (
	"errors"
	"fmt"
)

// ErrorKind classifies SDK errors for programmatic handling. Use the Is*
// helpers rather than inspecting ErrorKind directly when possible — they
// work through wrapped errors.
type ErrorKind int

const (
	KindUnknown ErrorKind = iota
	// KindTransport covers network-level failures (DNS, connection, timeout).
	KindTransport
	// KindUnauthorized is a 401 from the PDP — bad or missing API key.
	KindUnauthorized
	// KindForbidden is a 403 from the PDP — your key lacks the scope.
	KindForbidden
	// KindRateLimit is a 429 — back off. Retry-After is honored automatically.
	KindRateLimit
	// KindInvalid is a 4xx other than 401/403/429 — malformed request.
	KindInvalid
	// KindServer is a 5xx — PDP-side error, retried automatically.
	KindServer
	// KindValidation is a client-side validation failure (empty action, etc).
	KindValidation
)

func (k ErrorKind) String() string {
	switch k {
	case KindTransport:
		return "transport"
	case KindUnauthorized:
		return "unauthorized"
	case KindForbidden:
		return "forbidden"
	case KindRateLimit:
		return "rate_limit"
	case KindInvalid:
		return "invalid"
	case KindServer:
		return "server"
	case KindValidation:
		return "validation"
	default:
		return "unknown"
	}
}

// APIError is the typed error returned by Check/CheckMany on PDP or
// transport failure. Inspect Kind for programmatic handling.
type APIError struct {
	Kind   ErrorKind
	Status int    // HTTP status, 0 for transport/validation errors.
	Body   string // Truncated response body, empty when not available.
	Cause  error  // Underlying error (net.OpError, *json.SyntaxError, etc).
}

func (e *APIError) Error() string {
	switch {
	case e.Status != 0 && e.Body != "":
		return fmt.Sprintf("atlasent: %s (status %d): %s", e.Kind, e.Status, e.Body)
	case e.Status != 0:
		return fmt.Sprintf("atlasent: %s (status %d)", e.Kind, e.Status)
	case e.Cause != nil:
		return fmt.Sprintf("atlasent: %s: %v", e.Kind, e.Cause)
	default:
		return fmt.Sprintf("atlasent: %s", e.Kind)
	}
}

func (e *APIError) Unwrap() error { return e.Cause }

// IsTransport reports whether err was a transport-level failure.
func IsTransport(err error) bool { return kindOf(err) == KindTransport }

// IsUnauthorized reports whether the PDP rejected the API key (HTTP 401).
func IsUnauthorized(err error) bool { return kindOf(err) == KindUnauthorized }

// IsForbidden reports whether the API key lacks scope (HTTP 403). This is
// NOT the same as a policy denial — see DeniedError for that.
func IsForbidden(err error) bool { return kindOf(err) == KindForbidden }

// IsRateLimit reports whether the PDP returned HTTP 429.
func IsRateLimit(err error) bool { return kindOf(err) == KindRateLimit }

// IsInvalid reports whether the PDP rejected the request as malformed
// (HTTP 4xx other than 401/403/429).
func IsInvalid(err error) bool { return kindOf(err) == KindInvalid }

// IsServer reports whether the PDP returned a 5xx after all retries.
func IsServer(err error) bool { return kindOf(err) == KindServer }

// IsValidation reports whether the SDK rejected the request locally before
// sending (empty action, empty resource type, etc).
func IsValidation(err error) bool { return kindOf(err) == KindValidation }

func kindOf(err error) ErrorKind {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae.Kind
	}
	return KindUnknown
}

// errBreakerOpen is the cause wrapped inside breakerError's APIError.
var errBreakerOpen = errors.New("circuit breaker open")

// IsBreakerOpen reports whether err is the sentinel returned when the
// circuit breaker blocked a call.
func IsBreakerOpen(err error) bool { return errors.Is(err, errBreakerOpen) }

// isTransientFailure classifies an error for circuit-breaker purposes:
// transport / 429 / 5xx and unknown errors are transient and may indicate
// PDP unavailability; 401 / 403 / 400 / validation are caller mistakes
// and must NOT trip the breaker (otherwise a typo in an API key would
// open the circuit and mask the real cause).
func isTransientFailure(err error) bool {
	switch kindOf(err) {
	case KindTransport, KindRateLimit, KindServer:
		return true
	case KindUnauthorized, KindForbidden, KindInvalid, KindValidation:
		return false
	default:
		// Unknown error class — be conservative and count it; the
		// alternative is silently ignoring real outages.
		return true
	}
}

// classifyHTTP maps a response status to an ErrorKind.
func classifyHTTP(status int) ErrorKind {
	switch {
	case status == 401:
		return KindUnauthorized
	case status == 403:
		return KindForbidden
	case status == 429:
		return KindRateLimit
	case status >= 500:
		return KindServer
	case status >= 400:
		return KindInvalid
	default:
		return KindUnknown
	}
}
