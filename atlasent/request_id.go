package atlasent

import (
	"crypto/rand"
	"encoding/hex"
)

// newRequestID returns a random hex string suitable for use as an
// X-Request-ID / action.id correlation identifier. Falls back to a fixed
// string on rand failure so callers never get an empty id.
func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "req_unknown"
	}
	return hex.EncodeToString(b[:])
}
