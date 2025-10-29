package main

// These variables will be set by the linker at build time
var (
	// Version is the application version, usually set to a git tag (e.g., v1.2.3)
	Version = "dev"

	// Commit is the git commit hash
	Commit = "none"

	// Date is the build date
	Date = "unknown"
)
