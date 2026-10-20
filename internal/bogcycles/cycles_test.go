package bogcycles

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/bog"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/golden"
)

// TestGolden replays the cases frozen from tools/bog-cycles.py by the retired testdata/capture.py.
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }

// The behaviour tests below port tools/tests/test_bog_cycles.py and the bog-cycles cases of
// test_r5_advisories.py and test_t6a_quick_wins.py.

func doc(body string) string {
	return "<?xml version='1.0'?><bajaObjectGraph version='4.0'><p n='R' h='1' m='b=baja' t='b:Folder'>" +
		body + "</p></bajaObjectGraph>"
}

func link(name string, src int, sslot, tslot string) string {
	return fmt.Sprintf("<p n='%s' t='b:Link'><p n='sourceOrd' v='h:%d'/><p n='sourceSlotName' v='%s'/>"+
		"<p n='targetSlotName' v='%s'/></p>", name, src, sslot, tslot)
}

func folder(name string, h int, body string) string {
	return fmt.Sprintf("<p n='%s' h='%d' t='b:Folder'>%s</p>", name, h, body)
}

func find(xml string) Result { return Find(bog.Parse("<xml>", xml)) }

// only is the single cycle of res; it fails the test unless there is exactly one, with edges.
func only(t *testing.T, res Result) Cycle {
	t.Helper()
	if len(res.Cycles) != 1 || len(res.Cycles[0].Edges) == 0 {
		t.Fatalf("want one cycle with edges, got %+v", res.Cycles)
	}
	return res.Cycles[0]
}

func TestTwoComponentLoopDetected(t *testing.T) {
	res := find(doc(folder("A", 2, link("Link", 3, "out", "in")) + folder("B", 3, link("Link", 2, "out", "in"))))
	if len(res.Cycles) != 1 || strings.Join(res.Cycles[0].Components, ",") != "/R/A,/R/B" {
		t.Fatalf("cycles %+v", res.Cycles)
	}
}

func TestChainHasNoCycle(t *testing.T) {
	res := find(doc(folder("A", 2, "") + folder("B", 3, link("Link", 2, "out", "in")) + folder("C", 4, link("Link", 3, "out", "in"))))
	if len(res.Cycles) != 0 {
		t.Fatalf("cycles %+v", res.Cycles)
	}
}

func TestSelfLoopAndItsEdge(t *testing.T) {
	res := find(doc(folder("A", 2, link("Link", 2, "out", "in"))))
	c := only(t, res)
	if c.Components[0] != "/R/A" {
		t.Fatalf("cycles %+v", res.Cycles)
	}
	e := c.Edges[0]
	if e.Source != "/R/A" || *e.SourceSlot != "out" || e.Target != "/R/A" || *e.TargetSlot != "in" {
		t.Errorf("edge %+v", e)
	}
}

func TestThreeCycleAndUnrelatedNode(t *testing.T) {
	res := find(doc(folder("A", 2, link("Link", 4, "out", "in")) + folder("B", 3, link("Link", 2, "out", "in")) +
		folder("C", 4, link("Link", 3, "out", "in")) + folder("D", 5, link("Link", 2, "out", "in"))))
	if len(res.Cycles) != 1 || strings.Join(res.Cycles[0].Components, ",") != "/R/A,/R/B,/R/C" || res.Links != 4 {
		t.Fatalf("result %+v", res)
	}
}

func TestDanglingLinkIsReportedAndNotCounted(t *testing.T) {
	res := find(doc(folder("A", 2, link("Link", 3, "out", "in")+link("Link1", 99, "o", "i")) + folder("B", 3, "")))
	if len(res.Cycles) != 0 || len(res.Dangling) != 1 || res.Links != 1 {
		t.Fatalf("result %+v", res)
	}
}

func TestCycleListsItsSlotEdgesSorted(t *testing.T) {
	res := find(doc(folder("A", 2, link("Link", 3, "out", "in")) +
		folder("B", 3, link("Link", 2, "out", "in")+link("Link1", 2, "x", "y")) + folder("C", 4, link("Link", 2, "out", "in"))))
	var got []string
	for _, e := range only(t, res).Edges {
		got = append(got, e.Source+"."+*e.SourceSlot+">"+e.Target+"."+*e.TargetSlot)
	}
	if strings.Join(got, " ") != "/R/A.out>/R/B.in /R/A.x>/R/B.y /R/B.out>/R/A.in" {
		t.Errorf("edges %v", got)
	}
}

func TestEscapedNamesAreDecoded(t *testing.T) {
	res := find(doc("<p n='Room$201' h='2' t='b:Folder'>" + link("Link", 2, "out$20a", "in") + "</p>"))
	if c := only(t, res); c.Components[0] != "/R/Room 1" || *c.Edges[0].SourceSlot != "out a" {
		t.Errorf("cycle %+v", c)
	}
}

func TestUnnamedStationRootRendersAsSlash(t *testing.T) {
	res := find("<?xml version='1.0'?><bajaObjectGraph version='4.0'><p h='1' m='b=baja' t='b:Station'>" +
		"<p n='A' h='2' t='b:Folder'><p n='Link' t='b:Link'><p n='sourceOrd' v='h:2'/></p></p></p></bajaObjectGraph>")
	if c := only(t, res); c.Components[0] != "/A" {
		t.Errorf("components %v", c.Components)
	}
}

func run(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := Main(args, cli.Stdio{Out: &out, Err: &errb})
	return code, out.String(), errb.String()
}

func write(t *testing.T, name string, data []byte) string {
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
	if err == nil {
		_, err = w.Write(data)
	}
	if err == nil {
		err = z.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestZipDistAndExitCodes(t *testing.T) {
	loop := []byte(doc(folder("A", 2, link("Link", 2, "out", "in"))))
	inner := zipOf(t, "file.xml", loop)
	if code, _, _ := run(write(t, "x.bog", inner)); code != 1 {
		t.Errorf("cycles found: exit %d", code)
	}
	if code, _, _ := run(write(t, "y.bog", zipOf(t, "file.xml", []byte(doc(folder("A", 2, "")))))); code != 0 {
		t.Errorf("clean graph: exit %d", code)
	}
	if code, _, _ := run(write(t, "b.dist", zipOf(t, "stations/S/config.bog", inner))); code != 1 {
		t.Errorf("dist backup: exit %d", code)
	}
}

func TestUnreadableOrMalformedInputExits2(t *testing.T) {
	for name, data := range map[string]string{"e.bog": "", "t.bog": "not xml", "z.bog": "PK\x03\x04junk",
		"trunc.xml": "<?xml version='1.0'?><bajaObjectGraph version='4.0'><p n='A' h='1'>",
		"bad.xml":   "<?xml version='1.0'?><bajaObjectGraph version='4.0'><p n='A' h='1 t=x></p></bajaObjectGraph>"} {
		code, out, errText := run(write(t, name, []byte(data)))
		if code != 2 || out != "" || !strings.HasPrefix(errText, "bog-cycles: ") {
			t.Errorf("%s: exit %d out %q err %q", name, code, out, errText)
		}
		if strings.HasSuffix(name, ".xml") && !strings.Contains(errText, "not well-formed XML") {
			t.Errorf("%s: %q", name, errText)
		}
	}
	if code, _, _ := run(filepath.Join(t.TempDir(), "missing.bog")); code != 2 {
		t.Errorf("missing: exit %d", code)
	}
	if code, _, _ := run(); code != 2 {
		t.Errorf("no arguments: exit %d", code)
	}
}

func TestMissingSlotNamesPrintAsDash(t *testing.T) {
	code, out, _ := run(write(t, "x.xml", []byte(doc("<p n='A' h='2' t='b:Folder'><p n='Link' t='b:Link'>"+
		"<p n='sourceOrd' v='h:2'/></p><p n='Link1' t='b:Link'><p n='sourceOrd' v='h:77'/></p></p>"))))
	if code != 1 || !strings.Contains(out, "      /R/A.- -> /R/A.-\n") || !strings.Contains(out, "  DANGLING: h:77 -> /R/A.-\n") ||
		strings.Contains(out, "None") {
		t.Errorf("exit %d:\n%s", code, out)
	}
}
