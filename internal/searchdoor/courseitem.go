package searchdoor

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/angeles725/niagara-go/internal/course/key"
	"github.com/angeles725/niagara-go/internal/pypath"
)

// Item-ID routing (decision D41): a query that names an item of the N4 developer
// course's answer keys leads with that item, exactly as `course show <ID>` prints it,
// before the three sources (which still answer the whole query). The keys are
// labs.md, review-assignments.md and certification-exam.md under
// <keys>/docs/n4-dev-course-solutions, parsed by course/key (non-printing) and rendered
// by showLines exactly as `course show` prints them.
//
// An ID is a whole token of the query (split at anything that is not a letter,
// a digit or a dot; trailing dots dropped) in the course package's ID grammar,
// upper case as the keys write it:
//
//	L<lab>.<n>  LB.<n>  LA.<n>      lab items (L10.23)
//	RA<k>.<X><n>[a-z]  RA<k>.S      review requirements (RA1.V12 finds RA1.V9–V14)
//	D<k>.<n>  T<k>.<n>  X<n>        review decisions, traps, needs-a-run checks
//	P<n>                            certification-exam problems
//
// Only a token that names an item routes: a token the keys do not hold stays
// ordinary query text; a missing or unparsable answer key is the lead's typed
// UNAVAILABLE. At most maxCourseIDs tokens are looked up, against one key.Load.

const (
	courseItemTitle = "COURSE ITEM · answer key"
	maxCourseIDs    = 5
)

var courseIDRx = regexp.MustCompile(`^(?:L\d+\.\d+|L[AB]\.\d+|RA\d+\.(?:[A-Z]\d+[a-z]?|S)|[DT]\d+\.\d+|[XP]\d+)$`)

// courseItemIDs are the item-ID tokens of query, once each, in query order.
func courseItemIDs(query string) []string {
	var ids []string
	seen := map[string]bool{}
	for _, tok := range strings.FieldsFunc(query, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.')
	}) {
		tok = strings.TrimRight(tok, ".")
		if courseIDRx.MatchString(tok) && !seen[tok] && len(ids) < maxCourseIDs {
			seen[tok] = true
			ids = append(ids, tok)
		}
	}
	return ids
}

// showLines renders it exactly as `course show` does (course/commands.go printItem):
// the header line "<ID> (<kind>)", then the field and answer lines.
// TestShowLinesMatchCourseShow pins the two together.
func showLines(it *key.Item) []string {
	var ls []string
	field := func(label, value string) { ls = append(ls, fmt.Sprintf("  %-10s%s", label+":", value)) }
	ls = append(ls, fmt.Sprintf("%s (%s)", it.ID, it.Kind))
	section := it.Section
	if it.Subsection != "" {
		section += " / " + it.Subsection
	}
	field("section", section)
	if len(it.Covers) > 0 {
		field("covers", strings.Join(it.Covers, ", "))
	}
	field("step", it.Step)
	text := "(none)"
	if it.TextFile != "" && len(it.Ranges) > 0 {
		rs := make([]string, len(it.Ranges))
		for i, r := range it.Ranges {
			rs[i] = r.String()
		}
		text = it.TextFile + ":" + strings.Join(rs, ", ")
	}
	field("text", text)
	field("status", strings.Join(it.StatusList(), ", "))
	if it.LiveRun {
		field("live run", "needed")
	}
	field("source", fmt.Sprintf("%s:%d", it.File, it.Line))
	ls = append(ls, "  answer:")
	for _, l := range strings.Split(it.Answer, "\n") {
		if strings.TrimSpace(l) == "" {
			ls = append(ls, "")
		} else {
			ls = append(ls, "    "+l)
		}
	}
	for len(ls) > 0 && ls[len(ls)-1] == "" {
		ls = ls[:len(ls)-1]
	}
	return ls
}

// courseItems is the lead outcome for a query naming course items, or nil.
func (d *door) courseItems(query, keys string) (o *Outcome) {
	ids := courseItemIDs(query)
	if len(ids) == 0 {
		return nil
	}
	command := "course show " + strings.Join(ids, " ")
	defer func() {
		if r := recover(); r != nil {
			o = unavailable("course", courseItemTitle, command,
				fmt.Sprintf("unavailable: the front door failed on the course item: panic: %v", r))
		}
	}()
	if !key.HasMarker(keys) {
		return unavailable("course", courseItemTitle, command,
			"unavailable: "+pypath.PurePosixPath(keys)+" has no "+key.LabsFile+" (set NSEARCH_COURSE_KEYS)")
	}
	c, problems := key.Load(keys)
	if c == nil {
		return unavailable("course", courseItemTitle, command, "unavailable: "+strings.Join(problems, "; "))
	}
	var found []string
	var items []Item
	seen := map[string]bool{}
	for _, id := range ids {
		it := c.Lookup(id)
		if it == nil { // not an item: ordinary query text
			continue
		}
		found = append(found, id)
		if !seen[it.ID] { // L9.9 and L9.13 can be one range item
			seen[it.ID] = true
			ls := showLines(it)
			row := keyed(it.ID, ls[0], "")
			row.Body = ls[1:]
			items = append(items, row)
		}
	}
	if len(items) == 0 {
		return nil
	}
	o = newOutcome("course", courseItemTitle, "course show "+strings.Join(found, " "), OK)
	o.Items, o.Enumeration, o.Limit = items, true, maxCourseIDs
	return o
}
