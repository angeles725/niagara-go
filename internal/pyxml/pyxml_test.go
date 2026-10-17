package pyxml

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type xmlCase struct {
	In    *string `json:"in"`
	Hex   string  `json:"hex"`
	Expat *string `json:"expat"`
	Etree string  `json:"etree"`
	Tree  any     `json:"tree"`
}

// treeJSON renders el like the generator's tree(): [tag, [[name, value]...], [children]].
func treeJSON(el *Element) any {
	attrs := []any{}
	for _, a := range el.Attrs {
		attrs = append(attrs, []any{a.Name, a.Value})
	}
	kids := []any{}
	for _, c := range el.Children {
		kids = append(kids, treeJSON(c))
	}
	return []any{el.Tag, attrs, kids}
}

func xmlVerdict(data []byte, mode Mode) string {
	if _, err := Parse(data, mode); err != nil {
		return err.Error()
	}
	return ""
}

// TestPyXMLMatchesExpat replays testdata/xmlerrors.json (gen_xmlerrors.py, from expat).
func TestPyXMLMatchesExpat(t *testing.T) {
	raw, err := os.ReadFile("testdata/xmlerrors.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []xmlCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	bad := 0
	for _, c := range cases {
		var data []byte
		if c.In != nil {
			data = []byte(*c.In)
		} else if data, err = hex.DecodeString(c.Hex); err != nil {
			t.Fatal(err)
		}
		if c.Expat != nil {
			if got := xmlVerdict(data, Mode{}); got != *c.Expat {
				bad++
				if bad <= 60 {
					t.Errorf("expat %q:\n got  %q\n want %q", data, got, *c.Expat)
				}
			}
		}
		got := xmlVerdict(data, Mode{ETree: true})
		if c.Etree == "!LookupError" && strings.HasPrefix(got, errEncoding+":") {
			continue
		}
		if c.Tree != nil {
			root, err := Parse(data, Mode{ETree: true})
			if err != nil || root == nil {
				bad++
				t.Errorf("tree %q: rejected (%v), want %s", data, err, mustJSON(t, c.Tree))
			} else if g, w := mustJSON(t, treeJSON(root)), mustJSON(t, c.Tree); g != w {
				bad++
				t.Errorf("tree %q:\n got  %s\n want %s", data, g, w)
			}
		}
		if got != c.Etree {
			bad++
			if bad <= 60 {
				t.Errorf("etree %q:\n got  %q\n want %q", data, got, c.Etree)
			}
		}
	}
	if bad > 0 {
		t.Errorf("%d of %d verdicts differ", bad, 2*len(cases))
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestElementGet covers Element.get: the first attribute of that exact name (a namespaced
// one is "{uri}local" in ETree mode), and false for a missing one.
func TestElementGet(t *testing.T) {
	root, err := Parse([]byte(`<r xmlns:a="urn:a" n="1" a:n="2" e=""/>`), Mode{ETree: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, want string
		ok         bool
	}{
		{"n", "1", true}, {"{urn:a}n", "2", true}, {"e", "", true}, {"a:n", "", false}, {"xmlns:a", "", false}, {"x", "", false},
	} {
		if got, ok := root.Get(c.name); got != c.want || ok != c.ok {
			t.Errorf("Get(%q) = %q, %v; want %q, %v", c.name, got, ok, c.want, c.ok)
		}
	}
	plain, err := Parse([]byte(`<r a:n="2"/>`), Mode{})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := plain.Get("a:n"); got != "2" || !ok {
		t.Errorf("pyexpat mode Get(a:n) = %q, %v; want the raw name", got, ok)
	}
}
