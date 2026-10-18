package help

// inherited-slots and slots --inherited (slots_search.cmd_inherited_slots):
// every slot of a class, walking its inheritance chain from the root and
// keeping the first declaration of each slot name.

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// inheritanceEntry is the entry _get_chain follows for name: the exact key
// when its entry is truthy, else the first key whose generic-stripped form
// is name; nil when that entry is missing or empty.
func inheritanceEntry(inh *index.Object, name string) *index.Object {
	v, _ := inh.Get(name)
	if !index.Truthy(v) {
		for _, k := range inh.Keys() {
			if stripGenerics(k) == name {
				v, _ = inh.Get(k)
				break
			}
		}
	}
	if !index.Truthy(v) {
		return nil
	}
	o, _ := v.(*index.Object)
	return o
}

// ancestry is the _get_chain the member commands share, self first:
// [name, parent, grandparent, ...]. Unlike the hierarchy chain it follows
// every truthy extends, java.lang.Object included, and stops at a name
// already visited (after appending it once more, as Python does).
func ancestry(inh *index.Object, name string) []string {
	chain := []string{name}
	seen := map[string]bool{}
	for cur := name; cur != "" && !seen[cur]; {
		seen[cur] = true
		entry := inheritanceEntry(inh, cur)
		if entry == nil {
			break
		}
		parent := truthyStr(entry, "extends")
		if parent == "" {
			break
		}
		chain = append(chain, parent)
		cur = parent
	}
	return chain
}

// reversed returns a reversed copy of s.
func reversed(s []string) []string {
	out := make([]string, len(s))
	for i, v := range s {
		out[len(s)-1-i] = v
	}
	return out
}

// resolveStripped is the case-insensitive fallback of the chain commands:
// the generic-stripped form of the first inheritance key matching name.
func resolveStripped(inh *index.Object, name string) (string, bool) {
	want := lowerStr(name)
	for _, k := range inh.Keys() {
		if s := stripGenerics(k); lowerStr(s) == want {
			return s, true
		}
	}
	return "", false
}

// InheritedSlotsParams are the arguments of `inherited-slots`.
type InheritedSlotsParams struct {
	Name string
	JSON bool
}

// inheritedSlot is one slot of the merged chain: its name and type (raw
// values, as json.dumps prints them), declaring class and object.
type inheritedSlot struct {
	name, typ any
	origin    string
	slot      *index.Object
}

// slotKind is one section of the merged view: its key, the type key of
// its slots and that type's default.
type slotKind struct{ section, typeKey, typeDef string }

var inheritedKinds = []slotKind{
	{"properties", "type", "?"}, {"actions", "parameterType", ""}, {"topics", "eventType", ""},
}

// InheritedSlots is slots_search.cmd_inherited_slots.
func (e *Env) InheritedSlots(p InheritedSlotsParams) error {
	slots, err := e.loadIndex("slots-index.json", false)
	if err != nil {
		return err
	}
	inh, err := e.loadIndex("inheritance.json", false)
	if err != nil {
		return err
	}
	name := p.Name
	if !slots.Has(name) && !inh.Has(name) {
		s, ok := resolveStripped(inh, name)
		if !ok {
			return e.reportMiss("inherited-slots", name, "inheritance")
		}
		name = s
	}
	chain := reversed(ancestry(inh, name))
	merged := make([][]inheritedSlot, len(inheritedKinds))
	for i, k := range inheritedKinds {
		seen := map[string]bool{}
		for _, ancestor := range chain {
			list, err := objects(ancestor, slots.Object(ancestor), k.section)
			if err != nil {
				return err
			}
			for _, s := range list {
				n := get(s, "name", "")
				if key := pyStr(n); !seen[key] {
					seen[key] = true
					merged[i] = append(merged[i], inheritedSlot{n, get(s, k.typeKey, k.typeDef), ancestor, s})
				}
			}
		}
	}
	if p.JSON {
		out := pyjson.Obj{{"class", name}, {"chain", chain}}
		for i, k := range inheritedKinds {
			rows := make([]any, len(merged[i]))
			for j, s := range merged[i] {
				rows[j] = pyjson.Obj{{"name", s.name}, {k.typeKey, s.typ}, {"declared_in", s.origin}, {"slot", slotValue(s.slot)}}
			}
			out = append(out, pyjson.KV{k.section, rows})
		}
		fmt.Fprintln(e.Out, pyjson.DumpsAny(out, pyjson.Indent2))
		return nil
	}
	e.printInheritedSlots(name, chain, merged)
	return nil
}

func (e *Env) printInheritedSlots(name string, chain []string, merged [][]inheritedSlot) {
	props, actions, topics := merged[0], merged[1], merged[2]
	fmt.Fprintf(e.Out, "INHERITED SLOTS: %s\n", name)
	fmt.Fprintf(e.Out, "CHAIN: %s\n", strings.Join(chain, " > "))
	fmt.Fprintf(e.Out, "TOTAL: %d properties, %d actions, %d topics\n", len(props), len(actions), len(topics))
	section := func(title string, rows []inheritedSlot, line func(inheritedSlot) string) {
		if len(rows) == 0 {
			return
		}
		fmt.Fprintf(e.Out, "\n%s (%d):\n\n", title, len(rows))
		current := ""
		for i, s := range rows {
			if i == 0 || s.origin != current {
				current = s.origin
				marker := ""
				if s.origin == name {
					marker = " (this)"
				}
				fmt.Fprintf(e.Out, "  -- from %s%s --\n", s.origin, marker)
			}
			fmt.Fprintf(e.Out, "    %s\n", line(s))
		}
	}
	section("PROPERTIES", props, func(s inheritedSlot) string {
		return pyStr(s.name) + ": " + pyStr(s.typ) + defaultSuffix(s.slot, 50) + flagsSuffix(s.slot)
	})
	section("ACTIONS", actions, func(s inheritedSlot) string {
		if index.Truthy(s.typ) {
			return pyStr(s.name) + " (" + pyStr(s.typ) + ")"
		}
		return pyStr(s.name) + "()"
	})
	section("TOPICS", topics, func(s inheritedSlot) string {
		if index.Truthy(s.typ) {
			return pyStr(s.name) + " -> " + pyStr(s.typ)
		}
		return pyStr(s.name)
	})
	if len(props) == 0 && len(actions) == 0 && len(topics) == 0 {
		fmt.Fprintln(e.Out, "\n  (no annotated slots found in entire chain)")
	}
}
