package help

// The iteration order of a CPython set of str built one element at a time,
// as the reference CLI runs with PYTHONHASHSEED=0 (the golden suite fixes
// it). module_summary.py iterates set(class names) and then sorts by count
// only, so equal counts keep the set order and it reaches the output.
//
// This is CPython 3.15's str hash (SipHash-1-3 over the PEP 393 buffer with
// the all-zero key PYTHONHASHSEED=0 selects) and setobject.c's open
// addressing: 8 slots, 9 linear probes then perturbed jumps, resize to
// 4 x used once fill*5 >= mask*3, iteration in slot order.

import (
	"encoding/binary"
	"math/bits"
	"unicode/utf8"
)

// pyStrHash is hash(s) for a Python str under PYTHONHASHSEED=0.
func pyStrHash(s string) int64 {
	if s == "" {
		return 0
	}
	h := int64(siphash13(pep393Buffer(s)))
	if h == -1 {
		return -2
	}
	return h
}

// pep393Buffer is the little-endian code-unit buffer CPython hashes: one
// byte per code point when all are below U+0100, else two, else four.
func pep393Buffer(s string) []byte {
	width := 1
	for _, r := range s {
		switch {
		case r > 0xFFFF:
			width = 4
		case r > 0xFF && width < 2:
			width = 2
		}
	}
	buf := make([]byte, 0, width*utf8.RuneCountInString(s))
	for _, r := range s {
		switch width {
		case 1:
			buf = append(buf, byte(r))
		case 2:
			buf = binary.LittleEndian.AppendUint16(buf, uint16(r))
		default:
			buf = binary.LittleEndian.AppendUint32(buf, uint32(r))
		}
	}
	return buf
}

// siphash13 is Python/pyhash.c siphash13 with k0 = k1 = 0.
func siphash13(in []byte) uint64 {
	v0 := uint64(0x736f6d6570736575)
	v1 := uint64(0x646f72616e646f6d)
	v2 := uint64(0x6c7967656e657261)
	v3 := uint64(0x7465646279746573)
	round := func() {
		v0 += v1
		v2 += v3
		v1 = bits.RotateLeft64(v1, 13) ^ v0
		v3 = bits.RotateLeft64(v3, 16) ^ v2
		v0 = bits.RotateLeft64(v0, 32)
		v2 += v1
		v0 += v3
		v1 = bits.RotateLeft64(v1, 17) ^ v2
		v3 = bits.RotateLeft64(v3, 21) ^ v0
		v2 = bits.RotateLeft64(v2, 32)
	}
	b := uint64(len(in)) << 56
	for len(in) >= 8 {
		m := binary.LittleEndian.Uint64(in)
		in = in[8:]
		v3 ^= m
		round()
		v0 ^= m
	}
	var tail [8]byte
	copy(tail[:], in)
	b |= binary.LittleEndian.Uint64(tail[:])
	v3 ^= b
	round()
	v0 ^= b
	v2 ^= 0xff
	round()
	round()
	round()
	return v0 ^ v1 ^ v2 ^ v3
}

const (
	setLinearProbes = 9
	setPerturbShift = 5
)

type setSlot struct {
	key  string
	hash uint64
	used bool
}

// pySetOrder returns the distinct items in the order `for x in set(items)`
// visits them (set_add_entry, set_table_resize, set_insert_clean).
func pySetOrder(items []string) []string {
	table := make([]setSlot, 8)
	used := 0
	for _, it := range items {
		h := uint64(pyStrHash(it))
		if setFind(table, it, h) {
			continue
		}
		setInsert(table, it, h)
		used++
		mask := len(table) - 1
		if used*5 >= mask*3 {
			table = setResize(table, used)
		}
	}
	out := make([]string, 0, used)
	for _, s := range table {
		if s.used {
			out = append(out, s.key)
		}
	}
	return out
}

// setProbe visits the slots CPython probes for hash h until visit returns
// true (the slot sequence is the same for lookups and clean inserts when
// the table has no deleted entries).
func setProbe(table []setSlot, h uint64, visit func(i int) bool) {
	mask := uint64(len(table) - 1)
	i := h & mask
	perturb := h
	for {
		n := uint64(0)
		if i+setLinearProbes <= mask {
			n = setLinearProbes
		}
		for j := uint64(0); j <= n; j++ {
			if visit(int(i + j)) {
				return
			}
		}
		perturb >>= setPerturbShift
		i = (i*5 + 1 + perturb) & mask
	}
}

func setFind(table []setSlot, key string, h uint64) bool {
	found := false
	setProbe(table, h, func(i int) bool {
		s := table[i]
		if !s.used {
			return true
		}
		if s.hash == h && s.key == key {
			found = true
			return true
		}
		return false
	})
	return found
}

func setInsert(table []setSlot, key string, h uint64) {
	setProbe(table, h, func(i int) bool {
		if table[i].used {
			return false
		}
		table[i] = setSlot{key: key, hash: h, used: true}
		return true
	})
}

// setResize is set_table_resize(so, used*4) (used*2 above 50000 entries).
func setResize(old []setSlot, used int) []setSlot {
	minused := used * 4
	if used > 50000 {
		minused = used * 2
	}
	size := 8
	for size <= minused {
		size <<= 1
	}
	table := make([]setSlot, size)
	for _, s := range old {
		if s.used {
			setInsert(table, s.key, s.hash)
		}
	}
	return table
}
