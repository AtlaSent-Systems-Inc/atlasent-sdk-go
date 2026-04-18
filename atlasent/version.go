package atlasent

// Version is the SDK version embedded in the User-Agent header.
//
// It is set at build time via -ldflags, e.g.:
//
//	go build -ldflags "-X github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent.Version=0.5.0"
//
// The release workflow (.github/workflows/release.yml) injects the git
// tag. Local builds fall through to "dev".
var Version = "dev"
