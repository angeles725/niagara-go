package bbmdverify

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"runtime"
	"testing"
)

// TestInetAtonMatchesGlibc checks inetAton against glibc's numeric host
// parsing, frozen from Python 3.15 as getaddrinfo(h, None, AF_INET,
// SOCK_DGRAM, 0, AI_NUMERICHOST) (testdata/inet-aton.json; null = rejected).
func TestInetAtonMatchesGlibc(t *testing.T) {
	raw, err := os.ReadFile("testdata/inet-aton.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows [][2]*string
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		ip, ok := inetAton(*r[0])
		switch {
		case r[1] == nil && ok:
			t.Errorf("%q: %s, want rejected", *r[0], ip)
		case r[1] != nil && (!ok || ip.String() != *r[1]):
			t.Errorf("%q: %s %v, want %s", *r[0], ip, ok, *r[1])
		}
	}
}

// TestResolveHost mirrors CPython's AF_INET sendto host resolution; the
// resolver is stubbed so nothing leaves the host.
func TestResolveHost(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("gaierror texts are glibc's")
	}
	saved := lookupIPv4
	t.Cleanup(func() { lookupIPv4 = saved })
	lookupIPv4 = func(host string) ([]net.IP, error) {
		if host == "localhost" {
			return []net.IP{net.IPv4(127, 0, 0, 1)}, nil
		}
		if host == "flaky" {
			return nil, &net.DNSError{Err: "i/o timeout", Name: host, IsTimeout: true}
		}
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	for _, tc := range []struct{ host, ip, err string }{
		{"", "0.0.0.0", ""},
		{"<broadcast>", "255.255.255.255", ""},
		{"127.1", "127.0.0.1", ""},
		{"localhost", "127.0.0.1", ""},
		{"not a host", "", "[Errno -2] Name or service not known"},
		{"999.1.1.1", "", "[Errno -2] Name or service not known"},
		{"::1", "", "[Errno -9] Address family for hostname not supported"},
		{"flaky", "", "[Errno -3] Temporary failure in name resolution"},
	} {
		ip, err := resolveHost(tc.host)
		var gai *gaiError
		switch {
		case tc.err != "":
			if !errors.As(err, &gai) || err.Error() != tc.err {
				t.Errorf("%q: %v %v, want gaierror %q", tc.host, ip, err, tc.err)
			}
		case err != nil || ip.String() != tc.ip:
			t.Errorf("%q: %v %v, want %s", tc.host, ip, err, tc.ip)
		}
	}
}
