package course

import (
	"bytes"
	"encoding/json"
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
			// L1.14 is decided offline from the lab sources (C1.12) and the jar (C1.6).
			"mark L1.14 (labs.md:12): *(verified offline 2026-10-18)* evidence: C1.6 ",
			"progress 13: checked 12, CONFIRMED 6, PARTIAL 6, CONTRADICTED 0, UNDECIDED 1; 23 checks run;",
		}, []string{"redo ", "mark L10.23"}}, // L10.23 is CONFIRMED and already verified live
		{"bad station", "testdata/stations/bad", 1, []string{
			"redo C4.5 (lab 4): training Tstat diff = 2: 0 of 1 pass, want 2.0: Lab4/My Tstat.diff = 0.0 (default)\n" +
				"  L4.13 (labs.md:25, LI:21): Set diff to 2 (`LI:21`)\n",
			"redo C1.2 (lab 1): ",
			"  L1.25 (labs.md:13, LI:4): Invoke Execute",
			"progress 13: checked 12, CONFIRMED 2, PARTIAL 3, CONTRADICTED 7, UNDECIDED 1;",
		}, []string{"mark L4.13"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, out, errb := run("--root", "testdata/predict-root", "progress", "--station", c.station,
				"--install", fixtureInstall, "--labsrc", "testdata/labsrc", "--date", "2026-10-18")
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

// The exit-2 paths: a malformed --date (with a station, so the date is what fails), and a
// selection whose every linked check is skipped (L2 of the root fixture links only a jar check,
// read without --install).
func TestProgressCannotCheck(t *testing.T) {
	clearStationEnv(t)
	cases := []struct {
		name, stderr string
		args         []string
	}{
		{"bad date", "course: --date 18/10/2026 is not a YYYY-MM-DD date\n",
			[]string{"--root", "testdata/predict-root", "progress", "--station", goodStation, "--date", "18/10/2026"}},
		{"every check skipped", "course: nothing could be checked (every linked check was skipped)\n",
			[]string{"--root", "testdata/root", "progress", "--lab", "2", "--station", goodStation, "--date", "2026-10-18"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, _, errb := run(c.args...)
			if code != 2 || !strings.HasSuffix(errb, c.stderr) {
				t.Errorf("exit %d, stderr %q; want 2 and %q", code, errb, c.stderr)
			}
		})
	}
}

// In JSON an action's group is the answer-key group (L4, RA1), the key --lab and the group rows
// use, for redo and mark alike; only the text prints the label "lab 4".
func TestProgressJSONGroup(t *testing.T) {
	clearStationEnv(t)
	_, out, _ := run("--root", "testdata/predict-root", "progress", "--station", "testdata/stations/bad",
		"--install", fixtureInstall, "--date", "2026-10-18", "--json")
	var doc struct {
		Actions []struct{ Kind, ID, Group string }
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"C4.5": "L4", "C1.2": "L1", "L10.6–L10.7": "L10", "C11.10": "L11"}
	for _, a := range doc.Actions {
		if g, ok := want[a.ID]; ok && a.Group != g {
			t.Errorf("%s %s: group %q, want %q", a.Kind, a.ID, a.Group, g)
		}
		delete(want, a.ID)
	}
	if len(want) > 0 {
		t.Errorf("actions missing: %v", want)
	}
}

// In text a redo names its group by groupLabel: "lab N" for a lab (L4, LB), the key group
// itself otherwise (RA1); an item that points at no course text shows its source only.
func TestProgressTextRedoLabel(t *testing.T) {
	var b bytes.Buffer
	printProgress(&b, nil, []action{
		{Kind: "redo", ID: "C4.5", Group: "L4", Text: "diff",
			Items: []actionItem{{ID: "L4.13", Source: "labs.md:25", Text: "LI:21", Step: "Set diff"}}},
		{Kind: "redo", ID: "CB.1", Group: "LB", Text: "bonus",
			Items: []actionItem{{ID: "LB.2", Source: "labs.md:40", Step: "Bonus step"}}},
		{Kind: "redo", ID: "CR.1", Group: "RA1", Text: "requirement",
			Items: []actionItem{{ID: "RA1.3", Source: "reqs.md:7", Step: "Meet it"}}},
	})
	for _, want := range []string{
		"redo C4.5 (lab 4): diff\n  L4.13 (labs.md:25, LI:21): Set diff\n",
		"redo CB.1 (lab B): bonus\n  LB.2 (labs.md:40): Bonus step\n",
		"redo CR.1 (RA1): requirement\n  RA1.3 (reqs.md:7): Meet it\n",
	} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("text lacks %q:\n%s", want, b.String())
		}
	}
}
