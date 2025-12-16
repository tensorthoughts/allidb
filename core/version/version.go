package version

import "fmt"

// Version information for AlliDB.
// These values can be overridden at build time using ldflags:
//   go build -ldflags "-X github.com/tensorthoughts25/allidb/core/version.Version=1.0.0 -X github.com/tensorthoughts25/allidb/core/version.BuildDate=2024-01-01 -X github.com/tensorthoughts25/allidb/core/version.GitCommit=abc123"
var (
	// Version is the semantic version of AlliDB (e.g., "1.0.0-beta")
	Version = "1.0.0-beta"

	// BuildDate is the date when the binary was built (ISO 8601 format)
	BuildDate = "unknown"

	// GitCommit is the git commit hash of the build
	GitCommit = "unknown"

	// BuildInfo returns a formatted string with version information
	BuildInfo = func() string {
		return Version
	}
)

// GetVersion returns the version string
func GetVersion() string {
	return Version
}

// GetBuildInfo returns detailed build information
func GetBuildInfo() map[string]string {
	return map[string]string{
		"version":    Version,
		"build_date": BuildDate,
		"git_commit": GitCommit,
	}
}

// GetFullVersion returns a formatted version string with all information
func GetFullVersion() string {
	info := GetBuildInfo()
	return fmt.Sprintf("%s (build: %s, commit: %s)", info["version"], info["build_date"], info["git_commit"])
}

