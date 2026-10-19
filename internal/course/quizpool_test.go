package course

import (
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/course/key"
)

// The new kinds and the items the pool leaves out, on the quiz-root fixture: L1.1 asks its why in
// the middle of the answer, L1.2 and L1.3 are Name = value recall items (L1.3 has no answer of its
// own, the value is the answer), L1.4 and L1.10 are *Observe* predict items; L1.5 (placeholder),
// L1.6 (too short), L1.7 and L1.8 (no course text) are left out.
func TestQuizKinds(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string
		not  []string
	}{
		{"why in the middle of the answer", []string{"--kind", "why"},
			[]string{"quiz: 2 of 2 questions (why; every group)", "L1.1 (why", "L1.9 (why",
				"    Read the log (`LI:2-3`)\n    *Why the console?*\n",
				"    It keeps every station message, even after Workbench closes.\n"},
			[]string{"L1.8 (", "The console shows it."}},
		{"recall", []string{"--kind", "recall"},
			[]string{"quiz: 2 of 2 questions (recall; every group)", "L1.2 (recall", "L1.3 (recall",
				"    What is the ORD of My Host?\n", "    What is the ORD of Alarm?\n",
				"    `local:`\n    One query, empty body = this machine.\n", "    `local:|alarm:`\n"},
			[]string{"(step only)", "My Host = "}},
		{"predict", []string{"--kind", "predict"},
			[]string{"quiz: 2 of 2 questions (predict; every group)", "L1.4 (predict", "L1.10 (predict",
				"    Predict what you observe, and why: *Observe* two jars (`LI:6`)\n",
				"    *(predicted)* `a-rt.jar` and `a-wb.jar`, one per module part.\n"},
			[]string{"L1.5 (", "L1.6 (", "L1.7 ("}},
		{"every kind", nil,
			[]string{"quiz: 8 of 8 questions (why, decision, trap, recall, predict; every group)"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := append([]string{"--root", "testdata/quiz-root", "quiz", "-n", "20", "--seed", "1", "--answers"}, c.args...)
			code, out, errb := run(args...)
			if code != 0 {
				t.Fatalf("exit %d; stdout:\n%s\nstderr: %s", code, out, errb)
			}
			for _, w := range c.want {
				if !strings.Contains(out, w) {
					t.Errorf("stdout lacks %q:\n%s", w, out)
				}
			}
			for _, n := range c.not {
				if strings.Contains(out, n) {
					t.Errorf("stdout has %q:\n%s", n, out)
				}
			}
		})
	}
}

// placeholderAnswers are the answers of the keys that answer nothing: a step with no answer
// (key/labs.go) and the setup steps.
var placeholderAnswers = []string{"(step only)", "Setup.", "Setup only."}

// checkPool fails for a pool item that cannot be studied: an empty or placeholder answer, a lab
// question with no course text range, a predict answer under quizMinAnswer runes, an answer the
// prompt already states, or two items with the same prompt or the same answer. It returns the
// pool size per kind.
func checkPool(t *testing.T, c *key.Corpus) map[string]int {
	t.Helper()
	pool, _ := buildPool(c, nil, quizKinds)
	counts := map[string]int{}
	prompts, answers := map[string]string{}, map[string]string{}
	for _, q := range pool {
		id, a := q.it.ID, strings.TrimSpace(q.answer)
		counts[q.kind]++
		switch {
		case a == "":
			t.Errorf("%s (%s): empty answer", id, q.kind)
		case strings.Contains(q.prompt, a):
			t.Errorf("%s (%s): the prompt states the answer %q", id, q.kind, a)
		}
		for _, p := range placeholderAnswers {
			if a == p {
				t.Errorf("%s (%s): placeholder answer %q", id, q.kind, a)
			}
		}
		if q.it.Kind == "lab item" && (q.it.TextFile == "" || len(q.it.Ranges) == 0) {
			t.Errorf("%s (%s): no course text range to trace the answer to", id, q.kind)
		}
		if n := utf8.RuneCountInString(substance(q.answer)); q.kind == quizPredict && n < quizMinAnswer {
			t.Errorf("%s (%s): answer of %d runes, under %d", id, q.kind, n, quizMinAnswer)
		}
		if other, dup := prompts[q.prompt]; dup {
			t.Errorf("%s and %s ask the same prompt %q", other, id, q.prompt)
		}
		if other, dup := answers[a]; dup {
			t.Errorf("%s and %s share the answer %q", other, id, a)
		}
		prompts[q.prompt], answers[a] = id, id
	}
	return counts
}

// Every question of the fixtures can be studied, and the new kinds are drawn.
func TestQuizPoolSolid(t *testing.T) {
	for root, want := range map[string]map[string]int{
		"testdata/root":      {quizWhy: 3, quizDecision: 1, quizTrap: 1, quizPredict: 1},
		"testdata/quiz-root": {quizWhy: 2, quizDecision: 1, quizTrap: 1, quizRecall: 2, quizPredict: 2},
	} {
		c, problems := key.Load(root)
		if c == nil {
			t.Fatalf("%s: %v", root, problems)
		}
		got := checkPool(t, c)
		for _, k := range quizKinds {
			if got[k] != want[k] {
				t.Errorf("%s: %d %s questions, want %d (%v)", root, got[k], k, want[k], got)
			}
		}
	}
}

// The same on the real answer keys, when NIAGARA_RESEARCH_DIR points at them; it logs the pool
// size per kind (docs/course.md, the live-run table).
func TestQuizPoolReal(t *testing.T) {
	root := os.Getenv(key.RootEnv)
	if root == "" || !key.HasMarker(root) {
		t.Skipf("%s is not a niagara-research checkout: the real quiz pool is not checked", key.RootEnv)
	}
	c, problems := key.Load(root)
	if c == nil {
		t.Fatalf("%v", problems)
	}
	got := checkPool(t, c)
	for _, k := range quizKinds {
		if got[k] == 0 {
			t.Errorf("no %s question in the real answer keys", k)
		}
	}
	t.Logf("real quiz pool: %v", got)
}

// The course text follows the answers, never the questions (it may state the answer: L4.14's
// LI:787-789 names the 1-degree band); --no-context drops it. A question whose answer is not
// confirmed (predicted or unverified, not verified since) carries its status on the question line.
func TestQuizContextAfterAnswers(t *testing.T) {
	quiz := func(args ...string) string {
		t.Helper()
		code, out, errb := run(append([]string{"--root", "testdata/quiz-root", "quiz", "-n", "20", "--seed", "1"}, args...)...)
		if code != 0 {
			t.Fatalf("exit %d; stdout:\n%s\nstderr: %s", code, out, errb)
		}
		return out
	}
	out := quiz("--kind", "recall", "--answers")
	answers := strings.Index(out, "\nanswers\n")
	if c := strings.Index(out, "quiz fixture line 4"); answers < 0 || c < answers {
		t.Errorf("course text at %d, want after the answers at %d:\n%s", c, answers, out)
	}
	if want := "    `local:`\n    One query, empty body = this machine.\n" +
		"    sources/n4-dev-course/text/Lab Instructions.txt:4 (L1.2)\n        4  Lab Instructions quiz fixture line 4\n"; !strings.Contains(out, want) {
		t.Errorf("want the answer, then its course text:\n%s", out)
	}
	if out := quiz("--kind", "recall"); strings.Contains(out, "quiz fixture line") {
		t.Errorf("course text without --answers:\n%s", out)
	}
	out = quiz("--no-context", "--answers")
	if strings.Contains(out, "quiz fixture line") || strings.Contains(out, "Lab Instructions.txt:") {
		t.Errorf("course text with --no-context:\n%s", out)
	}
	if out := quiz("--no-context"); !strings.Contains(out, "answers: course quiz -n 20 --seed 1 --no-context --answers\n") {
		t.Errorf("the replay line drops --no-context:\n%s", out)
	}
	out = quiz()
	for _, w := range []string{" (predict · Lab 1 — Investigating ORDs (`LI:1-12`)) [predicted]\n    Predict what you observe, and why: *Observe* two jars",
		". L1.9 (why · Lab 1 — Investigating ORDs (`LI:1-12`)) [unverified]\n",
		". L1.10 (predict · Lab 1 — Investigating ORDs (`LI:1-12`))\n"} {
		if !strings.Contains(out, w) {
			t.Errorf("stdout lacks %q:\n%s", w, out)
		}
	}
}

// --no-context reads no course text, so a missing text file does not stop the quiz.
func TestQuizNoContextReadsNoText(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/quiz-root")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(root + "/sources/n4-dev-course/text/Lab Instructions.txt"); err != nil {
		t.Fatal(err)
	}
	if code, out, errb := run("--root", root, "quiz", "--no-context", "--answers", "--seed", "1"); code != 0 {
		t.Errorf("exit %d, want 0; stdout:\n%s\nstderr: %s", code, out, errb)
	}
}
