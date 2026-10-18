package course

import (
	"fmt"
	"io"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
)

// checkLab normalises a --lab spelling of the check command: 3, 03, L3 -> "3"; B, LB -> "B".
func checkLab(name string) string {
	n := strings.ToUpper(strings.TrimSpace(name))
	if len(n) > 1 && n[0] == 'L' {
		n = n[1:]
	}
	if n != "" && strings.Trim(n, "0123456789") == "" {
		n = strings.TrimLeft(n, "0")
	}
	return n
}

// checkLabs lists the labs that have checks, in spec order.
func checkLabs() []string {
	var labs []string
	seen := map[string]bool{}
	for _, ck := range checkSpec {
		if !seen[ck.Lab] {
			seen[ck.Lab] = true
			labs = append(labs, ck.Lab)
		}
	}
	return labs
}

// selectChecks returns the spec rows of the given labs (all when none), or the first lab
// that has no checks.
func selectChecks(labs []string) ([]*Check, string) {
	want := map[string]bool{}
	for _, l := range labs {
		n := checkLab(l)
		found := false
		for _, ck := range checkSpec {
			found = found || ck.Lab == n
		}
		if !found {
			return nil, l
		}
		want[n] = true
	}
	var out []*Check
	for i := range checkSpec {
		if len(want) == 0 || want[checkSpec[i].Lab] {
			out = append(out, &checkSpec[i])
		}
	}
	return out, ""
}

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
	checks, unknown := selectChecks(labs)
	if unknown != "" {
		fmt.Fprintf(s.Err, "course: no check for lab %s (labs with checks: %s; lab 8 changes nothing)\n",
			unknown, strings.Join(checkLabs(), ", "))
		return cli.ExitNoMatch
	}
	st, err := openStation(stationPath, install, needsNested(checks))
	if err != nil {
		fmt.Fprintf(s.Err, "course: cannot check the station: %v\n", err)
		return cli.ExitCannotSearch
	}
	defer st.close()
	results := make([]Result, len(checks))
	pass, fail, skip := 0, 0, 0
	for i, ck := range checks {
		results[i] = st.run(ck)
		switch results[i].Verdict {
		case verdictPass:
			pass++
		case verdictFail:
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
	scope := fmt.Sprintf("station %s (%s, %s), install %s", st.name, plural(st.b.Comps.Len(), "component"),
		plural(len(st.b.Links), "link"), orNone(install))
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

func printResults(w io.Writer, results []Result) {
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
