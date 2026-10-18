package course

import (
	"strings"
	"testing"
)

// progress over the predict-root fixture: one row per group, the totals, then one action per FAIL
// check (the step to redo) and one per CONFIRMED item without a verified marker (the line to add).
func TestProgress(t *testing.T) {
	clearStationEnv(t)
	cases := []struct {
		name    string
		station string
		exit    int
		want    []string
		absent  []string
	}{
		{"good station", goodStation, 0, []string{
			"group  items  checked  CONFIRMED  PARTIAL  CONTRADICTED  UNDECIDED  live  offline",
			"L4         2        2          1        1             0          0     0        0",
			"L10        3        3          3        0             0          0     1        0",
			"mark L4.13 (labs.md:25): *(verified live 2026-10-18)* evidence: C4.5 Lab4/My Tstat.diff = 2.0 (stored)\n",
			"mark L10.6–L10.7 (labs.md:33): *(verified offline 2026-10-18)* evidence: C10.7",
			"progress 13: checked 11, CONFIRMED 5, PARTIAL 6, CONTRADICTED 0, UNDECIDED 2; 19 checks run;",
		}, []string{"redo ", "mark L10.23"}}, // L10.23 is CONFIRMED and already verified live
		{"bad station", "testdata/stations/bad", 1, []string{
			"redo C4.5 (lab 4): training Tstat diff = 2: 0 of 1 pass, want 2.0: Lab4/My Tstat.diff = 0.0 (default)\n" +
				"  L4.13 (labs.md:25, LI:21): Set diff to 2 (`LI:21`)\n",
			"redo C1.2 (lab 1): ",
			"  L1.25 (labs.md:13, LI:4): Invoke Execute",
			"progress 13: checked 11, CONFIRMED 1, PARTIAL 3, CONTRADICTED 7, UNDECIDED 2;",
		}, []string{"mark L4.13"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, out, errb := run("--root", "testdata/predict-root", "progress", "--station", c.station,
				"--install", fixtureInstall, "--date", "2026-10-18")
			if code != c.exit {
				t.Errorf("exit %d, want %d; stderr: %s", code, c.exit, errb)
			}
			for _, w := range c.want {
				if !strings.Contains(out, w) {
					t.Errorf("stdout lacks %q:\n%s", w, out)
				}
			}
			for _, a := range c.absent {
				if strings.Contains(out, a) {
					t.Errorf("stdout has %q:\n%s", a, out)
				}
			}
		})
	}
}

// An unknown group is the searched miss of verify (exit 1 with the group list).
func TestProgressUnknownGroup(t *testing.T) {
	clearStationEnv(t)
	code, _, errb := run("--root", "testdata/predict-root", "progress", "--lab", "12", "--station", goodStation)
	if code != 1 || !strings.Contains(errb, "no group 12 (groups: ") {
		t.Errorf("exit %d, stderr %q", code, errb)
	}
}
