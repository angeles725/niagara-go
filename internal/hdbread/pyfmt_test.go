package hdbread

import (
	"bytes"
	"testing"
)

// newCSVWriter is csv.writer(..., lineterminator="\n"): rows end in "\n", not csv.writer's
// default "\r\n", with minimal quoting and an empty string as an empty field.
func TestNewCSVWriterTerminator(t *testing.T) {
	var out bytes.Buffer
	w := newCSVWriter(&out)
	w.WriteRow(stringRow("timestamp", "value"))
	w.WriteRow(stringRow("a,b", ""))
	if got, want := out.String(), "timestamp,value\n\"a,b\",\n"; got != want {
		t.Errorf("csv output %q, want %q", got, want)
	}
}
