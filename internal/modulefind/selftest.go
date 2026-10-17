package modulefind

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/pyre"
)

// selftest is modulefind/selftest.py: in-memory assertions on synthetic Java sources,
// written to a temporary tree, with the same check labels.

var selftestFiles = [][2]string{
	{"BColdRoom", `
package com.x;
public class BColdRoom extends BComponent {
  @NiagaraProperty(
    name = "setpoint",
    type = "baja:StatusNumeric",
    flags = Flags.SUMMARY | Flags.OPERATOR,
    facets = @Facet("BFacets.make(BFacets.MIN, BDouble.make(-40.0))")
  )
  @NiagaraProperty(name = "hidden1", type = "baja:Double", flags = Flags.HIDDEN)
  @NiagaraAction(name = "applySetpoint", flags = Flags.OPERATOR)
  public void doApplySetpoint() { setSetpoint(BStatusNumeric.make(1.0)); }
  void f() { this.set("setpoint", x); }
  final String ORD = "slot:/Programacion/ColdRoom_1";
}
`},
	{"BDashboardServlet", `
package com.x;
public class BDashboardServlet extends BWebServlet {
  public String getServletName() { return "dashboardpan"; }
  void handleSetpointWrite() { parent.set(prop, toSet, null); }
}
`},
	// OPERATOR complex slot with NO action on the class -> the S19 WARN case.
	{"BRoomPanel", `
package com.x;
public class BRoomPanel extends BComponent {
  @NiagaraProperty(name = "setpoint", type = "BStatusNumeric", flags = Flags.SUMMARY | Flags.OPERATOR)
  @NiagaraProperty(name = "differentialUp", type = "double", flags = Flags.SUMMARY)
}
`},
}

func selftest(out io.Writer) int {
	var fails []string
	check := func(cond bool, label string) {
		if cond {
			fmt.Fprintln(out, "  ok   "+label)
		} else {
			fmt.Fprintln(out, "  FAIL "+label)
			fails = append(fails, label)
		}
	}
	d, err := os.MkdirTemp("", "module-find-selftest-")
	if err != nil {
		fmt.Fprintf(out, "\nSELFTEST FAILED: cannot create a temporary directory: %v\n", err)
		return 1
	}
	defer os.RemoveAll(d)
	write := func(path, body string) { _ = os.WriteFile(path, []byte(body), 0o644) }
	_ = os.Mkdir(filepath.Join(d, ".git"), 0o755) // dot-dir must be pruned
	write(filepath.Join(d, ".git", "BNoise.java"), "class BNoise extends BFake {}")
	for _, f := range selftestFiles {
		write(filepath.Join(d, f[0]+".java"), f[1])
	}
	m := newModule(d)
	slot := func(cls, name string) slotInfo {
		if s, ok := m.slots.get(cls); ok {
			return s.m[name]
		}
		return slotInfo{}
	}
	sp := slot("BColdRoom", "setpoint")
	_, noise := m.extends.get("BNoise")
	check(!noise, "dot-dir (.git) pruned")
	check(sp.typ == "baja:StatusNumeric", "multi-line @NiagaraProperty joined by paren balance")
	check(strings.Contains(sp.flags, "SUMMARY") && strings.Contains(sp.flags, "OPERATOR"),
		`multi-token flags "SUMMARY | OPERATOR" captured WHOLE (not truncated at |)`)
	check(flagsHave(sp.flags, "o") && flagsHave(sp.flags, "s"), "--flags OPERATOR matches a SUMMARY|OPERATOR slot")
	check(sp.min != nil && *sp.min == -40.0, "MIN facet parsed")
	check(complexType.MatchString(sp.typ), "StatusNumeric detected as complex")
	acts, _ := m.actions.get("BColdRoom")
	check(acts != nil && acts.m["applySetpoint"] == "o", "@NiagaraAction OPERATOR flag")
	anyHas := func(set map[string]bool, sub string) bool {
		for x := range set {
			if strings.Contains(x, sub) {
				return true
			}
		}
		return false
	}
	w := m.writers["setpoint"]
	check(anyHas(w, `.set("setpoint"`) && anyHas(w, "setSetpoint"),
		`both .set("setpoint",..) and setSetpoint(..) writers found`)
	check(strings.Join(m.resolveChain("BDashboardServlet"), ",") == "BDashboardServlet,BWebServlet",
		"extends chain resolved (servlet name inherited from BWebServlet)")
	check(anyHas(m.dynWriters["BDashboardServlet"], "parent.set(prop"),
		"dynamic .set(prop,..) writer detected (runtime-resolved slot)")
	ord := false
	for _, x := range m.ords {
		ord = ord || x == [2]string{"BColdRoom", "slot:/Programacion/ColdRoom_1"}
	}
	check(ord, "ORD literal captured")

	// new-command pins: run each end-to-end and assert the fact it must report
	call := func(fn func(o *opts)) string {
		var buf bytes.Buffer
		fn(&opts{out: &buf, err: io.Discard})
		return buf.String()
	}
	outST := call(func(o *opts) { cmdSlotTypes(m, o) })
	check(strings.Contains(outST, "StatusNumeric") && strings.Contains(outST, "complex"),
		"slot-types: aggregates StatusNumeric as complex")
	outEW := call(func(o *opts) { cmdExtWritable(m, o) })
	check(strings.Contains(outEW, "WARN") && strings.Contains(outEW, "BRoomPanel.setpoint"),
		"ext-writable: flags BRoomPanel.setpoint (OPERATOR complex, no action) as S19 WARN")
	check(strings.Contains(outEW, "BColdRoom.setpoint") && strings.Contains(outEW, "ok(has-action)"),
		"ext-writable: BColdRoom.setpoint is ok (class has applySetpoint action)")
	outCL := call(func(o *opts) { cmdCallers(m, o, "setSetpoint") })
	check(strings.Contains(outCL, "BColdRoom"), "callers: finds setSetpoint call site")
	outGS := call(func(o *opts) { cmdGrep(m, o, pyre.MustCompile(`parent\.set`, pyre.IGNORECASE), true) })
	check(strings.Contains(outGS, "BDashboardServlet.java:5"), "grep --source: finds a method-body line with file:line")
	outGN := call(func(o *opts) { cmdGrep(m, o, pyre.MustCompile(`parent`, pyre.IGNORECASE), false) })
	check(strings.Contains(outGN, "names in"), "grep (names): the miss line says it searched names, not the code")

	// compare across two synthetic versions (slot added in B)
	da, db := filepath.Join(d, "cmpA"), filepath.Join(d, "cmpB")
	_ = os.Mkdir(da, 0o755)
	_ = os.Mkdir(db, 0o755)
	write(filepath.Join(da, "BX.java"), "class BX extends BComponent {\n@NiagaraProperty(name=\"a\", type=\"double\")\n}")
	write(filepath.Join(db, "BX.java"), "class BX extends BComponent {\n@NiagaraProperty(name=\"a\", type=\"int\")\n"+
		"@NiagaraProperty(name=\"defrostSkipped\", type=\"boolean\")\n}")
	outCmp := call(func(o *opts) { cmdCompare(newModule(da), o, db) })
	check(strings.Contains(outCmp, "slot-added") && strings.Contains(outCmp, "defrostSkipped"),
		"compare: detects an added slot")
	check(strings.Contains(outCmp, "slot-retyped") && strings.Contains(outCmp, "double -> int"),
		"compare: detects a retype (schema-risk)")

	if len(fails) > 0 {
		fmt.Fprintf(out, "\nSELFTEST FAILED: %d check(s)\n", len(fails))
		return 1
	}
	fmt.Fprintln(out, "\nSELFTEST OK")
	return 0
}
