package argparse

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// TestFixtures runs every case recorded from Python argparse and requires the same stdout,
// stderr, exit status and namespace.
func TestFixtures(t *testing.T) {
	doc := loadFixtures(t)
	if len(doc.Cases) == 0 {
		t.Fatal("no fixture cases")
	}
	for i, c := range doc.Cases {
		name := fmt.Sprintf("%03d_%s_%d_%s", i, c.Parser, c.Columns, strings.Join(c.Argv, "_"))
		t.Run(name, func(t *testing.T) {
			t.Setenv("COLUMNS", strconv.Itoa(c.Columns))
			parents := map[string]*Parser{}
			for pname, ps := range doc.Parents {
				parents[pname] = specParser(t, ps, parents)
			}
			p := specParser(t, doc.Parsers[c.Parser], parents)
			var out, errb bytes.Buffer
			ns, code, done := p.Parse(append([]string{}, c.Argv...), cli.Stdio{Out: &out, Err: &errb})
			checkResult(c, out.String(), errb.String(), ns, code, done, t.Errorf)
		})
	}
}

// checkResult compares one parse with its fixture and reports each difference through errorf.
func checkResult(c fixtureCase, stdout, stderr string, ns *Namespace, code int, done bool,
	errorf func(string, ...any)) {
	if stdout != c.Stdout {
		errorf("stdout differs at %s", firstDiff(stdout, c.Stdout))
	}
	if stderr != c.Stderr {
		errorf("stderr differs at %s", firstDiff(stderr, c.Stderr))
	}
	switch {
	case c.Exit == nil && c.NS == nil:
		errorf("malformed fixture: exit is null and there is no namespace")
	case c.Exit == nil && done:
		errorf("exited with %d, want a namespace %s", code, *c.NS)
	case c.Exit != nil && !done:
		errorf("returned a namespace, want exit %d", *c.Exit)
	case c.Exit != nil && code != *c.Exit:
		errorf("exit %d, want %d", code, *c.Exit)
	case c.Exit == nil && ns.Repr() != *c.NS:
		errorf("namespace\n   got %s\n  want %s", ns.Repr(), *c.NS)
	}
}
