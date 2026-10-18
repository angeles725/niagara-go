package course

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
)

// now is the clock of the default --date; tests replace it.
var now = time.Now

// Prediction verdicts of verify-predictions. CONFIRMED needs every linked check PASS and one of
// them observing the whole answer (no Unobserved note for the item); PARTIAL is every check PASS
// but each observes only the stored part; one FAIL contradicts; anything else (no check, a SKIP)
// is undecided.
const (
	predConfirmed    = "CONFIRMED"
	predContradicted = "CONTRADICTED"
	predUndecided    = "UNDECIDED"
	predPartial      = "PARTIAL"
)

// prediction is one answer-key item still marked *(predicted)* and the checks that link it.
type prediction struct {
	Item    *Item
	Results []linkedResult // the checks whose keys resolve to the item, once each, in spec order
	Verdict string
	Reason  string // the evidence (CONFIRMED, CONTRADICTED; PARTIAL with what is not observed) or why it is undecided
}

// link is one check whose keys resolve to an item (Corpus.lookup: the ID, a range ID or a covered
// ID), once per check; Notes are the Unobserved notes of the matched keys (none = the check
// observes the whole answer).
type link struct {
	Check *Check
	Notes []string
}

// linkedResult is a link's check run on the station.
type linkedResult struct {
	Result
	Notes []string
}

// itemLinks resolves every key of the spec to its item, as verify-predictions and
// TestCheckKeysReal both do: a note is read under the key that matched, never the item's ID, and
// a check whose several keys reach one item is one link carrying every matched note.
func itemLinks(c *Corpus, spec []Check) map[*Item][]link {
	links := map[*Item][]link{}
	for i := range spec {
		ck := &spec[i]
		for _, k := range ck.Keys {
			it := c.lookup(k)
			if it == nil {
				continue
			}
			ls := links[it]
			if len(ls) == 0 || ls[len(ls)-1].Check != ck {
				ls = append(ls, link{Check: ck})
			}
			if note, ok := ck.Unobserved[k]; ok && !slices.Contains(ls[len(ls)-1].Notes, note) {
				ls[len(ls)-1].Notes = append(ls[len(ls)-1].Notes, note)
			}
			links[it] = ls
		}
	}
	return links
}

func joinResults(results []linkedResult, verdict string) string {
	var parts []string
	for _, r := range results {
		if verdict == "" || r.Verdict == verdict {
			parts = append(parts, fmt.Sprintf("%s %s: %s", r.ID, r.Verdict, r.Evidence))
		}
	}
	return strings.Join(parts, "; ")
}

// decide turns the results of an item's checks into its verdict and reason.
func decide(results []linkedResult) (string, string) {
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
	var notes []string
	for _, r := range results {
		if len(r.Notes) == 0 {
			return predConfirmed, joinResults(results, "") // this check observes the whole answer
		}
		for _, n := range r.Notes {
			if !slices.Contains(notes, n) {
				notes = append(notes, n)
			}
		}
	}
	return predPartial, joinResults(results, "") + "; not observed: " + strings.Join(notes, "; ")
}

// suggestion is the line a human (or an agent) applies to a CONFIRMED item; course never edits
// the answer key.
func suggestion(id, date string, results []linkedResult) string {
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
	links := itemLinks(c, checkSpec)
	var checks []*Check
	seen := map[*Check]bool{}
	for _, it := range c.Items {
		for _, l := range links[it] {
			if it.hasStatus("predicted") && !seen[l.Check] {
				seen[l.Check] = true
				checks = append(checks, l.Check)
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
		for _, l := range links[it] {
			p.Results = append(p.Results, linkedResult{Result: results[l.Check], Notes: l.Notes})
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
// CONFIRMED ones (only those) and a summary (stdout; stderr with --json). Exit: 0 nothing
// contradicted (PARTIAL counts like UNDECIDED), 1 at least one CONTRADICTED, 2 the station
// cannot be read, a bad --date, or every linked check was skipped.
func cmdVerifyPredictions(c *Corpus, s cli.Stdio, stationPath, install, date string, asJSON bool) int {
	if date == "" {
		date = now().Format(time.DateOnly)
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
	fmt.Fprintf(summary, "predicted %d: CONFIRMED %d, PARTIAL %d, CONTRADICTED %d, UNDECIDED %d; %s run; station %s (%s, %s), install %s\n",
		len(preds), counts[predConfirmed], counts[predPartial], counts[predContradicted], counts[predUndecided], plural(len(ran), "check"),
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
		fmt.Fprintln(w, "suggested updates (CONFIRMED only; course never edits the answer key):")
		for _, l := range lines {
			fmt.Fprintln(w, l)
		}
	}
}
