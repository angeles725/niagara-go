package help

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// TestDumpsIndent2 pins json.dumps(json.loads(src), indent=2) as Python 3.15 prints it, for
// the decoded index values (*index.Object as a pyjson.Dict) the --json views dump.
func TestDumpsIndent2(t *testing.T) {
	tests := []struct{ name, src, want string }{
		{"scalars and number re-printing",
			`[null, true, false, 0, -0, 12345678901234567890123, 1.50, -0.0, 1e-7, 1E22, 1e400, -1e400, 2.5e-5, 123456789012345678.0]`,
			"[\n  null,\n  true,\n  false,\n  0,\n  0,\n  12345678901234567890123,\n  1.5,\n  -0.0,\n  1e-07,\n" +
				"  1e+22,\n  Infinity,\n  -Infinity,\n  2.5e-05,\n  1.2345678901234568e+17\n]"},
		{"empty containers", `{"a": [], "b": {}, "c": [[]]}`,
			"{\n  \"a\": [],\n  \"b\": {},\n  \"c\": [\n    []\n  ]\n}"},
		{"ensure_ascii escapes", `["q\" b\\ \n\r\t\b\f \u0001 \u007f ~ é — 😀"]`,
			"[\n  \"q\\\" b\\\\ \\n\\r\\t\\b\\f \\u0001 \\u007f ~ \\u00e9 \\u2014 \\ud83d\\ude00\"\n]"},
		{"key order kept", `{"z": 1, "a": {"y": [1, "x", {"k": null}]}}`,
			"{\n  \"z\": 1,\n  \"a\": {\n    \"y\": [\n      1,\n      \"x\",\n      {\n        \"k\": null\n      }\n    ]\n  }\n}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := index.Decode([]byte(tt.src))
			if err != nil {
				t.Fatal(err)
			}
			if got := pyjson.DumpsAny(v, pyjson.Indent2); got != tt.want {
				t.Errorf("got\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestDumpsIndent2BuiltValues(t *testing.T) {
	got := pyjson.DumpsAny(pyjson.Obj{{"chain", []string{"A", "B"}}, {"n", 2}, {"none", pyjson.Obj{}}}, pyjson.Indent2)
	want := "{\n  \"chain\": [\n    \"A\",\n    \"B\"\n  ],\n  \"n\": 2,\n  \"none\": {}\n}"
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}
