package licensador

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"strconv"
	"strings"
)

const saltDefault = "REF-LICENSE/v1"

// RefHostID mirrors ref_host_id: "REF-" + upper hex of the first 6 bytes of
// SHA-256(salt || id1 || id2 ...).
func RefHostID(salt []byte, ids []string) string {
	h := sha256.New()
	h.Write(salt)
	for _, id := range ids {
		h.Write([]byte(id))
	}
	return "REF-" + strings.ToUpper(hex.EncodeToString(h.Sum(nil)[:6]))
}

// hostIDNow computes the host id of this machine.
func hostIDNow(salt string) (string, error) {
	ids, err := stableIdentifiers()
	if err != nil {
		return "", err
	}
	return RefHostID([]byte(salt), ids), nil
}

// nodeID mirrors str(uuid.getnode()): the 48-bit hardware address of the
// first interface with one, preferring universally administered addresses.
// Python falls back to a random multicast node when none exists; so does this.
func nodeID() string {
	ifs, _ := net.Interfaces()
	var local []byte
	for _, it := range ifs {
		a := it.HardwareAddr
		if it.Flags&net.FlagLoopback != 0 || len(a) != 6 || strings.Count(a.String(), "00") == 6 {
			continue
		}
		if a[0]&0x02 == 0 {
			return macInt(a)
		}
		if local == nil {
			local = a
		}
	}
	if local != nil {
		return macInt(local)
	}
	return randomNode()
}

func macInt(a []byte) string {
	var n uint64
	for _, b := range a {
		n = n<<8 | uint64(b)
	}
	return strconv.FormatUint(n, 10)
}
