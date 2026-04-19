//go:build tools

// tools.go used to import oapi-codegen to keep it in go.mod. We've dropped
// that import so the main module's dependency closure stays minimal for
// pkg.go.dev consumers. Install the generator out-of-band:
//
//	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.6.0
//	go generate ./internal/atlasentapi/...
package atlasentapi
