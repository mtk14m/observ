// Package version exposes build information injected at link time.
package version

import "fmt"

// Version and Commit are set with -ldflags "-X" by the Makefile.
var (
	Version = "dev"
	Commit  = "unknown"
)

// String returns a human-readable version line.
func String() string {
	return fmt.Sprintf("obsrv %s (%s)", Version, Commit)
}
