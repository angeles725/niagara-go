// Package version is tools/lib/version.py: the one version line every
// command prints for --version, "<tool> <ToolsVersion>" plus " (git <describe>)"
// when the build knows its source revision.
package version

import "runtime/debug"

// ToolsVersion is version.TOOLS_VERSION, bumped by hand when a tooling round
// changes a CLI contract; a test keeps it equal to the Python constant.
const ToolsVersion = "3.0.0"

// gitDescribe, when set at link time
// (-ldflags "-X github.com/angeles725/niagara-go/internal/version.gitDescribe=$(git describe --always --dirty --abbrev=9)"),
// is printed as is. Otherwise the revision the Go toolchain stamps into the
// binary is used (vcs.revision cut to 9 characters, "-dirty" when modified).
var gitDescribe string

// Describe is the git description of the build, or "" when unknown (a build
// outside a git checkout, or go test).
func Describe() string {
	if gitDescribe != "" {
		return gitDescribe
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	var rev, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	return describeVCS(rev, modified)
}

func describeVCS(rev, modified string) string {
	if rev == "" {
		return ""
	}
	if len(rev) > 9 {
		rev = rev[:9]
	}
	if modified == "true" {
		rev += "-dirty"
	}
	return rev
}

// Line is version.version_line: "bog-nav 3.0.0 (git d62aeb56a-dirty)".
func Line(tool string) string { return lineWith(tool, Describe()) }

func lineWith(tool, describe string) string {
	if describe == "" {
		return tool + " " + ToolsVersion
	}
	return tool + " " + ToolsVersion + " (git " + describe + ")"
}
