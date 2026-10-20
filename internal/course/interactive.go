package course

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/course/key"
)

// session is one self-graded run over drawn questions: quiz --interactive and exam.
type session struct {
	s        cli.Stdio
	in       *bufio.Reader
	history  string              // the file the graded answers are appended to
	texts    map[string][]string // the course text per file, nil under --no-context
	deadline time.Time           // stop asking once now() reaches it; zero = no limit
}

// tally counts the answers of a session.
type tally struct {
	asked, right, wrong, skipped int
	stop                         string // why the session ended early, "" when every question was asked
}

// readLine is the next line of input without its line end; false at the end of input.
func (ss *session) readLine() (string, bool) {
	line, err := ss.in.ReadString('\n')
	if err != nil && line == "" {
		return "", false
	}
	return strings.TrimRight(line, "\r\n"), true
}

// The grades grade returns besides resultRight and resultWrong: a skip (not recorded) and the end
// of input (the session stops).
const (
	gradeSkip = "skip"
	gradeEnd  = "end of input"
)

// grade reads a self-grade: y (or yes) right, n (or no) wrong, s (or skip) gradeSkip; anything else
// asks again. gradeEnd at the end of input.
func (ss *session) grade() string {
	fmt.Fprint(ss.s.Out, "grade: y right, n wrong, s skip? ")
	for {
		line, ok := ss.readLine()
		if !ok {
			fmt.Fprintln(ss.s.Out)
			return gradeEnd
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return resultRight
		case "n", "no":
			return resultWrong
		case "s", "skip":
			return gradeSkip
		}
		fmt.Fprint(ss.s.Out, "grade y, n or s? ")
	}
}

// ask runs the questions in order: the question, its answer once Enter is read (so you answer
// before you see it), then a self-grade; each right or wrong grade is appended to the history at
// once, so an interrupted session keeps what it graded. It stops at the end of input or at the
// deadline. The error is the history's: the session stops at the first answer it cannot record.
func (ss *session) ask(drawn []question) (tally, error) {
	var t tally
	for i, q := range drawn {
		if !ss.deadline.IsZero() && !now().Before(ss.deadline) {
			t.stop = fmt.Sprintf("time is up after %d of %d questions", t.asked, len(drawn))
			break
		}
		t.asked++
		fmt.Fprintln(ss.s.Out)
		printQuestion(ss.s.Out, i+1, q)
		fmt.Fprint(ss.s.Out, "(Enter shows the answer) ")
		if _, ok := ss.readLine(); !ok {
			fmt.Fprintln(ss.s.Out)
			t.stop = fmt.Sprintf("end of input after %d of %d questions", t.asked, len(drawn))
			break
		}
		printAnswer(ss.s.Out, i+1, q, ss.texts[q.it.TextFile])
		g := ss.grade()
		if g == gradeEnd {
			t.stop = fmt.Sprintf("end of input after %d of %d questions", t.asked, len(drawn))
			break
		}
		switch g {
		case gradeSkip:
			t.skipped++
			continue
		case resultRight:
			t.right++
		default:
			t.wrong++
		}
		r := record{Time: now().UTC().Format(time.RFC3339), ID: q.it.ID, Kind: q.kind, Group: q.it.Group, Result: g}
		if err := appendRecord(ss.history, r); err != nil {
			return t, err
		}
	}
	return t, nil
}

// printScore prints why the session stopped early, then the score of the graded answers.
func (ss *session) printScore(t tally) {
	w := ss.s.Out
	fmt.Fprintln(w)
	if t.stop != "" {
		fmt.Fprintln(w, t.stop)
	}
	graded := t.right + t.wrong
	fmt.Fprintf(w, "score: %d of %d graded right", t.right, graded)
	if graded > 0 {
		fmt.Fprintf(w, " (%d%%)", (200*t.right+graded)/(2*graded)) // rounded half up
	}
	if t.skipped > 0 {
		fmt.Fprintf(w, ", %d skipped", t.skipped)
	}
	if graded == 0 {
		fmt.Fprintln(w, "; nothing recorded")
		return
	}
	fmt.Fprintf(w, "; %s recorded in %s\n", key.Plural(graded, "answer"), ss.history)
}

// runSession asks drawn interactively, recording to path, and prints the score. Exit 0, or 2 when
// an answer cannot be recorded.
func runSession(s cli.Stdio, drawn []question, texts map[string][]string, path string, deadline time.Time) int {
	in := s.In
	if in == nil {
		in = strings.NewReader("")
	}
	ss := &session{s: s, in: bufio.NewReader(in), history: path, texts: texts, deadline: deadline}
	t, err := ss.ask(drawn)
	if err != nil {
		fmt.Fprintf(s.Err, "course: history %v\n", err)
		return cli.ExitCannotSearch
	}
	ss.printScore(t)
	return cli.ExitFound
}

// printAnswer prints "A<n>. <ID> [<status>] <file>:<line>", the text before a mid-answer why, the
// answer and its course text (lines: the item's text file, nil for none).
func printAnswer(w io.Writer, n int, q question, lines []string) {
	fmt.Fprintf(w, "\nA%d. %s [%s] %s:%d\n", n, q.it.ID, strings.Join(q.it.StatusLabels(), ", "), q.it.File, q.it.Line)
	if q.before != "" {
		fmt.Fprintf(w, "    before the why: %s\n", q.before)
	}
	printIndented(w, q.answer)
	printContext(w, q.it, lines)
}
