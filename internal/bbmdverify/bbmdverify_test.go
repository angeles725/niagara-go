package bbmdverify

import (
	"encoding/binary"
	"encoding/json"
	"net"
	"os"
	"reflect"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/golden"
)

// The golden cases were frozen from tools/bacnet-bbmd-verify.py (Python 3.15)
// against a loopback-only fake: 127.0.0.2 is a full device + BBMD, 127.0.0.5
// answers only unicast Who-Is, 127.0.0.6 sends odd frames (forwarded, routed,
// non-device and truncated I-Ams), 127.0.0.4 is silent, and 0.0.0.0:47932 is
// held so a --src-port bind fails. startFake is the same fake in Go; nothing
// is ever sent beyond 127.0.0.0/8.
const fakePort, busyPort = 47931, 47932

func iam(inst uint32, objType uint32) []byte {
	b := []byte{0x10, 0x00, 0xC4, 0, 0, 0, 0, 0x22, 0x01, 0xE0, 0x91, 0x00, 0x21, 0x0F}
	binary.BigEndian.PutUint32(b[3:], objType<<22|inst)
	return b
}

func addr6(ip string, port uint16) []byte {
	b := append([]byte(nil), net.ParseIP(ip).To4()...)
	return binary.BigEndian.AppendUint16(b, port)
}

func cat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

var personalities = map[string]func(fn byte) [][]byte{
	"127.0.0.2": func(fn byte) [][]byte {
		switch fn {
		case 0x0A, 0x0B:
			return [][]byte{bvlcWrap(0x0A, cat([]byte{1, 0}, iam(1234, 8)))}
		case 0x02:
			return [][]byte{bvlcWrap(0x03, cat(addr6("127.0.0.2", 0xBAC0), []byte{255, 255, 255, 255},
				addr6("10.1.0.1", 0xBAC0), []byte{255, 255, 255, 0}))}
		case 0x06:
			return [][]byte{bvlcWrap(0x07, cat(addr6("10.9.9.9", 0xBAC0), []byte{0, 60, 0, 45}))}
		}
		return nil
	},
	"127.0.0.5": func(fn byte) [][]byte {
		switch fn {
		case 0x0A:
			return [][]byte{bvlcWrap(0x0A, cat([]byte{1, 0}, iam(77, 8)))}
		case 0x02:
			return [][]byte{bvlcWrap(0x00, []byte{0x00, 0x20})}
		}
		return nil
	},
	"127.0.0.6": func(fn byte) [][]byte {
		switch fn {
		case 0x0A, 0x0B:
			return [][]byte{[]byte("hello"), {0x81},
				bvlcWrap(0x04, cat(addr6("10.2.3.4", 0xBAC0), []byte{1, 0}, iam(4194303, 8))),
				bvlcWrap(0x0B, cat([]byte{1, 0x08, 0, 7, 1, 0x2a}, iam(5, 8))),
				bvlcWrap(0x0A, cat([]byte{1, 0x20, 0xFF, 0xFF, 0, 0xFF}, iam(6, 8))),
				bvlcWrap(0x0A, cat([]byte{1, 0}, iam(9, 0))),
				bvlcWrap(0x0A, []byte{1, 0, 0x10, 0x00, 0xC4, 0x02}),
				bvlcWrap(0x0A, []byte{1})}
		case 0x02:
			return [][]byte{bvlcWrap(0x03, nil)}
		}
		return nil
	},
}

// startFake binds the loopback personalities; it skips the test where
// 127.0.0.x aliases or the fixed ports are unavailable.
func startFake(t *testing.T) {
	t.Helper()
	busy, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: busyPort})
	if err != nil {
		t.Skipf("cannot hold :%d for the bind-busy case: %v", busyPort, err)
	}
	t.Cleanup(func() { busy.Close() })
	for ip, reply := range personalities {
		c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP(ip), Port: fakePort})
		if err != nil {
			t.Skipf("loopback alias %s:%d unavailable: %v", ip, fakePort, err)
		}
		t.Cleanup(func() { c.Close() })
		go func(c *net.UDPConn, reply func(byte) [][]byte) {
			buf := make([]byte, 1500)
			for {
				n, peer, err := c.ReadFromUDP(buf)
				if err != nil {
					return
				}
				if n < 4 || buf[0] != 0x81 {
					continue
				}
				for _, pkt := range reply(buf[1]) {
					_, _ = c.WriteToUDP(pkt, peer)
					time.Sleep(2 * time.Millisecond)
				}
			}
		}(c, reply)
	}
}

func TestGolden(t *testing.T) {
	if testing.Short() {
		t.Skip("loopback probe windows take ~16 s; run without -short (make check)")
	}
	startFake(t)
	golden.Run(t, "testdata", Main)
}

func TestPacketEncoding(t *testing.T) {
	for name, tc := range map[string]struct{ got, want []byte }{
		"whois-unicast":   {buildWhoIs(nil, false), []byte{0x81, 0x0A, 0, 8, 1, 0, 0x10, 0x08}},
		"whois-broadcast": {buildWhoIs(nil, true), []byte{0x81, 0x0B, 0, 8, 1, 0, 0x10, 0x08}},
		"whois-range":     {buildWhoIs(&[2]uint32{0, 4194303}, true), []byte{0x81, 0x0B, 0, 14, 1, 0, 0x10, 0x08, 0x09, 0x00, 0x1B, 0x3F, 0xFF, 0xFF}},
		"read-bdt":        {buildReadBDT(), []byte{0x81, 0x02, 0, 4}},
		"read-fdt":        {buildReadFDT(), []byte{0x81, 0x06, 0, 4}},
	} {
		if !reflect.DeepEqual(tc.got, tc.want) {
			t.Errorf("%s: % x, want % x", name, tc.got, tc.want)
		}
	}
}

func TestVerdictHostOnDeviceSubnetIsNotOK(t *testing.T) {
	if _, k := verdict(true, true, true, false); k == "ok" {
		t.Error("on-subnet host must be inconclusive")
	}
	if _, k := verdict(false, true, true, false); k != "ok" {
		t.Error("crossed broadcast must be ok")
	}
	if !inSubnet("10.1.7.9", "10.1.2.3", 16) || inSubnet("10.1.2.9", "10.1.3.3", 24) {
		t.Error("in_subnet prefix")
	}
}

// TestSetOrderMatchesPython checks pySetOrder against list(set) orders frozen
// from Python 3.15 (testdata/set-order.json).
func TestSetOrderMatchesPython(t *testing.T) {
	raw, err := os.ReadFile("testdata/set-order.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows [][2][]int
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if got := pySetOrder(r[0]); !reflect.DeepEqual(got, r[1]) {
			t.Errorf("set(%v): %v, want %v", r[0], got, r[1])
		}
	}
}

func TestOSErrorText(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("errno numbers are Linux's")
	}
	if got := pyOSError(syscall.ENETUNREACH); got != "[Errno 101] Network is unreachable" {
		t.Errorf("got %q", got)
	}
	if got := pyOSError(syscall.EADDRINUSE); got != "[Errno 98] Address already in use" {
		t.Errorf("got %q", got)
	}
}

func TestInvalidNetworkMessages(t *testing.T) {
	for _, tc := range []struct {
		ip     string
		prefix int
		want   string
	}{
		{"", 24, "Address cannot be empty"},
		{"1.2.3", 24, "Expected 4 octets in '1.2.3'"},
		{"1..2.3", 24, "Empty octet not permitted in '1..2.3'"},
		{"1.a.2.3", 24, "Only decimal digits permitted in 'a' in '1.a.2.3'"},
		{"1.0002.2.3", 24, "At most 3 characters permitted in '0002' in '1.0002.2.3'"},
		{"10.0.0.5/8", 24, "Only one '/' permitted in '10.0.0.5/8/24'"},
		{"10.0.0.5", 33, "'33' is not a valid netmask"},
	} {
		if err := checkNetwork(tc.ip, tc.prefix); err == nil || err.Error() != tc.want {
			t.Errorf("%q/%d: %v, want %q", tc.ip, tc.prefix, err, tc.want)
		}
	}
}
