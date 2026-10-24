package modulexml

import (
	"reflect"
	"strings"
	"testing"
)

// kitControlUX is a trimmed module.xml with one of each element nh indexes,
// laid out as Niagara 4.15 writes them (a java-permission indented off its
// siblings included, as opcUaServer-rt's is).
const kitControlUX = `<?xml version="1.0" encoding="UTF-8"?>
<module name="kitControl-ux" bajaVersion="0" vendor="Tridium" vendorVersion="4.15.6.20" description="Kit Control UX" moduleName="kitControl" runtimeProfile="ux">
 <dependencies>
  <dependency name="baja" vendor="Tridium" vendorVersion="4.15.6"/>
  <dependency name="kitControl-rt" vendor="Tridium" vendorVersion="4.15.6"/>
 </dependencies>
 <dirs/>
 <installation>
  <dependencies>
   <nre name="nre-core-*" version="4.15.6.20"/>
  </dependencies>
 </installation>
 <defs>
  <def name="kit.icon" value="module://kitControl/icon.png"/>
 </defs>
 <types>
  <type class="com.tridium.kitControl.ux.BKitControlJsBuild" name="KitControlJsBuild"/>
  <type name="LatchTypeExt" class="com.tridium.kitControl.ux.BLatchTypeExt">
   <agent requiredPermissions="r" app="webShell">
    <on type="kitControl:BooleanLatch"/>
    <on type="kitControl:NumericLatch"/>
   </agent>
   <adapter from="baja:RelTime" to="control:TriggerMode"/>
  </type>
 </types>
 <permissions>
  <java-permissions type="station">
   <java-permission class="java.lang.RuntimePermission" name="setIO"/>
       <java-permission action="read" class="java.io.FilePermission" name="${/}x"/>
  </java-permissions>
  <java-permissions type="all"/>
 </permissions>
</module>
`

func TestParseReadsEveryIndexedElement(t *testing.T) {
	m, err := Parse([]byte(kitControlUX))
	if err != nil {
		t.Fatal(err)
	}
	latch := "com.tridium.kitControl.ux.BLatchTypeExt"
	want := &Module{Name: "kitControl-ux", Part: "kitControl", Profile: "ux", Vendor: "Tridium",
		Version: "4.15.6.20", Description: "Kit Control UX",
		Dependencies: []Dependency{{"baja", "Tridium", "4.15.6"}, {"kitControl-rt", "Tridium", "4.15.6"}},
		Types:        []Type{{"KitControlJsBuild", "com.tridium.kitControl.ux.BKitControlJsBuild"}, {"LatchTypeExt", latch}},
		Agents: []Agent{{Type: "LatchTypeExt", Class: latch, On: []string{"kitControl:BooleanLatch", "kitControl:NumericLatch"},
			App: "webShell", Perms: "r"}},
		JavaPermissions: []JavaPermission{{"station", "java.lang.RuntimePermission", "setIO", ""},
			{"station", "java.io.FilePermission", "${/}x", "read"}},
		Defs:     []Def{{"kit.icon", "module://kitControl/icon.png"}},
		Adapters: []Adapter{{"LatchTypeExt", "baja:RelTime", "control:TriggerMode"}},
	}
	if !reflect.DeepEqual(m, want) {
		t.Errorf("Parse:\n got %+v\nwant %+v", m, want)
	}
	if c := m.Counts(); c != (Counts{Dependencies: 2, Types: 2, Agents: 1, JavaPermissions: 2, Defs: 1, Adapters: 1}) {
		t.Errorf("counts = %+v", c)
	}
}

// TestParseFailsClosed: a document nh cannot read as a module descriptor is an
// error naming why, never an empty module.
func TestParseFailsClosed(t *testing.T) {
	for _, tt := range []struct{ name, doc, want string }{
		{"malformed", `<module name="a"><types></module>`, "XML syntax error"},
		{"wrong root", `<project name="a"/>`, "root element is <project>, not <module>"},
		{"nameless", `<module vendor="x"/>`, "<module> has no name"},
		{"empty", ``, "no root element"},
		{"agent outside a type", `<module name="a"><agent><on type="b:C"/></agent></module>`, "<agent> outside a <type>"},
		{"on outside an agent", `<module name="a"><types><type name="T" class="c"><on type="b:C"/></type></types></module>`,
			"<on> outside an <agent>"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m, err := Parse([]byte(tt.doc))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Parse = %v, %v; want an error containing %q", m, err, tt.want)
			}
		})
	}
}

// TestParseReadsTheLexicons: a niagaraLexicon* translation descriptor names,
// per <lexicon>, the base module it overrides, the resource in its own jar and
// the language; a descriptor without <lexicons> has none.
func TestParseReadsTheLexicons(t *testing.T) {
	doc := `<module name="niagaraLexiconBe-rt" moduleName="niagaraLexiconBe" runtimeProfile="rt">
 <lexicons>
  <lexicon module="baja" resource="baja.lexicon" language="be" default="true"/>
  <lexicon module="pdf" resource="pdf.lexicon" language="be"/>
 </lexicons>
</module>`
	m, err := Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	want := []Lexicon{{"baja", "baja.lexicon", "be", "true"}, {"pdf", "pdf.lexicon", "be", ""}}
	if !reflect.DeepEqual(m.Lexicons, want) {
		t.Errorf("Lexicons = %+v, want %+v", m.Lexicons, want)
	}
	if m, err := Parse([]byte(kitControlUX)); err != nil || m.Lexicons != nil {
		t.Errorf("kitControl-ux Lexicons = %+v, %v; want none", m, err)
	}
}
