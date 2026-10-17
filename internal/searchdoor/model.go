package searchdoor

// Backend answer and outcome states (searchdoor/backend.py OK, EMPTY, UNAVAILABLE).
const (
	OK          = "ok"
	Empty       = "empty"
	Unavailable = "unavailable"
)

// Item is one ranked row: Key is what the golden measurement compares (block
// label, doc path, class or feature name); Keyed is false for a row no parser
// recognised (Python key None).
type Item struct {
	Key     string
	Keyed   bool
	Display string
	Snippet string
}

func keyed(key, display, snippet string) Item { return Item{key, true, display, snippet} }

func unkeyed(display string) Item { return Item{Display: display} }

// Raw is one backend run: its typed state, stdout and why it is unavailable.
type Raw struct {
	State  string
	Out    string
	Reason string
	MS     int
	RC     int  // the backend's exit code
	HasRC  bool // false when it did not run to an exit (Python rc None)
}

// Outcome is one source (or sub-scope) as the front door shows it.
type Outcome struct {
	Key, Title, Command string
	State               string
	Items               []Item
	Scope               []string // backend count / scope / miss lines, verbatim
	Notes               []string // front-door notes (fallbacks, resolution)
	Reason              string
	MS                  int
	Subs                []*Outcome // labelled sub-scopes of this source
	Lead                bool       // a sub-scope shown BEFORE its parent's ranked rows
	Limit               int        // rows to show; 0 = the front door's --limit
	Enumeration         bool       // rows are a complete listing, not a ranking
}

func newOutcome(key, title, command, state string) *Outcome {
	return &Outcome{Key: key, Title: title, Command: command, State: state}
}

func unavailable(key, title, command, reason string) *Outcome {
	o := newOutcome(key, title, command, Unavailable)
	o.Reason = reason
	return o
}

// HasRows reports whether a source found something: its own rows or a sub-scope's.
func HasRows(o *Outcome) bool {
	if o.State == OK && len(o.Items) > 0 {
		return true
	}
	for _, s := range o.Subs {
		if HasRows(s) {
			return true
		}
	}
	return false
}
