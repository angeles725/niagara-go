package check

import (
	"fmt"
	"slices"
	"strings"

	"github.com/angeles725/niagara-go/internal/course/key"
)

// Verdicts of verify (and verify-predictions) for one item. CONFIRMED needs every linked check PASS and one of
// them observing the whole answer (no Unobserved note for the item); PARTIAL is every check PASS
// but each observes only the stored part; one FAIL contradicts; anything else (no check, a SKIP)
// is undecided.
const (
	Confirmed    = "CONFIRMED"
	Contradicted = "CONTRADICTED"
	Undecided    = "UNDECIDED"
	Partial      = "PARTIAL"
)

// Prediction is one answer-key item verify decides (a predicted one for verify-predictions) and the
// checks that link it.
type Prediction struct {
	Item    *key.Item
	Results []LinkedResult // the checks whose keys resolve to the item, once each, in spec order
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

// LinkedResult is a link's check run on the station.
type LinkedResult struct {
	Result
	Notes []string
}

// itemLinks resolves every key of the spec to its item, as verify-predictions and
// TestCheckKeysReal both do: a note is read under the key that matched, never the item's ID, and
// a check whose several keys reach one item is one link carrying every matched note.
func itemLinks(c *key.Corpus, spec []Check) map[*key.Item][]link {
	links := map[*key.Item][]link{}
	for i := range spec {
		ck := &spec[i]
		for _, k := range ck.Keys {
			it := c.Lookup(k)
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

func joinResults(results []LinkedResult, verdict string) string {
	var parts []string
	for _, r := range results {
		if verdict == "" || r.Verdict == verdict {
			parts = append(parts, fmt.Sprintf("%s %s: %s", r.ID, r.Verdict, r.Evidence))
		}
	}
	return strings.Join(parts, "; ")
}

// decide turns the results of an item's checks into its verdict and reason.
func Decide(results []LinkedResult) (string, string) {
	if len(results) == 0 {
		return Undecided, "no check covers it"
	}
	fail, skip := false, false
	for _, r := range results {
		fail = fail || r.Verdict == Fail
		skip = skip || r.Verdict == Skip
	}
	switch {
	case fail:
		return Contradicted, joinResults(results, Fail)
	case skip:
		return Undecided, joinResults(results, Skip)
	}
	var notes []string
	for _, r := range results {
		if len(r.Notes) == 0 {
			return Confirmed, joinResults(results, "") // this check observes the whole answer
		}
		for _, n := range r.Notes {
			if !slices.Contains(notes, n) {
				notes = append(notes, n)
			}
		}
	}
	return Partial, joinResults(results, "") + "; not observed: " + strings.Join(notes, "; ")
}

// Evidence is the evidence kind of the marker the checks of an item give it: live only when every
// one of them reads a station (evidenceOf), offline otherwise (a jar, the source tree or the lab
// sources decided at least part of it).
func Evidence(results []LinkedResult) string {
	for _, r := range results {
		if evidenceOf[r.Kind] != key.EvidenceLive {
			return key.EvidenceOffline
		}
	}
	return key.EvidenceLive
}

// Suggestion is the line a human (or an agent) applies to a CONFIRMED item, with the marker of
// its Evidence; course never edits the answer key.
func Suggestion(id, date string, results []LinkedResult) string {
	ev := make([]string, len(results))
	for i, r := range results {
		ev[i] = r.ID + " " + r.Evidence
	}
	return fmt.Sprintf("%s: *(verified %s %s)* evidence: %s", id, Evidence(results), date, strings.Join(ev, "; "))
}

// LinkedChecks are the checks linked to at least one item keep selects, once each, in item order.
func LinkedChecks(c *key.Corpus, keep func(*key.Item) bool) []*Check {
	links := itemLinks(c, Spec)
	var checks []*Check
	seen := map[*Check]bool{}
	for _, it := range c.Items {
		for _, l := range links[it] {
			if keep(it) && !seen[l.Check] {
				seen[l.Check] = true
				checks = append(checks, l.Check)
			}
		}
	}
	return checks
}

// Verify runs, on the stations, every check linked to an item keep selects (only those checks)
// and decides each such item.
func Verify(c *key.Corpus, ss *Stations, keep func(*key.Item) bool) []Prediction {
	links := itemLinks(c, Spec)
	results := map[*Check]Result{}
	for _, ck := range LinkedChecks(c, keep) {
		results[ck] = ss.Run(ck)
	}
	var preds []Prediction
	for _, it := range c.Items {
		if !keep(it) {
			continue
		}
		p := Prediction{Item: it}
		for _, l := range links[it] {
			p.Results = append(p.Results, LinkedResult{Result: results[l.Check], Notes: l.Notes})
		}
		p.Verdict, p.Reason = Decide(p.Results)
		preds = append(preds, p)
	}
	return preds
}
