package modulefind

import (
	"fmt"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// cmdSlotTypes is cmd_slot_types: per Java type, how many slots, OPERATOR, complex and
// TRANSIENT; most frequent first (ties in first-seen order).
func cmdSlotTypes(m *module, o *opts) {
	type agg struct{ count, operator, complex, transient int }
	byType := newOmap[*agg]()
	for _, cls := range m.slots.keys {
		slots, _ := m.slots.get(cls)
		for _, slot := range slots.keys {
			s := slots.m[slot]
			t := s.typ
			if t == "" {
				t = "(none)"
			}
			a, ok := byType.get(t)
			if !ok {
				a = &agg{}
				byType.set(t, a)
			}
			a.count++
			flags := strings.ToUpper(s.flags)
			if strings.Contains(flags, "OPERATOR") {
				a.operator++
			}
			if complexType.MatchString(t) {
				a.complex++
			}
			if strings.Contains(flags, "TRANSIENT") {
				a.transient++
			}
		}
	}
	types := append([]string(nil), byType.keys...)
	sort.SliceStable(types, func(i, j int) bool { return byType.m[types[i]].count > byType.m[types[j]].count })
	var rows []row
	for _, t := range types {
		a := byType.m[t]
		shape := "simple"
		if complexType.MatchString(t) {
			shape = "complex"
		}
		rows = append(rows, row{{"type", t}, {"count", a.count}, {"operator", a.operator}, {"shape", shape},
			{"transient", a.transient}})
	}
	render(rows, o, func(r row) string {
		return fmt.Sprintf("%sx  %s %s OPERATOR=%d TRANSIENT=%d", pycompat.RJust(fmt.Sprint(r.get("count")), 3, ' '),
			pycompat.LJust(r.str("type"), 20, ' '), pycompat.LJust(r.str("shape"), 8, ' '), r.get("operator"), r.get("transient"))
	}, "(no @NiagaraProperty)")
}

// cmdExtWritable is cmd_ext_writable, the S19 lint preview: an OPERATOR complex property
// on a class without any @NiagaraAction is a WARN.
func cmdExtWritable(m *module, o *opts) {
	var rows []row
	warns := 0
	for _, cls := range m.slots.sortedKeys() {
		acts, _ := m.actions.get(cls)
		hasAction := acts != nil && len(acts.keys) > 0
		slots, _ := m.slots.get(cls)
		for _, slot := range slots.keys {
			s := slots.m[slot]
			if !complexType.MatchString(s.typ) || !strings.Contains(strings.ToUpper(s.flags), "OPERATOR") {
				continue
			}
			verdict, note := "ok(has-action)", "class exposes an action"
			if !hasAction {
				verdict = "WARN"
				note = "external write must use the oBIX child-leaf bare <real> (…/value, B826-G2) or add an OPERATOR action (B822)"
				warns++
			}
			rows = append(rows, row{{"class", cls}, {"slot", slot}, {"type", s.typ}, {"verdict", verdict}, {"note", note}})
		}
	}
	render(rows, o, func(r row) string {
		return fmt.Sprintf("%s %s.%s  <%s>  %s", pycompat.LJust(r.str("verdict"), 14, ' '), r.str("class"), r.str("slot"),
			r.str("type"), r.str("note"))
	}, "(no OPERATOR complex properties)")
	if !o.json && !o.csv {
		fmt.Fprintf(o.out, "# %d OPERATOR complex properties; %d WARN (S19)\n", len(rows), warns)
	}
}

// cmdCompare is cmd_compare: the annotation-level schema diff of root (A, older) and
// srcB (B, newer): added/removed/retyped/reflagged slots, added/removed actions.
func cmdCompare(a *module, o *opts, srcB string) {
	b := newModule(srcB)
	classes := map[string]bool{}
	for _, om := range []*omap[*omap[slotInfo]]{a.slots, b.slots} {
		for _, k := range om.keys {
			classes[k] = true
		}
	}
	for _, om := range []*omap[*omap[string]]{a.actions, b.actions} {
		for _, k := range om.keys {
			classes[k] = true
		}
	}
	var rows []row
	add := func(change, ref, detail string) {
		rows = append(rows, row{{"change", change}, {"ref", ref}, {"detail", detail}})
	}
	for _, cls := range sortedSet(classes) {
		sa, sb := slotsOf(a, cls), slotsOf(b, cls)
		for _, slot := range unionKeys(sa.m, sb.m) {
			x, inA := sa.m[slot]
			y, inB := sb.m[slot]
			ref := cls + "." + slot
			switch {
			case !inA:
				add("slot-added", ref, y.typ)
			case !inB:
				add("slot-removed", ref, x.typ)
			default:
				if x.typ != y.typ {
					add("slot-retyped", ref, x.typ+" -> "+y.typ)
				}
				if x.flags != y.flags {
					add("slot-reflagged", ref, x.flags+" -> "+y.flags)
				}
			}
		}
		aa, ab := actionsOf(a, cls), actionsOf(b, cls)
		for _, act := range unionKeys(aa.m, ab.m) {
			if _, inA := aa.m[act]; !inA {
				add("action-added", cls+"."+act+"()", "")
			} else if _, inB := ab.m[act]; !inB {
				add("action-removed", cls+"."+act+"()", "")
			}
		}
	}
	render(rows, o, func(r row) string {
		return fmt.Sprintf("%s %s  %s", pycompat.LJust(r.str("change"), 15, ' '), r.str("ref"), r.str("detail"))
	}, "(no schema changes)")
	if !o.json && !o.csv {
		risky := 0
		for _, r := range rows {
			if c := r.str("change"); c == "slot-removed" || c == "slot-retyped" {
				risky++
			}
		}
		fmt.Fprintf(o.out, "# %d changes; %d schema-RISK (removed/retyped — run schema-risk.sh, B795)\n", len(rows), risky)
	}
}

func slotsOf(m *module, cls string) *omap[slotInfo] {
	if s, ok := m.slots.get(cls); ok {
		return s
	}
	return newOmap[slotInfo]()
}

func actionsOf(m *module, cls string) *omap[string] {
	if s, ok := m.actions.get(cls); ok {
		return s
	}
	return newOmap[string]()
}

func unionKeys[V any](a, b map[string]V) []string {
	set := map[string]bool{}
	for k := range a {
		set[k] = true
	}
	for k := range b {
		set[k] = true
	}
	return sortedSet(set)
}
