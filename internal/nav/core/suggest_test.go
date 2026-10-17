package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestCloseNamesMatchesPython compares CloseNames with suggest.close_names on
// testdata/suggest.json (gen_suggest.py).
func TestCloseNamesMatchesPython(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "suggest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Names []string             `json:"names"`
		Cases [][2]json.RawMessage `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatal(err)
	}
	for _, c := range fx.Cases {
		var query string
		var want []string
		if err := json.Unmarshal(c[0], &query); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(c[1], &want); err != nil {
			t.Fatal(err)
		}
		limit := MaxSuggestions
		if q, ok := strings.CutSuffix(query, " limit 2"); ok {
			query, limit = q, 2
		}
		got := CloseNames(query, fx.Names, limit)
		if len(got) == 0 && len(want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("CloseNames(%q, %d) = %q, Python %q", query, limit, got, want)
		}
	}
}
