package hdbread

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func decodeFixture(t *testing.T, name string) *History {
	t.Helper()
	h, err := DecodeRecords(fixtures()[name])
	if err != nil {
		t.Fatalf("DecodeRecords(%s): %v", name, err)
	}
	return h
}

func TestDecodeLogRecords(t *testing.T) {
	h := decodeFixture(t, "log-v2.hdb")
	if h.ID != "/S/LogHistory" || h.RecordType != "history:LogRecord" || h.Version != 2 || h.Declared != 3 {
		t.Fatalf("history = %+v", h)
	}
	want := []LogRecord{
		{LogName: "sys.engine", Severity: 800, Level: "INFO", Message: "Station started"},
		{LogName: "trafficLightDriver", Severity: 1000, Level: "SEVERE", Message: "poll failed, retrying \"dev1\"",
			Exception: "java.io.IOException: timeout\n\tat a.B.c(B.java:1)\n\tat a.B.d(B.java:2)"},
		{LogName: "custom", Severity: 850, Level: "850", Message: "café \u0000 \U0001F600"},
	}
	if len(h.Records) != len(want) {
		t.Fatalf("%d records, want %d", len(h.Records), len(want))
	}
	for i, r := range h.Records {
		if r.Kind() != KindLog || r.Log == nil || r.Audit != nil || r.Trend != nil {
			t.Fatalf("record %d: kind %q, variants %v %v %v", i, r.Kind(), r.Log, r.Audit, r.Trend)
		}
		if *r.Log != want[i] {
			t.Errorf("record %d = %+v\nwant %+v", i, *r.Log, want[i])
		}
	}
	if h.Records[0].Timestamp != "2026-10-16T18:33:00.000Z" || h.Records[1].Millis != t0+1500 {
		t.Errorf("timestamps %q %d", h.Records[0].Timestamp, h.Records[1].Millis)
	}
}

// The walk starts at the header's first page and that page's first block, wraps at the page
// count and stops after the last page.
func TestDecodeAuditRingInCursorOrder(t *testing.T) {
	h := decodeFixture(t, "audit-ring.hdb")
	var got []AuditRecord
	for _, r := range h.Records {
		if r.Kind() != KindAudit {
			t.Fatalf("kind %q", r.Kind())
		}
		got = append(got, *r.Audit)
	}
	want := []AuditRecord{
		{Operation: "Changed", Target: "/Test/Ramp", SlotName: "amplitude", OldValue: "1.0", Value: "2.5", UserName: "admin"},
		{Operation: "Invoked", Target: "/Test/Ramp", SlotName: "set", UserName: "operator, one"},
		{Operation: "Login", UserName: "admin"},
		{Operation: "Flags Changed", Target: "/Test/Limiter/out", SlotName: "out", Value: "hidden,summary", UserName: "admin"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("records\n got %+v\nwant %+v", got, want)
	}
	if h.Declared != 4 || h.Records[0].Millis != t0-60000 {
		t.Errorf("declared %d, first millis %d", h.Declared, h.Records[0].Millis)
	}
}

func TestDecodeVersion1Timestamps(t *testing.T) {
	h := decodeFixture(t, "audit-v1.hdb")
	if len(h.Records) != 1 || h.Records[0].Millis != t0 || h.Records[0].Audit.Operation != "Removed" {
		t.Fatalf("records %+v", h.Records)
	}
}

func TestDecodeEmptyStore(t *testing.T) {
	h := decodeFixture(t, "log-empty.hdb")
	if len(h.Records) != 0 || h.Declared != 0 {
		t.Fatalf("history %+v", h)
	}
}

func TestDecodeRejectsMalformedStores(t *testing.T) {
	good := fixtures()["audit-v1.hdb"]
	section := func(f func(sect []byte)) []byte {
		d := append([]byte(nil), good...)
		f(d[1600:])
		return d
	}
	cases := []struct {
		name, want string
		data       []byte
	}{
		{"overrun", "runs past its 1 block", fixtures()["audit-overrun.hdb"]},
		{"magic", "record-store magic", section(func(s []byte) { s[0] = 0 })},
		{"short", "no record-store header", good[:1600+20]},
		{"block size", "block size", section(func(s []byte) { s[11] = 8 })},
		{"first page", "outside the 1 pages", section(func(s []byte) { s[19] = 5 })},
		{"continuation", "continues a record", section(func(s []byte) { s[256+12+7] = 1 })},
		{"blocks", "declares 9 blocks", section(func(s []byte) { s[256+12+3] = 9 })},
		{"utf", "modified UTF-8", section(func(s []byte) { s[256+12+8+8+2] = 0xff })},
		{"part", "expected part 1/2", section(func(s []byte) { s[256+12+3] = 2 })},
		{"unused block", "declares 2 blocks but its fields use 1", section(func(s []byte) {
			s[256+12+3], s[256+12+64+3], s[256+12+64+7] = 2, 2, 1
		})},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := DecodeRecords(c.data)
			var fe FormatError
			if !errors.As(err, &fe) || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want a FormatError containing %q", err, c.want)
			}
		})
	}
}

func TestDecodeRefusesOtherVariableRecords(t *testing.T) {
	_, err := DecodeRecords(fixtures()["security-type.hdb"])
	if err == nil || !strings.Contains(err.Error(), "history:SecurityAuditRecord") {
		t.Fatalf("err = %v", err)
	}
}

// Trend histories come back through the same API with the Trend variant.
func TestReadRecordsTrend(t *testing.T) {
	h, err := ReadRecords(filepath.Join("testdata", "inputs", "numeric-v2.hdb"))
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Records) == 0 || h.Records[0].Kind() != KindTrend {
		t.Fatalf("history %+v", h)
	}
	if _, ok := h.Records[0].Trend.Value.(float64); !ok || h.Records[0].Trend.Status == "" {
		t.Fatalf("trend record %+v", h.Records[0].Trend)
	}
}

func TestReadRecordsMissingFile(t *testing.T) {
	if _, err := ReadRecords(filepath.Join(t.TempDir(), "none.hdb")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v", err)
	}
}

func TestReadUTFRejectsJavaInvalidBytes(t *testing.T) {
	for _, b := range []string{"\x00\x01\x80", "\x00\x02\xc3\x28", "\x00\x01\xc3", "\x00\x01\xf0"} {
		r := fieldReader{b: []byte(b)}
		r.utf()
		if r.err == nil {
			t.Errorf("readUTF(%q) accepted", b)
		}
	}
	r := fieldReader{b: []byte("\x00\x06\xed\xa0\xbd\xed\xb8\x80")} // a surrogate pair
	if s := r.utf(); r.err != nil || s != "\U0001F600" {
		t.Errorf("surrogate pair = %q, %v", s, r.err)
	}
	r = fieldReader{b: []byte("\x00\x03\xed\xa0\xbd")} // a lone high surrogate
	if s := r.utf(); r.err != nil || s != "�" {
		t.Errorf("lone surrogate = %q, %v", s, r.err)
	}
}
