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

// now is the clock of the default --date; tests replace it.
var now = time.Now

type predCheckJSON struct {
	ID       string `json:"id"`
	Verdict  string `json:"verdict"`
	Evidence string `json:"evidence"`
}

type predictionJSON struct {
	ID         string          `json:"id"`
	Verdict    string          `json:"verdict"`
	Reason     string          `json:"reason"`
	Checks     []predCheckJSON `json:"checks"`
	File       string          `json:"file"`
	Line       int             `json:"line"`
	Suggestion *string         `json:"suggestion"`
}

// cmdVerifyPredictions prints a verdict per predicted item, the suggestion lines of the
// CONFIRMED ones (only those) and a summary (stdout; stderr with --json). Exit: 0 nothing
// contradicted (PARTIAL counts like UNDECIDED), 1 at least one CONTRADICTED, 2 the station
// cannot be read, a bad --date, or every linked check was skipped.
func cmdVerifyPredictions(c *key.Corpus, s cli.Stdio, stationPath, install, date string, asJSON bool) int {
	if date == "" {
		date = now().Format(time.DateOnly)
	} else if _, err := time.Parse(time.DateOnly, date); err != nil {
		fmt.Fprintf(s.Err, "course: --date %s is not a YYYY-MM-DD date\n", date)
		return cli.ExitCannotSearch
	}
	preds, st, err := check.VerifyPredictions(c, stationPath, install)
	if err != nil {
		fmt.Fprintf(s.Err, "course: cannot check the station: %v\n", err)
		return cli.ExitCannotSearch
	}
	defer st.Close()
	counts := map[string]int{}
	ran, skipped := map[string]bool{}, map[string]bool{}
	for _, p := range preds {
		counts[p.Verdict]++
		for _, r := range p.Results {
			ran[r.ID] = true
			if r.Verdict == check.Skip {
				skipped[r.ID] = true
			}
		}
	}
	summary := s.Out
	if asJSON {
		summary = s.Err // keep stdout one JSON document
		out := make([]predictionJSON, len(preds))
		for i, p := range preds {
			j := predictionJSON{ID: p.Item.ID, Verdict: p.Verdict, Reason: p.Reason, Checks: []predCheckJSON{},
				File: p.Item.File, Line: p.Item.Line}
			for _, r := range p.Results {
				j.Checks = append(j.Checks, predCheckJSON{ID: r.ID, Verdict: r.Verdict, Evidence: r.Evidence})
			}
			if p.Verdict == check.Confirmed {
				line := check.Suggestion(p.Item.ID, date, p.Results)
				j.Suggestion = &line
			}
			out[i] = j
		}
		writeJSON(s.Out, out)
	} else {
		printPredictions(s.Out, preds, date)
	}
	fmt.Fprintf(summary, "predicted %d: CONFIRMED %d, PARTIAL %d, CONTRADICTED %d, UNDECIDED %d; %s run; %s, install %s\n",
		len(preds), counts[check.Confirmed], counts[check.Partial], counts[check.Contradicted], counts[check.Undecided], key.Plural(len(ran), "check"),
		st.Scope(), orNone(install))
	switch {
	case counts[check.Contradicted] > 0:
		return cli.ExitNoMatch
	case len(ran) > 0 && len(skipped) == len(ran):
		fmt.Fprintln(s.Err, "course: nothing could be checked (every linked check was skipped)")
		return cli.ExitCannotSearch
	}
	return cli.ExitFound
}

func printPredictions(w io.Writer, preds []check.Prediction, date string) {
	idW, vW := len("ID"), len("VERDICT")
	for _, p := range preds {
		idW, vW = max(idW, len(p.Item.ID)), max(vW, len(p.Verdict))
	}
	row := func(id, verdict, reason string) {
		fmt.Fprintln(w, strings.TrimRight(fmt.Sprintf("%-*s  %-*s  %s", idW, id, vW, verdict, reason), " "))
	}
	row("ID", "VERDICT", "EVIDENCE")
	var lines []string
	for _, p := range preds {
		row(p.Item.ID, p.Verdict, p.Reason)
		if p.Verdict == check.Confirmed {
			lines = append(lines, check.Suggestion(p.Item.ID, date, p.Results))
		}
	}
	if len(lines) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "suggested updates (CONFIRMED only; course never edits the answer key):")
		for _, l := range lines {
			fmt.Fprintln(w, l)
		}
	}
}
