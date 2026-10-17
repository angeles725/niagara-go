package pxrender

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/golden"
)

// optionsFromArgs maps the argv of a non-usage golden case onto Options (a test fixture, not a
// parser: usage, help and error text belong to internal/argparse).
func optionsFromArgs(t *testing.T, args []string) Options {
	t.Helper()
	o := Options{OnRatio: 0.7}
	str := map[string]*string{"--shared": &o.Shared, "--organized": &o.Organized, "--out": &o.Out,
		"--seed": &o.Seed, "--title": &o.Title, "--subtitle": &o.Subtitle}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if p, ok := str[a]; ok && i+1 < len(args) {
			*p = args[i+1]
			i++
			continue
		}
		if a == "--on-ratio" && i+1 < len(args) {
			v, err := strconv.ParseFloat(args[i+1], 64)
			if err != nil {
				t.Fatalf("golden --on-ratio %q", args[i+1])
			}
			o.OnRatio = v
			i++
			continue
		}
		if strings.HasPrefix(a, "-") || o.Px != "" {
			t.Fatalf("golden argv %q is not a plain case; name it usage-*", args)
		}
		o.Px = a
	}
	return o
}

// intended lists the cases where Python ends in a traceback; Go prints one line instead.
var intended = map[string]struct {
	exit   int
	stderr string
}{
	"malformed-xml": {2, "px-render: cannot parse inputs/station/shared/px/malformed.px: " +
		"XML syntax error on line 1: element <CanvasPane> closed by </px>\n"},
	"missing-px": {2, "px-render: cannot read inputs/station/shared/px/nope.px: No such file or directory\n"},
}

// TestGolden checks Run against the frozen Python cases at the logic level (options already
// parsed). Once Main exists (internal/argparse), golden.Run(t, "testdata", Main) replaces it.
func TestGolden(t *testing.T) {
	cases, err := golden.Load("testdata")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir("testdata")
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if strings.HasPrefix(c.Name, "usage-") {
				t.Skip("awaiting internal/argparse")
			}
			var out, errb bytes.Buffer
			code := Run(optionsFromArgs(t, c.Args), cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
			wantExit, wantErr := c.Exit, c.Stderr
			if d, ok := intended[c.Name]; ok {
				wantExit, wantErr = d.exit, []byte(d.stderr)
			}
			if code != wantExit {
				t.Errorf("exit = %d, want %d", code, wantExit)
			}
			if !bytes.Equal(out.Bytes(), c.Stdout) {
				t.Errorf("stdout: %s", golden.Diff(c.Stdout, out.Bytes()))
			}
			if !bytes.Equal(errb.Bytes(), wantErr) {
				t.Errorf("stderr: %s", golden.Diff(wantErr, errb.Bytes()))
			}
		})
	}
}
