package hdbread

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// Expected strings are Python 3 repr() output.
func TestPyRepr(t *testing.T) {
	cases := map[float64]string{0: "0.0", 1: "1.0", 0.5: "0.5", 1e-4: "0.0001", 1.5e-4: "0.00015",
		1e-5: "1e-05", 123.456: "123.456", 1e15: "1000000000000000.0", 1e16: "1e+16",
		9999999999999998.0: "9999999999999998.0", math.MaxFloat64: "1.7976931348623157e+308",
		5e-324: "5e-324", 2.5e-10: "2.5e-10", 100: "100.0", -12345.678: "-12345.678", 0.1: "0.1",
		1.0 / 3: "0.3333333333333333", math.Inf(1): "inf", math.Inf(-1): "-inf"}
	for v, want := range cases {
		if got := pyRepr(v); got != want {
			t.Errorf("pyRepr(%v) = %q, want %q", v, got, want)
		}
	}
	if got := pyRepr(math.Copysign(0, -1)); got != "-0.0" {
		t.Errorf("pyRepr(-0) = %q", got)
	}
	if got := jsonFloat(math.NaN()); got != "NaN" {
		t.Errorf("jsonFloat(NaN) = %q", got)
	}
}

func TestDecodeReplaceMaximalSubpart(t *testing.T) {
	cases := map[string]string{"\xe2\x82A": "�A", "\xf0\x9f\x98": "�", "\xc0\xaf": "��",
		"\xf4\x90\x80\x80": "����", "ok\xffz": "ok�z", "\xe0\x80\x80": "���",
		"caf\xc3\xa9": "café"}
	for in, want := range cases {
		if got := decodeReplace([]byte(in)); got != want {
			t.Errorf("decodeReplace(%q) = %q, want %q", in, got, want)
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

func TestMask(t *testing.T) {
	for in, want := range map[string]string{"admin": "admin", "jdoe": "jdoe",
		"QWxhZGRpbjpvcGVuIHNlc2FtZQ==": "<VALUE-MASKED>", "adminadminadminadmin": "<VALUE-MASKED>"} {
		if got := mask(in); got != want {
			t.Errorf("mask(%q) = %q, want %q", in, got, want)
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

// --strings --mask must not print any user identity: the userName of every framed audit record
// and the identity fields of a login-family record (here jdoe, admin, guest, station, root and the
// "bad" login attempt), role accounts included. Python's mask() let all of them through.
func TestStringsMaskHidesUserIdentities(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run(Options{Path: "testdata/inputs/audit.hdb", Strings: true, Mask: true},
		cli.Stdio{Out: &out, Err: &errb})
	if code != cli.ExitFound {
		t.Fatalf("code %d, stderr %q", code, errb.String())
	}
	lines := strings.Split(out.String(), "\n")
	for _, id := range []string{"jdoe", "admin", "guest", "station", "root", "bad"} {
		for _, l := range lines {
			if strings.Contains(l, id) {
				t.Errorf("identity %q leaks under --mask: line %q", id, l)
			}
		}
	}
	if n := strings.Count(out.String(), userMasked+"\n"); n != 11 {
		t.Errorf("want 11 %s lines (one per identity field), got %d:\n%s", userMasked, n, out.String())
	}
	// Non-identity strings stay readable: operation names and slot names.
	for _, keep := range []string{"Login Failure\n", "Logout (Timeout)\n", "setpoint\n"} {
		if !strings.Contains(out.String(), keep) {
			t.Errorf("%q was masked but is not an identity", keep)
		}
	}
}

func TestIdentityRangesFraming(t *testing.T) {
	utf := func(s string) string { return string([]byte{byte(len(s) >> 8), byte(len(s))}) + s }
	rec := utf("Login Failure") + utf("x") + utf("") + utf("") + utf("eve") + utf("root") +
		utf("Changed") + utf("/a") + utf("out") + utf("1") + utf("2") + utf("ann")
	got := planMask([]byte(rec)).ids
	var names []string
	for _, r := range got {
		names = append(names, rec[r[0]:r[1]])
	}
	if want := "x,eve,root,ann"; strings.Join(names, ",") != want {
		t.Fatalf("identity fields = %q, want %q", strings.Join(names, ","), want)
	}
}

// Rule v2 (fail-closed): once a file has framed audit records, --mask must not let a user name
// through an audit record the framing rejected. Each case pairs one valid record with a damaged
// one whose userName is "mallory".
func TestStringsMaskFailsClosedOnUnframedRecords(t *testing.T) {
	utf := func(s string) string { return string([]byte{byte(len(s) >> 8), byte(len(s))}) + s }
	valid := utf("Changed") + utf("/a/b") + utf("out") + utf("1.0") + utf("2.0") + utf("ann")
	cases := map[string]string{
		"truncated trailing record": utf("Changed") + utf("/a/c") + utf("out") + utf("1.0") + utf("2.0") +
			"\x00\x10mallory",
		"bad modified UTF-8": utf("Changed") + utf("/a/c") + utf("o\xffut") + utf("1.0") + utf("2.0") +
			utf("mallory"),
		"unknown operation": utf("Frobnicated") + utf("xyz") + utf("out") + utf("1.0") + utf("2.0") +
			utf("mallory"),
	}
	for name, damaged := range cases {
		lines := stringsLines([]byte(valid+damaged), true)
		joined := strings.Join(lines, "\n")
		for _, id := range []string{"mallory", "ann"} {
			if strings.Contains(joined, id) {
				t.Errorf("%s: %q leaks under --mask:\n%s", name, id, joined)
			}
		}
		if !strings.Contains(joined, valueMasked) {
			t.Errorf("%s: want the unframed text as %s:\n%s", name, valueMasked, joined)
		}
		// Text inside the framed record keeps the old rule.
		if lines[0] != "Changed" || lines[1] != "/a/b" || lines[2] != "out" {
			t.Errorf("%s: framed record text changed: %q", name, lines[:3])
		}
	}
}

// Outside framed records, an exact operation name of the vocabulary or an exact role account
// still prints; with no framed record at all the old rule applies unchanged.
func TestStringsMaskUnframedExceptions(t *testing.T) {
	utf := func(s string) string { return string([]byte{byte(len(s) >> 8), byte(len(s))}) + s }
	valid := utf("Changed") + utf("/a/b") + utf("out") + utf("1.0") + utf("2.0") + utf("ann")
	tail := "\x00\x01Logout\x00\x01admin\x00\x01Removed\x00\x01Changedx\x00\x01admins"
	got := strings.Join(stringsLines([]byte(valid+tail), true)[6:], ",")
	if want := "Logout,admin,Removed,<VALUE-MASKED>,<VALUE-MASKED>"; got != want {
		t.Errorf("unframed runs = %q, want %q", got, want)
	}
	got = strings.Join(stringsLines([]byte("\x00\x01mallory\x00\x01QWxhZGRpbjpvcGVuIHNlc2FtZQ=="), true), ",")
	if want := "mallory,<VALUE-MASKED>"; got != want {
		t.Errorf("no framed record: %q, want the old rule %q", got, want)
	}
}
