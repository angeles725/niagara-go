package hdbread

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// goldenCase is one frozen Python run: testdata/golden/<name>/{cmd.json,stdout,stderr,exit}.
type goldenCase struct {
	name   string
	args   []string
	stdout []byte
	stderr []byte
	exit   int
}

func loadGolden(t *testing.T) []goldenCase {
	t.Helper()
	dirs, err := filepath.Glob(filepath.Join("testdata", "golden", "*"))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no golden cases under testdata/golden (%v)", err)
	}
	var cases []goldenCase
	for _, d := range dirs {
		var cmd struct {
			Args []string `json:"args"`
		}
		raw, err := os.ReadFile(filepath.Join(d, "cmd.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &cmd); err != nil {
			t.Fatalf("%s: %v", d, err)
		}
		c := goldenCase{name: filepath.Base(d), args: cmd.Args}
		if c.stdout, err = os.ReadFile(filepath.Join(d, "stdout")); err != nil {
			t.Fatal(err)
		}
		if c.stderr, err = os.ReadFile(filepath.Join(d, "stderr")); err != nil {
			t.Fatal(err)
		}
		code, err := os.ReadFile(filepath.Join(d, "exit"))
		if err != nil {
			t.Fatal(err)
		}
		if c.exit, err = strconv.Atoi(strings.TrimSpace(string(code))); err != nil {
			t.Fatalf("%s/exit: %v", d, err)
		}
		cases = append(cases, c)
	}
	return cases
}

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

func TestGolden(t *testing.T) {
	for _, c := range loadGolden(t) {
		t.Run(c.name, func(t *testing.T) {
			if strings.HasPrefix(c.name, "usage-") {
				t.Skip("awaiting internal/argparse")
			}
			var out, errb bytes.Buffer
			code := Run(optionsFromArgs(t, c.args), cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
			if code != c.exit {
				t.Errorf("exit = %d, want %d", code, c.exit)
			}
			if !bytes.Equal(out.Bytes(), c.stdout) {
				t.Errorf("stdout differs\n got: %q\nwant: %q", out.String(), c.stdout)
			}
			if !bytes.Equal(errb.Bytes(), c.stderr) {
				t.Errorf("stderr differs\n got: %q\nwant: %q", errb.String(), c.stderr)
			}
		})
	}
}
