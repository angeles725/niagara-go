package modulefind

import (
	"reflect"
	"sync"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyre"
)

// oldRes caches the per-cls patterns: compiling one dominates a fuzz iteration.
var oldRes sync.Map

// oldExtends is the per-file compiled search extendsOf replaces.
func oldExtends(text []rune, cls string) (string, bool, error) {
	v, ok := oldRes.Load(cls)
	if !ok {
		re, err := pyre.Compile(`\bclass\s+`+escape(cls)+`\s+extends\s+(\w+)`, 0)
		if err != nil {
			return "", false, err
		}
		v, _ = oldRes.LoadOrStore(cls, re)
	}
	re := v.(*pyre.Regexp)
	if mt := re.SearchRunes(text, 0); mt != nil {
		return mt.Group(1), true, nil
	}
	return "", false, nil
}

// spans flattens each match's spans of groups 0..2 ((-1, -1) past the last group).
func spans(ms []*pyre.Match) [][]int {
	var out [][]int
	for _, m := range ms {
		var row []int
		for g := 0; g <= 2; g++ {
			a, b := m.Span(g)
			row = append(row, a, b)
		}
		out = append(out, row)
	}
	return out
}

// checkScan asserts extendsOf, dynSetMatches and findIterFrom (setterRe) agree with the regex formulations.
func checkScan(t *testing.T, text, cls string) {
	t.Helper()
	runes := []rune(text)
	want, wantOK, err := oldExtends(runes, cls)
	if err == nil {
		if got, ok := extendsOf(runes, cls); got != want || ok != wantOK {
			t.Errorf("extendsOf(%q, %q) = %q, %v; regex says %q, %v", text, cls, got, ok, want, wantOK)
		}
	}
	if got, want := spans(dynSetMatches(runes)), spans(dynSetRe.FindIter(runes)); !reflect.DeepEqual(got, want) {
		t.Errorf("dynSetMatches(%q) = %v; regex says %v", text, got, want)
	}
	if got, want := spans(findIterFrom(setterRe, setRunes, runes)), spans(setterRe.FindIter(runes)); !reflect.DeepEqual(got, want) {
		t.Errorf("findIterFrom(setterRe, %q) = %v; regex says %v", text, got, want)
	}
}

var scanSeeds = []struct{ text, cls string }{
	{"public class Foo extends Bar {}", "Foo"},
	{"subclass Foo extends Bar; class Foo extends Baz", "Foo"},
	{"class  Foo\textends Bar", "Foo"},
	{"classFoo extends Bar class Foo extends\n\tQux", "Foo"},
	{"class Foo extends", "Foo"},
	{"class FooBar extends A class Foo extends B", "Foo"},
	{"class Foo implements X extends Y", "Foo"},
	{"class $Foo extends Bar", "$Foo"},
	{"class  Foo extends Bar", " Foo"},
	{"class class Foo extends Bar", "class"},
	{"class Foo extends Bar", ""},
	{"class extends Bar", ""},
	{"xclass Foo extends Bar class Foo extends Ünï", "Foo"},
	{"a.set(b,c.set(d,", ""},
	{"x.set( 1, y.set( z , w.set(_q9,", ""},
	{"ñandú.set(ü, x)", ""},
	{".set(a, b) a.set(a.set(b,", ""},
	{"a.b.set(c,d) é1.set(x\n,", ""},
	{"foo.set(bar", ""},
	{"foo.set(", ""},
	{"a.set(b,a.set(b,", ""},
	{"x .set(a,", ""},
	{"setFoo( resetBar(x) setsetBaz ( set Qux(", ""},
	{"_setA(setB(setÀb\u2028(setC", ""},
	{"setsetA(", ""},
}

func TestScanMatchesRegex(t *testing.T) {
	for _, s := range scanSeeds {
		checkScan(t, s.text, s.cls)
	}
	if got, ok := extendsOf([]rune("class  Foo\textends Bar"), "Foo"); !ok || got != "Bar" {
		t.Errorf("extendsOf = %q, %v", got, ok)
	}
	if got := len(dynSetMatches([]rune("a.set(b,c.set(d,"))); got != 2 {
		t.Errorf("dynSetMatches found %d, want 2", got)
	}
}

func FuzzScanMatchesRegex(f *testing.F) {
	for _, s := range scanSeeds {
		f.Add(s.text, s.cls)
	}
	f.Fuzz(func(t *testing.T, text, cls string) {
		checkScan(t, text, cls)
	})
}
