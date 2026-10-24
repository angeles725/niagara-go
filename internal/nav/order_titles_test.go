package nav

import (
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
		words := strings.FieldsFunc(strings.ToLower(g.Title), func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-'
		})
		for _, w := range words {
			if retired[w] {
				t.Errorf("group %q heading %q names the retired command %q", g.Name, g.Title, w)
			}
		}
	}
}
