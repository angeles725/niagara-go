package course

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
)

// Prediction verdicts of verify-predictions. CONFIRMED needs at least one linked check and
// every one of them PASS; one FAIL contradicts; anything else (no check, a SKIP) is undecided.
const (
	predConfirmed    = "CONFIRMED"
	predContradicted = "CONTRADICTED"
	predUndecided    = "UNDECIDED"
)

// prediction is one answer-key item still marked *(predicted)* and the checks that link it.
type prediction struct {
	Item    *Item
	Results []Result // the results of the checks whose Keys name the item, in spec order
	Verdict string
	Reason  string // the evidence (CONFIRMED, CONTRADICTED) or why it is undecided
}

func joinResults(results []Result, verdict string) string {
	var parts []string
	for _, r := range results {
		if verdict == "" || r.Verdict == verdict {
			parts = append(parts, fmt.Sprintf("%s %s: %s", r.ID, r.Verdict, r.Evidence))
		}
	}
	return strings.Join(parts, "; ")
}

// decide turns the results of an item's checks into its verdict and reason.
func decide(results []Result) (string, string) {
	if len(results) == 0 {
		return predUndecided, "no check covers it"
	}
	fail, skip := false, false
	for _, r := range results {
		fail = fail || r.Verdict == verdictFail
		skip = skip || r.Verdict == verdictSkip
	}
	switch {
	case fail:
		return predContradicted, joinResults(results, verdictFail)
	case skip:
		return predUndecided, joinResults(results, verdictSkip)
	}
	return predConfirmed, joinResults(results, "")
}

// suggestion is the line a human (or an agent) applies to a CONFIRMED item; course never edits
// the answer key.
func suggestion(id, date string, results []Result) string {
	ev := make([]string, len(results))
	for i, r := range results {
		ev[i] = r.ID + " " + r.Evidence
	}
	return fmt.Sprintf("%s: *(verified live %s)* evidence: %s", id, date, strings.Join(ev, "; "))
}

// verifyPredictions runs, on the station, every check linked to an item still marked
// predicted, and decides each such item. Only the linked checks run. The caller closes the
// station.
func verifyPredictions(c *Corpus, path, install string) ([]prediction, *station, error) {
	var preds []prediction
	linked := map[*Item][]*Check{}
	var checks []*Check
	seen := map[*Check]bool{}
	for i := range checkSpec {
		ck := &checkSpec[i]
		for _, k := range ck.Keys {
			if it := c.lookup(k); it != nil && it.hasStatus("predicted") {
				linked[it] = append(linked[it], ck)
				if !seen[ck] {
					seen[ck] = true
					checks = append(checks, ck)
				}
			}
		}
	}
	st, err := openStation(path, install, needsNested(checks))
	if err != nil {
		return nil, nil, err
	}
	results := map[*Check]Result{}
	for _, ck := range checks {
		results[ck] = st.run(ck)
	}
	for _, it := range c.Items {
		if !it.hasStatus("predicted") {
			continue
		}
		p := prediction{Item: it}
		for _, ck := range linked[it] {
			p.Results = append(p.Results, results[ck])
		}
		p.Verdict, p.Reason = decide(p.Results)
		preds = append(preds, p)
	}
	return preds, st, nil
}

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
// CONFIRMED ones and a summary (stdout; stderr with --json). Exit: 0 nothing contradicted,
// 1 at least one CONTRADICTED, 2 the station cannot be read, a bad --date, or every linked
// check was skipped.
func cmdVerifyPredictions(c *Corpus, s cli.Stdio, stationPath, install, date string, asJSON bool) int {
	if date == "" {
		date = time.Now().Format(time.DateOnly)
	} else if _, err := time.Parse(time.DateOnly, date); err != nil {
		fmt.Fprintf(s.Err, "course: --date %s is not a YYYY-MM-DD date\n", date)
		return cli.ExitCannotSearch
	}
	preds, st, err := verifyPredictions(c, stationPath, install)
	if err != nil {
		fmt.Fprintf(s.Err, "course: cannot check the station: %v\n", err)
		return cli.ExitCannotSearch
	}
	defer st.close()
	counts := map[string]int{}
	ran, skipped := map[string]bool{}, map[string]bool{}
	for _, p := range preds {
		counts[p.Verdict]++
		for _, r := range p.Results {
			ran[r.ID] = true
			if r.Verdict == verdictSkip {
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
			if p.Verdict == predConfirmed {
				line := suggestion(p.Item.ID, date, p.Results)
				j.Suggestion = &line
			}
			out[i] = j
		}
		writeJSON(s.Out, out)
	} else {
		printPredictions(s.Out, preds, date)
	}
	fmt.Fprintf(summary, "predicted %d: CONFIRMED %d, CONTRADICTED %d, UNDECIDED %d; %s run; station %s (%s, %s), install %s\n",
		len(preds), counts[predConfirmed], counts[predContradicted], counts[predUndecided], plural(len(ran), "check"),
		st.name, plural(st.b.Comps.Len(), "component"), plural(len(st.b.Links), "link"), orNone(install))
	switch {
	case counts[predContradicted] > 0:
		return cli.ExitNoMatch
	case len(ran) > 0 && len(skipped) == len(ran):
		fmt.Fprintln(s.Err, "course: nothing could be checked (every linked check was skipped)")
		return cli.ExitCannotSearch
	}
	return cli.ExitFound
}

func printPredictions(w io.Writer, preds []prediction, date string) {
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
		if p.Verdict == predConfirmed {
			lines = append(lines, suggestion(p.Item.ID, date, p.Results))
		}
	}
	if len(lines) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "suggested updates (course never edits the answer key; the evidence covers what the station")
		fmt.Fprintln(w, "and the jars store, not a console, log or UI observation: read each item before applying):")
		for _, l := range lines {
			fmt.Fprintln(w, l)
		}
	}
}
