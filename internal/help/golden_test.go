package help

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// TestGolden runs the frozen niagara_help.py cases under testdata/golden.
// testdata is a minimal niagara-help checkout (versions/registry.json and
// indexes/class-index.json) found by walking up from the working directory.
func TestGolden(t *testing.T) {
	t.Setenv("NIAGARA_HELP_DIR", "")
	t.Setenv(envVersionForTest, "")
	defer func(f func() (string, error)) { executable = f }(executable)
	executable = func() (string, error) { return "", nil } // no walk-up from the test binary
	golden.Run(t, "testdata", Main)
}
