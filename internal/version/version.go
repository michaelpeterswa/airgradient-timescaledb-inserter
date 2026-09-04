// Package version exposes build information stamped in at link time.
package version

// These are overridden via -ldflags "-X ...=..." at build time (see the
// Dockerfile). The defaults keep `go run` and tests meaningful.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)
