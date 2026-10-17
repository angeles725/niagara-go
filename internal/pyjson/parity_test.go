package pyjson

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"strconv"
	"testing"
)

// testdata/pyparity.json was produced by Python 3.15 on curated and seeded
// random inputs: each row is [input, expected...] (expected "ERR:<str(exc)>"
// on an exception). The json rows are the plain UTF-8 subset of licensador's
// table (json.detect_encoding stays in licensador): [hex input,
// dumps(indent=2, separators=(",", ": ")), dumps(sort_keys, compact)].

func loadParity(t *testing.T) map[string][][]string {
	t.Helper()
	raw, err := os.ReadFile("testdata/pyparity.json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string][][]string
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestParityLoadsDumps(t *testing.T) {
	rows := loadParity(t)["json"]
	if len(rows) == 0 {
		t.Fatal("no json parity rows")
	}
	for _, row := range rows {
		in, err := hex.DecodeString(row[0])
		if err != nil {
			t.Fatalf("hex %s: %v", row[0], err)
		}
		indent, canon := "", ""
		v, err := Loads([]rune(string(in)))
		if err == nil {
			indent = Dumps(v, DumpOpts{EnsureASCII: true, Indent: 2, ItemSep: ",", KeySep: ": "})
			canon = Dumps(v, Canonical)
		} else {
			indent = "ERR:" + err.Error()
		}
		if indent != row[1] || canon != row[2] {
			t.Errorf("json %q:\n got %q %q\nwant %q %q", in, indent, canon, row[1], row[2])
		}
	}
}

func TestParityFloatRepr(t *testing.T) {
	for _, row := range loadParity(t)["float"] {
		f, err := strconv.ParseFloat(row[0], 64)
		if err != nil {
			t.Fatalf("hex float %s: %v", row[0], err)
		}
		if got := FloatRepr(f); got != row[1] {
			t.Errorf("repr(%s): got %s want %s", row[0], got, row[1])
		}
	}
}

func TestParityStrRepr(t *testing.T) {
	for _, row := range loadParity(t)["repr"] {
		if got := StrRepr(row[0]); got != row[1] {
			t.Errorf("repr(%q): got %s want %s", row[0], got, row[1])
		}
	}
}
