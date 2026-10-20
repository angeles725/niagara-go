package searchdoor

import (
	"os"
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// The frozen nsearch/find3 CLI goldens of niagara-research, converted once from the
// Python goldens, recorded against fake backend scripts and a four-file course tree.
// The backends are scriptFakes, the Go port of those scripts; testdata/fake keeps only
// one-line marker files under the names the NSEARCH_* paths give. They run in process
// only: a binary has no fake backends, so GOLDEN_BIN skips them.

// useFakes answers every backend call of the test with scriptFakes.
func useFakes(t *testing.T) {
	t.Helper()
	if os.Getenv(golden.BinEnv) != "" {
		t.Skip("the fake backends exist only in process; GOLDEN_BIN cannot use them")
	}
	testBackend = scriptFakes{}
	t.Cleanup(func() { testBackend = nil })
}

func TestGoldenNsearch(t *testing.T) {
	useFakes(t)
	golden.Run(t, "testdata", Main)
}

func TestGoldenNsearchTimeouts(t *testing.T) {
	if testing.Short() {
		t.Skip("each case waits for a --timeout 1 backend")
	}
	useFakes(t)
	golden.Run(t, "testdata/timeouts", Main)
}

func TestGoldenFind3(t *testing.T) {
	useFakes(t)
	golden.Run(t, "testdata/find3", Find3Main)
}
