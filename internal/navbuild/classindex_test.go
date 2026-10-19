package navbuild

import (
	"reflect"
	"testing"
)

func TestLoadZKMModules(t *testing.T) {
	for _, tc := range []struct {
		name, inventory string
		want            map[string]bool
		found, fails    bool
	}{
		{"absent", "", map[string]bool{}, false, false},
		{"flags", `{"modules": {"a": {"zkm": true}, "b": {"zkm": 0}, "c": {}, "d": {"zkm": "yes"}}}`,
			map[string]bool{"a": true, "d": true}, true, false},
		{"no modules key", `{"_meta": {}}`, map[string]bool{}, true, false},
		{"invalid JSON", `{"modules": `, nil, true, true},
		{"not an object", `[1]`, nil, true, true},
		{"modules not an object", `{"modules": []}`, nil, true, true},
		{"module entry not an object", `{"modules": {"a": 1}}`, nil, true, true},
		{"BOM", "\ufeff{}", nil, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := newBase(t)
			if tc.inventory != "" {
				mustWrite(t, IndexPath(base, InventoryFile), tc.inventory)
			}
			got, found, err := LoadZKMModules(base)
			if (err != nil) != tc.fails || found != tc.found || (!tc.fails && !reflect.DeepEqual(got, tc.want)) {
				t.Errorf("got %v %v %v, want %v %v fails=%v", got, found, err, tc.want, tc.found, tc.fails)
			}
		})
	}
}
