package stationmodules

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

const station = "<?xml version='1.0'?><bajaObjectGraph version='4.0'>" +
	"<p h='1' m='b=baja CRP=ColdRoomPan c=control' t='b:Station'>" +
	"<p n='R' h='2' t='CRP:ColdRoom'/><p n='W' h='3' t='c:NumericWritable'/>" +
	"<p n='D' h='4' m=\"dsb=dashboard\" t=\"dsb:Dashboard\"/></p></bajaObjectGraph>"

func TestMultiDeclarationAliasesAllResolved(t *testing.T) {
	alias, types := stationRefs(station)
	if want := map[string]string{"b": "baja", "CRP": "ColdRoomPan", "c": "control", "dsb": "dashboard"}; !reflect.DeepEqual(alias, want) {
		t.Errorf("aliases %v", alias)
	}
	if got := keys(types["ColdRoomPan"]); !reflect.DeepEqual(got, []string{"ColdRoom"}) {
		t.Errorf("ColdRoomPan types %v", got)
	}
	if got := keys(types["control"]); !reflect.DeepEqual(got, []string{"NumericWritable"}) {
		t.Errorf("control types %v", got)
	}
}

func keys(m map[string]bool) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestStationRefsUseUnicodeWordBoundaries(t *testing.T) {
	alias, _ := stationRefs("<p xm='a=b' ém='c=d' m='e=f g' t='a:X'/><p ext='e:Y'/>")
	if want := map[string]string{"e": "f"}; !reflect.DeepEqual(alias, want) {
		t.Errorf("aliases %v", alias)
	}
	_, types := stationRefs("<p m='e=f' ext='e:Y' t=\"e:Z_1\" t='e:bad-type'/>")
	if got := keys(types["f"]); !reflect.DeepEqual(got, []string{"Y", "Z_1"}) {
		t.Errorf("types %v", got)
	}
}

func TestLocateFixPrefersTheRealManifestVersion(t *testing.T) {
	versions := map[string]string{"a": "4.14.0.162", "b": "1.0", "c": "4.14.0.9", "d": "4.14.0.10", "e": ""}
	ver := func(p string) string { return versions[p] }
	cases := []struct {
		cands []string
		want  string
		hint  string
	}{
		{[]string{"b", "a"}, "a", "4.14.0.162"},
		{[]string{"b", "c", "d"}, "c", "4.14.0.1"}, // highest of the same x.y.z, as strings
		{[]string{"b", "e"}, "b", "4.13.0.1"},      // no near match: highest real version, else all
		{[]string{"e", "b", "c"}, "c", ""},
	}
	for _, c := range cases {
		if got, _ := locateFix(c.cands, c.hint, ver); got != c.want {
			t.Errorf("%v hint %q: got %q want %q", c.cands, c.hint, got, c.want)
		}
	}
}

// TestParseModuleXMLLikeElementTree: root attributes, every <type>/<dependency> name in
// document order (the root included, as root.iter() includes it), namespaced tags not
// matched, and None for a document ElementTree rejects.
func TestParseModuleXMLLikeElementTree(t *testing.T) {
	p := parseModuleXML([]byte(`<?xml version="1.0" encoding="UTF-8"?><module name="kitControl-rt" moduleName="kitControl"
 runtimeProfile="rt" vendor="Tridium" vendorVersion="4.14.0.162"><dependencies><dependency name="baja"/>
 <dependency name=""/><dependency name="control-rt"/></dependencies><types><type name="Add"/><type class="x"/>
 <x:type xmlns:x="urn:x" name="Hidden"/></types></module>`))
	if p == nil || p.name != "kitControl-rt" || p.module != "kitControl" || p.profile != "rt" || p.vendor != "Tridium" ||
		p.version != "4.14.0.162" || !reflect.DeepEqual(p.deps, []string{"baja", "control-rt"}) ||
		!reflect.DeepEqual(keys(p.types), []string{"Add"}) {
		t.Errorf("part %+v", p)
	}
	if p := parseModuleXML([]byte(`<type name="Self"/>`)); p == nil || !p.types["Self"] {
		t.Errorf("root <type> not counted: %+v", p)
	}
	if p := parseModuleXML([]byte(`<module name="a"><dependency></module>`)); p != nil {
		t.Errorf("malformed module.xml parsed: %+v", p)
	}
}

// TestReadZipMemberKeepsTheLastEntry: zipfile.read() returns the last of repeated names;
// a missing member or a file that is not a ZIP is None.
func TestReadZipMemberKeepsTheLastEntry(t *testing.T) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for _, body := range []string{"first", "second"} {
		w, err := z.Create("META-INF/module.xml")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	jar, notZip := filepath.Join(dir, "a.jar"), filepath.Join(dir, "b.jar")
	if err := os.WriteFile(jar, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(notZip, []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if data, ok := readZipMember(jar, "META-INF/module.xml"); !ok || string(data) != "second" {
		t.Errorf("repeated member: %q %v", data, ok)
	}
	if _, ok := readZipMember(jar, "missing"); ok {
		t.Error("missing member read")
	}
	if _, ok := readZipMember(notZip, "META-INF/module.xml"); ok {
		t.Error("non-ZIP read")
	}
}
