package bogwrite

import (
	"encoding/hex"
	"encoding/json"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"os"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// TestPlanDecoderMatchesPython replays testdata/jsoncases.json (json.loads results) through
// decodePlan, i.e. internal/pyjson.Loads.
func TestPlanDecoderMatchesPython(t *testing.T) {
	var cases []struct {
		In    string `json:"in"`
		Value any    `json:"value"`
		Error string `json:"error"`
	}
	raw, err := os.ReadFile("testdata/jsoncases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	bad := 0
	for _, c := range cases {
		v, err := decodePlan(c.In)
		var got, want string
		if err != nil {
			got = "error " + err.(*pyErr).msg
		} else {
			b, _ := json.Marshal(typed(v))
			got = "value " + string(b)
		}
		if c.Error != "" {
			want = "error " + c.Error
		} else {
			b, _ := json.Marshal(c.Value)
			want = "value " + string(b)
		}
		if got != want {
			if bad++; bad <= 30 {
				t.Errorf("%q:\n got  %s\n want %s", c.In, got, want)
			}
		}
	}
	if bad > 0 {
		t.Errorf("%d of %d cases differ", bad, len(cases))
	}
}

// typed renders a decoded plan value like capture.py's typed().
func typed(v pyjson.Value) any {
	switch v.Kind {
	case pyjson.Null:
		return nil
	case pyjson.Bool:
		return v.B
	case pyjson.Int:
		return map[string]any{"int": v.I.String()}
	case pyjson.Float:
		return map[string]any{"float": pycompat.FloatRepr(v.F)}
	case pyjson.Str:
		return map[string]any{"str": hex.EncodeToString([]byte(v.S))}
	case pyjson.Array:
		out := []any{}
		for _, e := range v.A {
			out = append(out, typed(e))
		}
		return out
	}
	out := []any{}
	for _, k := range v.Keys {
		out = append(out, []any{hex.EncodeToString([]byte(k)), typed(v.M[k])})
	}
	return map[string]any{"dict": out}
}
