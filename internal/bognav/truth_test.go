package bognav

import (
	"strings"
	"testing"
)

// Truth tests: a typo'd reference must not read as a clean, empty search.

func TestLinksUnknownRefIsNotAnEmptySearch(t *testing.T) {
	p := writeFile(t, "t.xml", []byte(testXML))
	tests := []struct {
		name string
		args []string
		code int
		has  string
		not  string
	}{
		{"unknown --to path", []string{"--to", "Nowhere"}, 1, "bog-nav: cannot resolve Nowhere", "(no matching links)"},
		{"unknown --from handle", []string{"--from", "h:999"}, 1, "bog-nav: cannot resolve h:999", "(no matching links)"},
		{"unknown --from with a real --to", []string{"--to", "B", "--from", "Typo"}, 1, "bog-nav: cannot resolve Typo", ""},
		{"known component, no links into it", []string{"--to", "A"}, 1, "(no matching links)", "cannot resolve"},
		{"known component with a link", []string{"--to", "B"}, 0, "", "cannot resolve"},
		{"path.slot needle", []string{"--to", "B.in10"}, 0, "", "cannot resolve"},
		{"unknown --to under --json", []string{"--to", "Nowhere", "--json"}, 1, "bog-nav: cannot resolve Nowhere", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, errOut, code := runMain(t, append([]string{p, "links"}, tt.args...)...)
			if code != tt.code {
				t.Errorf("exit = %d, want %d (stderr %q)", code, tt.code, errOut)
			}
			both := out + errOut
			if tt.has != "" && !strings.Contains(both, tt.has) {
				t.Errorf("output %q lacks %q", both, tt.has)
			}
			if tt.not != "" && strings.Contains(both, tt.not) {
				t.Errorf("output %q must not contain %q", both, tt.not)
			}
		})
	}
}

func TestSlotUnknownSlotNameExitsOne(t *testing.T) {
	p := writeFile(t, "t.xml", []byte(testXML))
	tests := []struct {
		name string
		args []string
		code int
		has  string
	}{
		{"unknown slot, text", []string{"A", "nosuchslot"}, 1, "A: no direct slot 'nosuchslot' (children: none)"},
		{"unknown slot, json", []string{"A", "nosuchslot", "--json"}, 1, "A: no direct slot 'nosuchslot'"},
		{"unknown slot on a handle ref", []string{"h:4", "nosuchslot"}, 1, "no direct slot 'nosuchslot'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, errOut, code := runMain(t, append([]string{p, "slot"}, tt.args...)...)
			if code != tt.code {
				t.Errorf("exit = %d, want %d", code, tt.code)
			}
			if !strings.Contains(out+errOut, tt.has) {
				t.Errorf("output %q lacks %q", out+errOut, tt.has)
			}
		})
	}
}
