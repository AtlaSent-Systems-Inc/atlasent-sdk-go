// Package atlasentapi contains the oapi-codegen-generated types and client
// for the canonical AtlaSent REST API defined in atlasent-api/openapi.yaml.
//
// The generated code lives in zz_generated.go and is not checked in when
// atlasent-api is not vendored locally; run `make codegen` (or
// `go generate ./internal/atlasentapi/...` after fetching the spec) to
// produce it.
package atlasentapi

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config config.yaml ../../third_party/atlasent-api/openapi.yaml
