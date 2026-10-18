package check

import (
	"strings"
	"testing"
)

const fixtureSolutions = "../testdata/solutions"

// TestSolutionsRoot: annotation rows with Root solutions read the corpus solutions tree (not the
// lab sources), and the extends and defaults facts render the class declaration and the declared
// property defaults.
func TestSolutionsRoot(t *testing.T) {
	st := &Station{solutions: fixtureSolutions, labsrc: fixtureLabsrc}
	for _, c := range []struct {
		id, evidence string
	}{
		{"CRA1.11", "review/Cristian_Angeles_Review/Cristian_Angeles_Review-rt/src/com/review/Cristian_Angeles_Review/BComponentWithId.java: BComponentWithId extends BComponent"},
		{"CRA1.12", "review/Cristian_Angeles_Review/Cristian_Angeles_Review-rt/src/com/review/Cristian_Angeles_Review/BComponentWithId.java: BComponentWithId defaults id: String = \"\""},
		{"CRA1.13", "review/Cristian_Angeles_Review/Cristian_Angeles_Review-rt/src/com/review/Cristian_Angeles_Review/BComponentWithId.java: BComponentWithId flags id=rsd"},
		{"CRA1.14", "review/Cristian_Angeles_Review/Cristian_Angeles_Review-rt/src/com/review/Cristian_Angeles_Review/BComponentWithId.java: BComponentWithId slots action regenerateId()"},
		{"CRA1.15", "review/Cristian_Angeles_Review/Cristian_Angeles_Review-rt/src/com/review/Cristian_Angeles_Review/BComponentWithId.java: BComponentWithId flags regenerateId=c"},
		{"CRA2.13", "review/Cristian_Angeles_Review/Cristian_Angeles_Review-rt/src/com/review/Cristian_Angeles_Review/BPointSummaryService.java: BPointSummaryService extends BAbstractService"},
		{"CRA2.14", "review/Cristian_Angeles_Review/Cristian_Angeles_Review-rt/src/com/review/Cristian_Angeles_Review/BPointSummaryService.java: BPointSummaryService defaults summaryFile: BOrd = BOrd.DEFAULT"},
		{"CRA2.15", "review/Cristian_Angeles_Review/Cristian_Angeles_Review-rt/src/com/review/Cristian_Angeles_Review/BPointSummaryService.java: BPointSummaryService slots action createSummaryFile()"},
	} {
		ck := specRow(c.id)
		if ck == nil {
			t.Errorf("%s: no spec row", c.id)
			continue
		}
		if r := st.Run(ck); r.Verdict != Pass || r.Evidence != c.evidence {
			t.Errorf("%s: %s %q, want PASS %q", c.id, r.Verdict, r.Evidence, c.evidence)
		}
	}
	// Without the tree the rows SKIP naming --solutions; the lab sources are not a fallback.
	st.solutions = ""
	if r := st.Run(specRow("CRA2.13")); r.Verdict != Skip || !strings.HasPrefix(r.Evidence, "no --solutions DIR: review/") {
		t.Errorf("no solutions: %s %q", r.Verdict, r.Evidence)
	}
	if !NeedsSolutions([]*Check{specRow("CRA2.13")}) || NeedsLabSources([]*Check{specRow("CRA2.13")}) {
		t.Error("a solutions row must name the solutions tree, not the lab sources")
	}
}

// The extends and defaults facts over variants: another superclass, a generic one, no extends,
// another default, and a property without a default.
func TestExtendsAndDefaults(t *testing.T) {
	dir := t.TempDir()
	writeLabFile(t, dir, "A.java", "@NiagaraType\n@NiagaraProperty(name = \"a\", type = \"int\", defaultValue = \"3\")\n"+
		"@NiagaraProperty(name = \"b\", type = \"String\")\npublic final class BA\n  extends BStruct implements BIService {}\n")
	writeLabFile(t, dir, "G.java", "@NiagaraType\npublic class BG extends BList<BObject> {}\n")
	writeLabFile(t, dir, "N.java", "@NiagaraType\npublic class BN {}\n")
	st := &Station{solutions: dir}
	for _, c := range []struct {
		ck       Check
		verdict  string
		evidence string
	}{
		{Check{Glob: "A.java", Fact: factExtends, Want: "BComponent"}, Fail, "A.java: BA extends BStruct, want BComponent"},
		{Check{Glob: "G.java", Fact: factExtends, Want: "BList<BObject>"}, Pass, "G.java: BG extends BList<BObject>"},
		{Check{Glob: "N.java", Fact: factExtends, Want: "BObject"}, Skip, "N.java: BN extends not decided: no extends clause"},
		{Check{Glob: "A.java", Fact: factDefaults, Want: "a: int = 3, b: String = \"\""}, Pass, "A.java: BA defaults a: int = 3, b: String = \"\""},
		{Check{Glob: "A.java", Fact: factDefaults, Slot: "a", Want: "a: int = 0"}, Fail, "A.java: BA defaults a: int = 3, want a: int = 0"},
	} {
		c.ck.Kind, c.ck.Root = kindAnnotation, rootSolutions
		if r := st.Run(&c.ck); r.Verdict != c.verdict || r.Evidence != c.evidence {
			t.Errorf("%s %s: %s %q, want %s %q", c.ck.Glob, c.ck.Fact, r.Verdict, r.Evidence, c.verdict, c.evidence)
		}
	}
}

// The extends fact ignores braces and words inside annotations, comments and strings.
func TestExtendsIgnoresBraces(t *testing.T) {
	dir := t.TempDir()
	writeLabFile(t, dir, "B.java", "@NiagaraType\n@NiagaraProperty(name = \"a\", type = \"String\", defaultValue = \"{ extends X }\")\n"+
		"@Facets({ \"x\" }) public class BB /* { extends BWrong } */ // {\n  extends BRight {\n}\n")
	st := &Station{solutions: dir}
	ck := Check{Kind: kindAnnotation, Root: rootSolutions, Glob: "B.java", Fact: factExtends, Want: "BRight"}
	if r := st.Run(&ck); r.Verdict != Pass {
		t.Errorf("%s %q", r.Verdict, r.Evidence)
	}
}
