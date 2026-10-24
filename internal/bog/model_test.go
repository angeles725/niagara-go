package bog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
)

// loadFixture parses a model fixture with the tolerant Parse that model.py parity is about: some
// fixtures (edge.bog: line breaks Python's splitlines() honours inside a tag) are deliberately not
// well-formed XML, which Load rejects.
func loadFixture(t *testing.T, name string) *Bog {
	t.Helper()
	path := filepath.Join("testdata", "model", name)
	text, err := ReadText(path)
	if err != nil {
		t.Fatal(err)
	}
	return Parse(path, text)
}

// ParseTest.test_self_closing_component_is_indexed
func TestSelfClosingComponentIsIndexed(t *testing.T) {
	b := loadFixture(t, "basic.xml")
	c, ok := b.Comps.Get("c")
	if !ok || c.Type != "b:JobService" {
		t.Fatalf("h:c = %+v, %v", c, ok)
	}
	parent, _ := b.Comps.Get("2")
	if parent == nil || !slices.Contains(parent.Children, "c") {
		t.Fatalf("h:c is not a child of h:2: %+v", parent)
	}
}

// ParseTest.test_conversion_link_is_a_link
func TestConversionLinkIsALink(t *testing.T) {
	b := loadFixture(t, "basic.xml")
	if len(b.Links) != 1 {
		t.Fatalf("links = %d, want 1", len(b.Links))
	}
	row := b.LinkRow(b.Links[0])
	if row.Source != "A.out" || row.Target != "B.in10" || !row.SrcResolved || row.Link != "Link20" {
		t.Fatalf("row = %+v", row)
	}
}

// ParseTest.test_escaped_name_decoded_and_resolvable_both_ways
func TestEscapedNameDecodedAndResolvableBothWays(t *testing.T) {
	b := loadFixture(t, "basic.xml")
	c, _ := b.Comps.Get("3")
	if c == nil || c.Name != "object tags" {
		t.Fatalf("h:3 = %+v", c)
	}
	for _, ref := range []string{"Services/object tags", "Services/object$20tags"} {
		if got := b.Resolve(ref); got != c {
			t.Fatalf("Resolve(%q) = %+v", ref, got)
		}
	}
}

// ParseTest.test_unescape_forms
func TestUnescapeForms(t *testing.T) {
	for in, want := range map[string]string{
		"a$20b$3ac": "a b:c", "x$u00e9y": "xéy", "cost$": "cost$", "$u00E9$4": "é$4", "$$41": "$A",
		// Java's SlotPath.unescape builds a UTF-16 string, so a surrogate pair is one code point;
		// a lone surrogate stays invalid (U+FFFD in Go's UTF-8).
		"$ud83d$ude00!": "\U0001F600!", "$ud83dx": "\uFFFDx", "$ude00$ud83d": "\uFFFD\uFFFD",
	} {
		if got := UnescapeName(in); got != want {
			t.Errorf("UnescapeName(%q) = %q, want %q", in, got, want)
		}
	}
}

// EscapedLinkSlotTest.test_link_row_shows_the_decoded_slot
func TestLinkRowShowsTheDecodedSlot(t *testing.T) {
	b := loadFixture(t, "basic-escaped.xml")
	if got := b.LinkRow(b.Links[0]).Target; got != "B.in 10" {
		t.Fatalf("target = %q", got)
	}
}

func TestParseTextEqualsLoad(t *testing.T) {
	path := filepath.Join("testdata", "model", "station.xml")
	text, err := ReadText(path)
	if err != nil {
		t.Fatal(err)
	}
	if a, b := dumpBog(Parse(path, text), -1), dumpBog(loadFixture(t, "station.xml"), -1); !reflect.DeepEqual(a, b) {
		t.Fatal("Parse(text) and Load(path) disagree")
	}
}

// Every fixture graph equals the one tools/bognav/model.py builds (model/<case>.json).
func TestModelMatchesPython(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "model", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no golden files: %v", err)
	}
	for _, golden := range files {
		base := strings.TrimSuffix(filepath.Base(golden), ".json")
		t.Run(base, func(t *testing.T) {
			input := base + ".xml"
			if _, err := os.Stat(filepath.Join("testdata", "model", input)); err != nil {
				input = base + ".bog"
			}
			data, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			var want map[string]any
			if err := json.Unmarshal(data, &want); err != nil {
				t.Fatal(err)
			}
			got := normalize(t, dumpBog(loadFixture(t, input), -1))
			for _, key := range []string{"comps", "handle_count", "prefix_map", "roots", "links", "link_rows", "resolve"} {
				compareJSON(t, key, got[key], want[key])
			}
		})
	}
}

func compareJSON(t *testing.T, key string, got, want any) {
	t.Helper()
	gl, gok := got.([]any)
	wl, wok := want.([]any)
	if gok && wok && len(gl) == len(wl) {
		for i := range gl {
			if !reflect.DeepEqual(gl[i], wl[i]) {
				g, _ := json.Marshal(gl[i])
				w, _ := json.Marshal(wl[i])
				t.Errorf("%s[%d]\n got %s\nwant %s", key, i, g, w)
			}
		}
		return
	}
	if !reflect.DeepEqual(got, want) {
		g, _ := json.Marshal(got)
		w, _ := json.Marshal(want)
		t.Errorf("%s\n got %s\nwant %s", key, g, w)
	}
}

func normalize(t *testing.T, v any) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func pairs[V any](o *Ordered[V]) [][2]any {
	out := [][2]any{}
	for k, v := range o.All() {
		out = append(out, [2]any{k, v})
	}
	return out
}

// dumpBog renders a graph in the shape the retired gen_fixtures.py dump() wrote; refComps < 0 takes Resolve
// refs from every component.
func dumpBog(b *Bog, refComps int) map[string]any {
	comps := []any{}
	refs := []string{"", "/", "nope", "h:nope", "h:", "h:1"}
	for key, c := range b.Comps.All() {
		slots := [][2]any{}
		for name, s := range c.Slots.All() {
			entry := map[string]any{"type": s.Type, "value": s.Value, "flags": s.Flags}
			if s.Child.Len() > 0 {
				entry["child"] = pairs(&s.Child)
			}
			slots = append(slots, [2]any{name, entry})
		}
		present := make([]string, 0, len(c.Present))
		for n := range c.Present {
			present = append(present, n)
		}
		sort.Strings(present)
		children := c.Children
		if children == nil {
			children = []string{}
		}
		var valueType any
		if c.ValueType != "" {
			valueType = c.ValueType
		}
		comps = append(comps, map[string]any{
			"key": key, "name": c.Name, "handle": c.Handle, "type": c.Type, "pfx": c.Pfx,
			"module": c.Module, "path": c.Path, "parent_h": c.ParentH, "children": children,
			"slots": slots, "actions": pairs(&c.Actions), "has_fallback": c.HasFallback,
			"is_writable": c.IsWritable, "value_type": valueType, "value_val": c.ValueVal,
			"present": present, "simple_type": c.SimpleType(),
		})
		if refComps < 0 || len(comps) <= refComps {
			refs = append(refs, c.Path, "/"+c.Path+"/", c.Name, "h:"+c.Handle, strings.ReplaceAll(c.Path, " ", "$20"))
		}
	}
	seen := map[string]bool{}
	resolved := []any{}
	for _, r := range refs {
		if seen[r] {
			continue
		}
		seen[r] = true
		if hit := b.Resolve(r); hit != nil {
			resolved = append(resolved, []any{r, hit.Handle, hit.Path})
		} else {
			resolved = append(resolved, []any{r, nil, nil})
		}
	}
	links := []any{}
	rows := []any{}
	for _, l := range b.Links {
		links = append(links, map[string]any{
			"container_h": l.ContainerH, "container_path": l.ContainerPath, "link_name": l.LinkName,
			"src_h": l.SrcH, "src_ord": l.SrcOrd, "src_slot": l.SrcSlot, "tgt_slot": l.TgtSlot,
		})
		row := b.LinkRow(l)
		rows = append(rows, map[string]any{
			"link": row.Link, "source": row.Source, "target": row.Target, "src_resolved": row.SrcResolved,
		})
	}
	roots := b.Roots
	if roots == nil {
		roots = []string{}
	}
	return map[string]any{
		"comps": comps, "handle_count": pairs(&b.HandleCount), "prefix_map": pairs(&b.Prefixes),
		"roots": roots, "links": links, "link_rows": rows, "resolve": resolved,
	}
}
