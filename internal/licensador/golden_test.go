package licensador

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"net"
	"path/filepath"
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// Golden cases were frozen from tools/licensador/licensador.py (Python 3.15,
// cryptography 50) with COLUMNS=100 NO_COLOR=1, cwd = testdata. selftest
// writes into a fresh {tmp} directory. hostid and hostid-salt live under
// testdata/hostbound and run with cwd = testdata/hostbound, which hostid never
// reads: the REF hashes the machine id and the node id, so they run only where
// both match the capture host (TestGoldenHostBound).

func sp(s string) *string { return &s }

func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }

// captureNodeSHA256 is the hex SHA-256 of the capture host's node id (the
// decimal 48-bit hardware address uuid.getnode() returned). The machine id is
// gated by requires.machine_id_sha256 in each cmd.json; the node id is the
// first universally administered interface address, which changes on the same
// machine when another adapter is listed ahead of the capture NIC.
const captureNodeSHA256 = "4ebacfb9bc13f7b2570361bf4244c31c685ca49e7a41b7c8734e8cfb29e39f5b"

// captureNIC reports whether one of ifs carries the address that hashes to
// sum (present) and whether uuid.getnode() would pick that address (picked:
// the first universally administered non-loopback address, else the first
// locally administered one). It is written apart from nodeID, with its own
// filtering and big.Int formatting, so a nodeID bug cannot make the host-bound
// gate skip: on the capture host such a bug fails the golden cases instead.
// Only hashes are compared; no raw address is stored.
func captureNIC(ifs []net.Interface, sum string) (present, picked bool) {
	var universal, local []byte
	for _, it := range ifs {
		a := []byte(it.HardwareAddr)
		if it.Flags&net.FlagLoopback != 0 || len(a) != 6 || bytes.Equal(a, make([]byte, 6)) {
			continue
		}
		if nodeSum(a) == sum {
			present = true
		}
		switch {
		case a[0]&0x02 == 0 && universal == nil:
			universal = a
		case a[0]&0x02 != 0 && local == nil:
			local = a
		}
	}
	pick := universal
	if pick == nil {
		pick = local
	}
	return present, pick != nil && nodeSum(pick) == sum
}

func nodeSum(a []byte) string {
	s := sha256.Sum256([]byte(new(big.Int).SetBytes(a).String()))
	return hex.EncodeToString(s[:])
}

// TestGoldenHostBound replays the hostid cases, frozen from Python on the
// capture host, only when this host's node id is the capture host's too.
func TestGoldenHostBound(t *testing.T) {
	ifs, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	switch present, picked := captureNIC(ifs, captureNodeSHA256); {
	case !present:
		t.Skip("host-bound cases: no interface carries the capture host's node id (the REF hashes it)")
	case !picked:
		t.Skip("host-bound cases: the capture NIC is present but uuid.getnode() picks another interface listed ahead of it")
	}
	golden.Run(t, filepath.Join("testdata", "hostbound"), Main)
}

func TestCaptureNIC(t *testing.T) {
	univ := net.HardwareAddr{0x00, 0x15, 0x5d, 0x01, 0x02, 0x03}
	other := net.HardwareAddr{0x00, 0x15, 0x5d, 0x0a, 0x0b, 0x0c}
	localAddr := net.HardwareAddr{0x02, 0x42, 0xac, 0x11, 0x00, 0x02}
	nic := func(a net.HardwareAddr, f net.Flags) net.Interface {
		return net.Interface{HardwareAddr: a, Flags: f}
	}
	sum := nodeSum(univ)
	for _, tt := range []struct {
		name            string
		ifs             []net.Interface
		present, picked bool
	}{
		{"capture NIC first", []net.Interface{nic(univ, 0), nic(other, 0)}, true, true},
		{"locally administered adapter ahead", []net.Interface{nic(localAddr, 0), nic(univ, 0)}, true, true},
		{"universal adapter ahead", []net.Interface{nic(other, 0), nic(univ, 0)}, true, false},
		{"loopback and zero addresses ignored", []net.Interface{nic(other, net.FlagLoopback), nic(make(net.HardwareAddr, 6), 0), nic(univ, 0)}, true, true},
		{"absent", []net.Interface{nic(other, 0)}, false, false},
		{"only a local address", []net.Interface{nic(localAddr, 0)}, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			present, picked := captureNIC(tt.ifs, sum)
			if present != tt.present || picked != tt.picked {
				t.Errorf("captureNIC = (%v, %v), want (%v, %v)", present, picked, tt.present, tt.picked)
			}
		})
	}
	// The gate must agree with the decimal node id format nodeID produces.
	if got, want := nodeSum(univ), sha256.Sum256([]byte("91754660355")); got != hex.EncodeToString(want[:]) {
		t.Errorf("nodeSum formats %v differently from str(uuid.getnode())", univ)
	}
}
