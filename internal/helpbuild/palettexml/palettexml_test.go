package palettexml

import (
	"archive/zip"
	"bytes"
	"reflect"
	"strings"
	"testing"
)

// kitControlPalette is a trimmed kitControl-rt module.palette laid out as
// Niagara 4.15 ships it: a module prefix declared once and used afterwards,
// nested folders, an entry with properties (one of them typed, one only a
// value) and a property that holds its own child.
const kitControlPalette = `<?xml version="1.0" encoding="UTF-8"?>
<bajaObjectGraph version="1.0">
<p m="b=baja" t="b:UnrestrictedFolder">
 <!-- ControlPalette -->
 <p n="ControlPalette" t="b:UnrestrictedFolder">
  <p n="Points" t="b:Folder">
   <p n="BooleanPoint" m="c=control" t="c:BooleanPoint"/>
  </p>
  <p n="Trigger" t="b:UnrestrictedFolder">
   <p n="Daily" t="c:TimeTrigger">
    <p n="triggerMode" t="c:DailyTriggerMode">
     <p n="days" m="k=kitControl" t="k:Weekdays" v="mon,tue"/>
    </p>
    <p n="enabled" v="true"/>
   </p>
  </p>
 </p>
 <p n="Latch$20Blocks" t="b:UnrestrictedFolder">
  <p n="BooleanLatch" t="k:BooleanLatch"/>
 </p>
</p>
</bajaObjectGraph>
`

func TestParseBuildsTheFolderTree(t *testing.T) {
	p, err := Parse([]byte(kitControlPalette))
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	var walk func(prefix string, n *Node)
	walk = func(prefix string, n *Node) {
		for _, c := range n.Children {
			lines = append(lines, prefix+c.Name+" "+c.Type)
			walk(prefix+c.Name+"/", c)
		}
	}
	walk("", p.Root)
	want := []string{
		"ControlPalette baja:UnrestrictedFolder",
		"ControlPalette/Points baja:Folder",
		"ControlPalette/Points/BooleanPoint control:BooleanPoint",
		"ControlPalette/Trigger baja:UnrestrictedFolder",
		"ControlPalette/Trigger/Daily control:TimeTrigger",
		"Latch Blocks baja:UnrestrictedFolder",
		"Latch Blocks/BooleanLatch kitControl:BooleanLatch",
	}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("walk:\n%s", strings.Join(lines, "\n"))
	}
	daily := p.Root.Children[0].Children[1].Children[0]
	wantProps := []Property{{"triggerMode", "control:DailyTriggerMode", ""}, {"enabled", "", "true"}}
	if !reflect.DeepEqual(daily.Properties, wantProps) {
		t.Errorf("properties = %+v", daily.Properties)
	}
	if !reflect.DeepEqual(daily.Contains, []string{"control:DailyTriggerMode", "kitControl:Weekdays"}) {
		t.Errorf("contains = %v", daily.Contains)
	}
}

// TestParseReadsWhatNiagaraShips: a copyright comment before the XML
// declaration (edgeIo-rt) and a zipped object graph (honeywellVenomGraphics)
// both read; a long value is cut with a visible mark.
func TestParseReadsWhatNiagaraShips(t *testing.T) {
	lead := "<!--\n Copyright (c) 2017 Tridium\n -->\n" + kitControlPalette
	if p, err := Parse([]byte(lead)); err != nil || len(p.Root.Children) != 2 {
		t.Fatalf("leading comment: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("file.xml")
	long := strings.Repeat("x", 300)
	_, _ = w.Write([]byte(strings.Replace(kitControlPalette, `v="true"`, `v="`+long+`"`, 1)))
	_ = zw.Close()
	p, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	v := p.Root.Children[0].Children[1].Children[0].Properties[1].Value
	if len([]rune(v)) != MaxValue+1 || !strings.HasSuffix(v, "…") {
		t.Errorf("value not cut: %d runes", len([]rune(v)))
	}
}

func TestParseRefusesWhatItCannotRead(t *testing.T) {
	for name, doc := range map[string]string{
		"not xml":        `<bajaObjectGraph><p t="b:Folder">`,
		"wrong root":     `<module name="x"/>`,
		"no palette":     `<bajaObjectGraph version="1.0"/>`,
		"unknown prefix": `<bajaObjectGraph><p m="b=baja" t="b:Folder"><p n="x" t="q:Thing"/></p></bajaObjectGraph>`,
		"bad module":     `<bajaObjectGraph><p m="baja" t="b:Folder"/></bajaObjectGraph>`,
		"zip without":    "PK\x03\x04 not really a zip",
	} {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
