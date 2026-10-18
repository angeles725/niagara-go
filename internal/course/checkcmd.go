package course

import (
	"fmt"
	"io"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/course/check"
)

type resultJSON struct {
	ID          string   `json:"id"`
	Lab         string   `json:"lab"`
	Keys        []string `json:"keys"`
	Kind        string   `json:"kind"`
	Description string   `json:"description"`
	Verdict     string   `json:"verdict"`
	Evidence    string   `json:"evidence"`
}

// cmdCheck runs the selected checks and prints a table (or a JSON list) and a summary line
// (stdout; stderr with --json). Exit: 0 no FAIL (SKIPs are counted, never passed),
// 1 at least one FAIL, 2 the station cannot be read or nothing could be checked.
func cmdCheck(s cli.Stdio, stationPath, install string, labs []string, asJSON bool) int {
	checks, unknown := check.Select(labs)
	if unknown != "" {
		fmt.Fprintf(s.Err, "course: no check for lab %s (labs with checks: %s; lab 8 changes nothing)\n",
			unknown, strings.Join(check.Labs(), ", "))
		return cli.ExitNoMatch
	}
	st, err := check.Open(stationPath, install, check.NeedsNested(checks))
	if err != nil {
		fmt.Fprintf(s.Err, "course: cannot check the station: %v\n", err)
		return cli.ExitCannotSearch
	}
	defer st.Close()
	results := make([]check.Result, len(checks))
	pass, fail, skip := 0, 0, 0
	for i, ck := range checks {
		results[i] = st.Run(ck)
		switch results[i].Verdict {
		case check.Pass:
			pass++
		case check.Fail:
			fail++
		default:
			skip++
		}
	}
	summary := s.Out
	if asJSON {
		summary = s.Err // keep stdout one JSON document
		out := make([]resultJSON, len(results))
		for i, r := range results {
			out[i] = resultJSON{ID: r.ID, Lab: r.Lab, Keys: append([]string{}, r.Keys...), Kind: r.Kind,
				Description: r.Desc, Verdict: r.Verdict, Evidence: r.Evidence}
		}
		writeJSON(s.Out, out)
	} else {
		printResults(s.Out, results)
	}
	scope := fmt.Sprintf("%s, install %s", st.Scope(), orNone(install))
	fmt.Fprintf(summary, "checked %d: PASS %d, FAIL %d, SKIP %d; %s\n", len(results), pass, fail, skip, scope)
	switch {
	case fail > 0:
		return cli.ExitNoMatch
	case pass == 0:
		fmt.Fprintln(s.Err, "course: nothing could be checked (every check was skipped)")
		return cli.ExitCannotSearch
	}
	return cli.ExitFound
}

func orNone(s string) string {
	if s == "" {
		return "(none: jar checks skipped)"
	}
	return s
}

func printResults(w io.Writer, results []check.Result) {
	head := []string{"ID", "LAB", "VERDICT", "DESCRIPTION", "EVIDENCE"}
	rows := [][]string{head}
	for _, r := range results {
		rows = append(rows, []string{r.ID, r.Lab, r.Verdict, r.Desc + " [" + strings.Join(r.Keys, " ") + "]", r.Evidence})
	}
	width := make([]int, len(head)-1)
	for _, row := range rows {
		for i := range width {
			width[i] = max(width[i], len([]rune(row[i])))
		}
	}
	for _, row := range rows {
		var b strings.Builder
		for i := range width {
			b.WriteString(row[i])
			b.WriteString(strings.Repeat(" ", width[i]-len([]rune(row[i]))+2))
		}
		b.WriteString(row[len(row)-1])
		fmt.Fprintln(w, strings.TrimRight(b.String(), " "))
	}
}
