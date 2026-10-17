package licensador

import (
	"fmt"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// TestCheckFeatureLimitKinds pins check_feature's `qty > fqty` / `fqty - qty`
// for every JSON limit kind; the rows were frozen from licensador.py on
// Python 3.15 ([limit JSON, qty, code, repr(rest)], code "ERR" = exception).
func TestCheckFeatureLimitKinds(t *testing.T) {
	for _, tc := range []struct {
		limit     string
		qty       int64
		code, out string
	}{
		{"5.0", 4, OK, "1.0"},
		{"5.0", 5, OK, "0.0"},
		{"5.0", 6, EFeatureQty, "5.0"},
		{"4.5", 4, OK, "0.5"},
		{"4.5", 5, EFeatureQty, "4.5"},
		{"1e400", 1000000000000000000, OK, "inf"},
		{"-1e400", 0, EFeatureQty, "-inf"},
		{"NaN", 1, OK, "nan"},
		{"99999999999999999999", 1000000000000000000, OK, "98999999999999999999"},
		{"99999999999999999999", 9, OK, "99999999999999999990"},
		{"-99999999999999999999", 0, EFeatureQty, "-99999999999999999999"},
		{"true", 1, OK, "0"},
		{"true", 2, EFeatureQty, "True"},
		{"false", 0, OK, "0"},
		{"false", 1, EFeatureQty, "False"},
		{"9007199254740993.0", 9007199254740993, EFeatureQty, "9007199254740992.0"},
		{`"5"`, 1, "ERR", "TypeError: '>' not supported between instances of 'int' and 'str'"},
		{"[5]", 1, "ERR", "TypeError: '>' not supported between instances of 'int' and 'list'"},
		{`{"a":1}`, 1, "ERR", "TypeError: '>' not supported between instances of 'int' and 'dict'"},
		{"4", 4, OK, "0"},
		{"4", 5, EFeatureQty, "4"},
	} {
		claims, err := pyjson.Loads([]rune(fmt.Sprintf(`{"features":{"f":{"qty":%s}}}`, tc.limit)))
		if err != nil {
			t.Fatal(err)
		}
		code, rest, err := CheckFeature(claims, "f", tc.qty)
		got := ""
		switch {
		case err != nil:
			code, got = "ERR", err.Error()
		case rest != nil:
			got = rest.Repr()
		}
		if code != tc.code || got != tc.out {
			t.Errorf("qty %d vs limit %s: %s %s, want %s %s", tc.qty, tc.limit, code, got, tc.code, tc.out)
		}
	}
}
