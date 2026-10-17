package bognav

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/bog"
	"github.com/angeles725/niagara-go/internal/cli"
)

// The behavioural tests of niagara-research tools/tests/test_bog_nav.py that the CLI goldens do
// not pin (the reader and model internals are tested in internal/bog).

const testXML = `<?xml version='1.0' encoding='UTF-8'?>
<bajaObjectGraph version='4.0'>
 <p h='1' m='b=baja c=control conv=converters' t='b:Station'>
  <p n='Services' h='2' t='b:ServiceContainer'>
   <p n="JobService" h="c" t="b:JobService"/>
   <p n='object$20tags' h='3' t='b:Folder'/>
  </p>
  <p n='A' h='4' t='c:NumericWritable'/>
  <p n='B' h='5' t='c:BooleanWritable'>
   <p n='Link20' t='b:ConversionLink'>
    <p n="sourceOrd" t="b:Ord" v="h:4"/>
    <p n="sourceSlotName" v="out"/>
    <p n="targetSlotName" v="in10"/>
    <p n="converter" t="conv:NumericToStatusBoolean"/>
   </p>
  </p>
 </p>
</bajaObjectGraph>`

func runMain(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	t.Setenv("NIAGARA_ORGANIZED", "off")
	var out, errb bytes.Buffer
	code := Main(args, cli.Stdio{Out: &out, Err: &errb})
	return out.String(), errb.String(), code
}

func writeFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func zipOf(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	w, err := z.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	w.Write(data)
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestFindListsSelfClosingComponent(t *testing.T) {
	out, _, code := runMain(t, writeFile(t, "x.xml", []byte(testXML)), "find", "--type", "b:JobService")
	if code != 0 || !strings.Contains(out, "h:c ") {
		t.Errorf("code %d out %q", code, out)
	}
}

func TestMissReportsScopeAndCounts(t *testing.T) {
	p := writeFile(t, "x.xml", []byte(testXML))
	out, _, code := runMain(t, p, "find", "--type", "b:Nope")
	if code != cli.ExitNoMatch || !strings.Contains(out, "searched "+p+": 6 components, 1 links") {
		t.Errorf("code %d out %q", code, out)
	}
}

func TestUnreadableInputsExit3(t *testing.T) {
	deep := []byte(testXML)
	deep = zipOf(t, "file.xml", deep)
	for i := 0; i < 5; i++ {
		deep = zipOf(t, "config.bog", deep)
	}
	cases := map[string][]byte{
		"empty.bog":   {},
		"text.bog":    []byte("hello, not a bog\n"),
		"badzip.bog":  []byte("PK\x03\x04garbage"),
		"deep.dist":   deep,
		"nobog.dist":  zipOf(t, "readme.txt", []byte("x")),
		"blank.bog":   []byte("  \n\t\n"),
		"notbaja.xml": []byte("<?xml version='1.0'?><other/>"),
	}
	for name, data := range cases {
		_, errOut, code := runMain(t, writeFile(t, name, data), "tree")
		if code != bog.ExitBadInput || !strings.HasPrefix(errOut, "bog-nav: ") {
			t.Errorf("%s: code %d stderr %q", name, code, errOut)
		}
	}
	if _, _, code := runMain(t, filepath.Join(t.TempDir(), "nope.bog"), "tree"); code != bog.ExitBadInput {
		t.Errorf("missing file: code %d", code)
	}
}

func TestDistWithNestedBogAndPlainXMLRead(t *testing.T) {
	dist := zipOf(t, "stations/S/config.bog", zipOf(t, "file.xml", []byte(testXML)))
	for name, data := range map[string][]byte{"backup.dist": dist, "x.bog": []byte(testXML)} {
		out, errOut, code := runMain(t, writeFile(t, name, data), "find", "--type", "b:JobService")
		if code != 0 || !strings.Contains(out, "h:c ") {
			t.Errorf("%s: code %d out %q err %q", name, code, out, errOut)
		}
	}
}

func TestEscapedLinkSlotAcceptsEitherSpelling(t *testing.T) {
	xml := strings.Replace(testXML, `<p n="targetSlotName" v="in10"/>`, `<p n="targetSlotName" v="in$2010"/>`, 1)
	p := writeFile(t, "x.xml", []byte(xml))
	for _, needle := range []string{"in 10", "in$2010"} {
		out, _, code := runMain(t, p, "links", "--to", "B", "--slot", needle)
		if code != 0 || !strings.Contains(out, "A.out  -->  B.in 10") {
			t.Errorf("%s: code %d out %q", needle, code, out)
		}
	}
}

func TestSlotResolvesEscapedNameBothWays(t *testing.T) {
	p := writeFile(t, "x.xml", []byte(testXML))
	for _, ref := range []string{"Services/object tags", "Services/object$20tags", "h:3"} {
		out, _, code := runMain(t, p, "slot", ref, "--no-defaults")
		if code != 0 || !strings.HasPrefix(out, "Services/object tags  h:3  <b:Folder>") {
			t.Errorf("%s: code %d out %q", ref, code, out)
		}
	}
}

const javaWithProps = `package com.angeles.ColdRoomPan;
@NiagaraProperty(
  name = "valveMode",
  type = "String",
  defaultValue = "auto"
)
@NiagaraProperty(name = "setpoint", type = "double", defaultValue = "3")
public class BColdRoom extends BComponent {}
`

func TestSrcTypesAndDanglingLinks(t *testing.T) {
	src := t.TempDir()
	pkg := filepath.Join(src, "com", "angeles")
	hidden := filepath.Join(src, ".git")
	for _, d := range []string{pkg, hidden} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(pkg, "BColdRoom.java"), []byte(strings.ReplaceAll(javaWithProps, "\n", "\r\n")), 0o644)
	os.WriteFile(filepath.Join(hidden, "BHidden.java"), []byte(`@NiagaraProperty(name = "ghost", type = "int")`), 0o644)
	got := sourceTypes(src)
	if len(got) != 2 || got["valveMode"] != "String" || got["setpoint"] != "double" {
		t.Errorf("sourceTypes = %v", got)
	}
	xml := `<bajaObjectGraph version='4.0'>
 <p h='1' t='b:Station' m='b=baja CRP=ColdRoomPan c=control'>
  <p n='Room' h='2' t='CRP:ColdRoom'>
   <p n="valveMode" v="hand"/>
   <p n='Link' t='b:Link'>
    <p n="sourceOrd" v="h:3"/>
    <p n="sourceSlotName" v="out"/>
    <p n="targetSlotName" v="nosuchSlot"/>
   </p>
   <p n='Link1' t='b:Link'>
    <p n="sourceOrd" v="h:3"/>
    <p n="sourceSlotName" v="out"/>
    <p n="targetSlotName" v="setpoint"/>
   </p>
  </p>
  <p n='Pt' h='3' t='c:NumericWritable'/>
 </p>
</bajaObjectGraph>`
	p := writeFile(t, "x.xml", []byte(xml))
	out, _, code := runMain(t, p, "links", "--dangling", "--src", src)
	if code != 0 || out != "Pt.out  -->  Room.nosuchSlot  [DANGLING tgt]\n" {
		t.Errorf("dangling: code %d out %q", code, out)
	}
	out, _, _ = runMain(t, p, "slot", "Room", "valveMode", "--src", src)
	if !strings.Contains(out, "type=String (from source)") || !strings.Contains(out, "write=simple") {
		t.Errorf("slot --src: %q", out)
	}
}

func TestSelftestPasses(t *testing.T) {
	out, _, code := runMain(t, "selftest")
	if code != 0 || !strings.HasSuffix(out, "\nSELFTEST OK\n") || strings.Contains(out, "FAIL") {
		t.Errorf("code %d\n%s", code, out)
	}
}

func TestOrganizedOffSaysSo(t *testing.T) {
	p := writeFile(t, "x.xml", []byte(testXML))
	out, _, _ := runMain(t, p, "slot", "A")
	if !strings.HasSuffix(out, "  (defaults: no organized/ tree; set NIAGARA_ORGANIZED)\n") {
		t.Errorf("out %q", out)
	}
}

// Intended difference: Python's re.error traceback (exit 1) becomes one usage line, exit 2.
func TestGrepInvalidRegexIsAUsageError(t *testing.T) {
	out, errOut, code := runMain(t, writeFile(t, "x.xml", []byte(testXML)), "grep", "(")
	if code != cli.ExitCannotSearch || out != "" || !strings.HasPrefix(errOut, "bog-nav: invalid regex '(': ") {
		t.Errorf("code %d out %q err %q", code, out, errOut)
	}
}
