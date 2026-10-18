package searchdoor

import (
	"os"
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// The frozen nsearch/find3 CLI goldens of niagara-research (convert_goldens.py),
// run against fake backend scripts (testdata/fake) and a four-file course tree:
// every case needs a python3 on PATH (or $NSEARCH_PYTHON) for the fakes.
// Exec mode: GOLDEN_BIN=<nsearch> runs the nsearch cases, GOLDEN_BIN_FIND3=<find3>
// the find3 ones; build both with -buildvcs=false so --version prints the plain
// "nsearch 3.0.0" the Python checkout printed with GIT_DIR=/nonexistent (D18).

func TestGoldenNsearch(t *testing.T) { golden.Run(t, "testdata", Main) }

// TestGoldenNsearchExec runs the same cases with every backend forced through
// exec (NSEARCH_EXEC=1): the fakes have no data an in-process backend can read,
// so both modes must print the same bytes.
func TestGoldenNsearchExec(t *testing.T) {
	t.Setenv(ExecEnv, "1")
	golden.Run(t, "testdata", Main)
}

func TestGoldenNsearchTimeouts(t *testing.T) {
	if testing.Short() {
		t.Skip("each case waits for a --timeout 1 backend")
	}
	golden.Run(t, "testdata/timeouts", Main)
}

func TestGoldenFind3(t *testing.T) {
	if os.Getenv(golden.BinEnv) != "" {
		bin := os.Getenv("GOLDEN_BIN_FIND3")
		if bin == "" {
			t.Skip("GOLDEN_BIN runs the nsearch binary; set GOLDEN_BIN_FIND3 for the find3 cases")
		}
		t.Setenv(golden.BinEnv, bin)
	}
	golden.Run(t, "testdata/find3", Find3Main)
}
