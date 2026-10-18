package searchdoor

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/course"
)

// Item-ID routing (decision D41): a query that names an item of the N4 developer
// course's answer keys leads with that item, exactly as `course show <ID>` prints it,
// before the three sources (which still answer the whole query). The keys are
// labs.md, review-assignments.md and certification-exam.md under
// <keys>/docs/n4-dev-course-solutions; course.Main runs in process into buffers
// (decision D42), with --root pinned to the checkout.
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
// Only a token that names an item routes: course show exiting 1 (no such item)
// leaves the query as ordinary text; exit 2 (no answer key) is the lead's typed
// UNAVAILABLE. At most maxCourseIDs tokens are looked up.

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

// courseShow is `course --root <keys> show <id>` in process: exit code, stdout, stderr.
func courseShow(keys, id string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := course.Main([]string{"--root", keys, "show", id}, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	return rc, out.String(), errb.String()
}

// showItem reads one course show item: the header line "<ID> (<kind>)" is the row,
// the lines after it its body, trailing blank lines dropped.
func showItem(out string) Item {
	ls := strings.Split(strings.TrimRight(out, "\n"), "\n")
	key, _, _ := strings.Cut(ls[0], " (")
	it := keyed(key, ls[0], "")
	it.Body = ls[1:]
	return it
}

// courseItems is the lead outcome for a query naming course items, or nil.
func (d *door) courseItems(query, keys string) (o *Outcome) {
	ids := courseItemIDs(query)
	if len(ids) == 0 {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			o = unavailable("course", courseItemTitle, "course show "+strings.Join(ids, " "),
				fmt.Sprintf("unavailable: the front door failed on the course item: panic: %v", r))
		}
	}()
	var found []string
	var items []Item
	seen := map[string]bool{}
	for _, id := range ids {
		rc, out, errs := courseShow(keys, id)
		switch {
		case rc == cli.ExitFound && out != "":
			found = append(found, id)
			if it := showItem(out); !seen[it.Key] { // L9.9 and L9.13 can be one range item
				seen[it.Key] = true
				items = append(items, it)
			}
		case rc == cli.ExitNoMatch: // not an item: ordinary query text
		default:
			reason := strings.TrimSpace(errs)
			if reason == "" {
				reason = fmt.Sprintf("course show exited %d", rc)
			}
			return unavailable("course", courseItemTitle, "course show "+strings.Join(ids, " "),
				"unavailable: "+strings.TrimPrefix(firstLine(reason), "course: "))
		}
	}
	if len(items) == 0 {
		return nil
	}
	o = newOutcome("course", courseItemTitle, "course show "+strings.Join(found, " "), OK)
	o.Items, o.Enumeration, o.Limit = items, true, maxCourseIDs
	return o
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
