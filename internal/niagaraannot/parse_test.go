package niagaraannot

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func parseFixture(t *testing.T, name string) *Type {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	ty, err := Parse(string(b))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return ty
}

// TestParseSlots: only the annotations before the top-level declaration count (not those in
// comments, strings, the generated region, members or a nested type); multi-line and qualified
// annotations, concatenated strings, facets and every slot element are read.
func TestParseSlots(t *testing.T) {
	ty := parseFixture(t, "BTricky.java")
	if ty.Name != "BTricky" || !ty.NiagaraType || ty.Line != 36 || ty.Enum != nil {
		t.Errorf("type = %q line %d niagaraType %v enum %v", ty.Name, ty.Line, ty.NiagaraType, ty.Enum)
	}
	want := []Slot{
		{Kind: Property, Name: "out", Type: "BStatusNumeric", Default: "new BStatusNumeric(0, BStatus.nullStatus)",
			Flags: "Flags.SUMMARY | Flags.READONLY", Line: 11},
		{Kind: Property, Name: "label", Type: "String", Default: `@NiagaraProperty(name = "inString")`, Line: 18},
		{Kind: Property, Name: "url", Type: "String", Default: "http://localhost",
			Flags:  "(Flags.HIDDEN | 0x10) | javax.baja.sys.Flags.OPERATOR",
			Facets: []string{"BFacets.FIELD_WIDTH=40", "SfUtil.incl(SfUtil.MGR_EDIT)"}, Line: 19},
		{Kind: Action, Name: "submit", Type: "BWorkOrder", ReturnType: "BString", Default: "new BWorkOrder()",
			Flags: "Flags.ASYNC", Line: 26},
		{Kind: Action, Name: "reset", Line: 33},
		{Kind: Topic, Name: "submitted", Type: "BString", Flags: "Flags.SUMMARY", Line: 34},
	}
	if !reflect.DeepEqual(ty.Slots, want) {
		t.Errorf("slots:\n got %+v\nwant %+v", ty.Slots, want)
	}
	var names []string
	for _, a := range ty.Annotations {
		names = append(names, a.Name)
	}
	if got := strings.Join(names, " "); got != "NiagaraType NiagaraProperty NiagaraProperty NiagaraProperty NiagaraAction NiagaraAction NiagaraTopic SuppressWarnings" {
		t.Errorf("annotations = %s", got)
	}
	if ty.Slot("url") == nil || ty.Slot("nested") != nil || ty.Slot("inComment") != nil {
		t.Error("Slot: want url, not the nested type's or the comment's slots")
	}
}

// TestParseContainers: @NiagaraSlots, @NiagaraProperties, a single annotation for an array
// element, and an empty array.
func TestParseContainers(t *testing.T) {
	ty := parseFixture(t, "BSlots.java")
	var got []string
	for _, s := range ty.Slots {
		l, err := FlagLetters(s.Flags)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, s.Kind+" "+s.Name+" "+l)
	}
	if want := "property a rs|action go cNL|property b "; strings.Join(got, "|") != want {
		t.Errorf("slots = %q, want %q", strings.Join(got, "|"), want)
	}
	if !ty.Slots[2].Override || ty.Name != "BSlots" {
		t.Errorf("b override %v, name %s", ty.Slots[2].Override, ty.Name)
	}
	if d, err := ty.Enum.DefaultTag(); d != "only" || err != nil {
		t.Errorf("single-range default = %q, %v", d, err)
	}
}

// TestEnumConstants: names, omitted ordinals numbered from the previous one, an explicit -1 kept,
// and the default.
func TestEnumConstants(t *testing.T) {
	ty := parseFixture(t, "BLoop.java")
	cs, err := ty.Enum.Constants()
	if err != nil {
		t.Fatal(err)
	}
	want := []Constant{{"DIRECT", 0, "direct"}, {"REVERSE_ACTING", 5, "reverseActing"}, {"IO_16V1", 6, "io16V1"}, {"NUL", -1, "nul"}}
	if !reflect.DeepEqual(cs, want) {
		t.Errorf("constants = %v, want %v", cs, want)
	}
	if d, err := ty.Enum.DefaultTag(); d != "reverseActing" || err != nil {
		t.Errorf("default = %q, %v", d, err)
	}
}

func TestConstName(t *testing.T) {
	for tag, want := range map[string]string{
		"direct": "DIRECT", "toActive": "TO_ACTIVE", "stopBits2": "STOP_BITS_2", "io16V1": "IO_16V1",
		"ScptLimitCO2": "SCPT_LIMIT_CO2", "ai_0to10_vdc": "AI_0TO_10_VDC", "ai_Thermister": "AI_THERMISTER",
		"jGordonDesign": "J_GORDON_DESIGN", "ScptAirTemp1Day": "SCPT_AIR_TEMP_1DAY",
	} {
		if got := ConstName(tag); got != want {
			t.Errorf("ConstName(%q) = %q, want %q", tag, got, want)
		}
	}
}

func TestFlagLetters(t *testing.T) {
	cases := []struct{ expr, want, err string }{
		{"", "", ""},
		{"0", "", ""},
		{"Flags.SUMMARY | Flags.READONLY", "rs", ""},
		{"Flags.READONLY|Flags.SUMMARY", "rs", ""},
		{"Flags.HIDDEN + Flags.READONLY", "rh", ""},
		{"(Flags.HIDDEN | 0x10) | javax.baja.sys.Flags.OPERATOR", "hao", ""},
		{"Flags.NON_CRITICAL | Flags.LINK_TARGET | Flags.USER_DEFINED_4", "NL4", ""},
		{"Flags.LINK_TARGET | Flags.NON_CRITICAL", "NL", ""},
		{"Flags.USER_DEFINED_4 | Flags.USER_DEFINED_3 | Flags.USER_DEFINED_2 | Flags.USER_DEFINED_1", "1234", ""},
		{"1026L", "tf", ""},
		{"Flags.BOGUS", "", "unknown flag Flags.BOGUS"},
		{"Flags.SUMMARY & ~Flags.READONLY", "", "cannot evaluate"},
		{"SUMMARY", "", "cannot evaluate"},
		{"Other.SUMMARY", "", "cannot evaluate"},
		{"(Flags.SUMMARY", "", "cannot evaluate"},
		{"flags", "", "cannot evaluate"},
	}
	for _, c := range cases {
		got, err := FlagLetters(c.expr)
		if got != c.want || (err == nil) != (c.err == "") || (err != nil && !strings.Contains(err.Error(), c.err)) {
			t.Errorf("FlagLetters(%q) = %q, %v; want %q, %q", c.expr, got, err, c.want, c.err)
		}
	}
}

// TestParseErrors: what the parser refuses instead of guessing.
func TestParseErrors(t *testing.T) {
	cases := []struct{ name, src, err string }{
		{"no type", "package a;\n@NiagaraType\n", "no type declaration"},
		{"open comment", "/* x\nclass A {}", "line 1: unterminated comment"},
		{"open string", "@NiagaraProperty(name = \"a)\nclass A {}", "line 1: unterminated \" literal"},
		{"name not a literal", "@NiagaraProperty(name = NAME, type = \"int\")\nclass A {}", "name is not a string literal: NAME"},
		{"no type element", "@NiagaraProperty(name = \"a\")\nclass A {}", "@NiagaraProperty has no type"},
		{"duplicate slot", "@NiagaraAction(name = \"a\")\n@NiagaraTopic(name = \"a\")\nclass A {}", "line 2: slot a declared twice"},
		{"duplicate element", "@NiagaraAction(name = \"a\", name = \"b\")\nclass A {}", "element name given twice"},
		{"bad array", "@NiagaraEnum(range = { @Range(\"a\") @Range(\"b\") })\nclass A {}", "',' or '}' expected"},
		{"not a Range", "@NiagaraEnum(range = { \"a\" })\nclass A {}", "@Range expected"},
		{"two enums", "@NiagaraEnum(range = @Range(\"a\"))\n@NiagaraEnum(range = @Range(\"b\"))\nclass A {}", "a second @NiagaraEnum"},
		{"container element", "@NiagaraProperties({ @NiagaraAction(name = \"a\") })\nclass A {}", "@NiagaraProperty expected"},
		{"unclosed", "@NiagaraAction(name = \"a\"\nclass A {}", "unterminated annotation value"},
	}
	for _, c := range cases {
		if _, err := Parse(c.src); err == nil || !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.err)
		}
	}
}

// TestEnumErrors: a default that is not a tag, shared ordinals, and an ordinal the parser does
// not evaluate are errors of Constants or DefaultTag, not of Parse.
func TestEnumErrors(t *testing.T) {
	cases := []struct{ name, src, err string }{
		{"default not a tag", `@NiagaraEnum(range = @Range("a"), defaultValue = "b")`, "defaultValue b is not a tag"},
		{"shared ordinal", `@NiagaraEnum(range = { @Range("a"), @Range(value = "b", ordinal = 0) })`, "tags a and b share ordinal 0"},
		{"same name", `@NiagaraEnum(range = { @Range("aB"), @Range("a_b") })`, "declared twice"},
		{"constant ordinal", `@NiagaraEnum(range = @Range(value = "a", ordinal = A_ORD))`, "not an int literal: A_ORD"},
	}
	for _, c := range cases {
		ty, err := Parse(c.src + "\nfinal class BE {}")
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		_, err1 := ty.Enum.Constants()
		_, err2 := ty.Enum.DefaultTag()
		if (err1 == nil || !strings.Contains(err1.Error(), c.err)) && (err2 == nil || !strings.Contains(err2.Error(), c.err)) {
			t.Errorf("%s: errors %v / %v, want %q", c.name, err1, err2, c.err)
		}
	}
}
