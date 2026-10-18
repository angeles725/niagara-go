package searchdoor

import (
	"fmt"
	"io"
	"strings"
)

const searchHint = "check each source's scope line above and retry with --or or other terms."

var rule = strings.Repeat("=", 72)

// label is a sub-scope's short name: its title before " · ".
func label(o *Outcome) string { return strings.SplitN(o.Title, " · ", 2)[0] }

func renderRows(w io.Writer, o *Outcome, limit int, zero, indent string) {
	for _, note := range o.Notes {
		fmt.Fprintf(w, "%snote: %s\n", indent, note)
	}
	if o.State == Unavailable {
		fmt.Fprintf(w, "%s[!] %s   (unavailable: not a zero)\n", indent, o.Reason)
	}
	for i, ln := range o.Scope {
		if i == scopeLines {
			break
		}
		fmt.Fprintf(w, "%s%s\n", indent, ln)
	}
	if o.State == Empty {
		fmt.Fprintf(w, "%s%s\n", indent, zero)
	}
	shown := limit
	if o.Limit != 0 {
		shown = o.Limit
	}
	for i, it := range o.Items {
		if i == shown {
			break
		}
		fmt.Fprintf(w, "%s%2d. %s\n", indent, i+1, it.Display)
		if it.Snippet != "" {
			fmt.Fprintf(w, "%s      %s\n", indent, it.Snippet)
		}
		for _, ln := range it.Body { // a whole answer (a course item), verbatim
			fmt.Fprintln(w, strings.TrimRight(indent+"  "+ln, " "))
		}
	}
	if len(o.Items) > shown {
		more := "ranked (raise --limit)"
		if o.Enumeration {
			more = "listed (module_nav shows all)"
		}
		fmt.Fprintf(w, "%s... %d more %s\n", indent, len(o.Items)-shown, more)
	}
}

func renderSub(w io.Writer, s *Outcome, limit int) {
	fmt.Fprintf(w, "  -- %s   (%s) --\n", s.Title, s.Command)
	renderRows(w, s, limit, "0 rows in this sub-scope (searched; see its scope line)", "    ")
}

// render prints every outcome: the source header, its lead sub-scopes, its
// ranked rows, then its other sub-scopes.
func render(w io.Writer, outcomes []*Outcome, limit int) {
	for _, o := range outcomes {
		fmt.Fprintf(w, "%s\n%s   (%s)\n%s\n", rule, o.Title, o.Command, rule)
		lead := false
		for _, s := range o.Subs {
			if s.Lead {
				renderSub(w, s, limit)
				lead = true
			}
		}
		if lead {
			fmt.Fprintf(w, "  -- ranked   (%s) --\n", o.Command)
		}
		renderRows(w, o, limit, "0 results (searched; see the scope line above -- not proof of absence)", "  ")
		for _, s := range o.Subs {
			if !s.Lead {
				renderSub(w, s, limit)
			}
		}
		fmt.Fprintln(w)
	}
}

func count(o *Outcome) string {
	switch o.State {
	case OK:
		if o.Enumeration {
			return fmt.Sprintf("%d listed", len(o.Items))
		}
		return fmt.Sprintf("%d ranked", len(o.Items))
	case Empty:
		return "0 (searched)"
	}
	return "UNAVAILABLE"
}

// finish prints the summary and returns the exit code; every unavailable
// source or sub-scope is named on stderr. hint is the advice after exit 1.
func finish(out, errw io.Writer, outcomes []*Outcome, prog, hint string) int {
	for _, o := range outcomes {
		if o.State == Unavailable {
			fmt.Fprintf(errw, "%s: unavailable source: %s (%s): %s\n", prog, o.Key, o.Command, o.Reason)
		}
		for _, s := range o.Subs {
			if s.State == Unavailable {
				fmt.Fprintf(errw, "%s: unavailable source: %s/%s (%s): %s\n", prog, o.Key, label(s), s.Command, s.Reason)
			}
		}
	}
	parts := make([]string, len(outcomes))
	anyRows, anyUnavailable := false, false
	for i, o := range outcomes {
		parts[i] = o.Key + " " + count(o)
		for _, s := range o.Subs {
			parts[i] += " + " + label(s) + " " + count(s)
			anyUnavailable = anyUnavailable || s.State == Unavailable
		}
		anyRows = anyRows || HasRows(o)
		anyUnavailable = anyUnavailable || o.State == Unavailable
	}
	fmt.Fprintln(out, "Summary: "+strings.Join(parts, " · "))
	if anyRows {
		return 0
	}
	if anyUnavailable {
		fmt.Fprintln(out, "No rows, and at least one source was UNAVAILABLE ([!] above): no negative finding is possible.")
		return 2
	}
	fmt.Fprintf(out, "All %d sources searched and returned no rows. NOTE: still not proof of absence --\n", len(outcomes))
	fmt.Fprintln(out, hint)
	return 1
}
