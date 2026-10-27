// Package buildinfo exposes version information injected at link time:
//
//	go build -ldflags "-X github.com/yelnurq/startify/backend/internal/platform/buildinfo.Version=1.2.3 ..."
package buildinfo

// Product is the platform name shown in metadata and generated documents.
const Product = "Qadam"

var (
	// Version is the semantic version of the build.
	Version = "0.1.0-dev"
	// Commit is the short git commit hash of the build.
	Commit = "dev"
	// BuiltAt is the RFC 3339 build timestamp.
	BuiltAt = "unknown"
)
