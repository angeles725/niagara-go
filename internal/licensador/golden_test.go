package licensador

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// Golden cases were frozen from tools/licensador/licensador.py (Python 3.15,
// cryptography 50) with COLUMNS=100 NO_COLOR=1, cwd = testdata. selftest
// writes into a fresh {tmp} directory. hostid and hostid-salt live under
// testdata/hostbound: the REF hashes the machine id and the node id, so they
// run only where both match the capture host (TestGoldenHostBound).

func sp(s string) *string { return &s }

func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }

// captureNodeSHA256 is the hex SHA-256 of nodeID() on the capture host. The
// machine id is gated by requires.machine_id_sha256 in each cmd.json; the node
// id is the first universally administered interface address, which changes
// on the same machine when a virtual adapter is listed ahead of the NIC.
const captureNodeSHA256 = "4ebacfb9bc13f7b2570361bf4244c31c685ca49e7a41b7c8734e8cfb29e39f5b"

// TestGoldenHostBound replays the hostid cases, frozen from Python on the
// capture host, only when this host's node id is the capture host's too.
func TestGoldenHostBound(t *testing.T) {
	if sum := sha256.Sum256([]byte(nodeID())); hex.EncodeToString(sum[:]) != captureNodeSHA256 {
		t.Skip("host-bound cases: node id differs from the capture host (the REF hashes it)")
	}
	golden.Run(t, filepath.Join("testdata", "hostbound"), Main)
}
