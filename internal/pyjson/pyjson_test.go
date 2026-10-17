package pyjson

import (
	"math"
	"strings"
	"testing"
)

func mustLoads(t *testing.T, s string) Value {
	t.Helper()
	v, err := Loads([]rune(s))
	if err != nil {
		t.Fatalf("Loads(%q): %v", s, err)
	}
	return v
}

func TestLoadsErrorsMirrorJSONDecodeError(t *testing.T) {
	cases := map[string]string{
		"":             "Expecting value: line 1 column 1 (char 0)",
		"\ufeff{}":     "Unexpected UTF-8 BOM (decode using utf-8-sig): line 1 column 1 (char 0)",
		"{}\n x":       "Extra data: line 2 column 2 (char 4)",
		`{"a" 1}`:      "Expecting ':' delimiter: line 1 column 6 (char 5)",
		`[1,]`:         "Illegal trailing comma before end of array: line 1 column 3 (char 2)",
		`{"a":1,}`:     "Illegal trailing comma before end of object: line 1 column 7 (char 6)",
		"\"a\tb\"":     "Invalid control character at: line 1 column 3 (char 2)",
		`"\x"`:         "Invalid \\escape: line 1 column 2 (char 1)",
		`"\u12"`:       "Invalid \\uXXXX escape: line 1 column 3 (char 2)",
		`"abc`:         "Unterminated string starting at: line 1 column 1 (char 0)",
		`{1:2}`:        "Expecting property name enclosed in double quotes: line 1 column 2 (char 1)",
		"-":            "Expecting value: line 1 column 1 (char 0)",
		"[1 2]":        "Expecting ',' delimiter: line 1 column 4 (char 3)",
		`{"k": "v" x}`: "Expecting ',' delimiter: line 1 column 11 (char 10)",
		// A \uXXXX escape (or a surrogate pair) that ends the input is complete: the
		// string is then unterminated (CPython 3.15 checks end > len, not end >= len).
		`"x\u00e9`:      "Unterminated string starting at: line 1 column 1 (char 0)",
		`"\ud83d\ude00`: "Unterminated string starting at: line 1 column 1 (char 0)",
		`"\ud83d`:       "Unterminated string starting at: line 1 column 1 (char 0)",
		`"x\u00e`:       "Invalid \\uXXXX escape: line 1 column 4 (char 3)",
		`"\ud83d\ude0`:  "Invalid \\uXXXX escape: line 1 column 9 (char 8)",
		`"\ud83d\u`:     "Invalid \\uXXXX escape: line 1 column 9 (char 8)",
		`"\ud83d\uzz00`: "Invalid \\uXXXX escape: line 1 column 9 (char 8)",
	}
	for in, want := range cases {
		if _, err := Loads([]rune(in)); err == nil || err.Error() != want {
			t.Errorf("Loads(%q): got %v, want %q", in, err, want)
		}
	}
}

func TestValueKeepsPythonSemantics(t *testing.T) {
	v := mustLoads(t, `{"b": 1, "a": [1.0, true, null, "x"], "b": 12345678901234567890123}`)
	if strings.Join(v.Keys, ",") != "b,a" {
		t.Errorf("dict order: got %v, want first-insertion order b,a", v.Keys)
	}
	if b, _ := v.Get("b"); b.Kind != Int || b.I.String() != "12345678901234567890123" {
		t.Errorf("big int: got %v", b.Repr())
	}
	if got := v.Repr(); got != "{'b': 12345678901234567890123, 'a': [1.0, True, None, 'x']}" {
		t.Errorf("Repr: got %s", got)
	}
	if got := Dumps(v, Canonical); got != `{"a":[1.0,true,null,"x"],"b":12345678901234567890123}` {
		t.Errorf("Canonical: got %s", got)
	}
	if got := String("x").Str(); got != "x" {
		t.Errorf("Str of str: got %q", got)
	}
	if got := IntValue(3).Str(); got != "3" {
		t.Errorf("Str of int: got %q", got)
	}
	if got := mustLoads(t, "[]").TypeName(); got != "list" {
		t.Errorf("TypeName: got %s", got)
	}
}

func TestEqualMirrorsPythonEquality(t *testing.T) {
	one, oneF, tru := mustLoads(t, "1"), mustLoads(t, "1.0"), mustLoads(t, "true")
	if !Equal(one, oneF) || !Equal(one, tru) {
		t.Error("1 == 1.0 == True must hold")
	}
	if Equal(String("1"), one) {
		t.Error(`"1" == 1 must not hold`)
	}
	nan := Value{Kind: Float, F: math.NaN()}
	if Equal(nan, nan) {
		t.Error("nan == nan must not hold")
	}
	if !Equal(mustLoads(t, `{"a":[1],"b":2}`), mustLoads(t, `{"b":2.0,"a":[true]}`)) {
		t.Error("dict equality must ignore order and compare numerically")
	}
	if mustLoads(t, "0.0").Truthy() || !mustLoads(t, `" "`).Truthy() || mustLoads(t, "{}").Truthy() {
		t.Error("Truthy must mirror bool()")
	}
}

func TestLoneSurrogatesRoundTrip(t *testing.T) {
	v := mustLoads(t, `"\ud800x\ud83d\ude00"`)
	rs := Runes(v.S)
	if len(rs) != 3 || rs[0] != 0xD800 || rs[1] != 'x' || rs[2] != 0x1F600 {
		t.Fatalf("Runes: got %U", rs)
	}
	if WTF8(rs) != v.S {
		t.Error("WTF8(Runes(s)) must round-trip")
	}
	if got := Dumps(v, Canonical); got != `"\ud800x\ud83d\ude00"` {
		t.Errorf("ensure_ascii dump: got %s", got)
	}
	if got := v.Repr(); got != `'\ud800x`+"\U0001F600'" {
		t.Errorf("Repr: got %s", got)
	}
}

// TestStrReprSurrogateFallback pins the WTF-8 path of StrRepr (pycompat.StrRepr only
// takes valid UTF-8): quote choice, escapes and printability next to a lone surrogate.
// Expected text from Python 3.15: repr('\ud800\'é\x01\x7f\xa0\u0378\t\\').
func TestStrReprSurrogateFallback(t *testing.T) {
	v := mustLoads(t, `"\ud800'\u00e9\u0001\u007f\u00a0\u0378\t\\"`)
	if got, want := StrRepr(v.S), `"\ud800'é\x01\x7f\xa0\u0378\t\\"`; got != want {
		t.Errorf("StrRepr: got %s want %s", got, want)
	}
}
