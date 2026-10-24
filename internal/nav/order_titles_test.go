package nav

import (
	"reflect"
	"strings"
	"testing"
	"unicode"
)

// A group heading in --help must not name a retired command: the docs audit of round 10 found
// "Types: hierarchy, members, imports" and "Niagara domain: drivers, servlets, ORDs, topics, BQL,
// alarms, BOG" advertising commands the Go port no longer runs. Any heading word that equals a
// retired command name fails, whatever its sense, so a heading can only describe what the group keeps.
func TestGroupTitlesNameNoRetiredCommand(t *testing.T) {
	for _, g := range Groups {
		for _, w := range retiredWords(g.Title) {
			t.Errorf("group %q heading %q names the retired command %q", g.Name, g.Title, w)
		}
	}
}

// retiredWords are the words of a heading (lower-cased, split at anything but a letter, a digit
// or '-') that are retired command names.
func retiredWords(title string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(strings.ToLower(title), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-'
	}) {
		if retired[w] {
			out = append(out, w)
		}
	}
	return out
}

// TestRetiredWordsCatchesTheOldHeadings is the negative control: the headings the audit found are
// rejected with exactly the retired names they carried, mixed case and plurals included.
func TestRetiredWordsCatchesTheOldHeadings(t *testing.T) {
	for title, want := range map[string][]string{
		"Types: hierarchy, members, imports":                                {"imports"},
		"Niagara domain: drivers, servlets, ORDs, topics, BQL, alarms, BOG": {"servlets", "ords", "topics", "bql"},
		"Find classes and concepts":                                         {"find"},
		"Source text, strings and resources":                                {"strings"},
		"Security, licensing and permissions":                               {"permissions"},
		"Module audits":                                                     nil,
	} {
		if got := retiredWords(title); !reflect.DeepEqual(got, want) {
			t.Errorf("retiredWords(%q) = %q, want %q", title, got, want)
		}
	}
}
