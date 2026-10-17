package pxrender

import (
	"bytes"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/golden"
)

// TestGolden runs every parity case (usage errors included) through Main.
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }

// intended lists the cases where Python ends in a traceback; Go prints one line instead.
var intended = map[string]struct {
	exit   int
	stderr string
}{
	"malformed-xml": {2, "px-render: cannot parse inputs/station/shared/px/malformed.px: " +
		"XML syntax error on line 1: element <CanvasPane> closed by </px>\n"},
	"missing-px": {2, "px-render: cannot read inputs/station/shared/px/nope.px: No such file or directory\n"},
}

// TestIntendedDiffs runs testdata/intended/golden: stdout must match Python; stderr and exit are Go's.
func TestIntendedDiffs(t *testing.T) {
	cases, err := golden.Load("testdata/intended")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir("testdata")
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			d, ok := intended[c.Name]
			if !ok {
				t.Fatalf("no intended result recorded for %s", c.Name)
			}
			var out, errb bytes.Buffer
			code := Main(c.Args, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
			if code != d.exit {
				t.Errorf("exit = %d, want %d", code, d.exit)
			}
			if !bytes.Equal(out.Bytes(), c.Stdout) {
				t.Errorf("stdout: %s", golden.Diff(c.Stdout, out.Bytes()))
			}
			if errb.String() != d.stderr {
				t.Errorf("stderr: %s", golden.Diff([]byte(d.stderr), errb.Bytes()))
			}
		})
	}
}
