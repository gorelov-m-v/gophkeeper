// Package version holds build information set via ldflags.
package version

import "fmt"

var (
	// Version is the semantic version of the build, set via -ldflags.
	Version = "dev"
	// Date is the build timestamp, set via -ldflags.
	Date = "unknown"
	// Commit is the git commit hash of the build, set via -ldflags.
	Commit = "none"
)

// Print outputs build information to stdout.
func Print() {
	fmt.Printf("Build version: %s\n", Version)
	fmt.Printf("Build date: %s\n", Date)
	fmt.Printf("Build commit: %s\n", Commit)
}
