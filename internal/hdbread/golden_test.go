package hdbread

import (
	"bytes"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/golden"
)

// optionsFromArgs maps the argv of a non-usage golden case onto Options. It is a test fixture,
// not a parser: usage, help and error text belong to internal/argparse.
func optionsFromArgs(t *testing.T, args []string) Options {
	t.Helper()
	var o Options
	flags := map[string]*bool{"--schema": &o.Schema, "--ops": &o.Ops, "--strings": &o.Strings,
		"--mask": &o.Mask, "--records": &o.Records, "--json": &o.JSON}
	for _, a := range args {
		if p, ok := flags[a]; ok {
			*p = true
			continue
		}
		if strings.HasPrefix(a, "-") || o.Path != "" {
			t.Fatalf("golden argv %q is not a plain case; name it usage-*", args)
		}
		o.Path = a
	}
	return o
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
			if code != c.Exit {
				t.Errorf("exit = %d, want %d", code, c.Exit)
			}
			if !bytes.Equal(out.Bytes(), c.Stdout) {
				t.Errorf("stdout: %s", golden.Diff(c.Stdout, out.Bytes()))
			}
			if !bytes.Equal(errb.Bytes(), c.Stderr) {
				t.Errorf("stderr: %s", golden.Diff(c.Stderr, errb.Bytes()))
			}
		})
	}
}
