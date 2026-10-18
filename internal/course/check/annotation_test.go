package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureLabsrc = "../testdata/labsrc"

// writeLabFile writes one file under a lab module tree.
func writeLabFile(t *testing.T, root, rel, text string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestAnnotationKind runs the annotation rows over the synthetic lab modules, and the kind over
// variants: a changed flag, a missing slot, a file the parser refuses, an undecidable flag, an
// enum fact on a class without one, and missing or unreadable trees.
func TestAnnotationKind(t *testing.T) {
	st := &Station{}
	add := "training/training-rt/src/com/tridiumuniversity/training/kitControl/BAdd.java"
	other := t.TempDir()
	writeLabFile(t, other, add, `@NiagaraType
@NiagaraProperty(name = "out", type = "BStatusNumeric", flags = Flags.SUMMARY | Flags.READONLY)
@NiagaraProperty(name = "in1", type = "BStatusNumeric", flags = Flags.SUMMARY)
@NiagaraProperty(name = "in2", type = "BStatusNumeric", flags = Flags.SUMMARY)
@NiagaraProperty(name = "in3", type = "BStatusNumeric", flags = Flags.SUMMARY)
@NiagaraProperty(name = "in4", type = "BStatusNumeric", flags = myFlags)
class BAdd {}
`)
	writeLabFile(t, other, "Broken.java", "@NiagaraProperty(name = \"a\"\nclass B {}")
	missing := filepath.Join(other, "nosuch")
	notDir := filepath.Join(other, "Broken.java")
	in4 := Check{Kind: kindAnnotation, Glob: add, Fact: factFlags, Slot: "in4", Want: "in4=-"}
	cases := []struct {
		labsrc   string
		ck       Check
		verdict  string
		evidence string
	}{
		{fixtureLabsrc, *specRow("C1.8"), Pass,
			"training/training-rt/src/com/tridiumuniversity/training/BFirstComponent.java: BFirstComponent slots property timeExecuted: BAbsTime, action execute()"},
		{fixtureLabsrc, *specRow("C2.5"), Pass,
			"training/training-rt/src/com/tridiumuniversity/training/kitControl/BAdd.java: BAdd flags out=rs in1=s in2=s in3=- in4=-"},
		{fixtureLabsrc, *specRow("C4.9"), Pass,
			"training/training-rt/src/com/tridiumuniversity/training/kitControl/BLoopAction.java: BLoopAction enum DIRECT=0 REVERSE=1 DEFAULT=direct"},
		{fixtureLabsrc, *specRow("C4.10"), Pass,
			"training/training-rt/src/com/tridiumuniversity/training/kitControl/BTstat.java: BTstat flags out=rs cv=- sp=- diff=- action=-"},
		{fixtureLabsrc, *specRow("C6.7"), Pass,
			"training/training-rt/src/com/tridiumuniversity/training/workOrder/BWorkOrderSubmissionState.java: BWorkOrderSubmissionState enum IDLE=0 SUBMITTING=1 DEFAULT=idle"},
		{fixtureLabsrc, *specRow("C6.8"), Pass,
			"training/training-rt/src/com/tridiumuniversity/training/workOrder/BWorkOrderService.java: BWorkOrderService slots action submit(BWorkOrder = new BWorkOrder())"},
		{fixtureLabsrc, *specRow("CB.5"), Pass,
			"trafficLightDriver/trafficLightDriver-rt/src/com/tridiumuniversity/trafficLightDriver/BTrafficLightDriverDevice.java: BTrafficLightDriverDevice facets intersectionId=SfUtil.incl(SfUtil.MGR_EDIT)"},
		// A flags expression the parser cannot evaluate leaves the whole fact undecided (SKIP).
		{other, *specRow("C2.5"), Skip,
			"training/training-rt/src/com/tridiumuniversity/training/kitControl/BAdd.java: BAdd flags not decided: in4: cannot evaluate flags \"myFlags\""},
		{other, Check{Kind: kindAnnotation, Glob: add, Fact: factFlags, Slot: "in5", Want: "in5=-"}, Fail,
			"training/training-rt/src/com/tridiumuniversity/training/kitControl/BAdd.java: BAdd declares no slot in5"},
		{other, in4, Skip, "training/training-rt/src/com/tridiumuniversity/training/kitControl/BAdd.java: BAdd flags not decided: in4: cannot evaluate flags \"myFlags\""},
		{other, Check{Kind: kindAnnotation, Glob: add, Fact: factEnum, Want: "X=0 DEFAULT=x"}, Skip,
			"training/training-rt/src/com/tridiumuniversity/training/kitControl/BAdd.java: BAdd enum not decided: no @NiagaraEnum"},
		{other, Check{Kind: kindAnnotation, Glob: "Broken.java", Fact: factSlots, Want: "x"}, Skip,
			"cannot read the annotations of Broken.java: line 2: unterminated annotation value"},
		{other, Check{Kind: kindAnnotation, Glob: "Nothing.java", Fact: factSlots, Want: "x"}, Fail,
			"missing Nothing.java under " + other},
		{"", *specRow("C2.5"), Skip, "no --labsrc DIR: training/training-rt/src/com/tridiumuniversity/training/kitControl/BAdd.java not checked"},
		// A tree that is not there is a configuration error (SKIP), never a missing file (FAIL).
		{missing, *specRow("C2.5"), Skip, "cannot read " + missing + ": "},
		{notDir, *specRow("C2.5"), Skip, "cannot read " + notDir + ": not a directory"},
	}
	for _, c := range cases {
		st.labsrc = c.labsrc
		r := st.Run(&c.ck)
		if r.Verdict != c.verdict || !strings.HasPrefix(r.Evidence, c.evidence) {
			t.Errorf("%s %s on %q:\n got %s %q\nwant %s %q", c.ck.ID, c.ck.Glob, c.labsrc, r.Verdict, r.Evidence, c.verdict, c.evidence)
		}
	}
	// A changed flag with every flag decidable is FAIL, naming what was declared and the Want.
	writeLabFile(t, other, add, `@NiagaraType
@NiagaraProperty(name = "out", type = "BStatusNumeric", flags = Flags.SUMMARY | Flags.READONLY)
@NiagaraProperty(name = "in1", type = "BStatusNumeric", flags = Flags.SUMMARY)
@NiagaraProperty(name = "in2", type = "BStatusNumeric", flags = Flags.SUMMARY)
@NiagaraProperty(name = "in3", type = "BStatusNumeric", flags = Flags.SUMMARY)
@NiagaraProperty(name = "in4", type = "BStatusNumeric")
class BAdd {}
`)
	st.labsrc = other
	want := "training/training-rt/src/com/tridiumuniversity/training/kitControl/BAdd.java: BAdd flags out=rs in1=s in2=s in3=s in4=-, want out=rs in1=s in2=s in3=- in4=-"
	if r := st.Run(specRow("C2.5")); r.Verdict != Fail || r.Evidence != want {
		t.Errorf("changed in3: %s %q, want FAIL %q", r.Verdict, r.Evidence, want)
	}
	// An action's defaultValue shows even without a parameterType.
	for _, c := range []struct {
		slot string
		want string
	}{
		{`@NiagaraAction(name = "go", defaultValue = "BBoolean.TRUE")`, "action go(= BBoolean.TRUE)"},
		{`@NiagaraAction(name = "go", parameterType = "BWorkOrder")`, "action go(BWorkOrder)"},
		{`@NiagaraAction(name = "go", returnType = "BString")`, "action go(): BString"},
		{`@NiagaraTopic(name = "go", eventType = "BString")`, "topic go(BString)"},
	} {
		writeLabFile(t, other, "A.java", c.slot+"\nclass A {}")
		ck := Check{Kind: kindAnnotation, Glob: "A.java", Fact: factSlots, Want: c.want}
		if r := st.Run(&ck); r.Verdict != Pass {
			t.Errorf("%s: %s %q, want %q", c.slot, r.Verdict, r.Evidence, c.want)
		}
	}
	if !NeedsLabSources([]*Check{specRow("C2.1"), specRow("C2.5")}) || NeedsLabSources([]*Check{specRow("C2.1")}) {
		t.Error("NeedsLabSources: want true only with an annotation check")
	}
}

// TestAnnotationReal runs the annotation rows on the participant's real trees: the lab-source rows
// when COURSE_LAB_SOURCES names the Niagara dev home (training/, trafficLightDriver/), the solutions
// rows when COURSE_SOLUTIONS names the corpus solutions; each variable gates its own rows.
func TestAnnotationReal(t *testing.T) {
	for _, tree := range []struct {
		env  string
		root string
	}{{"COURSE_LAB_SOURCES", rootLabsrc}, {"COURSE_SOLUTIONS", rootSolutions}} {
		t.Run(tree.root, func(t *testing.T) {
			dir := os.Getenv(tree.env)
			if dir == "" {
				t.Skipf("%s is not set: the real %s rows are not checked", tree.env, tree.root)
			}
			st := &Station{labsrc: dir, solutions: dir}
			n := 0
			for i := range Spec {
				ck := &Spec[i]
				root := ck.Root
				if root == rootStation {
					root = rootLabsrc
				}
				if ck.Kind != kindAnnotation || root != tree.root {
					continue
				}
				n++
				if r := st.Run(ck); r.Verdict != Pass {
					t.Errorf("%s %s: %s", r.ID, r.Verdict, r.Evidence)
				}
			}
			if n == 0 {
				t.Fatalf("no %s annotation row in the spec", tree.root)
			}
		})
	}
}
