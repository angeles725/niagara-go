package course

import (
	"strings"
	"testing"
)

// progress --offline needs no station: per group, the items done (a verified marker), those
// that need a run (unverified, predicted, listed under "Answers that need a live run", or a
// needs-a-run check) and the rest to do, from the answer keys alone.
func TestProgressOffline(t *testing.T) {
	clearStationEnv(t)
	code, out, errb := run("--root", "testdata/root", "progress", "--offline")
	if code != 0 {
		t.Fatalf("exit %d; stdout:\n%s\nstderr: %s", code, out, errb)
	}
	for _, w := range []string{
		"group  items  done  needs-run  todo\n",
		"L1         6     0          2     4\n",
		"L2         4     0          2     2\n",
		"LB         1     0          1     0\n",
		"LA         2     2          0     0\n",
		"RAX        1     0          1     0\n",
		"EXAM       2     2          0     0\n",
		"done: a verified marker; needs-run: unverified, predicted, a live-run answer or a needs-a-run check, not verified; todo: the rest\n",
	} {
		if !strings.Contains(out, w) {
			t.Errorf("stdout lacks %q:\n%s", w, out)
		}
	}
	if !strings.HasPrefix(errb, "progress (offline) ") || !strings.HasSuffix(errb, "; no station read\n") {
		t.Errorf("stderr %q", errb)
	}
	code, out, _ = run("--root", "testdata/root", "progress", "--offline", "--lab", "2", "--json")
	for _, w := range []string{`"group": "L2"`, `"needs_run": 2`, `"needs_run_ids": [`, `"L2.5"`, `"todo": 2`} {
		if !strings.Contains(out, w) {
			t.Errorf("JSON lacks %s:\n%s", w, out)
		}
	}
	if code != 0 {
		t.Errorf("--json: exit %d", code)
	}
}

// --offline reads no station, so the station flags are usage errors; an unknown group is exit 1.
func TestProgressOfflineRefusals(t *testing.T) {
	clearStationEnv(t)
	if code, _, errb := run("--root", "testdata/root", "progress", "--offline", "--station", goodStation); code != 2 ||
		!strings.Contains(errb, "error: --offline reads no station: drop --station") {
		t.Errorf("exit %d, stderr %q", code, errb)
	}
	if code, _, errb := run("--root", "testdata/root", "progress", "--offline", "--lab", "9"); code != 1 ||
		!strings.Contains(errb, "course: no group 9") {
		t.Errorf("exit %d, stderr %q", code, errb)
	}
}
