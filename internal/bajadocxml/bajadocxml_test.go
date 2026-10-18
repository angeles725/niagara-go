package bajadocxml

import (
	"fmt"
	"strings"
	"testing"
)

// The reader and the clean-text helpers on hand-written documents. The
// whole-tree behaviour (walk, skip, write) is pinned by helpbuild's S7b
// fixture and parity tests; the query side by the nh goldens.

const demoDoc = `<?xml version="1.0" encoding="UTF-8"?>
<bajadoc version="2.0">
<class module="demo" runtimeProfile="rt" qualifiedName="javax.baja.demo.BDemo" name="BDemo"
  packageName="javax.baja.demo" public="true" abstract="true">
<description>A &lt;code&gt;demo&lt;/code&gt;<!-- dropped --> class<see ref="x"/>tail not read</description>
<tag name="@since">Niagara &lt;b&gt;4.0&lt;/b&gt;</tag>
<extends><type class="javax.baja.sys.BComponent"/></extends>
<implements><type class="com.tridium.sys.BIThing"/><type class="java.io.Serializable"/></implements>
<implements><type class="javax.baja.sys.BINotRead"/></implements>
<constructor name="BDemo" public="true"><parameter name="n"><type class="int"/></parameter></constructor>
<method name="zeta" public="true" static="true" abstract="true">
  <parameter name="names" isVarargs="true"><type class="java.lang.String" dimension="1"/></parameter>
  <return><parameterizedType class="java.util.List"><args><wildcardType><bounds><type class="javax.baja.sys.BObject"/></bounds></wildcardType><typeVariable name="T"/></args></parameterizedType></return>
</method>
<method name="alpha" public="true"><return><type class="javax.baja.sys.Type" dimension="2"/></return></method>
<field name="b" public="true" static="true" final="true"><type class="javax.baja.sys.Property"/></field>
<field name="a" public="true"><type class="int"/></field>
<property name="p"><type class="int"/></property>
<topic name="t"><eventType><type class="javax.baja.sys.BValue"/></eventType></topic>
</class>
<class name="NoQualifiedName"/>
</bajadoc>`

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		classes []string // "pkg.Name kind"
		ok      bool
	}{
		{"classes of a document", demoDoc, []string{"javax.baja.demo.BDemo class"}, true},
		{"index file has no class", `<bajadoc><module name="demo"/></bajadoc>`, nil, true},
		{"forbidden character reference sanitized",
			`<bajadoc><class qualifiedName="a.B" name="B" packageName="a" category="enum">` +
				`<description>x&#x0;y&#65;</description></class></bajadoc>`, []string{"a.B enum"}, true},
		{"unparseable after sanitizing", `<bajadoc><class qualifiedName="a.B"`, nil, false},
		{"empty file", ``, nil, false},
		{"kinds", `<bajadoc><class qualifiedName="a.A" name="A" packageName="a" annotation="true"/>` +
			`<class qualifiedName="a.I" name="I" packageName="a" interface="true"/></bajadoc>`,
			[]string{"a.A annotation_type", "a.I interface"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classes, ok := Parse([]byte(tt.data))
			var got []string
			for _, c := range classes {
				got = append(got, c.Pkg+"."+c.Name+" "+c.Kind)
			}
			if ok != tt.ok || fmt.Sprint(got) != fmt.Sprint(tt.classes) {
				t.Errorf("Parse: %v, %v; want %v, %v", got, ok, tt.classes, tt.ok)
			}
		})
	}
	classes, _ := Parse([]byte(tests[2].data))
	if got := classes[0].Desc; got != "x?yA" {
		t.Errorf("sanitized description %q; want %q", got, "x?yA")
	}
}

func TestParseClassModel(t *testing.T) {
	classes, ok := Parse([]byte(demoDoc))
	if !ok || len(classes) != 1 {
		t.Fatalf("Parse: %d classes, %v", len(classes), ok)
	}
	c := classes[0]
	checks := map[string][2]string{
		"module":     {c.Module, "demo-rt"},
		"modifiers":  {strings.Join(c.Modifiers, " "), "public abstract"},
		"since":      {c.Since, "Niagara 4.0"},
		"desc":       {c.Desc, "A demo class"},
		"extends":    {c.Extends, "javax.baja.sys.BComponent"},
		"implements": {strings.Join(c.Implements, "|"), "com.tridium.sys.BIThing|java.io.Serializable"},
		"members":    {fmt.Sprint(len(c.Members)), "7"},
		"varargs":    {c.Members[1].Params[0][0], "java.lang.String..."},
		"return":     {c.Members[1].Ret, "java.util.List<? extends javax.baja.sys.BObject, T>"},
		"topic type": {c.Members[6].Typ, "javax.baja.sys.BValue"},
	}
	for what, v := range checks {
		if v[0] != v[1] {
			t.Errorf("%s: %q; want %q", what, v[0], v[1])
		}
	}
}

func TestToCleanInfo(t *testing.T) {
	classes, _ := Parse([]byte(demoDoc))
	info := ToCleanInfo(classes[0])
	want := CleanInfo{ClassName: "BDemo", Kind: "class", Pkg: "javax.baja.demo", Since: "Niagara 4.0",
		Extends: "BComponent", Desc: "A demo class",
		Implements: []string{"BIThing", "java.io.Serializable"},
		Fields:     []string{"  int a", "  static Property b"},
		Ctors:      []string{"  BDemo(int n)"},
		Methods: []string{"  Type[][] alpha()",
			"  abstract static java.util.List<? extends BObject, T> zeta(java.lang.String... names)"},
	}
	if fmt.Sprintf("%#v", info) != fmt.Sprintf("%#v", want) {
		t.Errorf("ToCleanInfo:\n got %#v\nwant %#v", info, want)
	}
}

func TestFormatCleanInfo(t *testing.T) {
	tests := []struct {
		name string
		info CleanInfo
		want string
	}{
		{"header only", CleanInfo{ClassName: "BX", Kind: "annotation_type", Pkg: "a"},
			"ANNOTATION TYPE: BX\nPACKAGE: a\n"},
		{"every header line in order",
			CleanInfo{ClassName: "BX", Kind: "class", Pkg: "a", Since: "4.0", Extends: "BY",
				Implements: []string{"BI", "BJ"}, Subclasses: []string{"BS1", "BS2"}},
			"CLASS: BX\nPACKAGE: a\nSINCE: 4.0\nEXTENDS: BY\nIMPLEMENTS: BI, BJ\nSUBCLASSES: BS1, BS2\n"},
		{"description wrapped at 78 code points",
			CleanInfo{ClassName: "BX", Kind: "enum", Pkg: "a",
				Desc: strings.Repeat("é", 70) + " abcdefg " + strings.Repeat("y", 80) + " z"},
			"ENUM: BX\nPACKAGE: a\n\nDESCRIPTION:\n" + strings.Repeat("é", 70) + " abcdefg\n" +
				strings.Repeat("y", 80) + "\nz\n"},
		{"member sections",
			CleanInfo{ClassName: "BX", Kind: "interface", Pkg: "a", Fields: []string{"  int a"},
				Ctors: []string{"  BX()"}, Methods: []string{"  void m()"}},
			"INTERFACE: BX\nPACKAGE: a\n\nFIELDS:\n  int a\n\nCONSTRUCTORS:\n  BX()\n\nMETHODS:\n  void m()\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatCleanInfo(tt.info); got != tt.want {
				t.Errorf("FormatCleanInfo:\n got %q\nwant %q", got, tt.want)
			}
		})
	}
}

// ET.fromstring decodes the encodings expat knows (UTF-8, ISO-8859-1,
// US-ASCII, names case-insensitive); a byte above 0x7F in a US-ASCII
// document is not well-formed. Python's codecs also decode other
// single-byte names such as "latin1"; here they stay unparseable (see
// docs/helpbuild.md). The expected results are CPython 3.15's.
func TestParseEncodings(t *testing.T) {
	doc := func(enc, name string) []byte {
		return []byte(`<?xml version="1.0" encoding="` + enc + `"?><bajadoc><class qualifiedName="a.X" name="` +
			name + `" packageName="a" module="m"/></bajadoc>`)
	}
	for _, c := range []struct {
		enc, name, want string
		ok              bool
	}{
		{"ISO-8859-1", "BCaf\xe9", "BCafé", true},
		{"iso-8859-1", "B\xff", "Bÿ", true},
		{"us-ascii", "BCafe", "BCafe", true},
		{"US-ASCII", "BCaf\xe9", "", false},
		{"UTF-8", "BCafé", "BCafé", true},
		{"latin1", "BCaf\xe9", "", false},
	} {
		classes, ok := Parse(doc(c.enc, c.name))
		got := ""
		if len(classes) == 1 {
			got = classes[0].Name
		}
		if ok != c.ok || got != c.want {
			t.Errorf("%s %q: %q, %v; want %q, %v", c.enc, c.name, got, ok, c.want, c.ok)
		}
	}
}

// _sanitize keeps references to XML characters and replaces the others
// (NUL, other C0 controls, surrogates, U+FFFE/U+FFFF, beyond U+10FFFF,
// values beyond any integer type) with "?"; only a lowercase x starts a
// hexadecimal reference, as in the bytes pattern.
func TestSanitizeXML(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"&#x0;&#0;&#x9;&#10;&#13;&#x1F;", "??&#x9;&#10;&#13;?"},
		{"&#x41;&#00000065;&#x0000041;", "&#x41;&#00000065;&#x0000041;"},
		{"&#xD800;&#xDFFF;&#xD7FF;&#xE000;", "??&#xD7FF;&#xE000;"},
		{"&#xFFFE;&#xFFFF;&#xFFFD;&#x10000;&#x10FFFF;&#x110000;", "??&#xFFFD;&#x10000;&#x10FFFF;?"},
		{"&#99999999999999999999;&#xFFFFFFFFFFFFFFFFFF;", "??"},
		{"&#X0;&#x;&#;&amp;&#x0", "&#X0;&#x;&#;&amp;&#x0"},
	} {
		if got := string(sanitizeXML([]byte(c.in))); got != c.want {
			t.Errorf("%q: %q, want %q", c.in, got, c.want)
		}
	}
}

// parse_class's module ("<module>-<runtimeProfile>" when the profile is not
// empty) and _kind (enum, then annotation, then interface; only "true").
func TestClassModuleAndKind(t *testing.T) {
	for _, c := range []struct{ attrs, module, kind string }{
		{``, "", "class"},
		{`module="m" runtimeProfile="ux"`, "m-ux", "class"},
		{`module="m" runtimeProfile=""`, "m", "class"},
		{`runtimeProfile="rt"`, "-rt", "class"},
		{`interface="true" annotation="true" enum="true"`, "", "enum"},
		{`interface="true" category="annotation"`, "", "annotation_type"},
		{`interface="true" category="enum"`, "", "enum"},
		{`interface="TRUE"`, "", "class"},
		{`interface="true"`, "", "interface"},
	} {
		classes, ok := Parse([]byte(`<bajadoc><class qualifiedName="a.X" ` + c.attrs + `/></bajadoc>`))
		if !ok || len(classes) != 1 || classes[0].Module != c.module || classes[0].Kind != c.kind {
			t.Errorf("%s: %+v %v; want module %q, kind %q", c.attrs, classes, ok, c.module, c.kind)
		}
	}
}
