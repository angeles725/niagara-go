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
