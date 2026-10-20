package course

import (
	"fmt"
	"io"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/course/key"
)

// offlineRow is one group of progress --offline: its items split three ways by their markers.
type offlineRow struct {
	Group       string   `json:"group"`
	Items       int      `json:"items"`
	Done        int      `json:"done"`
	NeedsRun    int      `json:"needs_run"`
	Todo        int      `json:"todo"`
	NeedsRunIDs []string `json:"needs_run_ids"`
}

// offlineState is where an item stands from the answer keys alone: "done" when it carries a
// verified marker; else "needs-run" when its answer awaits a run (unverified, predicted, listed
// under "Answers that need a live run", or a needs-a-run check); else "todo".
func offlineState(it *key.Item) string {
	switch {
	case it.HasStatus("verified"):
		return "done"
	case it.HasStatus("unverified"), it.HasStatus("predicted"), it.LiveRun, it.Kind == "check":
		return "needs-run"
	}
	return "todo"
}

func (r *offlineRow) add(it *key.Item) {
	r.Items++
	switch offlineState(it) {
	case "done":
		r.Done++
	case "needs-run":
		r.NeedsRun++
		r.NeedsRunIDs = append(r.NeedsRunIDs, it.ID)
	default:
		r.Todo++
	}
}

type offlineJSON struct {
	Groups []offlineRow `json:"groups"`
	Total  offlineRow   `json:"total"`
}

// offlineFooter explains the columns of progress --offline.
const offlineFooter = "done: a verified marker; needs-run: unverified, predicted, a live-run answer or a " +
	"needs-a-run check, not verified; todo: the rest"

// cmdProgressOffline prints progress from the answer keys alone (no station): per group of groups
// (every group when nil), the items done, needing a run and to do, then the total; the summary
// goes to stderr. Exit 0; an unknown group was refused before (exit 1).
func cmdProgressOffline(c *key.Corpus, s cli.Stdio, groups []string, asJSON bool) int {
	if groups == nil {
		for _, g := range c.Groups {
			groups = append(groups, g.Key)
		}
	}
	rows := map[string]*offlineRow{}
	for _, g := range groups {
		rows[g] = &offlineRow{Group: g, NeedsRunIDs: []string{}}
	}
	total := offlineRow{Group: "total", NeedsRunIDs: []string{}}
	for _, it := range c.Items {
		if r := rows[it.Group]; r != nil {
			r.add(it)
			total.add(it)
		}
	}
	ordered := make([]offlineRow, len(groups))
	for i, g := range groups {
		ordered[i] = *rows[g]
	}
	if asJSON {
		if !writeJSON(s, offlineJSON{Groups: ordered, Total: total}) {
			return cli.ExitCannotSearch
		}
	} else {
		printOffline(s.Out, append(ordered, total))
	}
	fmt.Fprintf(s.Err, "progress (offline) %d: done %d, needs a run %d, todo %d; no station read\n",
		total.Items, total.Done, total.NeedsRun, total.Todo)
	return cli.ExitFound
}

func printOffline(w io.Writer, rows []offlineRow) {
	headers := []string{"items", "done", "needs-run", "todo"}
	w0 := len("group")
	for _, r := range rows {
		w0 = max(w0, len(r.Group))
	}
	fmt.Fprintf(w, "%-*s  %s\n", w0, "group", strings.Join(headers, "  "))
	for _, r := range rows {
		fmt.Fprintf(w, "%-*s  %*d  %*d  %*d  %*d\n", w0, r.Group, len(headers[0]), r.Items, len(headers[1]), r.Done,
			len(headers[2]), r.NeedsRun, len(headers[3]), r.Todo)
	}
	fmt.Fprintln(w, offlineFooter)
}
