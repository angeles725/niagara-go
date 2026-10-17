package bog

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// oracle is the ground truth of ParseTest.test_recall_every_handle_and_link_against_elementtree,
// taken from a real XML token stream: every <p h=..> handle and every <p t=..Link> element.
func oracle(t *testing.T, text string) (handles map[string]bool, links int) {
	t.Helper()
	handles = map[string]bool{}
	dec := xml.NewDecoder(strings.NewReader(text))
	dec.Strict = true
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return handles, links
		}
		if err != nil {
			t.Fatalf("oracle: input is not well-formed XML: %v", err)
		}
		el, ok := tok.(xml.StartElement)
		if !ok || el.Name.Local != "p" {
			continue
		}
		for _, a := range el.Attr {
			switch a.Name.Local {
			case "h":
				if a.Value != "" {
					handles[a.Value] = true
				}
			case "t":
				if strings.HasSuffix(a.Value, "Link") {
					links++
				}
			}
		}
	}
}

func checkRecall(t *testing.T, label, text string) {
	t.Helper()
	handles, links := oracle(t, text)
	b := Parse(label, text)
	got := map[string]bool{}
	for _, h := range b.Comps.Keys() {
		if h != "" {
			got[h] = true
		}
	}
	missing, extra := 0, 0
	for h := range handles {
		if !got[h] {
			missing++
		}
	}
	for h := range got {
		if !handles[h] {
			extra++
		}
	}
	if missing != 0 || extra != 0 || len(b.Links) != links {
		t.Fatalf("%s: handles oracle %d model %d (missing %d, extra %d); links oracle %d model %d",
			label, len(handles), len(got), missing, extra, links, len(b.Links))
	}
}

func TestRecallEveryHandleAndLinkAgainstXMLTokens(t *testing.T) {
	for _, name := range []string{"basic.xml", "basic-escaped.xml", "cycles.xml", "escaped.xml", "proxy.xml", "station.xml"} {
		text, err := ReadText(filepath.Join("testdata", "model", name))
		if err != nil {
			t.Fatal(err)
		}
		checkRecall(t, name, text)
	}
	checkRecall(t, "synthetic", syntheticStation(40, 25))
}

// TestRecallRealBogs runs the oracle on real station files named in BOG_RECALL_FILES
// (os.PathListSeparator-separated): station bogs never enter the repository.
func TestRecallRealBogs(t *testing.T) {
	list := os.Getenv("BOG_RECALL_FILES")
	if list == "" {
		t.Skip("BOG_RECALL_FILES not set")
	}
	for _, path := range filepath.SplitList(list) {
		text, err := ReadText(path)
		if err != nil {
			t.Fatal(err)
		}
		checkRecall(t, path, text)
	}
}

// syntheticStation is a well-formed station: folders x points, each point with a composite
// slot, a fallback, an action, mixed quoting and one link from the previous point.
func syntheticStation(folders, points int) string {
	var sb strings.Builder
	sb.WriteString("<?xml version='1.0' encoding='UTF-8'?>\n<bajaObjectGraph version='4.0'>\n")
	sb.WriteString(" <p h='1' m='b=baja c=control' t='b:Station'>\n")
	h := 2
	for f := 0; f < folders; f++ {
		fmt.Fprintf(&sb, "  <p n='Folder$20%d' h='%x' t='b:Folder'>\n", f, h)
		h++
		prev := 0
		for p := 0; p < points; p++ {
			fmt.Fprintf(&sb, "   <p n=\"Pt%d\" h=\"%x\" t=\"c:NumericWritable\">\n", p, h)
			sb.WriteString("    <p n='out' t='b:StatusNumeric'><p n='value' v='1.0'/></p>\n")
			sb.WriteString("    <p n='fallback' t='b:StatusNumeric'><p n='status' v='{null}'/></p>\n")
			sb.WriteString("    <p n='facets' v='units=u:celsius' f='r'/>\n    <a n='set' f='s'/>\n")
			if prev != 0 {
				fmt.Fprintf(&sb, "    <p n='Link' t='b:Link'>\n     <p n='sourceOrd' v='h:%x'/>\n", prev)
				sb.WriteString("     <p n='sourceSlotName' v='out'/>\n     <p n='targetSlotName' v='in16'/>\n    </p>\n")
			}
			fmt.Fprintf(&sb, "    <p n='Alarm' h='%x' t='alarm:OutOfRangeAlarmExt'/>\n", h+1)
			sb.WriteString("   </p>\n")
			prev = h
			h += 2
		}
		sb.WriteString("  </p>\n")
	}
	sb.WriteString(" </p>\n</bajaObjectGraph>\n")
	return sb.String()
}
