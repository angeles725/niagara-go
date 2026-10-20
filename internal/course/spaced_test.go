package course

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeHistory writes records (one JSON line each, as given) to a temporary history file.
func writeHistory(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "history.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func rec(at, id, kind, group, result string) string {
	return `{"time":"` + at + `","id":"` + id + `","kind":"` + kind + `","group":"` + group + `","result":"` + result + `"}`
}

// --missed asks the questions whose latest record is wrong (a later right answer clears a miss;
// an ID the keys no longer hold is ignored), and its replay line names them with --ids, since the
// history moves on.
func TestQuizMissed(t *testing.T) {
	hist := writeHistory(t,
		rec("2026-10-10T10:00:00Z", "L1.1", "why", "L1", "wrong"),
		rec("2026-10-11T10:00:00Z", "l1.1", "why", "L1", "right"),
		rec("2026-10-11T10:00:00Z", "D1.1", "decision", "RA1", "wrong"),
		rec("2026-10-11T10:00:00Z", "T1.1", "trap", "RA1", "right"),
		rec("2026-10-12T10:00:00Z", "T1.1", "trap", "RA1", "wrong"),
		rec("2026-10-12T10:00:00Z", "Z9", "why", "L9", "wrong"))
	code, out, errb := run("--root", "testdata/root", "quiz", "--missed", "--history", hist, "--seed", "1", "-n", "9")
	if code != 0 {
		t.Fatalf("exit %d; stdout:\n%s\nstderr: %s", code, out, errb)
	}
	for _, w := range []string{"; every group; missed), seed 1\n", "D1.1 (decision", "T1.1 (trap"} {
		if !strings.Contains(out, w) {
			t.Errorf("stdout lacks %q:\n%s", w, out)
		}
	}
	if strings.Contains(out, "L1.1 (") || !strings.Contains(out, "answers: course quiz --ids ") {
		t.Errorf("want D1.1 and T1.1 only, replayed with --ids:\n%s", out)
	}
	// --lab narrows the missed questions too.
	if code, _, errb := run("--root", "testdata/root", "quiz", "--missed", "--history", hist, "--lab", "1"); code != 1 ||
		!strings.Contains(errb, "course: no missed why, decision, trap, recall, predict question in L1 (history "+hist+": 6 records)\n") {
		t.Errorf("exit %d, stderr %q", code, errb)
	}
	// An empty (or missing) history is the honest zero.
	missing := filepath.Join(t.TempDir(), "none.jsonl")
	if code, _, errb := run("--root", "testdata/root", "quiz", "--missed", "--history", missing); code != 1 ||
		!strings.Contains(errb, "(history "+missing+": 0 records)") {
		t.Errorf("exit %d, stderr %q", code, errb)
	}
}

// --due draws the Leitner-due questions: a record moves its item up one box when right (at most
// 5), back to box 1 when wrong; the item is due once now reaches its last record plus 2^(box-1)
// days. An item never answered is not due (a plain quiz draws it).
func TestQuizDue(t *testing.T) {
	hist := writeHistory(t,
		rec("2026-10-10T00:00:00Z", "D1.1", "decision", "RA1", "right"), // box 2: due 10-12
		rec("2026-10-10T00:00:00Z", "T1.1", "trap", "RA1", "right"),
		rec("2026-10-15T00:00:00Z", "T1.1", "trap", "RA1", "right"), // box 3: due 10-19
		rec("2026-10-17T00:00:00Z", "L1.1", "why", "L1", "right"),
		rec("2026-10-18T00:00:00Z", "L1.1", "why", "L1", "wrong"), // box 1: due 10-19
		rec("2026-10-01T00:00:00Z", "L1.5", "why", "L1", "right"),
		rec("2026-10-02T00:00:00Z", "L1.5", "why", "L1", "right"),
		rec("2026-10-03T00:00:00Z", "L1.5", "why", "L1", "right"),
		rec("2026-10-04T00:00:00Z", "L1.5", "why", "L1", "right"),
		rec("2026-10-05T00:00:00Z", "L1.5", "why", "L1", "right"), // box 5 (capped): due 10-21
	)
	quiz := func(at string) (int, string, string) {
		return run("--root", "testdata/root", "quiz", "--due", "--now", at, "--history", hist, "-n", "9", "--seed", "1")
	}
	code, out, errb := quiz("2026-10-19T00:00:00Z")
	if code != 0 {
		t.Fatalf("exit %d; stdout:\n%s\nstderr: %s", code, out, errb)
	}
	for _, w := range []string{"quiz: 3 of 3 questions (", "; due 2026-10-19T00:00:00Z), seed 1\n", "D1.1 (", "T1.1 (", "L1.1 ("} {
		if !strings.Contains(out, w) {
			t.Errorf("stdout lacks %q:\n%s", w, out)
		}
	}
	if strings.Contains(out, "L1.5 (") {
		t.Errorf("L1.5 (box 5) is not due until 10-21:\n%s", out)
	}
	if _, out, _ := quiz("2026-10-21T00:00:00Z"); !strings.Contains(out, "L1.5 (") {
		t.Errorf("L1.5 is due on 10-21:\n%s", out)
	}
	if code, _, errb := quiz("2026-10-11T00:00:00Z"); code != 1 || !strings.Contains(errb, "course: no due ") {
		t.Errorf("exit %d, stderr %q; want nothing due on 10-11", code, errb)
	}
}

// --missed, --due and --ids choose the questions three ways: one at a time; --now must be RFC 3339.
func TestQuizHistoryUsage(t *testing.T) {
	hist := writeHistory(t, rec("2026-10-10T00:00:00Z", "D1.1", "decision", "RA1", "wrong"))
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"--missed", "--due"}, "error: --missed, --due and --ids exclude each other"},
		{[]string{"--missed", "--ids", "D1.1"}, "error: --missed, --due and --ids exclude each other"},
		{[]string{"--due", "--now", "yesterday"}, "error: --now yesterday is not an RFC 3339 time"},
	} {
		code, _, errb := run(append([]string{"--root", "testdata/root", "quiz", "--history", hist}, c.args...)...)
		if code != 2 || !strings.Contains(errb, c.want) {
			t.Errorf("%v: exit %d, stderr %q; want 2 and %q", c.args, code, errb, c.want)
		}
	}
}

// course history prints the accuracy per group and per kind; a malformed line is skipped with a
// warning; an empty history is the honest zero (exit 1).
func TestHistoryCommand(t *testing.T) {
	hist := writeHistory(t,
		rec("2026-10-10T00:00:00Z", "L10.1", "why", "L10", "right"),
		rec("2026-10-10T00:00:00Z", "L2.1", "why", "L2", "wrong"),
		rec("2026-10-10T00:00:00Z", "L2.1", "why", "L2", "right"),
		`{"time":"2026-10-10T00:00:00Z","id":"D1.1"`,
		rec("2026-10-11T00:00:00Z", "D1.1", "decision", "RA1", "right"),
		rec("2026-10-11T00:00:00Z", "P1", "exam", "EXAM", "wrong"))
	code, out, errb := run("history", "--history", hist)
	want := "history: 5 answers in " + hist + "\n\n" +
		"group  answers  right  wrong  accuracy\n" +
		"EXAM         1      0      1        0%\n" +
		"L2           2      1      1       50%\n" +
		"L10          1      1      0      100%\n" +
		"RA1          1      1      0      100%\n" +
		"total        5      3      2       60%\n\n" +
		"kind      answers  right  wrong  accuracy\n" +
		"decision        1      1      0      100%\n" +
		"exam            1      0      1        0%\n" +
		"why             3      2      1       67%\n"
	if code != 0 || out != want {
		t.Errorf("exit %d; stdout:\n%s\nwant:\n%s", code, out, want)
	}
	if errb != "course: history "+hist+":4: not a record, skipped\n" {
		t.Errorf("stderr %q", errb)
	}
	code, out, _ = run("history", "--history", hist, "--json")
	for _, w := range []string{`"answers": 5`, `"group": "L10"`, `"kind": "why"`, `"accuracy": 67`} {
		if !strings.Contains(out, w) {
			t.Errorf("JSON lacks %s:\n%s", w, out)
		}
	}
	if code != 0 {
		t.Errorf("--json: exit %d", code)
	}
	empty := filepath.Join(t.TempDir(), "none.jsonl")
	if code, out, errb := run("history", "--history", empty); code != 1 || out != "" ||
		errb != "course: no answer in the history "+empty+" (course quiz --interactive records them)\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, errb)
	}
}
