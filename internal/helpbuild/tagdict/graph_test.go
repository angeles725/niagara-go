package tagdict

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func zipped(t *testing.T, xml string) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	w, err := z.Create("file.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(xml)); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// dump is the tree below n, one "name type value" line per node, indented.
func dump(n *node, indent string) string {
	var b strings.Builder
	for _, k := range n.kids {
		b.WriteString(indent + k.name + " " + k.typ + " " + k.value + "\n")
		b.WriteString(dump(k, indent+" "))
	}
	return b.String()
}

func TestParseGraphResolvesTypes(t *testing.T) {
	graph := `<bajaObjectGraph version="4.0">
<p m="b=baja" t="b:UnrestrictedFolder">
 <p n="a$20b" m="td=tagdictionary" t="td:Dict">
  <p n="v" t="b:String" v="1.5"/>
  <p n="filter" v="hs:site"/>
 </p>
</p>
</bajaObjectGraph>`
	want := " baja:UnrestrictedFolder \n a b tagdictionary:Dict \n  v baja:String 1.5\n  filter  hs:site\n"
	for name, data := range map[string][]byte{
		"plain":         []byte(graph),
		"zipped":        zipped(t, graph),
		"comment-first": []byte("<!-- Copyright -->\n<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" + graph),
	} {
		top, err := parseGraph(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := dump(top, ""); got != want {
			t.Errorf("%s:\n%s", name, got)
		}
	}
}

func TestParseGraphFailsClosed(t *testing.T) {
	for name, data := range map[string]string{
		"malformed":        `<bajaObjectGraph><p t="x`,
		"wrong root":       `<graph><p/></graph>`,
		"undeclared":       `<bajaObjectGraph><p t="q:Folder"/></bajaObjectGraph>`,
		"bad declaration":  `<bajaObjectGraph><p m="b" t="b:Folder"/></bajaObjectGraph>`,
		"zip without file": string(zipped(t, "")[:20]),
	} {
		if top, err := parseGraph([]byte(data)); err == nil {
			t.Errorf("%s: read as %s", name, dump(top, ""))
		}
	}
}
