package hdbread

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Expected strings are Python 3 repr() output.
func TestPyRepr(t *testing.T) {
	cases := map[float64]string{0: "0.0", 1: "1.0", 0.5: "0.5", 1e-4: "0.0001", 1.5e-4: "0.00015",
		1e-5: "1e-05", 123.456: "123.456", 1e15: "1000000000000000.0", 1e16: "1e+16",
		9999999999999998.0: "9999999999999998.0", math.MaxFloat64: "1.7976931348623157e+308",
		5e-324: "5e-324", 2.5e-10: "2.5e-10", 100: "100.0", -12345.678: "-12345.678", 0.1: "0.1",
		1.0 / 3: "0.3333333333333333", math.Inf(1): "inf", math.Inf(-1): "-inf"}
	for v, want := range cases {
		if got := pycompat.FloatRepr(v); got != want {
			t.Errorf("pycompat.FloatRepr(%v) = %q, want %q", v, got, want)
		}
	}
	if got := pycompat.FloatRepr(math.Copysign(0, -1)); got != "-0.0" {
		t.Errorf("pycompat.FloatRepr(-0) = %q", got)
	}
	if got := pycompat.JSONFloat(math.NaN()); got != "NaN" {
		t.Errorf("pycompat.JSONFloat(NaN) = %q", got)
	}
}

func TestDecodeReplaceMaximalSubpart(t *testing.T) {
	cases := map[string]string{"\xe2\x82A": "�A", "\xf0\x9f\x98": "�", "\xc0\xaf": "��",
		"\xf4\x90\x80\x80": "����", "ok\xffz": "ok�z", "\xe0\x80\x80": "���",
		"caf\xc3\xa9": "café"}
	for in, want := range cases {
		if got := pycompat.DecodeReplace([]byte(in)); got != want {
			t.Errorf("pycompat.DecodeReplace(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestShortFloat32(t *testing.T) {
	if got := shortFloat32(math.Float32bits(36.0813064575)); got != 36.081306 {
		t.Errorf("got %v, want 36.081306", got)
	}
	if got := shortFloat32(math.Float32bits(20.25)); got != 20.25 {
		t.Errorf("got %v, want 20.25", got)
	}
}

func TestIsoUTC(t *testing.T) {
	for ms, want := range map[int64]string{1537983000008: "2018-09-26T17:30:00.008Z",
		-86400001: "1969-12-30T23:59:59.999Z", -62135596800000: "0001-01-01T00:00:00.000Z"} {
		if got, err := isoUTC(ms); err != nil || got != want {
			t.Errorf("isoUTC(%d) = %q, %v; want %q", ms, got, err, want)
		}
	}
	if _, err := isoUTC(1<<48 - 1); err == nil {
		t.Error("a year past 9999 must be an error (Python raises OverflowError)")
	}
}

func TestValidModifiedUTF8(t *testing.T) {
	for in, want := range map[string]bool{"abc": true, "\xc0\x80x": true, "\xed\xa0\x80": true,
		"\xc0\xaf": false, "\xff": false, "\xe2\x82": false} {
		if got := validModifiedUTF8([]byte(in)); got != want {
			t.Errorf("validModifiedUTF8(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestOpAtFraming(t *testing.T) {
	cases := []struct {
		in      string
		op      string
		counted bool
		ok      bool
	}{
		{"\x00\x07Changed\x00\x05/a/bc", "Changed", true, true},
		{"\x00\x07Changed\x01\x00/", "Changed", true, true}, // a 256-byte target length
		{"\x00\x05Changed\x00\x05/a", "Changed", false, true},
		{"\x00\x07Changed\x00\x00/", "", false, false}, // zero-length target
		{"\x00\x07Changed\x10\x00/", "", false, false}, // 4096+ target
		{"\x00\x07changed\x00\x05/", "", false, false},
		{"\x00\x07Changed\x00\x05x", "", false, false},
	}
	for _, c := range cases {
		op, counted, ok := opAt([]byte(c.in), 0)
		if op != c.op || counted != c.counted || ok != c.ok {
			t.Errorf("opAt(%q) = %q,%v,%v want %q,%v,%v", c.in, op, counted, ok, c.op, c.counted, c.ok)
		}
	}
}

// A version-2 record whose 8-byte timestamp form runs past the data section is an exit-1 error,
// not a panic (Python raises IndexError/struct.error there).
func TestRecordPastSectionIsAnError(t *testing.T) {
	sect := make([]byte, 8)
	if _, err := decodeRecord(sect, 0, 2, 4, "history:NumericTrendRecord"); err == nil {
		t.Fatal("want an error for a record past the section")
	}
	binary.BigEndian.PutUint32(sect, 0x01000000)
	if _, err := decodeRecord(sect, 6, 2, 4, "history:NumericTrendRecord"); err == nil {
		t.Fatal("want an error when the status bytes are missing")
	}
}

func TestRunMissingFileIsExit2(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run(Options{Path: "testdata/no-such.hdb"}, cli.Stdio{Out: &out, Err: &errb})
	if code != cli.ExitCannotSearch || !strings.Contains(errb.String(), "No such file or directory") {
		t.Fatalf("code %d, stderr %q", code, errb.String())
	}
}
