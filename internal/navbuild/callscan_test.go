package navbuild

import (
	"reflect"
	"slices"
	"testing"
)

func TestBlankLiterals(t *testing.T) {
	for in, want := range map[string]string{
		// an unterminated quote is left as it is
		`a "b\"c" 'd' "unterminated`: `a "" '' "unterminated`,
		`x = '\'' + "q\\" + 'é'`:     `x = '' + "" + ''`,
		`no literals`:                `no literals`,
	} {
		if got := blankLiterals(in); got != want {
			t.Errorf("blankLiterals(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestZoneCalls pins extract_calls_from_zone against outputs of the Python
// function: \b rules, receivers matched mid-word, literals and comments.
func TestZoneCalls(t *testing.T) {
	zone := []string{
		"void f() {",
		"    this.go(); this .no(); super.up(); super(1); a.super(); this(2); x. this();",
		"    new Foo(); new Bar<T>(); new int[3]; new String(); new Ünit (); renew Baz();",
		"    Members.this.run(); obj.call(); Util.café(); String.valueOf(1); Abc.if(x);",
		"    A.B.c(); /* Hidden.x() */ Vis.y(); // Gone.z()",
		"    /* open",
		"       Inside.no() */ After.yes();",
		`    s = "Str.no()" + 'C.no()';`,
		"    subclass.Up(); xThis.go(); thisX.go(); superb(); _Under.go();",
		"}",
	}
	for _, tc := range []struct {
		name   string
		lines  []string
		parent string
		want   []string
	}{
		{"with a parent", zone, "Par", []string{"After.yes", "B.c", "Bar.<init>", "Cur.<init>", "Cur.go", "Cur.run",
			"Foo.<init>", "Par.<init>", "Par.up", "Util.café", "Vis.y", "Ünit.<init>"}},
		{"super without a parent is the class itself", zone[:2], "", []string{"Cur.<init>", "Cur.go", "Cur.up"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for c := range zoneCalls(tc.lines, "Cur", tc.parent) {
				got = append(got, c)
			}
			slices.Sort(got)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("zoneCalls =\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

func TestSimpleParent(t *testing.T) {
	parents := map[string]string{"A": "org.x.Base<T>", "B": "Plain", "C": "<T>"}
	for cls, want := range map[string]string{"A": "Base", "B": "Plain", "C": "", "D": ""} {
		if got := simpleParent(parents, cls); got != want {
			t.Errorf("simpleParent(%s) = %q, want %q", cls, got, want)
		}
	}
}

func TestReadParentMap(t *testing.T) {
	base := newBase(t)
	if m, found, err := readParentMap(base); err != nil || found || len(m) != 0 {
		t.Errorf("no inheritance.json: %v %v %v", m, found, err)
	}
	mustWrite(t, IndexPath(base, InheritanceFile), `{"class_to_chain": {"A": ["P", "Q"], "B": [], "C": null}}`)
	if m, found, err := readParentMap(base); err != nil || !found || !reflect.DeepEqual(m, map[string]string{"A": "P"}) {
		t.Errorf("parents = %v %v %v", m, found, err)
	}
	mustWrite(t, IndexPath(base, InheritanceFile), `{"class_to_chain": {"A": [1]}}`)
	if _, _, err := readParentMap(base); err == nil {
		t.Error("a non-string parent is accepted")
	}
}
