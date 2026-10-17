package core

import "testing"

// TestDumpsMatchesPython pins json.dumps(v, indent=2) and json.dumps(v) of
// Python 3 on nested values, escapes and a float.
func TestDumpsMatchesPython(t *testing.T) {
	v := NewTestMeta(t, `{"a":[1,2.5,null,true,"é€😀\u007f\u0001\"\\\n"],"b":{},"c":[],"d":{"x":1e20}}`)
	indented := "{\n  \"a\": [\n    1,\n    2.5,\n    null,\n    true,\n    \"\\u00e9\\u20ac\\ud83d\\ude00\\u007f\\u0001\\\"\\\\\\n\"\n  ],\n  \"b\": {},\n  \"c\": [],\n  \"d\": {\n    \"x\": 1e+20\n  }\n}"
	compact := "{\"a\": [1, 2.5, null, true, \"\\u00e9\\u20ac\\ud83d\\ude00\\u007f\\u0001\\\"\\\\\\n\"], \"b\": {}, \"c\": [], \"d\": {\"x\": 1e+20}}"
	if got := Dumps(v, 2); got != indented {
		t.Errorf("Dumps(v, 2) =\n%s\nPython\n%s", got, indented)
	}
	if got := Dumps(v, -1); got != compact {
		t.Errorf("Dumps(v, -1) = %s, Python %s", got, compact)
	}
	if got := Dumps([]string{}, 2); got != "[]" {
		t.Errorf("Dumps([]) = %s", got)
	}
}
