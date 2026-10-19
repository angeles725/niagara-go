package course

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/course/key"
)

type match struct {
	ID   string `json:"id"`
	Step string `json:"step"`
}

type statusJSON struct {
	Groups  []countRow `json:"groups"`
	Total   countRow   `json:"total"`
	Status  *string    `json:"status"`
	Matches []match    `json:"matches"`
}

const tableFooter = "an item counts under every status it carries; written = none of them;\n" +
	"live, offline: the verified items by the evidence of their markers (station, or jar/source)"

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// cmdStatus prints the per-group counts and, with --status, the matching items.
func cmdStatus(c *key.Corpus, s cli.Stdio, hasLab bool, lab, status string, asJSON bool) int {
	keys := make([]string, 0, len(c.Groups))
	for _, g := range c.Groups {
		keys = append(keys, g.Key)
	}
	if hasLab {
		if keys = c.GroupKeys(lab); len(keys) == 0 {
			fmt.Fprintf(s.Err, "course: no group %s (groups: %s)\n", lab, c.GroupList())
			return cli.ExitNoMatch
		}
	}
	selected := map[string]bool{}
	for _, k := range keys {
		selected[k] = true
	}
	rows := map[string]*countRow{}
	for _, k := range keys {
		rows[k] = &countRow{Group: k}
	}
	total := countRow{Group: "total"}
	matches := []match{}
	searched := 0
	for _, it := range c.Items {
		if !selected[it.Group] {
			continue
		}
		searched++
		rows[it.Group].Add(it)
		total.Add(it)
		if status != "" && it.HasStatus(status) {
			matches = append(matches, match{it.ID, it.Step})
		}
	}
	ordered := make([]countRow, len(keys))
	for i, k := range keys {
		ordered[i] = *rows[k]
	}
	if asJSON {
		out := statusJSON{Groups: ordered, Total: total, Matches: matches}
		if status != "" {
			out.Status = &status
		}
		if !writeJSON(s, out) {
			return cli.ExitCannotSearch
		}
	} else {
		printTable(s, append(ordered, total))
		if status != "" && len(matches) > 0 {
			fmt.Fprintf(s.Out, "\n%s: %s\n", status, key.Plural(len(matches), "item"))
			w := 0
			for _, m := range matches {
				w = max(w, len([]rune(m.ID)))
			}
			for _, m := range matches {
				fmt.Fprintf(s.Out, "%s%s  %s\n", m.ID, strings.Repeat(" ", w-len([]rune(m.ID))), truncate(m.Step, 80))
			}
		}
	}
	if status != "" && len(matches) == 0 {
		fmt.Fprintf(s.Err, "course: no %s item in %s (searched %s)\n", status, strings.Join(keys, ", "),
			key.Plural(searched, "item"))
		return cli.ExitNoMatch
	}
	return cli.ExitFound
}

func printTable(s cli.Stdio, rows []countRow) {
	headers := key.CountColumns
	w0 := len("group")
	for _, r := range rows {
		w0 = max(w0, len(r.Group))
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-*s", w0, "group"))
	for _, h := range headers {
		b.WriteString("  " + h)
	}
	fmt.Fprintln(s.Out, b.String())
	for _, r := range rows {
		b.Reset()
		b.WriteString(fmt.Sprintf("%-*s", w0, r.Group))
		for i, v := range r.Values() {
			b.WriteString(fmt.Sprintf("  %*d", len(headers[i]), v))
		}
		fmt.Fprintln(s.Out, b.String())
	}
	fmt.Fprintln(s.Out, tableFooter)
}

// countRow is one line of the status table (and its JSON object): the group, then key.Counts.
type countRow struct {
	Group string `json:"group"`
	key.Counts
}
