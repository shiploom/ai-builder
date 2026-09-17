// Package version carries the core version string.
//
// It defaults to "dev" for plain `go build ./...` and is stamped with the
// contents of core/VERSION by scripts/build-go.sh via ldflags:
//
//	go build -ldflags "-X github.com/shiploom/ai-builder/internal/version.CoreVersion=$(cat core/VERSION)" ./cmd/shiploom
package version

// CoreVersion is the Shiploom core version (mirrors core/VERSION).
var CoreVersion = "dev"
