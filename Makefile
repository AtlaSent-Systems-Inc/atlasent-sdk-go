# atlasent-sdk-go development targets.

.PHONY: test race vet contract codegen update-vectors release-dry tidy

# Default: the CI gate.
test: vet race

vet:
	go vet ./...

race:
	go test -race -count=1 ./...

contract:
	go test -count=1 -run TestContract ./atlasent/...

# Regenerate oapi-codegen output. Requires the atlasent-api openapi.yaml
# to be vendored at third_party/atlasent-api/openapi.yaml.
codegen:
	go generate ./internal/atlasentapi/...

# Refresh the vendored cross-SDK contract vectors from atlasent-sdk/main.
update-vectors:
	./scripts/update-contract-vectors.sh

# Dry-run a release locally. Requires goreleaser + cosign installed.
release-dry:
	goreleaser release --clean --skip=publish --snapshot

tidy:
	go mod tidy
