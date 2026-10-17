package bbmdverify

import (
	"context"
	"errors"
	"net"
	"runtime"
	"strconv"
	"strings"
)

// gaiError is socket.gaierror: CPython's sendto could not resolve the host.
type gaiError struct{ msg string }

func (e *gaiError) Error() string { return e.msg }

// gaierror texts as str(socket.gaierror) prints them on each platform.
func newGAIError(eai int) *gaiError {
	if runtime.GOOS == "windows" {
		return &gaiError{"[Errno 11001] getaddrinfo failed"} // WSAHOST_NOT_FOUND
	}
	text := map[int]string{-2: "Name or service not known", -3: "Temporary failure in name resolution",
		-9: "Address family for hostname not supported"}[eai]
	return &gaiError{"[Errno " + strconv.Itoa(eai) + "] " + text}
}

// lookupIPv4 is the name-service step of getaddrinfo(host, AF_INET); tests
// stub it so no lookup leaves the host.
var lookupIPv4 = func(host string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(context.Background(), "ip4", host)
}

// resolveHost mirrors how CPython's sendto((host, port)) on an AF_INET socket
// resolves host (setipaddr): "" is INADDR_ANY, "<broadcast>" is
// INADDR_BROADCAST, a numeric host is parsed like glibc's inet_aton (127.1,
// 0x7f.0.0.1, 2130706433), anything else goes to the resolver. A failure is
// the socket.gaierror Python raises.
func resolveHost(host string) (net.IP, error) {
	switch host {
	case "":
		return net.IPv4zero.To4(), nil
	case "<broadcast>":
		return net.IPv4bcast.To4(), nil
	}
	if ip, ok := inetAton(host); ok {
		return ip, nil
	}
	if ip := net.ParseIP(host); ip != nil {
		return nil, newGAIError(-9) // an IPv6 literal on an AF_INET socket
	}
	ips, err := lookupIPv4(host)
	var dnsErr *net.DNSError
	switch {
	case err == nil && len(ips) > 0:
		return ips[0].To4(), nil
	case err == nil, errors.As(err, &dnsErr) && dnsErr.IsNotFound:
		return nil, newGAIError(-2)
	}
	return nil, newGAIError(-3)
}

// inetAton mirrors glibc's __inet_aton_exact (getaddrinfo's numeric step):
// one to four dot-separated parts, each decimal, octal (leading 0) or hex
// (0x); the last part fills the remaining bytes; nothing may follow.
func inetAton(s string) (net.IP, bool) {
	parts := strings.Split(s, ".")
	if len(parts) > 4 {
		return nil, false
	}
	vals := make([]uint64, len(parts))
	for i, p := range parts {
		base, digits := 10, p
		switch {
		case len(p) > 1 && p[0] == '0' && (p[1] == 'x' || p[1] == 'X'):
			base, digits = 16, p[2:]
		case len(p) > 1 && p[0] == '0':
			base, digits = 8, p[1:]
		}
		if digits == "" || digits[0] == '+' || digits[0] == '-' || digits[0] == '_' {
			return nil, false
		}
		v, err := strconv.ParseUint(digits, base, 32)
		if err != nil || strings.Contains(digits, "_") {
			return nil, false
		}
		vals[i] = v
	}
	last := len(vals) - 1
	var addr uint64
	for _, v := range vals[:last] {
		if v > 0xFF {
			return nil, false
		}
		addr = addr<<8 | v
	}
	shift := 8 * uint(4-last)
	if vals[last] >= 1<<shift {
		return nil, false
	}
	addr = addr<<shift | vals[last]
	return net.IPv4(byte(addr>>24), byte(addr>>16), byte(addr>>8), byte(addr)).To4(), true
}
