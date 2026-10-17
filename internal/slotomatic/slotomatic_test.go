package slotomatic

import (
	"reflect"
	"sort"
	"testing"
)

// The sources of niagara-research tools/tests/test_n4_type_catalog.py.
const originalSrc = `
package javax.baja.control;
public class BNumericWritable
  extends BNumericPoint
  implements BIWritablePoint
{
  public static final Property in1 = newProperty(Flags.READONLY, new BStatusNumeric(0, BStatus.nullStatus), null);
  public static final Property in10 = newProperty(Flags.SUMMARY | Flags.TRANSIENT, new BStatusNumeric(0, BStatus.nullStatus), null);
  public static final Property fallback = newProperty(0, new BStatusNumeric(0, BStatus.nullStatus), null);
  public static final Property overrideExpiration = newProperty(Flags.OPERATOR | Flags.READONLY, BAbsTime.NULL, null);
  public static final Action emergencyAuto = newAction(0, null);
  public static final Action set = newAction(Flags.OPERATOR, BDouble.DEFAULT, null);
  public static final Topic fired = newTopic(Flags.SUMMARY, null);
}
`

const cfrSrc = `
package com.tridium.kitControl;
public class BInterstartDelayControl
extends BComponent {
    public static final Property delay = BInterstartDelayControl.newProperty((int)0, (BValue)BRelTime.DEFAULT, null);
    public static final Property master = BInterstartDelayControl.newProperty((int)1026, (BValue)BOrd.NULL, BFacets.make((String)"a", (String)"b"));
    public static final Action reset = BInterstartDelayControl.newAction((int)256, null);
}
`

const multilineSrc = `
package a.b;
public class BMulti extends BComponent {
  public static final Property conversion = newProperty(0, BDefaultProxyConversion.DEFAULT, BFacets.make(BFacets.make(BFacets.FIELD_EDITOR, "driver:ProxyConversionFE"), BFacets.make(BFacets.UX_FIELD_EDITOR, "driver:ProxyConversionEditor")));
  public static final Property wrapped =
    newProperty(Flags.HIDDEN, "x",
      null);
}
`

func byName(ds []Decl) map[string]Decl {
	m := map[string]Decl{}
	for _, d := range ds {
		m[d.Name] = d
	}
	return m
}

func TestDecodeFlags(t *testing.T) {
	v, letters := DecodeFlags("Flags.SUMMARY | Flags.TRANSIENT")
	sort.Strings(letters)
	if v != 0x0A || !reflect.DeepEqual(letters, []string{"s", "t"}) {
		t.Errorf("symbolic: %d %v", v, letters)
	}
	v, letters = DecodeFlags("(int)1026")
	if v != 1026 || !reflect.DeepEqual(letters, []string{"t", "f"}) {
		t.Errorf("numeric: %d %v", v, letters)
	}
	for _, zero := range []string{"0", "(int)0", "", "Flags.NOPE"} {
		if v, letters := DecodeFlags(zero); v != 0 || len(letters) != 0 {
			t.Errorf("%q: %d %v", zero, v, letters)
		}
	}
	if v, _ := DecodeFlags("0x10 | javax.baja.sys.Flags.OPERATOR | ١٢"); v != 0x10|0x100|12 {
		t.Errorf("hex + qualified + Arabic-Indic digits: %d", v)
	}
	if v, letters := DecodeFlags("-1"); v != -1 || len(letters) != len(Flags) {
		t.Errorf("-1: %d %v", v, letters)
	}
}

func TestParseOriginalSource(t *testing.T) {
	ty := ParseSource(originalSrc)
	if ty.Class != "BNumericWritable" || ty.Package != "javax.baja.control" || ty.Extends != "BNumericPoint" {
		t.Fatalf("header: %+v", ty)
	}
	props := byName(ty.Properties)
	for name, want := range map[string]int64{"in1": 1, "in10": 0x0A, "fallback": 0, "overrideExpiration": 0x101} {
		if props[name].Flags != want {
			t.Errorf("%s flags = %d, want %d", name, props[name].Flags, want)
		}
	}
	if got := props["in1"].Default; got != "new BStatusNumeric(0, BStatus.nullStatus)" {
		t.Errorf("in1 default %q", got)
	}
	acts := byName(ty.Actions)
	if acts["emergencyAuto"].Flags != 0 || acts["set"].Flags != 0x100 || acts["set"].Default != "BDouble.DEFAULT" {
		t.Errorf("actions: %+v", ty.Actions)
	}
	if acts["emergencyAuto"].Default != "" || acts["emergencyAuto"].Facets != "null" {
		t.Errorf("two-argument action: %+v", acts["emergencyAuto"])
	}
	if len(ty.Topics) != 1 || ty.Topics[0].Name != "fired" {
		t.Errorf("topics: %+v", ty.Topics)
	}
}

func TestParseCFRStyle(t *testing.T) {
	ty := ParseSource(cfrSrc)
	props := byName(ty.Properties)
	if props["delay"].Flags != 0 || props["delay"].Default != "BRelTime.DEFAULT" {
		t.Errorf("delay: %+v", props["delay"])
	}
	if props["master"].Flags != 1026 || props["master"].Facets != `BFacets.make((String)"a", (String)"b")` {
		t.Errorf("master: %+v", props["master"])
	}
	if byName(ty.Actions)["reset"].Flags != 256 {
		t.Errorf("reset: %+v", ty.Actions)
	}
}

func TestParseNestedAndMultiline(t *testing.T) {
	props := byName(ParseSource(multilineSrc).Properties)
	if want := `BFacets.make(BFacets.make(BFacets.FIELD_EDITOR, "driver:ProxyConversionFE"), BFacets.make(BFacets.UX_FIELD_EDITOR, "driver:ProxyConversionEditor"))`; props["conversion"].Facets != want {
		t.Errorf("conversion facets %q", props["conversion"].Facets)
	}
	if props["wrapped"].Flags != 4 || props["wrapped"].Default != `"x"` {
		t.Errorf("wrapped: %+v", props["wrapped"])
	}
}

func TestParseProcyonStaticBlock(t *testing.T) {
	ty := ParseSource(`package p;
public final class BX extends BComponent {
  public static final Property speed;
  public static final Property broken;
  static {
    speedCheck = speed == null;
    a.speed = 3;
    speed = newProperty(Flags.OPERATOR, BDouble.make(1.5), null);
  }
}`)
	if len(ty.Properties) != 1 || ty.Properties[0].Name != "speed" || ty.Properties[0].Default != "BDouble.make(1.5)" {
		t.Errorf("procyon: %+v", ty.Properties)
	}
}

func TestNoDeclarations(t *testing.T) {
	ty := ParseSource("package p;\npublic class Plain { int x; }\n")
	if ty.Class != "Plain" || len(ty.Properties) != 0 || len(ty.Actions) != 0 || len(ty.Topics) != 0 {
		t.Errorf("%+v", ty)
	}
}

func TestClassNeedsUnicodeWordBoundary(t *testing.T) {
	if got := ParseSource("épublic class A {}\npublic class B {}").Class; got != "B" {
		t.Errorf("class = %q, want B (a word character before public is no boundary)", got)
	}
}

func TestSplitArgsAndClean(t *testing.T) {
	got := SplitArgs(` a , "x,\"y" , f(1, 2), [3, 4] , 'c'`)
	want := []string{"a", `"x,\"y"`, "f(1, 2)", "[3, 4]", "'c'"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SplitArgs = %q", got)
	}
	if got := SplitArgs("  "); got != nil {
		t.Errorf("blank = %q", got)
	}
	if got := Clean(" (BValue) (int)\n x  +\t y "); got != "x + y" {
		t.Errorf("Clean = %q", got)
	}
}

func TestDecodeReplace(t *testing.T) {
	cases := map[string]string{
		"caf\xc3\xa9":  "café",
		"\xe2\x82 x":   "� x",
		"\xff\xfe":     "��",
		"\xed\xa0\x80": "���",
		"\xf0\x9f\x98": "�",
	}
	for in, want := range cases {
		if got := DecodeReplace([]byte(in)); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
	if got := UniversalNewlines("a\r\nb\rc\n"); got != "a\nb\nc\n" {
		t.Errorf("newlines %q", got)
	}
}
