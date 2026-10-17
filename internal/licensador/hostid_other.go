//go:build !windows

package licensador

import (
	"crypto/rand"
	"errors"
	"os"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// stableIdentifiers mirrors _stable_identifiers on Linux: the first readable
// machine-id file (stripped), then the node id.
func stableIdentifiers() ([]string, error) {
	var ids []string
	for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		rs, err := decodeUTF8(data, false)
		if err != nil {
			return nil, errors.New(err.Error())
		}
		ids = append(ids, pycompat.Strip(pyjson.WTF8(rs)))
		break
	}
	return append(ids, nodeID()), nil
}

func randomNode() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	b[0] |= 0x01
	return macInt(b[:])
}
