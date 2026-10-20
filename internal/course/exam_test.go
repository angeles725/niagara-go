package course

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/course/key"
)

// An exam problem is a question: its step and **Requirement.** paragraph are the prompt, the rest
// of the section the answer (P2 of the fixture states only its requirement: no answer, left out).
// A requirement with a traceability row is a question too; one without (RA1.R3a) is left out.
func TestQuizExamAndRequirementKinds(t *testing.T) {
	code, out, errb := run("--root", "testdata/root", "quiz", "--lab", "EXAM", "--seed", "1", "--answers")
	if code != 0 {
		t.Fatalf("exit %d; stdout:\n%s\nstderr: %s", code, out, errb)
	}
	for _, w := range []string{"quiz: 1 of 1 question (why, decision, trap, recall, predict, requirement, exam; EXAM), seed 1\n",
		"Q1. P1 (exam · 1. Problem 1 — BLimiter)\n    Problem 1 — BLimiter\n    Clamp `in` between `min` and `max`.\n",
		"A1. P1 [verified (live)] ", "    **Verified.** `out` stayed between 25 and 75.\n"} {
		if !strings.Contains(out, w) {
			t.Errorf("stdout lacks %q:\n%s", w, out)
		}
	}
	code, out, errb = run("--root", "testdata/root", "quiz", "--kind", "requirement", "-n", "9", "--seed", "1", "--answers")
	if code != 0 {
		t.Fatalf("exit %d; stdout:\n%s\nstderr: %s", code, out, errb)
	}
	for _, w := range []string{"quiz: 4 of 4 questions (requirement; every group)",
		"(requirement · 1. Review Assignment 1 — BWithId)\n    How is this requirement satisfied and verified? `BWithId extends BComponent`\n",
		"    Satisfied by: `extends BComponent`\n    Verified by: compiles\n"} {
		if !strings.Contains(out, w) {
			t.Errorf("stdout lacks %q:\n%s", w, out)
		}
	}
	if strings.Contains(out, "RA1.R3a (") || strings.Contains(out, "(no traceability row)") {
		t.Errorf("a requirement without a traceability row is no question:\n%s", out)
	}
}

// examRoot is the root fixture with a second solvable problem.
func examRoot(t *testing.T) string {
	t.Helper()
	root := copyRoot(t)
	exam := filepath.Join(root, filepath.FromSlash(key.ExamFile))
	text := strings.Replace(readFile(t, exam), "**Requirement.** Export histories.\n",
		"**Requirement.** Export histories.\n\n**Design.** A service writes a CSV.\n", 1)
	if err := os.WriteFile(exam, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// course exam asks every EXAM problem (or --count of them) in the order of the seed, scored like
// quiz --interactive and recorded with kind exam.
func TestExam(t *testing.T) {
	fixNow(t, time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC))
	root, hist := examRoot(t), filepath.Join(t.TempDir(), "h.jsonl")
	code, out, errb := runIn("\ny\n\nn\n", nil, "--root", root, "exam", "--seed", "1", "--history", hist)
	if code != 0 {
		t.Fatalf("exit %d; stdout:\n%s\nstderr: %s", code, out, errb)
	}
	for _, w := range []string{"exam: 2 of 2 problems (EXAM), seed 1, no time limit\n", "Q1. P", "Q2. P",
		"score: 1 of 2 graded right (50%); 2 answers recorded in " + hist + "\n"} {
		if !strings.Contains(out, w) {
			t.Errorf("stdout lacks %q:\n%s", w, out)
		}
	}
	if got := readFile(t, hist); strings.Count(got, `"kind":"exam","group":"EXAM"`) != 2 {
		t.Errorf("history:\n%s", got)
	}
	if code, out, _ := runIn("\ny\n", nil, "--root", root, "exam", "--count", "1", "--seed", "1", "--history", hist); code != 0 ||
		!strings.Contains(out, "exam: 1 of 2 problems (EXAM), seed 1, no time limit\n") || strings.Contains(out, "Q2.") {
		t.Errorf("--count 1: exit %d:\n%s", code, out)
	}
}

// --count and --minutes below 1 ask nothing: usage errors; an exam with no solvable problem is the
// honest zero.
func TestExamRefusals(t *testing.T) {
	for _, c := range []struct {
		args []string
		exit int
		want string
	}{
		{[]string{"--count", "0"}, 2, "error: --count 0 asks no question (want 1 or more)"},
		{[]string{"--minutes", "0"}, 2, "error: --minutes 0 leaves no time (want 1 or more)"},
	} {
		code, _, errb := run(append([]string{"--root", "testdata/root", "exam"}, c.args...)...)
		if code != c.exit || !strings.Contains(errb, c.want) {
			t.Errorf("%v: exit %d, stderr %q", c.args, code, errb)
		}
	}
	root := copyRoot(t)
	exam := filepath.Join(root, filepath.FromSlash(key.ExamFile))
	text := strings.Replace(readFile(t, exam), "**Verified.** `out` stayed between 25 and 75.\n", "", 1)
	if err := os.WriteFile(exam, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out, errb := run("--root", root, "exam"); code != 1 || out != "" ||
		errb != "course: no exam question in EXAM (searched 2 items)\n" {
		t.Errorf("exit %d, stdout %q, stderr %q; want the honest zero", code, out, errb)
	}
}
