package hdbread

import (
	"bytes"
	"errors"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// shortWriter accepts room bytes, then fails every write.
type shortWriter struct{ room int }

var errDiskFull = errors.New("disk full")

func (w *shortWriter) Write(p []byte) (int, error) {
	if len(p) > w.room {
		n := w.room
		w.room = 0
		return n, errDiskFull
	}
	w.room -= len(p)
	return len(p), nil
}

// A failed stdout write is reported once and ends --records with ExitNoMatch, before the summary.
func TestPrintVarRecordsReportsWriteErrors(t *testing.T) {
	header := len("timestamp,millis,logName,severity,level,message,exception\n")
	for _, tc := range []struct {
		name string
		o    Options
		room int
		want string
	}{
		{"csv header", Options{Records: true}, 0, "hdbread: --records: disk full\n"},
		{"csv row", Options{Records: true}, header, "hdbread: --records: disk full\n"},
		{"json", Options{Records: true, JSON: true}, 0, "hdbread: --records --json: disk full\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var errb bytes.Buffer
			s := cli.Stdio{Out: &shortWriter{room: tc.room}, Err: &errb}
			if code := printVarRecords(tc.o, decodeFixture(t, "log-v2.hdb"), s); code != cli.ExitNoMatch {
				t.Errorf("exit = %d, want %d", code, cli.ExitNoMatch)
			}
			if errb.String() != tc.want {
				t.Errorf("stderr = %q, want %q", errb.String(), tc.want)
			}
		})
	}
}

// maskRecord in a login-family record treats every field but the operation (and a '/' slot-path
// target) as the user's identity; an operation that is not an AuditEvent constant is free text.
func TestMaskRecordAudit(t *testing.T) {
	const v, u = valueMasked, userMasked
	for _, tc := range []struct {
		name     string
		in, want AuditRecord
	}{
		{"login fields are the user",
			AuditRecord{"Login", "alice-pc", "slot", "old", "new", "alice"},
			AuditRecord{"Login", u, u, u, u, u}},
		{"login failure, slot-path target",
			AuditRecord{"Login Failure", "/Services/UserService", "", "", "bob", "bob"},
			AuditRecord{"Login Failure", v, "", "", u, u}},
		{"logout (timeout), empty fields stay empty",
			AuditRecord{"Logout (Timeout)", "", "", "", "", "carol"},
			AuditRecord{"Logout (Timeout)", "", "", "", "", u}},
		{"unknown operation is masked as free text",
			AuditRecord{"Logged In As dave", "dave-pc", "slot", "old", "new", "dave"},
			AuditRecord{v, v, v, v, v, u}},
		{"known non-login operation",
			AuditRecord{"Changed", "/Lab1/Folder", "out", "1", "2", "erin"},
			AuditRecord{"Changed", v, v, v, v, u}},
		{"empty operation is not a constant",
			AuditRecord{"", "", "", "", "", ""},
			AuditRecord{v, "", "", "", "", ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := tc.in
			got := maskRecord(Record{Audit: &in})
			if *got.Audit != tc.want {
				t.Errorf("masked = %+v\nwant     %+v", *got.Audit, tc.want)
			}
			if in != tc.in {
				t.Errorf("maskRecord changed its input: %+v", in)
			}
		})
	}
}

func TestMaskRecordLog(t *testing.T) {
	in := LogRecord{LogName: "sys", Severity: 1000, Level: "SEVERE", Message: "user alice", Exception: ""}
	got := maskRecord(Record{Log: &in}).Log
	want := LogRecord{LogName: valueMasked, Severity: 1000, Level: "SEVERE", Message: valueMasked}
	if *got != want || in.Message != "user alice" {
		t.Errorf("masked = %+v (input %+v)", *got, in)
	}
}
