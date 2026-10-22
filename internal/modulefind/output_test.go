package modulefind

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

// The expected outputs are module-find.json of the Python golden (CSV with the CRLF
// csv.writer writes; the golden harness reads it in text mode, as "\n").

func runCmd(t *testing.T, o opts, fn func(m *module, o *opts)) (string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	o.out, o.err = &out, &errb
	fn(newModule("testdata/srcA"), &o)
	return out.String(), errb.String()
}

func TestRenderFormats(t *testing.T) {
	csv, _ := runCmd(t, opts{csv: true}, func(m *module, o *opts) { cmdSlots(m, o, slotFilter{}) })
	wantCSV := "class,slot,type,flags,min,complex\r\n" +
		"BColdRoom,setpoint,baja:StatusNumeric,Flags.SUMMARY | Flags.OPERATOR,-40.0,True\r\n" +
		"BColdRoom,mode,baja:String,Flags.OPERATOR,,False\r\n" +
		"BRoomPanel,temp,BStatusNumeric,Flags.OPERATOR,,True\r\n" +
		"BRoomPanel,label,baja:String,Flags.READONLY | Flags.SUMMARY,,False\r\n" +
		"BRoomPanel,cache,baja:Integer,Flags.TRANSIENT | Flags.HIDDEN,,False\r\n" +
		"BRoomPanel,untyped,,,,False\r\n"
	if csv != wantCSV {
		t.Errorf("csv:\n%q\nwant\n%q", csv, wantCSV)
	}
	js, stderr := runCmd(t, opts{json: true, limit: 1}, func(m *module, o *opts) { cmdActions(m, o, nil) })
	wantJSON := "[\n  {\n    \"class\": \"BColdRoom\",\n    \"action\": \"applySetpoint\",\n    \"flags\": \"o\"\n  }\n]\n"
	if js != wantJSON || stderr != "... 2 more rows (raise --limit)\n" {
		t.Errorf("json: %q / %q", js, stderr)
	}
	text, _ := runCmd(t, opts{limit: 1}, func(m *module, o *opts) { cmdOrds(m, o, nil) })
	if text != "BColdRoom  station:|slot:/Drivers\n... 3 more rows (raise --limit)\n" {
		t.Errorf("text cap: %q", text)
	}
}

func TestJSONDumpsEnsureASCII(t *testing.T) {
	if got := jsonDumps("a\"\\\n\x7fñ😀\xffabc"); got != `"a\"\\\n\u007f\u00f1\ud83d\ude00\ufffdabc"` {
		t.Errorf("jsonDumps = %q", got)
	}
}

// failAfter accepts n writes, then refuses every write (a closed pipe, a full disk).
type failAfter struct {
	n      int
	writes int
}

var errWrite = errors.New("no space left on device")

func (f *failAfter) Write(p []byte) (int, error) {
	f.writes++
	if f.writes > f.n {
		return 0, errWrite
	}
	return len(p), nil
}

// TestRenderCSVStopsAtAWriteError: a CSV row that cannot be written stops the output (no
// further row is tried); the error is kept for Main to report.
func TestRenderCSVStopsAtAWriteError(t *testing.T) {
	out := &failAfter{n: 1} // the header is written, the first row fails
	stdout := &stickyWriter{w: out}
	cmdSlots(newModule("testdata/srcA"), &opts{csv: true, out: stdout, err: io.Discard}, slotFilter{})
	if out.writes != 2 || !errors.Is(stdout.err, errWrite) {
		t.Errorf("%d writes, error %v; want 2 (the header, then the failed row stops the output) and %v",
			out.writes, stdout.err, errWrite)
	}
}
