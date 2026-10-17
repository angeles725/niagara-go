package bbmdverify

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// BVLL / BACnet constants ([CERT] from the bacnet-rt bytecode corpus, see the
// Python tool's docstring).
const (
	bvlcType            = 0x81
	fnOriginalUnicast   = 0x0A
	fnOriginalBroadcast = 0x0B
	fnForwardedNPDU     = 0x04
	fnReadBDT           = 0x02
	fnReadBDTAck        = 0x03
	fnReadFDT           = 0x06
	fnReadFDTAck        = 0x07
	apduUnconfirmedReq  = 0x10
	svcWhoIs            = 0x08
	svcIAm              = 0x00
)

// encodeUnsigned is the minimal big-endian unsigned (at least one byte).
func encodeUnsigned(v uint32) []byte {
	if v == 0 {
		return []byte{0}
	}
	var out []byte
	for v > 0 {
		out = append([]byte{byte(v)}, out...)
		v >>= 8
	}
	return out
}

// contextUnsigned is a context-tagged unsigned: [tag<<4 | 0x08 | len] + data.
func contextUnsigned(tag byte, v uint32) []byte {
	data := encodeUnsigned(v)
	return append([]byte{tag<<4 | 0x08 | byte(len(data))}, data...)
}

func bvlcWrap(function byte, payload []byte) []byte {
	out := []byte{bvlcType, function, 0, 0}
	binary.BigEndian.PutUint16(out[2:], uint16(4+len(payload)))
	return append(out, payload...)
}

// buildWhoIs is an unrestricted Who-Is when lohi is nil, else a ranged one.
func buildWhoIs(lohi *[2]uint32, broadcast bool) []byte {
	apdu := []byte{apduUnconfirmedReq, svcWhoIs}
	if lohi != nil {
		apdu = append(apdu, contextUnsigned(0, lohi[0])...)
		apdu = append(apdu, contextUnsigned(1, lohi[1])...)
	}
	fn := byte(fnOriginalUnicast)
	if broadcast {
		fn = fnOriginalBroadcast
	}
	return bvlcWrap(fn, append([]byte{0x01, 0x00}, apdu...))
}

func buildReadBDT() []byte { return bvlcWrap(fnReadBDT, nil) }
func buildReadFDT() []byte { return bvlcWrap(fnReadFDT, nil) }

// parseIAm returns the device instance of an I-Am APDU (-1 when not one).
func parseIAm(apdu []byte) int {
	if len(apdu) < 2 || apdu[0] != apduUnconfirmedReq || apdu[1] != svcIAm {
		return -1
	}
	body := apdu[2:]
	if len(body) < 5 || body[0] != 0xC4 {
		return -1
	}
	objid := binary.BigEndian.Uint32(body[1:5])
	if objid>>22 != 8 {
		return -1
	}
	return int(objid & 0x3FFFFF)
}

// parseBVLC returns (function, payload), or ok=false when not a BVLC/IP frame.
func parseBVLC(data []byte) (byte, []byte, bool) {
	if len(data) < 4 || data[0] != bvlcType {
		return 0, nil, false
	}
	length := int(binary.BigEndian.Uint16(data[2:4]))
	if length <= len(data) {
		if length < 4 {
			return data[1], nil, true
		}
		return data[1], data[4:length], true
	}
	return data[1], data[4:], true
}

// extractIAm pulls an I-Am instance (-1 = none) from a BVLL payload, plus the
// 6-byte original address of a Forwarded-NPDU.
func extractIAm(function byte, payload []byte) (int, []byte) {
	var orig, npdu []byte
	switch function {
	case fnOriginalUnicast, fnOriginalBroadcast:
		npdu = payload
	case fnForwardedNPDU:
		if len(payload) < 6 {
			return -1, nil
		}
		orig, npdu = payload[:6], payload[6:]
	default:
		return -1, nil
	}
	if len(npdu) < 2 {
		return -1, orig
	}
	control, idx := npdu[1], 2
	if control&0x20 != 0 {
		if len(npdu) < idx+3 {
			return -1, orig
		}
		idx += 3 + int(npdu[idx+2])
	}
	if control&0x08 != 0 {
		if len(npdu) < idx+3 {
			return -1, orig
		}
		idx += 3 + int(npdu[idx+2])
	}
	if control&0x20 != 0 {
		idx++
	}
	if idx > len(npdu) {
		idx = len(npdu)
	}
	return parseIAm(npdu[idx:]), orig
}

func addr6ToStr(a []byte) string {
	return fmt.Sprintf("%d.%d.%d.%d:0x%04X", a[0], a[1], a[2], a[3], int(a[4])<<8|int(a[5]))
}

func dotted(b []byte) string {
	parts := make([]string, len(b))
	for i, c := range b {
		parts[i] = fmt.Sprint(c)
	}
	return strings.Join(parts, ".")
}

type bdtEntry struct{ addr, mask string }

type fdtEntry struct {
	addr     string
	ttl, rem int
}

// parseBDTAck parses a Read-BDT-Ack (nil, false when the frame is not one).
func parseBDTAck(data []byte) ([]bdtEntry, bool) {
	fn, payload, ok := parseBVLC(data)
	if !ok || fn != fnReadBDTAck {
		return nil, false
	}
	entries := []bdtEntry{}
	for i := 0; i < len(payload)-9; i += 10 {
		entries = append(entries, bdtEntry{addr6ToStr(payload[i : i+6]), dotted(payload[i+6 : i+10])})
	}
	return entries, true
}

// parseFDTAck parses a Read-FDT-Ack (nil, false when the frame is not one).
func parseFDTAck(data []byte) ([]fdtEntry, bool) {
	fn, payload, ok := parseBVLC(data)
	if !ok || fn != fnReadFDTAck {
		return nil, false
	}
	entries := []fdtEntry{}
	for i := 0; i < len(payload)-9; i += 10 {
		entries = append(entries, fdtEntry{addr6ToStr(payload[i : i+6]),
			int(binary.BigEndian.Uint16(payload[i+6:])), int(binary.BigEndian.Uint16(payload[i+8:]))})
	}
	return entries, true
}

// pySetOrder returns the iteration order of a CPython set of small
// non-negative ints built by add() in the given order (open addressing,
// 9 linear probes, perturbation, resize at 60% fill).
func pySetOrder(adds []int) []int {
	const probes, minSize = 9, 8
	table := make([]int, minSize)
	used := make([]bool, minSize)
	fill := 0
	insert := func(table []int, used []bool, key int) {
		mask := len(table) - 1
		perturb, i := key, key&mask
		for {
			for j := 0; j <= probes; j++ {
				if j > 0 && i+probes > mask {
					break
				}
				k := i + j
				if !used[k] {
					table[k], used[k] = key, true
					return
				}
				if table[k] == key {
					return
				}
			}
			perturb >>= 5
			i = (i*5 + 1 + perturb) & mask
		}
	}
	contains := func(key int) bool {
		for k := range table {
			if used[k] && table[k] == key {
				return true
			}
		}
		return false
	}
	for _, key := range adds {
		if contains(key) {
			continue
		}
		insert(table, used, key)
		fill++
		if fill*5 >= (len(table)-1)*3 {
			size := minSize
			for size <= fill*4 {
				size <<= 1
			}
			nt, nu := make([]int, size), make([]bool, size)
			for k := range table {
				if used[k] {
					insert(nt, nu, table[k])
				}
			}
			table, used = nt, nu
		}
	}
	var out []int
	for k := range table {
		if used[k] {
			out = append(out, table[k])
		}
	}
	return out
}
