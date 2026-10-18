package course

import (
	"strings"
	"testing"
)

func TestFind(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		code   int
		stdout string // every line must appear, in this order
		stderr string
	}{
		{"step and answer lines", []string{"jar"}, 0,
			"L1   L1.4  predicted   *Observe* the jar (`LI:7`)\nL1   L1.6  written     `gradlew jar` (`LI:10`).\n" +
				"L2   L2.1  written     `gradlew jar`, restart\nRAX  X1    unverified  Cheapest check: `gradlew jar`", ""},
		{"a later answer line", []string{"Slot-o-Matic"}, 0, "L1  L1.3  unverified, identical  The setter error before Slot-o-Matic is expected.", ""},
		{"case-sensitive by default", []string{"bfirst"}, 1, "",
			"course: no item matches bfirst (searched 24 items in labs.md, review-assignments.md, certification-exam.md)\n"},
		{"-i folds case", []string{"-i", "bfirst"}, 0, "L1.3", ""},
		{"a bad regexp", []string{"("}, 2, "", "course: bad regexp ("},
	}
	for _, c := range cases {
		code, out, errs := run(append([]string{"--root", "testdata/root", "find"}, c.args...)...)
		if code != c.code || !strings.HasPrefix(errs, c.stderr) {
			t.Errorf("%s: exit %d, stderr %q; want %d, %q", c.name, code, errs, c.code, c.stderr)
		}
		rest := out
		for _, want := range strings.Split(c.stdout, "\n") {
			i := strings.Index(rest, want)
			if i < 0 {
				t.Errorf("%s: stdout lacks %q (in order):\n%s", c.name, want, out)
				break
			}
			rest = rest[i+len(want):]
		}
	}
}
