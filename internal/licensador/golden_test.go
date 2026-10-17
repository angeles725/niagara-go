package licensador

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// Golden cases were frozen from tools/licensador/licensador.py (Python 3.15,
// cryptography 50) with COLUMNS=100 NO_COLOR=1, cwd = testdata. selftest
// writes into a fresh {tmp} directory; hostid and hostid-salt are bound to
// the capture host (requires.machine_id_sha256) and skip elsewhere.

func sp(s string) *string { return &s }

func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }
