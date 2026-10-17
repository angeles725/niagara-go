package bogwrite

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// selfCheck is one built-in check; skip is its unittest skip reason.
type selfCheck struct {
	name string
	run  func() error
	skip string
}

// dict builds a plan dict from key, value pairs; a value is a pyjson.Value or a Go string.
func dict(kv ...any) pyjson.Value {
	d := pyjson.NewObject()
	for i := 0; i < len(kv); i += 2 {
		v, ok := kv[i+1].(pyjson.Value)
		if !ok {
			v = str(kv[i+1].(string))
		}
		d.Set(kv[i].(string), v)
	}
	return d
}

func list(items ...pyjson.Value) pyjson.Value { return pyjson.Value{Kind: pyjson.Array, A: items} }

func ints(v ...int64) pyjson.Value {
	out := list()
	for _, x := range v {
		out.A = append(out.A, pyjson.IntValue(x))
	}
	return out
}

// selfPlan is the PLAN of tools/tests/test_bog_write.py.
func selfPlan() pyjson.Value {
	return dict(
		"root", dict("name", "Demo", "type", "baja:Folder"),
		"components", list(
			dict("name", "SelR", "type", "control:NumericWritable", "ws", ints(4, 10, 8),
				"slots", dict("facets", dict("type", "baja:Facets", "value", "min=0.0;max=4.0;precision=0"))),
			dict("name", "Mux", "type", "kitControl:BooleanSelect", "ws", ints(26, 8, 8)),
			dict("name", "Mux?", "type", "kitControl:BooleanSelect"),
		),
		"links", list(dict("from", "SelR.out", "to", "Mux.select"), dict("from", "SelR.out", "to", "Mux1.select")))
}

func noErrors(fs []finding) error {
	for _, f := range fs {
		if f[0] == "ERROR" {
			return fmt.Errorf("unexpected finding %s", f.repr())
		}
	}
	return nil
}

func findingWith(fs []finding, words ...string) error {
	for _, f := range fs {
		for _, w := range words {
			if strings.Contains(strings.ToLower(f[1]), strings.ToLower(w)) {
				return nil
			}
		}
	}
	return fmt.Errorf("no finding mentions %q in %v", words, fs)
}

func base(body string) []byte {
	return []byte("<?xml version='1.0'?><bajaObjectGraph version='4.0'>" +
		"<p n='R' h='1' m='b=baja' t='b:Folder'>" + body + "</p></bajaObjectGraph>")
}

const selfLink = "<p n='Link' t='b:Link'><p n='sourceOrd' v='h:%s'/><p n='sourceSlotName' v='out'/><p n='targetSlotName' v='in'/></p>"

func selfChecks() []selfCheck {
	gen := func() (string, error) { return generateXML(selfPlan()) }
	validates := func(body string, words ...string) func() error {
		return func() error { return findingWith(validateXML(base(body)), words...) }
	}
	return []selfCheck{
		{name: "unique name follows ObjectUtil", run: func() error {
			for _, c := range [][3]string{{"Foo", "Foo", "Foo1"}, {"Foo9", "Foo9", "Foo10"}, {"Foo", "", "Foo"}, {"Link", "Link Link1", "Link2"}} {
				taken := map[string]bool{}
				for _, n := range strings.Fields(c[1]) {
					taken[n] = true
				}
				if got, _ := uniqueName(c[0], taken); got != c[2] {
					return fmt.Errorf("uniqueName(%q) = %q, want %q", c[0], got, c[2])
				}
			}
			return nil
		}},
		{name: "generated document validates", run: func() error {
			xml, err := gen()
			if err != nil {
				return err
			}
			if !strings.Contains(xml, "Link") {
				return fmt.Errorf("no link emitted")
			}
			return noErrors(validateXML([]byte(xml)))
		}},
		{name: "'?' name made unique and its link resolved", run: func() error {
			xml, err := gen()
			if err != nil {
				return err
			}
			if !strings.Contains(xml, `n="Mux1"`) {
				return fmt.Errorf("Mux1 missing")
			}
			return noErrors(validateXML([]byte(xml)))
		}},
		{name: "links are persistent children of the target", run: func() error {
			xml, err := gen()
			if err != nil {
				return err
			}
			if strings.Count(xml, `t="b:Link"`) != 2 || strings.Count(xml, `<p n="Link" t="b:Link">`) != 2 ||
				strings.Contains(xml, " f=") || strings.Count(xml, `<p n="sourceOrd" v="h:`) != 2 {
				return fmt.Errorf("link shape: %s", xml)
			}
			return nil
		}},
		{name: "ZIP container round trip", run: zipRoundTrip},
		{name: "duplicate explicit name rejected", run: func() error {
			plan := dict("root", dict("name", "R", "type", "baja:Folder"), "components",
				list(dict("name", "A", "type", "baja:Folder"), dict("name", "A", "type", "baja:Folder")))
			if _, err := generateXML(plan); err == nil || !strings.HasPrefix(err.Error(), "ValueError: ") {
				return fmt.Errorf("duplicate accepted: %v", err)
			}
			return nil
		}},
		{name: "duplicate handle", run: validates("<p n='A' h='2' t='b:Folder'/><p n='B' h='2' t='b:Folder'/>", "duplicate handle")},
		{name: "dangling sourceOrd", run: validates("<p n='A' h='2' t='b:Folder'>"+fmt.Sprintf(selfLink, "99")+"</p>", "unresolved")},
		{name: "transient link flag", run: validates("<p n='A' h='2' t='b:Folder'>"+
			strings.Replace(fmt.Sprintf(selfLink, "2"), "t='b:Link'", "f='t' t='b:Link'", 1)+"</p>", "transient")},
		{name: "password value", run: validates("<p n='A' h='2' t='b:Folder'><p n='pw' t='b:Password' v='x'/></p>", "password")},
		{name: "link missing children", run: validates("<p n='A' h='2' t='b:Folder'><p n='Link' t='b:Link'><p n='sourceOrd' v='h:2'/></p></p>",
			"sourceSlotName", "targetSlotName")},
		{name: "bad wsAnnotation", run: validates("<p n='A' h='2' t='b:Folder'><p n='wsAnnotation' t='b:WsAnnotation' v='1,x,8'/></p>", "wsAnnotation")},
		{name: "undeclared prefix", run: validates("<p n='A' h='2' t='zz:Thing'/>", "prefix")},
		{name: "clean document", run: func() error { return noErrors(validateXML(base("<p n='A' h='2' t='b:Folder'/>"))) }},
		{name: "real template", skip: "template not present"},
	}
}

func zipRoundTrip() error {
	dir, err := os.MkdirTemp("", "bog-write-selftest")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "x.bog")
	if err := writeBog(selfPlan(), out, false); err != nil {
		return err
	}
	z, err := zip.OpenReader(out)
	if err != nil {
		return err
	}
	defer z.Close()
	if len(z.File) != 1 || z.File[0].Name != "file.xml" {
		return fmt.Errorf("entries %v", z.File)
	}
	r, err := z.File[0].Open()
	if err != nil {
		return err
	}
	xml, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		return err
	}
	if err := noErrors(validateXML(xml)); err != nil {
		return err
	}
	return noErrors(validatePath(out))
}

// selftest runs the checks and reports like unittest.TextTestRunner(verbosity=1).
func selftest(w io.Writer) int {
	start := time.Now()
	var failures []string
	skipped := 0
	checks := selfChecks()
	for _, c := range checks {
		switch {
		case c.skip != "":
			skipped++
			fmt.Fprint(w, "s")
		case c.run() != nil:
			failures = append(failures, fmt.Sprintf("FAIL: %s\n%v", c.name, c.run()))
			fmt.Fprint(w, "F")
		default:
			fmt.Fprint(w, ".")
		}
	}
	fmt.Fprintln(w)
	for _, f := range failures {
		fmt.Fprintf(w, "%s\n%s\n%s\n\n", strings.Repeat("=", 70), f, strings.Repeat("-", 70))
	}
	fmt.Fprintf(w, "%s\nRan %d tests in %.3fs\n\n", strings.Repeat("-", 70), len(checks), time.Since(start).Seconds())
	switch {
	case len(failures) > 0:
		fmt.Fprintf(w, "FAILED (failures=%d)\n", len(failures))
		return 1
	case skipped > 0:
		fmt.Fprintf(w, "OK (skipped=%d)\n", skipped)
	default:
		fmt.Fprintln(w, "OK")
	}
	return 0
}
