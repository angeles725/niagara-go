package course

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
)

// runInReader runs course with in as stdin.
func runInReader(in io.Reader, args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := Main(args, cli.Stdio{In: in, Out: &out, Err: &errb})
	return code, out.String(), errb.String()
}

// --history and --now apply only where they are read: --history to --interactive, --missed and
// --due (an empty one is refused whatever else is given), --now to --due.
func TestQuizHistoryFlagScope(t *testing.T) {
	hist := filepath.Join(t.TempDir(), "h.jsonl")
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"--history", ""}, "error: --history: an empty path"},
		{[]string{"--history", hist}, "error: --history applies to --interactive, --missed and --due"},
		{[]string{"--now", "2026-10-19T00:00:00Z"}, "error: --now applies to --due"},
		{[]string{"--missed", "--history", hist, "--now", "2026-10-19T00:00:00Z"}, "error: --now applies to --due"},
	} {
		code, out, errb := run(append([]string{"--root", "testdata/root", "quiz", "--seed", "1"}, c.args...)...)
		if code != 2 || out != "" || !strings.Contains(errb, c.want) {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want 2 and %q", c.args, code, out, errb, c.want)
		}
	}
}

// The default path falls back to ~/.local/state when XDG_STATE_HOME is unset; with neither it and
// HOME, there is no path: a usage error (quiz and exam alike).
func TestHistoryPathFallbacks(t *testing.T) {
	home := t.TempDir()
	want := filepath.Join(home, ".local", "state", "niagara-course", "history.jsonl")
	code, out, errb := runIn("\ny\n", map[string]string{"HOME": home}, "--root", "testdata/root", "quiz", "--interactive", "--ids", "D1.1")
	if code != 0 || !strings.Contains(out, "recorded in "+want+"\n") {
		t.Errorf("XDG_STATE_HOME unset: exit %d:\n%s%s", code, out, errb)
	}
	for _, cmd := range [][]string{{"quiz", "--interactive"}, {"exam"}} {
		code, out, errb := runIn("", map[string]string{}, append([]string{"--root", "testdata/root"}, cmd...)...)
		if code != 2 || out != "" || !strings.Contains(errb, "error: no history path: set --history, XDG_STATE_HOME or HOME") {
			t.Errorf("%v with no HOME: exit %d, stdout %q, stderr %q", cmd, code, out, errb)
		}
	}
}

// The end of input stops the session at the grade prompt too, also after a grade it did not take.
func TestQuizInteractiveEOFAtGrade(t *testing.T) {
	for _, in := range []string{"\n", "\nmaybe"} {
		hist := filepath.Join(t.TempDir(), "h.jsonl")
		code, out, _ := runIn(in, nil, "--root", "testdata/root", "quiz", "--interactive", "--ids", "D1.1,T1.1", "--history", hist)
		if code != 0 || !strings.Contains(out, "\nend of input after 1 of 2 questions\nscore: 0 of 0 graded right; nothing recorded\n") ||
			strings.Contains(out, "Q2.") {
			t.Errorf("input %q: exit %d:\n%s", in, code, out)
		}
	}
}

// A history line longer than any buffer is skipped like any other bad line: the records after it
// still count.
func TestHistoryOversizeLine(t *testing.T) {
	hist := writeHistory(t,
		`{"time":"2026-10-10T00:00:00Z","id":"`+strings.Repeat("x", 2<<20)+`"`,
		rec("2026-10-11T00:00:00Z", "D1.1", "decision", "RA1", "wrong"))
	code, out, errb := run("history", "--history", hist)
	if code != 0 || !strings.Contains(out, "history: 1 answer in ") || errb != "course: history "+hist+":1: not a record, skipped\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, errb)
	}
	if code, out, _ := run("--root", "testdata/root", "quiz", "--missed", "--history", hist, "--seed", "1"); code != 0 ||
		!strings.Contains(out, "D1.1 (decision") {
		t.Errorf("--missed after a long line: exit %d:\n%s", code, out)
	}
}

// Names that compare equal in natural order (leading zeros) still print in one order.
func TestNaturalSortTies(t *testing.T) {
	m := map[string]*accuracyRow{}
	for _, n := range []string{"L01", "L1", "L001", "L2"} {
		m[n] = &accuracyRow{Name: n}
	}
	for range 20 {
		var got []string
		for _, r := range rows(m) {
			got = append(got, r.Name)
		}
		if strings.Join(got, " ") != "L001 L01 L1 L2" {
			t.Fatalf("order %v", got)
		}
	}
}

// The replay line of --due names the drawn IDs; --due --interactive grades them into the history.
func TestQuizDueReplayAndInteractive(t *testing.T) {
	hist := writeHistory(t, rec("2026-10-10T00:00:00Z", "D1.1", "decision", "RA1", "wrong"))
	_, out, _ := run("--root", "testdata/root", "quiz", "--due", "--now", "2026-10-19T00:00:00Z", "--history", hist, "--seed", "1")
	if !strings.HasSuffix(out, "answers: course quiz --ids D1.1 --answers\n") {
		t.Errorf("replay line:\n%s", out)
	}
	fixNow(t, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC))
	code, out, _ := runIn("\ny\n", nil, "--root", "testdata/root", "quiz", "--due", "--interactive", "--history", hist, "--seed", "1")
	if code != 0 || !strings.Contains(out, "score: 1 of 1 graded right (100%)") ||
		!strings.HasSuffix(readFile(t, hist), `{"time":"2026-10-19T00:00:00Z","id":"D1.1","kind":"decision","group":"RA1","result":"right"}`+"\n") {
		t.Errorf("exit %d:\n%s", code, out)
	}
}

// --minutes above a day is refused: a typo must not overflow into a deadline in the past.
func TestExamMinutesBound(t *testing.T) {
	for _, m := range []string{"1441", "200000000"} {
		code, out, errb := run("--root", "testdata/root", "exam", "--minutes", m, "--history", filepath.Join(t.TempDir(), "h"))
		if code != 2 || out != "" || !strings.Contains(errb, "error: --minutes "+m+" is over a day (want at most 1440)") {
			t.Errorf("--minutes %s: exit %d, stdout %q, stderr %q", m, code, out, errb)
		}
	}
}

// pacedInput gives one line per read and moves the clock 10 minutes each time: time passes as
// the problems are answered, however often the code reads the clock.
type pacedInput struct {
	mu    sync.Mutex
	lines []string
	at    *time.Time
}

func (p *pacedInput) Read(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.lines) == 0 {
		return 0, os.ErrClosed
	}
	*p.at = p.at.Add(10 * time.Minute)
	n := copy(b, p.lines[0])
	p.lines = p.lines[1:]
	return n, nil
}

// With --minutes 15 and 10 minutes per answered line, the first problem (Enter + grade, 20
// minutes) ends past the limit: the second is not asked; the first is graded.
func TestExamMinutesPaced(t *testing.T) {
	at := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	old := now
	now = func() time.Time { return at }
	t.Cleanup(func() { now = old })
	in := &pacedInput{lines: []string{"\n", "y\n", "\n", "y\n"}, at: &at}
	code, out, errb := runInReader(in, "--root", examRoot(t), "exam", "--seed", "1", "--minutes", "15",
		"--history", filepath.Join(t.TempDir(), "h.jsonl"))
	if code != 0 || !strings.Contains(out, "\ntime is up after 1 of 2 questions\n") || strings.Contains(out, "Q2.") ||
		!strings.Contains(out, "score: 1 of 1 graded right (100%)") {
		t.Errorf("exit %d:\n%s%s", code, out, errb)
	}
}

// Without --seed the clock gives the seed, printed; that seed draws the same order again.
func TestExamDefaultSeed(t *testing.T) {
	fixNow(t, time.Unix(0, 1000042))
	root := examRoot(t)
	_, a, _ := runIn("\ns\n\ns\n", nil, "--root", root, "exam", "--history", filepath.Join(t.TempDir(), "h"))
	if !strings.HasPrefix(a, "exam: 2 of 2 problems (EXAM), seed 42, no time limit\n") {
		t.Fatalf("header:\n%s", a)
	}
	_, b, _ := runIn("\ns\n\ns\n", nil, "--root", root, "exam", "--seed", "42", "--history", filepath.Join(t.TempDir(), "h"))
	if a != b {
		t.Errorf("seed 42 drew another exam:\n%s\n%s", a, b)
	}
}

// Every station flag (and --date) is refused under --offline, and the total row and summary are
// the sums of the groups (L1 6, L2 4, LB 1, LA 2, RA1 7, RA2 1, RAX 1, EXAM 2).
func TestProgressOfflineEveryRefusalAndTotal(t *testing.T) {
	clearStationEnv(t)
	for _, f := range []string{"--station", "--install", "--source", "--labsrc", "--solutions", "--date"} {
		code, out, errb := run("--root", "testdata/root", "progress", "--offline", f, "x")
		if code != 2 || out != "" || !strings.Contains(errb, "error: --offline reads no station: drop "+f) {
			t.Errorf("%s: exit %d, stdout %q, stderr %q", f, code, out, errb)
		}
	}
	_, out, errb := run("--root", "testdata/root", "progress", "--offline")
	if !strings.Contains(out, "\ntotal     24     4          6    14\n") ||
		errb != "progress (offline) 24: done 4, needs a run 6, todo 14; no station read\n" {
		t.Errorf("stdout:\n%s\nstderr %q", out, errb)
	}
}
