package course

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
)

// runIn runs course with stdin and an environment of its own (nil reads the process).
func runIn(stdin string, env map[string]string, args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	s := cli.Stdio{In: strings.NewReader(stdin), Out: &out, Err: &errb}
	if env != nil {
		s.Getenv = func(name string) string { return env[name] }
	}
	code := Main(args, s)
	return code, out.String(), errb.String()
}

// fixNow pins the clock of the quiz records for one test.
func fixNow(t *testing.T, at time.Time) {
	t.Helper()
	old := now
	now = func() time.Time { return at }
	t.Cleanup(func() { now = old })
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// --interactive shows each question, then its answer on Enter, then reads a self-grade: y right,
// n wrong, s skip (anything else asks again). Each y or n appends one JSONL record to the history
// right away; a skip records nothing. The score closes the quiz.
func TestQuizInteractive(t *testing.T) {
	fixNow(t, time.Date(2026, 10, 19, 12, 0, 0, 0, time.FixedZone("x", 3600)))
	hist := filepath.Join(t.TempDir(), "state", "history.jsonl")
	code, out, errb := runIn("\ny\n\nmaybe\nn\n\ns\n", nil, "--root", "testdata/root", "quiz", "--interactive",
		"--ids", "D1.1,T1.1,L1.1", "--history", hist)
	if code != 0 {
		t.Fatalf("exit %d; stdout:\n%s\nstderr: %s", code, out, errb)
	}
	for _, w := range []string{
		"Q1. D1.1 (decision", "    Why this decision? Runtime only\n(Enter shows the answer) ",
		"A1. D1.1 [written] docs/n4-dev-course-solutions/review-assignments.md:21\n    Lab 1 shows it.\n",
		"grade: y right, n wrong, s skip? ", "grade y, n or s? ",
		"\nscore: 1 of 2 graded right (50%), 1 skipped; 2 answers recorded in " + hist + "\n"} {
		if !strings.Contains(out, w) {
			t.Errorf("stdout lacks %q:\n%s", w, out)
		}
	}
	if strings.Contains(out, "answers: course quiz") {
		t.Errorf("an interactive quiz prints no replay line:\n%s", out)
	}
	want := `{"time":"2026-10-19T11:00:00Z","id":"D1.1","kind":"decision","group":"RA1","result":"right"}` + "\n" +
		`{"time":"2026-10-19T11:00:00Z","id":"T1.1","kind":"trap","group":"RA1","result":"wrong"}` + "\n"
	if got := readFile(t, hist); got != want {
		t.Errorf("history:\n%s\nwant:\n%s", got, want)
	}
	// A second quiz appends.
	if code, out, _ := runIn("\ny\n", nil, "--root", "testdata/root", "quiz", "--interactive", "--ids", "L1.1", "--history", hist); code != 0 ||
		!strings.Contains(out, "score: 1 of 1 graded right (100%); 1 answer recorded in ") {
		t.Errorf("exit %d:\n%s", code, out)
	}
	if n := strings.Count(readFile(t, hist), "\n"); n != 3 {
		t.Errorf("%d records after the second quiz, want 3", n)
	}
}

// The end of input stops the quiz: the questions asked so far are scored, and a quiz with no
// grade creates no history file.
func TestQuizInteractiveEOF(t *testing.T) {
	hist := filepath.Join(t.TempDir(), "h.jsonl")
	code, out, _ := runIn("", nil, "--root", "testdata/root", "quiz", "--interactive", "--ids", "D1.1,T1.1", "--history", hist)
	if code != 0 || !strings.Contains(out, "\nend of input after 1 of 2 questions\nscore: 0 of 0 graded right; nothing recorded\n") {
		t.Errorf("exit %d:\n%s", code, out)
	}
	if _, err := os.Stat(hist); !os.IsNotExist(err) {
		t.Errorf("history file created with no grade: %v", err)
	}
}

// Without --history the records go to $XDG_STATE_HOME/niagara-course/history.jsonl, else (unset or
// relative, as the XDG spec says) ~/.local/state/niagara-course/history.jsonl.
func TestHistoryDefaultPath(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"XDG_STATE_HOME": filepath.Join(dir, "xdg"), "HOME": filepath.Join(dir, "home")},
			filepath.Join(dir, "xdg", "niagara-course", "history.jsonl")},
		{map[string]string{"XDG_STATE_HOME": "relative", "HOME": filepath.Join(dir, "home")},
			filepath.Join(dir, "home", ".local", "state", "niagara-course", "history.jsonl")},
	} {
		code, out, errb := runIn("\ny\n", c.env, "--root", "testdata/root", "quiz", "--interactive", "--ids", "D1.1")
		if code != 0 || !strings.Contains(out, "recorded in "+c.want+"\n") {
			t.Errorf("exit %d, want %s:\n%s%s", code, c.want, out, errb)
		}
		if _, err := os.Stat(c.want); err != nil {
			t.Errorf("%v", err)
		}
	}
}

// A history that cannot be written stops the quiz with exit 2; an empty --history is a usage error.
func TestQuizInteractiveCannotRecord(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errb := runIn("\ny\n", nil, "--root", "testdata/root", "quiz", "--interactive", "--ids", "D1.1",
		"--history", filepath.Join(file, "h.jsonl"))
	if code != 2 || !strings.Contains(errb, "course: history ") {
		t.Errorf("exit %d, stderr %q; want 2 and the history error", code, errb)
	}
	if code, _, errb := runIn("", nil, "--root", "testdata/root", "quiz", "--interactive", "--history", ""); code != 2 ||
		!strings.Contains(errb, "error: --history: an empty path") {
		t.Errorf("exit %d, stderr %q", code, errb)
	}
}
