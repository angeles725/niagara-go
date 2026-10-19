package course

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/course/check"
	"github.com/angeles725/niagara-go/internal/course/key"
)

// progressRow is one group of course progress: its items, the items at least one check links
// (checked), the verdicts of verify over every item, and the items already marked verified by
// evidence (key.Counts Live and Offline).
type progressRow struct {
	Group        string `json:"group"`
	Items        int    `json:"items"`
	Checked      int    `json:"checked"`
	Confirmed    int    `json:"confirmed"`
	Partial      int    `json:"partial"`
	Contradicted int    `json:"contradicted"`
	Undecided    int    `json:"undecided"`
	Live         int    `json:"verified_live"`
	Offline      int    `json:"verified_offline"`
}

func (r *progressRow) add(p check.Prediction) {
	r.Items++
	if len(p.Results) > 0 {
		r.Checked++
	}
	switch p.Verdict {
	case check.Confirmed:
		r.Confirmed++
	case check.Partial:
		r.Partial++
	case check.Contradicted:
		r.Contradicted++
	default:
		r.Undecided++
	}
	if p.Item.HasEvidence(key.EvidenceLive) {
		r.Live++
	}
	if p.Item.HasEvidence(key.EvidenceOffline) {
		r.Offline++
	}
}

func (r *progressRow) values() []int {
	return []int{r.Items, r.Checked, r.Confirmed, r.Partial, r.Contradicted, r.Undecided, r.Live, r.Offline}
}

// action is one next step: "redo" a FAIL check (what it found, then the steps it links) or "mark"
// a CONFIRMED item that carries no verified marker (the ready-to-apply line).
type action struct {
	Kind  string       `json:"kind"`  // "redo" or "mark"
	ID    string       `json:"id"`    // the check (redo) or the item (mark)
	Group string       `json:"group"` // the answer-key group (L4, RA1) of the first item it points at
	Text  string       `json:"text"`  // the check's description and evidence (redo) or the marker and evidence to add (mark)
	Items []actionItem `json:"items"` // redo: the selected items the check links; mark: the item
}

// actionItem is an item an action points at: where it is in the answer key and the course text,
// and its step.
type actionItem struct {
	ID     string `json:"id"`
	Source string `json:"source"` // labs.md:239
	Text   string `json:"text"`   // LI:576, "" when it points at no text
	Step   string `json:"step"`
}

func newActionItem(it *key.Item) actionItem {
	return actionItem{ID: it.ID, Source: fmt.Sprintf("%s:%d", it.File[strings.LastIndex(it.File, "/")+1:], it.Line),
		Text: textRef(it), Step: it.Step}
}

// groupLabel is how the text names a group: "lab 3", "lab B" for a lab (L3, LB), else the key
// (RA1, EXAM). JSON keeps the key.
func groupLabel(group string) string {
	if strings.HasPrefix(group, "L") {
		return "lab " + group[1:]
	}
	return group
}

type progressJSON struct {
	Groups  []progressRow `json:"groups"`
	Total   progressRow   `json:"total"`
	Actions []action      `json:"actions"`
}

// textRef is where an item points in the course text: "LI:576" for the lab instructions, else the
// file's base name and its ranges; "" when it points nowhere.
func textRef(it *key.Item) string {
	if it.TextFile == "" || len(it.Ranges) == 0 {
		return ""
	}
	rs := make([]string, len(it.Ranges))
	for i, r := range it.Ranges {
		rs[i] = r.String()
	}
	name := "LI"
	if it.TextFile != key.LabText {
		name = it.TextFile[strings.LastIndex(it.TextFile, "/")+1:]
	}
	return name + ":" + strings.Join(rs, ", ")
}

// progressActions lists a redo per FAIL check (once, at its first item, with every selected item it
// links) and a mark per CONFIRMED item without a verified marker, in answer-key order.
func progressActions(preds []check.Prediction, date string) []action {
	var out []action
	redo := map[string]int{} // check ID -> index in out
	for _, p := range preds {
		for _, r := range p.Results {
			if r.Verdict != check.Fail {
				continue
			}
			if i, ok := redo[r.ID]; ok {
				out[i].Items = append(out[i].Items, newActionItem(p.Item))
				continue
			}
			redo[r.ID] = len(out)
			out = append(out, action{Kind: "redo", ID: r.ID, Group: p.Item.Group, Items: []actionItem{newActionItem(p.Item)},
				Text: r.Desc + ": " + r.Evidence})
		}
		if p.Verdict == check.Confirmed && !p.Item.HasStatus("verified") {
			out = append(out, action{Kind: "mark", ID: p.Item.ID, Group: p.Item.Group, Items: []actionItem{newActionItem(p.Item)},
				Text: strings.TrimPrefix(check.Suggestion(p.Item.ID, date, p.Results), p.Item.ID+": ")})
		}
	}
	return out
}

// cmdProgress decides every item of the selected groups (all when groups is nil) from its linked
// checks, as verify does, and prints the table per group, then the next steps. Exit as verify: 0
// nothing contradicted; 1 at least one CONTRADICTED item (a FAIL check: a redo line); 2 the
// station cannot be read, a bad --date, or every linked check was skipped.
func cmdProgress(c *key.Corpus, s cli.Stdio, o stationOpts, groups []string, date string, asJSON bool) int {
	if date == "" {
		date = now().Format(time.DateOnly)
	} else if _, err := time.Parse(time.DateOnly, date); err != nil {
		fmt.Fprintf(s.Err, "course: --date %s is not a YYYY-MM-DD date\n", date)
		return cli.ExitCannotSearch
	}
	if groups == nil {
		for _, g := range c.Groups {
			groups = append(groups, g.Key)
		}
	}
	rows := map[string]*progressRow{}
	for _, g := range groups {
		rows[g] = &progressRow{Group: g}
	}
	keep := func(it *key.Item) bool { return rows[it.Group] != nil }
	linked := check.LinkedChecks(c, keep)
	st, code := o.open(s, linked)
	if st == nil {
		return code
	}
	defer st.Close()
	preds := check.Verify(c, st, keep)
	total := progressRow{Group: "total"}
	ran, skipped := map[string]bool{}, map[string]bool{}
	for _, p := range preds {
		rows[p.Item.Group].add(p)
		total.add(p)
		for _, r := range p.Results {
			ran[r.ID] = true
			if r.Verdict == check.Skip {
				skipped[r.ID] = true
			}
		}
	}
	ordered := make([]progressRow, len(groups))
	for i, g := range groups {
		ordered[i] = *rows[g]
	}
	actions := progressActions(preds, date)
	summary := s.Out
	if asJSON {
		summary = s.Err
		if !writeJSON(s, progressJSON{Groups: ordered, Total: total, Actions: append([]action{}, actions...)}) {
			return cli.ExitCannotSearch
		}
	} else {
		printProgress(s.Out, append(ordered, total), actions)
	}
	fmt.Fprintf(summary, "progress %d: checked %d, CONFIRMED %d, PARTIAL %d, CONTRADICTED %d, UNDECIDED %d; %s run; %s\n",
		total.Items, total.Checked, total.Confirmed, total.Partial, total.Contradicted, total.Undecided,
		key.Plural(len(ran), "check"), o.scope(st, linked))
	switch {
	case total.Contradicted > 0:
		return cli.ExitNoMatch
	case len(ran) > 0 && len(skipped) == len(ran):
		fmt.Fprintln(s.Err, "course: nothing could be checked (every linked check was skipped)")
		return cli.ExitCannotSearch
	}
	return cli.ExitFound
}

func printProgress(w io.Writer, rows []progressRow, actions []action) {
	headers := []string{"items", "checked", "CONFIRMED", "PARTIAL", "CONTRADICTED", "UNDECIDED", "live", "offline"}
	w0 := len("group")
	for _, r := range rows {
		w0 = max(w0, len(r.Group))
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-*s", w0, "group"))
	for _, h := range headers {
		b.WriteString("  " + h)
	}
	fmt.Fprintln(w, b.String())
	for _, r := range rows {
		b.Reset()
		b.WriteString(fmt.Sprintf("%-*s", w0, r.Group))
		for i, v := range r.values() {
			b.WriteString(fmt.Sprintf("  %*d", len(headers[i]), v))
		}
		fmt.Fprintln(w, b.String())
	}
	fmt.Fprintln(w, "checked: items at least one check links; live, offline: items already marked verified")
	if len(actions) == 0 {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "next steps (redo: a FAIL check, then the steps it links; mark: the line a CONFIRMED item without a marker takes):")
	for _, a := range actions {
		if a.Kind == "mark" {
			fmt.Fprintf(w, "mark %s (%s): %s\n", a.ID, a.Items[0].Source, a.Text)
			continue
		}
		fmt.Fprintf(w, "redo %s (%s): %s\n", a.ID, groupLabel(a.Group), a.Text)
		for _, it := range a.Items {
			where := it.Source
			if it.Text != "" {
				where += ", " + it.Text
			}
			fmt.Fprintf(w, "  %s (%s): %s\n", it.ID, where, truncate(it.Step, 100))
		}
	}
}
