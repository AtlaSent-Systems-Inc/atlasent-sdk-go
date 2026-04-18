package atlasent

import (
	"errors"
	"fmt"
	"time"
)

// ErrorCode is a coarse classification of an API error, aligned with
// the TypeScript and Python SDKs. Use it at call sites to branch on
// error type without string-matching the message.
type ErrorCode string

const (
	ErrInvalidAPIKey ErrorCode = "invalid_api_key"
	ErrForbidden     ErrorCode = "forbidden"
	ErrRateLimited   ErrorCode = "rate_limited"
	ErrTimeout       ErrorCode = "timeout"
	ErrNetwork       ErrorCode = "network"
	ErrBadResponse   ErrorCode = "bad_response"
	ErrBadRequest    ErrorCode = "bad_request"
	ErrServerError   ErrorCode = "server_error"
)

// Error is the single error type returned by Evaluate and VerifyPermit
// on any failure to reach, authenticate with, or parse a response
// from the AtlaSent API.
//
// A clean policy DENY is *not* an Error — it is returned as
// EvaluateResponse.Permitted == false. Errors signal that the SDK
// could not confirm authorization; the caller should fail closed.
type Error struct {
	// Code is a stable, machine-readable classification.
	Code ErrorCode
	// Status is the HTTP status, when the error came from a server
	// response. Zero for network / timeout / bad_response errors.
	Status int
	// RequestID is the UUID the SDK sent in the X-Request-ID header;
	// correlate with server logs.
	RequestID string
	// RetryAfter is the delay parsed from the Retry-After header on
	// a 429. Zero when the header was absent.
	RetryAfter time.Duration
	// Message is a human-readable explanation. On 4xx / 5xx this is
	// the server's `message` or `reason` field when present.
	Message string
	// Cause is the underlying error, if any (transport failure, JSON
	// decode error, ...). Recoverable via errors.Unwrap / errors.Is.
	Cause error
}

func (e *Error) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("atlasent: %s (status=%d, code=%s, request_id=%s)",
			e.Message, e.Status, e.Code, e.RequestID)
	}
	return fmt.Sprintf("atlasent: %s (code=%s)", e.Message, e.Code)
}

func (e *Error) Unwrap() error { return e.Cause }

// IsCode reports whether err is (or wraps) an *Error with the given
// code. Use at call sites:
//
//	if atlasent.IsCode(err, atlasent.ErrRateLimited) { backoff(); continue }
func IsCode(err error, code ErrorCode) bool {
	var e *Error
	if !errors.As(err, &e) {
		return false
	}
	return e.Code == code
}

// AsError returns err as an *Error if it is one (or wraps one), else nil.
func AsError(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return nil
}
