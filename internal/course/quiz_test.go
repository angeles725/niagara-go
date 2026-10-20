package course

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/course/key"
)

// The quiz pool of the root fixture: the why questions L1.1 (leading its answer), L1.5 and L2.1
// (in the step), the decision D1.1, the trap T1.1, the predict L1.4 (L2.6 answers too briefly),
// the four requirements with a traceability row and the exam problem P1.
func TestQuizPool(t *testing.T) {
	cases := []struct {
		name string
		args []string
		exit int
		want []string
		not  []string
	}{
		{"every kind", []string{"-n", "20", "--seed", "1"}, 0,
			[]string{"quiz: 11 of 11 questions (why, decision, trap, recall, predict, requirement, exam; every group), seed 1\n", "L1.1 ", "L1.5 ", "L2.1 ", "D1.1 ", "T1.1 ", "L1.4 (predict",
				"RA1.R1 (requirement", "P1 (exam", "answers: course quiz -n 20 --seed 1 --answers\n"},
			[]string{"L1.2 ", "\nanswers\n"}},
		{"a why question leading the answer", []string{"--kind", "why", "--lab", "1", "--seed", "3", "--answers"}, 0,
			[]string{"quiz: 2 of 2 questions (why; L1), seed 3\n", "    Start the console (`LI:2-3`)\n    *Why the console?*\n",
				"    sources/n4-dev-course/text/Lab Instructions.txt:2-3 (L1.1)\n", "\nanswers\n", "    It keeps the log.\n",
				"[written]"},
			[]string{"L2.1"}},
		{"trap", []string{"--kind", "trap", "--seed", "1", "--answers"}, 0,
			[]string{"T1.1 (trap", "What goes wrong, and what does this solution do? TRANSIENT on `id`", "Id is not saved"}, nil},
		{"decision", []string{"--kind", "decision", "--seed", "1"}, 0,
			[]string{"D1.1 (decision", "Why this decision? Runtime only"}, []string{"Lab 1 shows it."}},
		{"empty pool", []string{"--kind", "trap", "--lab", "1"}, 1, nil, nil},
		{"unknown group", []string{"--lab", "9"}, 1, nil, nil},
		{"no question asked", []string{"-n", "0"}, 2, nil, nil},
		{"-n does not apply under --ids", []string{"--ids", "L1.1", "-n", "0", "--answers"}, 0,
			[]string{"quiz: 1 of 11 questions (why, decision, trap, recall, predict, requirement, exam; every group), ids L1.1\n", "A1. L1.1 "}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, out, errb := run(append([]string{"--root", "testdata/root", "quiz"}, c.args...)...)
			if code != c.exit {
				t.Fatalf("exit %d, want %d; stdout:\n%s\nstderr: %s", code, c.exit, out, errb)
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

// One seed draws one order; another seed may draw another, and without --seed the clock gives
// one, printed so the same quiz can be drawn again.
func TestQuizSeed(t *testing.T) {
	defer func(f func() time.Time) { now = f }(now)
	now = func() time.Time { return time.Unix(0, 42) }
	_, a, _ := run("--root", "testdata/root", "quiz", "--seed", "7")
	_, b, _ := run("--root", "testdata/root", "quiz", "--seed", "7")
	if a != b {
		t.Errorf("seed 7 drew two quizzes:\n%s\n%s", a, b)
	}
	orders := map[string]bool{}
	for seed := range 10 {
		_, out, _ := run("--root", "testdata/root", "quiz", "-n", "5", "--seed", strconv.Itoa(seed))
		orders[out[strings.Index(out, "\n"):strings.Index(out, "answers: ")]] = true
	}
	if len(orders) < 2 {
		t.Errorf("ten seeds drew one order")
	}
	_, c, _ := run("--root", "testdata/root", "quiz")
	if !strings.Contains(c, ", seed 42\n") || !strings.Contains(c, "answers: course quiz --seed 42 --answers") {
		t.Errorf("no --seed: want the clock's seed 42 printed:\n%s", c)
	}
}

// An answer is followed by at most quizContext text lines, then the command that prints them all.
func TestQuizContextCap(t *testing.T) {
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line " + strconv.Itoa(i+1)
	}
	it := &key.Item{ID: "L9.1", Section: "Lab 9", Step: "*Why?*", TextFile: key.LabText, Ranges: []key.Range{{A: 2, B: 6}, {A: 10, B: 15}}}
	var b strings.Builder
	printContext(&b, it, lines)
	out := b.String()
	if !strings.Contains(out, "   10  line 10\n       11  line 11\n       12  line 12\n    … more: course text L9.1\n") ||
		strings.Contains(out, "line 13") {
		t.Errorf("want lines 2-6 and 10-12, then the more line:\n%s", out)
	}
}

// copyRoot copies the root fixture into a temporary directory the test may break.
func copyRoot(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	if err := os.CopyFS(dst, os.DirFS("testdata/root")); err != nil {
		t.Fatal(err)
	}
	return dst
}

// A text file that is missing, or shorter than an item's range, cannot be quoted: exit 2 with
// the message of course text, before any question is printed.
func TestQuizText(t *testing.T) {
	lab := filepath.Join("sources", "n4-dev-course", "text", "Lab Instructions.txt")
	cases := []struct {
		name   string
		breaks func(root string) error
		stderr string
	}{
		{"missing text file", func(root string) error { return os.Remove(filepath.Join(root, lab)) },
			"course: missing text file sources/n4-dev-course/text/Lab Instructions.txt\n"},
		{"range past the end", func(root string) error {
			return os.WriteFile(filepath.Join(root, lab), []byte("1\n2\n3\n4\n5\n"), 0o644)
		},
			"course: L1.5: range 8-9 is outside sources/n4-dev-course/text/Lab Instructions.txt (5 lines)\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := copyRoot(t)
			if err := c.breaks(root); err != nil {
				t.Fatal(err)
			}
			code, out, errb := run("--root", root, "quiz", "--lab", "1", "--kind", "why", "--seed", "2")
			if code != 2 || errb != c.stderr || out != "" {
				t.Errorf("exit %d, stdout %q, stderr %q; want 2 and %q", code, out, errb, c.stderr)
			}
		})
	}
}

// The replay line quotes a --lab value the shell would split or expand.
func TestQuizReplayQuotesLab(t *testing.T) {
	for in, want := range map[string]string{"3": "--lab 3", "L 1": "--lab 'L 1'", "it's": `--lab 'it'\''s'`} {
		if got := (quizOpts{lab: in, hasLab: true, count: 5, seed: 1}).again(); !strings.Contains(got, " "+want+" ") {
			t.Errorf("lab %q: %q, want %q", in, got, want)
		}
	}
}

// An item with ranges but no text file has no context to print: quiz does
// not refuse it (it did before the range check, review-6710683f89426a1f).
func TestRangeOutside(t *testing.T) {
	lines := []string{"1", "2", "3"}
	for _, c := range []struct {
		name string
		it   key.Item
		want string
	}{
		{"no text file", key.Item{ID: "L1.1", Ranges: []key.Range{{A: 8, B: 9}}}, ""},
		{"fits", key.Item{ID: "L1.2", TextFile: "t.txt", Ranges: []key.Range{{A: 1, B: 3}}}, ""},
		{"past the end", key.Item{ID: "L1.3", TextFile: "t.txt", Ranges: []key.Range{{A: 2, B: 4}}},
			"L1.3: range 2-4 is outside t.txt (3 lines)"},
		{"reversed", key.Item{ID: "L1.4", TextFile: "t.txt", Ranges: []key.Range{{A: 3, B: 2}}},
			"L1.4: range 3-2 is outside t.txt (3 lines)"},
		{"starts at 0", key.Item{ID: "L1.5", TextFile: "t.txt", Ranges: []key.Range{{A: 0, B: 2}}},
			"L1.5: range 0-2 is outside t.txt (3 lines)"},
		{"several ranges fit", key.Item{ID: "L1.6", TextFile: "t.txt", Ranges: []key.Range{{A: 1, B: 1}, {A: 2, B: 3}}}, ""},
		{"first failing range reported", key.Item{ID: "L1.7", TextFile: "t.txt",
			Ranges: []key.Range{{A: 1, B: 2}, {A: 5, B: 5}, {A: 0, B: 1}}},
			"L1.7: range 5 is outside t.txt (3 lines)"},
	} {
		if got := rangeOutside(&c.it, lines); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
