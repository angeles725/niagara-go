package pyjson

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
)

// testdata/build_cases.json is written by testdata/gen_build.py (Python 3.15): per
// case, json.dumps of the Python value under the five layouts in buildLayouts. Each
// case here builds the same value from Go data.

var buildLayouts = []struct {
	name string
	o    DumpOpts
}{
	{"indent=2", Indent2},
	{"plain", Defaults(-1)},
	{"canonical", Canonical},
	{"ensure_ascii=False", DumpOpts{Indent: -1, ItemSep: ", ", KeySep: ": "}},
	{"indent=0", Defaults(0)},
}

// testDict is an ordered mapping like *index.Object.
type testDict struct {
	keys []string
	vals []any
}

func (d *testDict) Keys() []string { return d.keys }
func (d *testDict) Values() []any  { return d.vals }

func TestFromAnyMatchesPython(t *testing.T) {
	raw, err := os.ReadFile("testdata/build_cases.json")
	if err != nil {
		t.Fatal(err)
	}
	want := mustLoads(t, string(raw))
	f25 := 2.5e-5
	cases := map[string]any{
		"scalars": []any{nil, true, false, 0, int64(-7), json.Number("12345678901234567890123"),
			json.Number("-0"), 1.5, math.Copysign(0, -1), json.Number("1e-7"), json.Number("1E22"),
			json.Number("1e400"), json.Number("-Infinity"), json.Number("NaN"), &f25,
			json.Number("123456789012345678.0"), 0.1, math.Inf(-1), (*float64)(nil)},
		"nested ordered dict": Obj{{"z", 1}, {"a", Obj{{"y", []any{1, "x", Obj{{"k", nil}}}}}},
			{"empty", Obj{}}, {"list", []string{}}, {"strs", []string{"A", "B"}}},
		"repeated key keeps first position, last value": Obj{{"a", 1}, {"b", 2}, {"a", 3}},
		"string escapes":                   "q\" b\\ \n\r\t\b\f \x01 \x7f ~ é — 😀",
		"undecodable bytes read as U+FFFD": Obj{{"k\xff", "\xffabc\xed\xa0\x80"}},
		"dict and nil dict": []any{&testDict{[]string{"b", "a"}, []any{1, []any{true, "s"}}},
			(*testDict)(nil)},
		"value passes through with its lone surrogate": []any{mustLoads(t, `"\ud800"`), 3},
	}
	if len(want.Keys) != len(cases) {
		t.Fatalf("%d Python cases, %d Go cases", len(want.Keys), len(cases))
	}
	for name, goValue := range cases {
		exp, ok := want.Get(name)
		if !ok || len(exp.A) != len(buildLayouts) {
			t.Fatalf("%s: no Python expectations", name)
		}
		v, err := FromAny(goValue)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for i, l := range buildLayouts {
			if got := Dumps(v, l.o); got != exp.A[i].S {
				t.Errorf("%s, %s:\n got %q\nwant %q", name, l.name, got, exp.A[i].S)
			}
		}
	}
}

func TestFromAnyRejectsUnsupported(t *testing.T) {
	for name, v := range map[string]any{
		"struct":            struct{}{},
		"in a list":         []any{1, uint8(2)},
		"in an Obj":         Obj{{"a", map[string]any{}}},
		"in a Dict":         &testDict{[]string{"a"}, []any{[]int{1}}},
		"bad int literal":   json.Number("1x"),
		"bad float literal": json.Number("1.x"),
	} {
		if _, err := FromAny(v); err == nil || !strings.HasPrefix(err.Error(), "pyjson.FromAny: ") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestDumpsAnyPanicsOnUnsupported(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("no panic")
		}
	}()
	DumpsAny(struct{}{}, Indent2)
}

func TestObjectOfAndConstructors(t *testing.T) {
	got := Dumps(ObjectOf(Field{"b", BoolValue(true)}, Field{"f", FloatValue(2)},
		Field{"l", StringList([]string{"x"})}, Field{"b", IntValue(1)}), Defaults(-1))
	// json.dumps({"b": True, "f": 2.0, "l": ["x"], "b": 1})
	if want := `{"b": 1, "f": 2.0, "l": ["x"]}`; got != want {
		t.Errorf("got %s want %s", got, want)
	}
	if got := DumpsAny(Obj{{"n", 1}}, Indent2); got != "{\n  \"n\": 1\n}" {
		t.Errorf("DumpsAny = %q", got)
	}
}
