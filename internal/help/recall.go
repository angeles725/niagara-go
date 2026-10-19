package help

// Recall oracle (recall.py, `selftest --recall`): every item the raw sources
// declare must be found by the resolver of the command that looks it up.
// Rows report expected / found / missed with the first misses; a row with
// a miss fails the run (exit 1). find and fuzzy scan the whole corpus per
// query, so they run on a fixed, evenly spaced sample.

import (
	"fmt"
	"os"
	"regexp"
	"sort"

	"github.com/angeles725/niagara-go/internal/help/scope"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// sortStable is sorted() with a less function (a stable sort).
func sortStable[T any](s []T, less func(a, b T) bool) {
	sort.SliceStable(s, func(i, j int) bool { return less(s[i], s[j]) })
}

const (
	recallFirstMisses = 10 // FIRST_MISSES
	recallSample      = 12 // SAMPLE
)

// recallEnumRoots are ENUM_ROOTS (and enum_search's enum parents).
var recallEnumRoots = map[string]bool{"BFrozenEnum": true, "BDynamicEnum": true, "BEnum": true, "BBitString": true}

// recallRow is one row of the table.
type recallRow struct {
	command                 string
	expected, found, missed int
	first                   []string
	note                    string
}

// newRow is recall._row: count the items found and keep the first misses.
func newRow[T any](command string, truth []T, label func(T) string, found func(T) bool, note string) recallRow {
	r := recallRow{command: command, expected: len(truth), note: note}
	for _, x := range truth {
		if !found(x) {
			r.missed++
			if len(r.first) < recallFirstMisses {
				r.first = append(r.first, label(x))
			}
		}
	}
	r.found = r.expected - r.missed
	return r
}

// recallSampleOf is recall._sample: n items evenly spaced over items.
func recallSampleOf[T any](items []T, n int) []T {
	if len(items) <= n {
		return items
	}
	step := float64(len(items)) / float64(n)
	out := make([]T, n)
	for i := range n {
		out[i] = items[int(float64(i)*step)]
	}
	return out
}

var noGenerics = regexp.MustCompile(`&lt;.*?&gt;|<.*?>`)

func classQName(k classKey) string { return k.pkg + "." + k.name }

func memberLabel(m memberKey) string { return m.pkg + "." + m.class + "#" + m.member }

// checkAll is recall.check_all: the rows of every recall family, in the
// table's order. A resolver error fails the run after every row ran; an
// error expanding the class index or running a sample fails it at once.
func (e *Env) checkAll(samples bool) ([]recallRow, float64, error) {
	c, err := e.newRecallCheck()
	if err != nil {
		return nil, 0, err
	}
	rows, err := c.classRows()
	if err != nil {
		return nil, 0, err
	}
	rows = append(rows, c.implementorsRow(), c.enumRow(), c.moduleRow(), c.jdkRow())
	rows = append(rows, c.memberRows()...)
	rows = append(rows, c.sourceRows()...)
	rows = append(rows, c.guideRows()...)
	if samples {
		sampled, err := c.sampleRows()
		if err != nil {
			return nil, 0, err
		}
		rows = append(rows, sampled...)
	}
	if c.firstErr != nil {
		return nil, 0, c.firstErr
	}
	rows = append(rows, recallRow{command: "callers", note: "excluded: keyed by call sites; a method " +
		"with no caller has no entry by definition (its class is covered by source/xref recall)"})
	return rows, c.truth.seconds, nil
}

// strEq is `value == s` for a decoded JSON value and a string.
func strEq(v any, s string) bool {
	x, ok := v.(string)
	return ok && x == s
}

// isRegularFile is os.path.isfile.
func isRegularFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}

// Recall is recall.cmd_recall: print the table; exit 1 when a row missed.
func (e *Env) Recall(samples bool) error {
	rows, seconds, err := e.checkAll(samples)
	if err != nil {
		return err
	}
	vid := scope.Identify(e.BaseDir)
	if vid == "" {
		vid = e.BaseDir
	}
	fmt.Fprintf(e.Out, "RECALL %s (ground truth from raw sources, parsed in %.0fs)\n", vid, seconds)
	fmt.Fprintf(e.Out, "  %s %s %s %s  note\n", pycompat.LJust("command", 18, ' '), pycompat.RJust("expected", 9, ' '),
		pycompat.RJust("found", 9, ' '), pycompat.RJust("missed", 8, ' '))
	bad := 0
	for _, r := range rows {
		fmt.Fprintf(e.Out, "  %s %s %s %s  %s\n", pycompat.LJust(r.command, 18, ' '), pycompat.RJust(pycompat.Commas(r.expected), 9, ' '),
			pycompat.RJust(pycompat.Commas(r.found), 9, ' '), pycompat.RJust(pycompat.Commas(r.missed), 8, ' '), r.note)
		for _, m := range r.first {
			fmt.Fprintf(e.Out, "      miss: %s\n", m)
		}
		if r.missed > 0 {
			bad++
		}
	}
	if bad > 0 {
		fmt.Fprintf(e.Out, "RESULT: FAIL (%d command(s) with misses)\n", bad)
		return ExitCode(1)
	}
	fmt.Fprintln(e.Out, "RESULT: PASS (100% recall)")
	return ExitCode(0)
}
