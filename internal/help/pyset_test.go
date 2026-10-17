package help

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

// testdata/pyset.json was printed by Python 3.15 with PYTHONHASHSEED=0:
// hash() of a few str (one, two and four bytes per code point, short and
// longer than one SipHash block) and list(set(items)) for 75 items, enough
// to resize the table twice.
func TestPySetOrderMatchesCPython(t *testing.T) {
	raw, err := os.ReadFile("testdata/pyset.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Items, Order []string
		Hash         map[string]int64
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	for s, want := range v.Hash {
		if got := pyStrHash(s); got != want {
			t.Errorf("hash(%q) = %d, want %d", s, got, want)
		}
	}
	if got := pySetOrder(v.Items); !slices.Equal(got, v.Order) {
		t.Errorf("set order\n got %q\nwant %q", got, v.Order)
	}
	if got := pySetOrder([]string{"b", "a", "b"}); len(got) != 2 {
		t.Errorf("duplicates kept: %q", got)
	}
}
