package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureSource = "../testdata/source"

// TestSourceTextKind runs the source-text kind over the synthetic source tree and over a variant
// whose BAbstractService declares a fourth property (a "Last Ok" slot the 4.15 class does not have).
func TestSourceTextKind(t *testing.T) {
	st, err := Open(writeStation(t), "", false)
	if err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	dir := filepath.Join(other, "baja", "javax", "baja", "sys")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join(fixtureSource, filepath.FromSlash(abstractService)))
	if err != nil {
		t.Fatal(err)
	}
	extra := strings.Replace(string(src), "  public void updateStatus()",
		"  public static final Property lastOk = newProperty(0, BAbsTime.DEFAULT, null);\n\n  public void updateStatus()", 1)
	if err := os.WriteFile(filepath.Join(dir, "BAbstractService.java"), []byte(extra), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(other, "nosuch")
	notDir := filepath.Join(dir, "BAbstractService.java")
	cases := []struct {
		source   string
		ck       Check
		verdict  string
		evidence string
	}{
		{fixtureSource, *specRow("C7.6"), Pass,
			"baja/javax/baja/sys/BAbstractService.java: 1 match (first at line 25: configOk() …)"},
		{fixtureSource, *specRow("C7.7"), Pass,
			"baja/javax/baja/sys/BAbstractService.java: 1 match (first at line 15: if (fatalFault || configFault) …)"},
		{fixtureSource, *specRow("C7.8"), Pass,
			"baja/javax/baja/sys/BAbstractService.java: 3 matches (first at line 7: public static final Property status = newProperty()"},
		{other, *specRow("C7.8"), Fail,
			"baja/javax/baja/sys/BAbstractService.java: 4 matches, want exactly 3 of "},
		{"", *specRow("C7.8"), Skip, "no --source DIR: baja/javax/baja/sys/BAbstractService.java not checked"},
		{other, Check{Kind: kindSourceText, Glob: "baja/javax/baja/sys/BNothing.java", Match: `x`, Op: opMin, N: 1}, Fail,
			"missing baja/javax/baja/sys/BNothing.java under " + other},
		{other, Check{Kind: kindSourceText, Glob: abstractService, Match: `(`, Op: opMin, N: 1}, Skip, "bad pattern ("},
		{other, Check{Kind: kindSourceText, Glob: abstractService, Match: `lastSuccess`, Op: opMin, N: 1}, Fail,
			"baja/javax/baja/sys/BAbstractService.java: 0 matches, want at least 1 of lastSuccess"},
		// A tree that is not there is a configuration error (SKIP), never a missing file (FAIL).
		{missing, *specRow("C7.8"), Skip, "cannot read " + missing + ": "},
		{notDir, *specRow("C7.6"), Skip, "cannot read " + notDir + ": not a directory"},
	}
	for _, c := range cases {
		st.source = c.source
		r := st.Run(&c.ck)
		if r.Verdict != c.verdict || !strings.HasPrefix(r.Evidence, c.evidence) {
			t.Errorf("%s %s on %q:\n got %s %q\nwant %s %q", c.ck.ID, c.ck.Glob, c.source, r.Verdict, r.Evidence, c.verdict, c.evidence)
		}
	}
	defer func(n int) { maxFileText = n }(maxFileText)
	maxFileText = 8
	st.source = fixtureSource
	if r := st.Run(specRow("C7.8")); r.Verdict != Skip || !strings.HasPrefix(r.Evidence, "cannot read baja/javax/baja/sys/BAbstractService.java: larger than 8 bytes") {
		t.Errorf("over the read bound: %s %q", r.Verdict, r.Evidence)
	}
	if !NeedsSource([]*Check{specRow("C7.1"), specRow("C7.6")}) || NeedsSource([]*Check{specRow("C7.1")}) {
		t.Error("NeedsSource: want true only with a source-text check")
	}
}

// TestSourceTextReal runs the source-text rows on the real 4.15 source tree when
// COURSE_NIAGARA_SOURCE names it.
func TestSourceTextReal(t *testing.T) {
	source := os.Getenv("COURSE_NIAGARA_SOURCE")
	if source == "" {
		t.Skip("COURSE_NIAGARA_SOURCE is not set: the real source tree is not checked")
	}
	st := &Station{source: source}
	n := 0
	for i := range Spec {
		if Spec[i].Kind == kindSourceText {
			n++
			if r := st.Run(&Spec[i]); r.Verdict != Pass {
				t.Errorf("%s %s: %s", r.ID, r.Verdict, r.Evidence)
			}
		}
	}
	if n == 0 {
		t.Fatal("no source-text row in the spec")
	}
}
