// Package version is the one version line every command prints for
// --version, "<tool> <ToolsVersion>" plus " (git <describe>)" when the build
// knows its source revision (it ported tools/lib/version.py).
//
// ToolsVersion is the canonical copy of the tools version (decision D44): the
// Python constant was deleted with the niagara-research tools, so no drift test
// compares it any more. To bump it, edit the constant in version.go; nothing
// else changes. testdata/fixtures.json is the Python line format recorded at
// tag pre-go-retire (the retired testdata/gen.py, TOOLS_VERSION 3.0.0), and TestLineFormat
// checks that format with the current ToolsVersion in place of the recorded
// one.
package version
