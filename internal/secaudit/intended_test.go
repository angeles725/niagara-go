package secaudit

import (
	"reflect"
	"testing"
)

// The intended differences from tools/lib/winacl.py (tooling-round3 open follow-ups). Python's
// behaviour is stated per test; Go deliberately differs.

// Python reads (DENY) as one more rights group, so `Everyone:(DENY)(W)` counted as a write grant
// and SEC-03 FAILed a folder that denies writes. A deny ACE grants nothing.
func TestDenyACEIsNotAGrant(t *testing.T) {
	listing := testWinpath + " NT AUTHORITY\\SYSTEM:(OI)(CI)(F)\r\n" +
		"             Everyone:(DENY)(W)\r\n" +
		"             BUILTIN\\Users:(OI)(CI)(DENY)(M)\r\n" +
		"             NT AUTHORITY\\Authenticated Users:(OI)(CI)(RX)\r\n"
	if got := loose(t, listing); len(got) != 0 {
		t.Errorf("deny ACEs read as grants: %q", got)
	}
	r := sec03Row(t, fakeEnv([]byte(listing), nil, 0, nil))
	if r.verdict != "PASS" || r.observed != "restricted" {
		t.Errorf("SEC-03 = %s %q, want PASS restricted", r.verdict, r.observed)
	}
	mixed := testWinpath + " Everyone:(DENY)(W)\r\n             Everyone:(M)\r\n"
	if got := loose(t, mixed); !reflect.DeepEqual(got, []string{"Everyone"}) {
		t.Errorf("a grant next to a deny: %q", got)
	}
}

// Python leaves MA (MAXIMUM_ALLOWED) out of the write rights, so a broad group holding it passed.
// MA grants every right the object allows, writes included.
func TestMaximumAllowedIsAWriteRight(t *testing.T) {
	for _, line := range []string{"Everyone:(MA)", "BUILTIN\\Users:(OI)(CI)(MA)", "Todos:(Rc,S,MA)"} {
		if got := loose(t, testWinpath+" "+line+"\r\n"); len(got) != 1 {
			t.Errorf("%s: loose = %q, want the group", line, got)
		}
	}
}

// Python decodes cp1252 with errors="replace": the five bytes cp1252 leaves undefined (0x81, 0x8d,
// 0x8f, 0x90, 0x9d) became U+FFFD and the listing lost them. Windows maps them to the C1 controls
// of the same value, so the cp1252 decoding keeps every byte.
func TestCp1252DecodingIsLossless(t *testing.T) {
	for _, b := range []byte{0x81, 0x8d, 0x8f, 0x90, 0x9d} {
		data := append(encode(t, "José Línea ", "cp1252"), b)
		if got, want := decodeConsole(data), "José Línea "+string(rune(b)); got != want {
			t.Errorf("decodeConsole = %q, want %q", got, want)
		}
	}
	for _, cp := range codepages {
		seen := map[rune]bool{}
		for _, r := range cp.upper {
			if r == '\ufffd' || seen[r] {
				t.Errorf("%s does not decode every byte to a distinct character (%U)", cp.name, r)
			}
			seen[r] = true
		}
	}
}
