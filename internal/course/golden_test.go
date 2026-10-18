package course

import (
	"runtime"
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// clearStationEnv clears the station and install variables, so a developer's own defaults never
// reach a case; the cases that test them set them in cmd.json.
func clearStationEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{installEnv, sourceEnv, labsrcEnv, solutionsEnv, stationEnv["training"], stationEnv["review"], stationEnv["cert"]} {
		t.Setenv(name, "")
	}
}

// The cases run in testdata over the synthetic roots root (every marker and item kind),
// dup-root (parse errors), partial-root (missing sources) and norelated (no labs.md: the bad --root and variable cases).
func TestGolden(t *testing.T) {
	clearStationEnv(t)
	golden.Run(t, "testdata", Main)
}

// TestGoldenLinux runs the cases that print the station's config.bog path joined with "/"
// (the native separator joins it on Windows); they run from testdata (cwd_fixture "..").
func TestGoldenLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("captured on Linux: the config.bog path is joined with the native separator")
	}
	clearStationEnv(t)
	golden.Run(t, "testdata/linux", Main)
}

// TestGoldenGo runs the Go-only cases under testdata/go/golden (D46: regenerated with GOLDEN_UPDATE=1
// and reviewed in the diff); they run from testdata (cwd_fixture ".."), over the same fixtures.
func TestGoldenGo(t *testing.T) {
	clearStationEnv(t)
	golden.RunGo(t, Main)
}
