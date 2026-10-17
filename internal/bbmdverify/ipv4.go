package bbmdverify

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"syscall"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

func isASCIIDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// parseIPv4 mirrors ipaddress.IPv4Address(str) and its AddressValueError texts.
func parseIPv4(s string) (uint32, error) {
	if s == "" {
		//lint:ignore ST1005 parity with the Python message
		return 0, errors.New("Address cannot be empty")
	}
	octets := strings.Split(s, ".")
	if len(octets) != 4 {
		//lint:ignore ST1005 parity with the Python message
		return 0, fmt.Errorf("Expected 4 octets in %s", pycompat.StrRepr(s))
	}
	var v uint32
	for _, o := range octets {
		var msg string
		switch {
		case o == "":
			msg = "Empty octet not permitted"
		case !isASCIIDigits(o):
			msg = "Only decimal digits permitted in " + pycompat.StrRepr(o)
		case len(o) > 3:
			msg = "At most 3 characters permitted in " + pycompat.StrRepr(o)
		case o != "0" && o[0] == '0':
			msg = "Leading zeros are not permitted in " + pycompat.StrRepr(o)
		}
		n, _ := strconv.Atoi(o)
		if msg == "" && n > 255 {
			msg = fmt.Sprintf("Octet %d (> 255) not permitted", n)
		}
		if msg != "" {
			return 0, fmt.Errorf("%s in %s", msg, pycompat.StrRepr(s))
		}
		v = v<<8 | uint32(n)
	}
	return v, nil
}

// checkNetwork mirrors ipaddress.IPv4Network(f"{ip}/{prefix}", strict=False).
func checkNetwork(ip string, prefix int) error {
	full := fmt.Sprintf("%s/%d", ip, prefix)
	if strings.Count(full, "/") > 1 {
		//lint:ignore ST1005 parity with the Python message
		return fmt.Errorf("Only one '/' permitted in %s", pycompat.StrRepr(full))
	}
	if _, err := parseIPv4(ip); err != nil {
		return err
	}
	if prefix < 0 || prefix > 32 {
		return fmt.Errorf("'%d' is not a valid netmask", prefix)
	}
	return nil
}

// inSubnet mirrors in_subnet: ip inside deviceIP/prefix (both already valid).
func inSubnet(ip, deviceIP string, prefix int) bool {
	a, err1 := parseIPv4(ip)
	d, err2 := parseIPv4(deviceIP)
	if err1 != nil || err2 != nil {
		return false
	}
	mask := uint32(0)
	if prefix > 0 {
		mask = ^uint32(0) << (32 - prefix)
	}
	return a&mask == d&mask
}

// pyOSError renders str(OSError) for a socket error: "[Errno N] Text" (POSIX)
// or "[WinError N] Text" (Windows).
func pyOSError(err error) string {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return err.Error()
	}
	msg := errno.Error()
	if msg != "" {
		msg = strings.ToUpper(msg[:1]) + msg[1:]
	}
	return fmt.Sprintf("[%s %d] %s", errnoLabel, int(errno), msg)
}
