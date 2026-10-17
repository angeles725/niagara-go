package slotomatic

import (
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Flag is one javax.baja.sys.Flags bit and its slot-sheet letter.
type Flag struct {
	Name   string
	Bit    int64
	Letter string
}

// Flags is javax.baja.sys.Flags (docSource baja/javax/baja/sys/Flags.java:179-198, B1169/B1174),
// in bit order.
var Flags = []Flag{
	{"READONLY", 0x00000001, "r"},
	{"TRANSIENT", 0x00000002, "t"},
	{"HIDDEN", 0x00000004, "h"},
	{"SUMMARY", 0x00000008, "s"},
	{"ASYNC", 0x00000010, "a"},
	{"NO_RUN", 0x00000020, "n"},
	{"DEFAULT_ON_CLONE", 0x00000040, "d"},
	{"CONFIRM_REQUIRED", 0x00000080, "c"},
	{"OPERATOR", 0x00000100, "o"},
	{"EXECUTE_ON_CHANGE", 0x00000200, "x"},
	{"FAN_IN", 0x00000400, "f"},
	{"NO_AUDIT", 0x00000800, "A"},
	{"COMPOSITE", 0x00001000, "p"},
	{"REMOVE_ON_CLONE", 0x00002000, "R"},
	{"METADATA", 0x00004000, "m"},
	{"LINK_TARGET", 0x00008000, "L"},
	{"NON_CRITICAL", 0x00010000, "N"},
	{"USER_DEFINED_1", 0x10000000, "1"},
}

var (
	intCastRE = Compile(`^\(\s*int\s*\)\s*`)
	flagRE    = Compile(`^(?:javax\.baja\.sys\.)?Flags\.([A-Z_0-9]+)$`)
	decRE     = Compile(`^-?\d+$`)
	hexRE     = Compile(`^0[xX][0-9a-fA-F]+$`)
)

// DecodeFlags is decode_flags: the value and letters of `Flags.A | Flags.B`, `(int)1026`, `0`
// or `0x10`. Unknown terms are ignored.
func DecodeFlags(expr string) (int64, []string) {
	var total int64
	for _, part := range strings.Split(expr, "|") {
		tok := pycompat.Strip(intCastRE.ReplaceAllLiteralString(pycompat.Strip(part), ""))
		if tok == "" {
			continue
		}
		if m := flagRE.FindStringSubmatch(tok); m != nil {
			if bit, ok := flagBit(m[1]); ok {
				total |= bit
				continue
			}
		}
		switch {
		case decRE.MatchString(tok):
			v, _ := pycompat.ParseInt(tok) // int(tok); beyond int64 it adds nothing
			total |= v
		case hexRE.MatchString(tok):
			v, _ := strconv.ParseInt(tok[2:], 16, 64)
			total |= v
		}
	}
	letters := []string{}
	for _, f := range Flags {
		if total&f.Bit != 0 {
			letters = append(letters, f.Letter)
		}
	}
	return total, letters
}

func flagBit(name string) (int64, bool) {
	for _, f := range Flags {
		if f.Name == name {
			return f.Bit, true
		}
	}
	return 0, false
}
