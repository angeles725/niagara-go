package bog

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyxml"
)

// The verdict CheckWellFormed must share with bog-cycles: pyexpat ParserCreate().Parse(str),
// which internal/pyxml reproduces as Mode{} (issue #170).
func pyxmlVerdict(text string) string {
	if _, err := pyxml.Parse([]byte(text), pyxml.Mode{}); err != nil {
		return err.Error()
	}
	return ""
}

func wfVerdict(text string) string {
	if err := CheckWellFormed(text); err != nil {
		return err.Error()
	}
	return ""
}

// wfProbes are the well-formedness classes of #170 (T37's probes and their neighbours), each a
// small document; pyxml rejects every one but the last three.
var wfProbes = []struct{ class, text string }{
	{"undefined entity in content", "<r>&foo;</r>"},
	{"undefined entity in attribute", "<r a='&foo;'/>"},
	{"duplicate attribute", "<r a='1' b='2' a='3'/>"},
	{"bare & in content", "<r>a & b</r>"},
	{"bare & in attribute", "<r a='a & b'/>"},
	{"control char in content", "<r>a\x01b</r>"},
	{"control char in attribute", "<r a='a\x1fb'/>"},
	{"&#0; in content", "<r>&#0;</r>"},
	{"char ref beyond U+10FFFF in attribute", "<r a='&#x110000;'/>"},
	{"-- inside a comment", "<r><!-- a -- b --></r>"},
	{"]]> in content", "<r>a]]>b</r>"},
	{"undecodable byte", "<r>a\xffb</r>"},
	{"noncharacter U+FFFE", "<r a='￾'/>"},
	{"malformed char ref", "<r>&#xZZ;</r>"},
	{"misplaced XML declaration", "<r/>\n<?xml version='1.0'?>"},
	{"text before the root", "x<r/>"},
	{"declared internal entity", "<!DOCTYPE r [<!ENTITY e 'x'>]><r a='&e;'>&e;</r>"},
	{"external DTD skips undefined entities", "<!DOCTYPE r SYSTEM 'r.dtd'><r>&e;</r>"},
	{"legal references", "<?xml version='1.0' encoding='UTF-8'?><r a='&#x9;&#65;&lt;&apos;'>&#x10FFFF;&amp;&gt;&quot;</r>"},
}

func TestCheckWellFormedProbesMatchPyxml(t *testing.T) {
	for _, p := range wfProbes {
		t.Run(p.class, func(t *testing.T) {
			if got, want := wfVerdict(p.text), pyxmlVerdict(p.text); got != want {
				t.Fatalf("CheckWellFormed %q, pyxml %q", got, want)
			}
		})
	}
}

// xmlerrors.json is expat's own verdict on 3,000+ documents (internal/pyxml's parity suite).
func TestCheckWellFormedMatchesExpatSuite(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "pyxml", "testdata", "xmlerrors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		In    *string `json:"in"`
		Hex   string  `json:"hex"`
		Expat *string `json:"expat"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	bad := 0
	for _, c := range cases {
		if c.Expat == nil {
			continue
		}
		text := ""
		if c.In != nil {
			text = *c.In
		} else {
			b, err := hex.DecodeString(c.Hex)
			if err != nil {
				t.Fatal(err)
			}
			text = string(b)
		}
		if got := wfVerdict(text); got != *c.Expat {
			if bad++; bad <= 20 {
				t.Errorf("%q: CheckWellFormed %q, expat %q", text, got, *c.Expat)
			}
		}
	}
	if bad > 0 {
		t.Fatalf("%d of %d expat verdicts differ", bad, len(cases))
	}
}

// BOG_PARITY_FILES (colon-separated real bogs, outside the repository) must pass both checks, the
// fast pass alone deciding.
func TestCheckWellFormedRealBogs(t *testing.T) {
	list := os.Getenv("BOG_PARITY_FILES")
	if list == "" {
		t.Skip("BOG_PARITY_FILES not set")
	}
	for _, path := range strings.Split(list, ":") {
		text, err := ReadText(path)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := wfVerdict(text), pyxmlVerdict(text); got != "" || want != "" {
			t.Errorf("%s: CheckWellFormed %q, pyxml %q", path, got, want)
		}
		if c := (wfScanner{s: text}); !c.run() { // a real bog must not cost a pyxml parse
			t.Errorf("%s: the fast pass left a well-formed bog to pyxml", path)
		}
	}
}

func FuzzCheckWellFormedMatchesPyxml(f *testing.F) {
	for _, p := range wfProbes {
		f.Add(p.text)
	}
	f.Add(wellFormedDoc)
	f.Fuzz(func(t *testing.T, text string) {
		if got, want := wfVerdict(text), pyxmlVerdict(text); got != want {
			t.Fatalf("%q: CheckWellFormed %q, pyxml %q", text, got, want)
		}
	})
}

func BenchmarkCheckWellFormed(b *testing.B) {
	text, err := ReadText(benchFile(b))
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(text)))
	for b.Loop() {
		if err := CheckWellFormed(text); err != nil {
			b.Fatal(err)
		}
	}
}
