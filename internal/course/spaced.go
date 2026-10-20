package course

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/course/key"
)

// Spaced review uses Leitner boxes folded from the history (D5): an item starts in box 1, a right
// answer moves it up one box (at most leitnerBoxes), a wrong one back to box 1; it is due once the
// clock reaches its last record plus 2^(box-1) days.
const leitnerBoxes = 5

// itemState is what the history says of one item: its box, its last record and that record's
// result.
type itemState struct {
	box    int
	last   time.Time
	result string
}

// due reports whether the item is due for review at at.
func (st *itemState) due(at time.Time) bool {
	return !at.Before(st.last.Add(time.Duration(1<<(st.box-1)) * 24 * time.Hour))
}

// foldHistory folds the records, in time order (file order between equal times), into the state of
// each item the keys hold; a record of an ID the keys no longer hold is ignored.
func foldHistory(c *key.Corpus, recs []record) map[*key.Item]*itemState {
	recs = slices.Clone(recs)
	slices.SortStableFunc(recs, func(a, b record) int { return a.at().Compare(b.at()) })
	out := map[*key.Item]*itemState{}
	for _, r := range recs {
		it := c.Lookup(r.ID)
		if it == nil {
			continue
		}
		st := out[it]
		if st == nil {
			st = &itemState{box: 1}
			out[it] = st
		}
		if r.Result == resultRight {
			st.box = min(st.box+1, leitnerBoxes)
		} else {
			st.box = 1
		}
		st.last, st.result = r.at(), r.Result
	}
	return out
}

// fromHistory keeps the pool questions the history selects: missed (the latest record is wrong)
// or due at at (Leitner, above). Questions never answered are neither.
func fromHistory(pool []question, states map[*key.Item]*itemState, missed bool, at time.Time) []question {
	var out []question
	for _, q := range pool {
		st := states[q.it]
		switch {
		case st == nil:
		case missed && st.result == resultWrong, !missed && st.due(at):
			out = append(out, q)
		}
	}
	return out
}

// historyPool narrows the quiz pool to the missed or due questions of the history and returns it
// with the scope label naming the selection; nil and the exit when the history cannot be read
// (2) or selects no question of the pool (1, the honest zero, with the record count).
func historyPool(c *key.Corpus, s cli.Stdio, o quizOpts, pool []question, kinds []string, scopeLabel, history string,
	at time.Time) ([]question, string, int) {
	recs, err := loadHistory(s, history)
	if err != nil {
		fmt.Fprintf(s.Err, "course: history %s: %v\n", history, err)
		return nil, "", cli.ExitCannotSearch
	}
	which := "missed"
	if o.due {
		which = "due"
	}
	if pool = fromHistory(pool, foldHistory(c, recs), o.missed, at); len(pool) == 0 {
		fmt.Fprintf(s.Err, "course: no %s %s question in %s (history %s: %s)\n", which, strings.Join(kinds, ", "),
			scopeLabel, history, key.Plural(len(recs), "record"))
		return nil, "", cli.ExitNoMatch
	}
	scopeLabel += "; " + which
	if o.due {
		scopeLabel += " " + at.Format(time.RFC3339)
	}
	return pool, scopeLabel, cli.ExitFound
}
