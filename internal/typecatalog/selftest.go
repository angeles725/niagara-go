package typecatalog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/slotomatic"
)

// The sources of niagara-research tools/tests/test_n4_type_catalog.py, which the Python
// `selftest` runs through unittest.
const originalSrc = `
package javax.baja.control;
public class BNumericWritable
  extends BNumericPoint
  implements BIWritablePoint
{
  public static final Property in1 = newProperty(Flags.READONLY, new BStatusNumeric(0, BStatus.nullStatus), null);
  public static final Property in10 = newProperty(Flags.SUMMARY | Flags.TRANSIENT, new BStatusNumeric(0, BStatus.nullStatus), null);
  public static final Property fallback = newProperty(0, new BStatusNumeric(0, BStatus.nullStatus), null);
  public static final Property overrideExpiration = newProperty(Flags.OPERATOR | Flags.READONLY, BAbsTime.NULL, null);
  public static final Action emergencyAuto = newAction(0, null);
  public static final Action set = newAction(Flags.OPERATOR, BDouble.DEFAULT, null);
  public static final Topic fired = newTopic(Flags.SUMMARY, null);
}
`

const cfrSrc = `
package com.tridium.kitControl;
public class BInterstartDelayControl
extends BComponent {
    public static final Property delay = BInterstartDelayControl.newProperty((int)0, (BValue)BRelTime.DEFAULT, null);
    public static final Property master = BInterstartDelayControl.newProperty((int)1026, (BValue)BOrd.NULL, BFacets.make((String)"a", (String)"b"));
    public static final Action reset = BInterstartDelayControl.newAction((int)256, null);
}
`

const multilineSrc = `
package a.b;
public class BMulti extends BComponent {
  public static final Property conversion = newProperty(0, BDefaultProxyConversion.DEFAULT, BFacets.make(BFacets.make(BFacets.FIELD_EDITOR, "driver:ProxyConversionFE"), BFacets.make(BFacets.UX_FIELD_EDITOR, "driver:ProxyConversionEditor")));
  public static final Property wrapped =
    newProperty(Flags.HIDDEN, "x",
      null);
}
`

// errSkip marks a check whose data is absent.
type errSkip string

func (e errSkip) Error() string { return string(e) }

type check struct {
	name string
	run  func() error
}

func expect(ok bool, format string, a ...any) error {
	if ok {
		return nil
	}
	return fmt.Errorf(format, a...)
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

func flagsOf(ds []slotomatic.Decl) map[string]slotomatic.Decl {
	m := map[string]slotomatic.Decl{}
	for _, d := range ds {
		m[d.Name] = d
	}
	return m
}

func checks() []check {
	return []check{
		{"FlagsTest.test_numeric_flags_decode", func() error {
			v, l := slotomatic.DecodeFlags("(int)1026")
			return expect(v == 1026 && strings.Contains(strings.Join(l, ""), "f") && strings.Contains(strings.Join(l, ""), "t"), "(int)1026 -> %d %v", v, l)
		}},
		{"FlagsTest.test_symbolic_flags_to_int_and_letters", func() error {
			v, l := slotomatic.DecodeFlags("Flags.SUMMARY | Flags.TRANSIENT")
			return expect(v == 0x0A && strings.Join(l, "") == "ts", "-> %d %v", v, l)
		}},
		{"FlagsTest.test_zero_flags", func() error {
			v1, l1 := slotomatic.DecodeFlags("0")
			v2, l2 := slotomatic.DecodeFlags("(int)0")
			return expect(v1 == 0 && v2 == 0 && len(l1)+len(l2) == 0, "zero flags decoded as %d %d", v1, v2)
		}},
		{"ParseTest.test_cfr_decompiled_style", func() error {
			t := slotomatic.ParseSource(cfrSrc)
			p, a := flagsOf(t.Properties), flagsOf(t.Actions)
			return expect(p["delay"].Flags == 0 && p["delay"].Default == "BRelTime.DEFAULT" && p["master"].Flags == 1026 &&
				strings.HasPrefix(p["master"].Facets, "BFacets.make(") && a["reset"].Flags == 256, "%+v", t)
		}},
		{"ParseTest.test_nested_parentheses_and_multiline", func() error {
			p := flagsOf(slotomatic.ParseSource(multilineSrc).Properties)
			return expect(strings.Contains(p["conversion"].Facets, "FIELD_EDITOR") && p["wrapped"].Flags == 4 &&
				p["wrapped"].Default == `"x"`, "%+v", p)
		}},
		{"ParseTest.test_no_declarations_returns_empty_lists", func() error {
			t := slotomatic.ParseSource("package p;\npublic class Plain { int x; }\n")
			return expect(len(t.Properties)+len(t.Actions) == 0, "%+v", t)
		}},
		{"ParseTest.test_original_source_properties_actions_topics", func() error {
			t := slotomatic.ParseSource(originalSrc)
			p, a := flagsOf(t.Properties), flagsOf(t.Actions)
			return firstErr(
				expect(t.Class == "BNumericWritable" && t.Package == "javax.baja.control" && t.Extends == "BNumericPoint", "header %+v", t),
				expect(p["in1"].Flags == 1 && p["in10"].Flags == 0x0A && p["fallback"].Flags == 0 && p["overrideExpiration"].Flags == 0x101, "flags %+v", p),
				expect(p["in1"].Default == "new BStatusNumeric(0, BStatus.nullStatus)", "in1 default %q", p["in1"].Default),
				expect(a["emergencyAuto"].Flags == 0 && a["set"].Flags == 0x100 && a["set"].Default == "BDouble.DEFAULT", "actions %+v", a),
				expect(len(t.Topics) == 1 && t.Topics[0].Name == "fired", "topics %+v", t.Topics))
		}},
		{"RealDataTest.test_bnumericwritable_in_docsource", realData},
		{"WalkTest.test_walk_tree_and_json", func() error {
			d, err := os.MkdirTemp("", "n4-type-catalog-selftest-")
			if err != nil {
				return err
			}
			defer os.RemoveAll(d)
			sub := filepath.Join(d, "a", "b")
			if err := os.MkdirAll(sub, 0o755); err != nil {
				return err
			}
			if err := firstErr(os.WriteFile(filepath.Join(sub, "BNumericWritable.java"), []byte(originalSrc), 0o644),
				os.WriteFile(filepath.Join(sub, "Plain.java"), []byte("package p;\npublic class Plain {}\n"), 0o644)); err != nil {
				return err
			}
			cat, _ := Build([]string{d})
			r := cat.Get("javax.baja.control.BNumericWritable")
			return expect(r != nil && cat.Get("p.Plain") == nil && r.Properties[0].Name == "in1", "catalog keys %v", cat.Keys)
		}},
	}
}

// realData checks the B1174 InN flag facts against the original Tridium source under organized/.
func realData() error {
	root := slotomatic.OrganizedRoot()
	if root == "" {
		return errSkip("no organized/ tree (set NIAGARA_ORGANIZED)")
	}
	path := filepath.Join(root, "docSource", "docSource-doc", "extracted", "control-rt", "javax", "baja", "control", "BNumericWritable.java")
	text, err := slotomatic.ReadSource(path)
	if err != nil {
		return errSkip("docSource tree not present under " + root)
	}
	t := slotomatic.ParseSource(text)
	p, a := flagsOf(t.Properties), flagsOf(t.Actions)
	return expect(p["in1"].Flags == 0x01 && p["in8"].Flags == 0x01 && p["in2"].Flags == 0x02 && p["in10"].Flags == 0x0A &&
		p["in16"].Flags == 0x0A && p["fallback"].Flags == 0 && a["emergencyOverride"].Flags == 0 && a["set"].Flags == 0x100,
		"%s: InN flags differ from B1174", path)
}

// selftest runs the checks and reports them like unittest's TextTestRunner(verbosity=1) on
// stderr: a skipped check is listed, never counted as passed. Exit 1 on any failure.
func selftest(w io.Writer) int {
	start := time.Now()
	var fails, skips []string
	cs := checks()
	for _, c := range cs {
		err := c.run()
		switch e := err.(type) {
		case nil:
			io.WriteString(w, ".")
		case errSkip:
			io.WriteString(w, "s")
			skips = append(skips, fmt.Sprintf("%s skipped: %s", c.name, string(e)))
		default:
			io.WriteString(w, "F")
			fails = append(fails, fmt.Sprintf("FAIL: %s\n%v", c.name, err))
		}
	}
	io.WriteString(w, "\n")
	for _, f := range fails {
		fmt.Fprintf(w, "%s\n%s\n", strings.Repeat("=", 70), f)
	}
	for _, s := range skips {
		fmt.Fprintln(w, s)
	}
	fmt.Fprintf(w, "%s\nRan %d tests in %.3fs\n\n", strings.Repeat("-", 70), len(cs), time.Since(start).Seconds())
	switch {
	case len(fails) > 0:
		fmt.Fprintf(w, "FAILED (failures=%d)\n", len(fails))
		return cli.ExitNoMatch
	case len(skips) > 0:
		fmt.Fprintf(w, "OK (skipped=%d)\n", len(skips))
	default:
		fmt.Fprintln(w, "OK")
	}
	return cli.ExitFound
}
