package help

// Slot lookups over slots-index.json (slots_search.py): one class's slots, a
// slot name or glob across every class, or every slot of a property type.

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/fnmatch"
	"github.com/angeles725/niagara-go/internal/index"
)

// slotSections are the three slot lists of a slots-index entry, in order.
var slotSections = []string{"properties", "actions", "topics"}

// SlotsParams are the arguments of `slots`.
type SlotsParams struct {
	Query     string
	SlotType  string // "", properties, actions or topics: filters the slot-name search
	ByType    string // search by slot type instead
	JSON      bool
	Inherited bool // every slot of the class Query, inherited ones included
}

// usageError is scope.UsageError: invalid input, "ERROR: ..." and exit 2.
func usageError(msg string) error { return errors.New(msg) }

// Slots is slots_search.cmd_slots.
func (e *Env) Slots(p SlotsParams) error {
	if p.Inherited {
		for _, f := range [][2]string{{p.ByType, "--by-type"}, {p.SlotType, "--type"}} {
			if f[0] != "" {
				return usageError("slots --inherited lists every slot of one class; it takes no " +
					f[1] + " filter")
			}
		}
		if p.Query == "" {
			return usageError("slots --inherited needs a class name")
		}
		return e.InheritedSlots(InheritedSlotsParams{Name: p.Query, JSON: p.JSON})
	}
	idx, err := e.loadIndex("slots-index.json", false)
	if err != nil {
		return err
	}
	if p.ByType != "" {
		return e.slotsByType(idx, p.ByType, p.JSON)
	}
	if idx.Has(p.Query) {
		e.printClassSlots(p.Query, idx.Object(p.Query), p.JSON)
		return nil
	}
	if keys := classSlotKeys(idx, p.Query); len(keys) > 0 {
		for _, k := range keys {
			e.printClassSlots(k, idx.Object(k), p.JSON)
		}
		return nil
	}
	return e.slotsByName(idx, p.Query, p.SlotType, p.JSON)
}

// classSlotKeys is class_slot_entries without a package: the keys naming a
// class case-insensitively, plus the "Name (module)" keys of colliding names.
func classSlotKeys(idx *index.Object, name string) []string {
	low := lowerStr(name)
	var out []string
	for _, k := range idx.Keys() {
		if kl := lowerStr(k); kl == low || strings.HasPrefix(kl, low+" (") {
			out = append(out, k)
		}
	}
	return out
}

// slotHit is one matched slot: its section, class key, package and object.
type slotHit struct {
	section, class string
	pkg            any
	slot           *index.Object
}

// objects is o.get(k, []) as the objects of a list (a non-object item is an
// empty dict, a non-list value an empty list).
func objects(o *index.Object, k string) []*index.Object {
	arr, _ := get(o, k, nil).([]any)
	out := make([]*index.Object, len(arr))
	for i, v := range arr {
		out[i], _ = v.(*index.Object)
	}
	return out
}

// globOrFold is the match of a slot name or type: a pattern with * or ?
// globs (as is, then lower-cased); else exact reports whether the
// lower-cased query must equal the value or only occur in it.
func globOrFold(value, query string, exact bool) bool {
	if strings.ContainsAny(query, "*?") {
		return fnmatch.Fnmatch(value, query) || fnmatch.Fnmatch(lowerStr(value), lowerStr(query))
	}
	if exact {
		return lowerStr(query) == lowerStr(value)
	}
	return strings.Contains(lowerStr(value), lowerStr(query))
}

// slotsByName is mode 3 of cmd_slots: slot names containing (or globbing)
// the query across every class.
func (e *Env) slotsByName(idx *index.Object, query, slotType string, asJSON bool) error {
	var hits []slotHit
	for _, cls := range idx.Keys() {
		data := idx.Object(cls)
		for _, section := range slotSections {
			if slotType != "" && section != slotType {
				continue
			}
			for _, slot := range objects(data, section) {
				if globOrFold(getStr(slot, "name", ""), query, false) {
					hits = append(hits, slotHit{section, cls, get(data, "package", ""), slot})
				}
			}
		}
	}
	if len(hits) == 0 {
		return e.reportMiss("slots", query, "slots")
	}
	if asJSON {
		e.printSlotHitsJSON(hits)
		return nil
	}
	fmt.Fprintf(e.Out, "%d slots matching '%s':\n\n", len(hits), query)
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].section != hits[j].section {
			return hits[i].section < hits[j].section
		}
		return hits[i].class < hits[j].class
	})
	for _, h := range hits {
		label := "[" + h.section[:len(h.section)-1] + "]" // "properties" -> "[propertie]"
		stype := get(h.slot, "type", get(h.slot, "parameterType", get(h.slot, "eventType", "")))
		fmt.Fprintf(e.Out, "  %s %s.%s: %s\n", label, h.class, getStr(h.slot, "name", "?"), pyStr(stype))
		if dv := truthyStr(h.slot, "defaultValue"); dv != "" {
			fmt.Fprintf(e.Out, "    default: %s\n", ellipsis(dv, 80))
		}
		if flags := truthyStr(h.slot, "flags"); flags != "" {
			fmt.Fprintf(e.Out, "    flags: %s\n", flags)
		}
	}
	return nil
}

// ellipsis shortens s to max code points: its first max-3 and "...".
func ellipsis(s string, max int) string {
	if runeLen(s) > max {
		return headRunes(s, max-3) + "..."
	}
	return s
}

func (e *Env) printSlotHitsJSON(hits []slotHit) {
	out := make([]any, len(hits))
	for i, h := range hits {
		out[i] = jsonObj{{"section", h.section}, {"class", h.class}, {"package", h.pkg}, {"slot", slotValue(h.slot)}}
	}
	fmt.Fprintln(e.Out, dumpsIndent2(out))
}

// slotValue is a slot for json.dumps: a non-object list item stays {}.
func slotValue(o *index.Object) any {
	if o == nil {
		return jsonObj{}
	}
	return o
}

// typeFields is the type key of each section for --by-type.
var typeFields = map[string]string{"properties": "type", "actions": "parameterType", "topics": "eventType"}

// slotsByType is _search_by_type: slots whose type equals (or globs) the query.
func (e *Env) slotsByType(idx *index.Object, query string, asJSON bool) error {
	var hits []slotHit
	for _, cls := range idx.Keys() {
		data := idx.Object(cls)
		pkg := get(data, "package", "")
		for _, section := range slotSections {
			for _, slot := range objects(data, section) {
				stype := pyStr(get(slot, typeFields[section], get(slot, "type", "")))
				if globOrFold(stype, query, true) {
					hits = append(hits, slotHit{section, cls, pkg, slot})
				}
			}
		}
	}
	if len(hits) == 0 {
		return e.reportMiss("slots --by-type", query, "slots")
	}
	if asJSON {
		e.printSlotHitsJSON(hits)
		return nil
	}
	var classes []string
	byClass := map[string][]slotHit{}
	for _, h := range hits {
		if _, ok := byClass[h.class]; !ok {
			classes = append(classes, h.class)
		}
		byClass[h.class] = append(byClass[h.class], h)
	}
	fmt.Fprintf(e.Out, "%d slots of type '%s' across %d classes:\n\n", len(hits), query, len(classes))
	sort.Strings(classes)
	for _, cls := range classes {
		group := byClass[cls]
		names := make([]string, len(group))
		for i, h := range group {
			names[i] = getStr(h.slot, "name", "?")
		}
		fmt.Fprintf(e.Out, "  %s (%s) — %s\n", cls, pyStr(group[0].pkg), strings.Join(names, ", "))
	}
	return nil
}

// printClassSlots is _print_class_slots.
func (e *Env) printClassSlots(class string, data *index.Object, asJSON bool) {
	pkg, module := get(data, "package", "?"), get(data, "module", "?")
	if asJSON {
		out := jsonObj{{"class", class}, {"package", pkg}, {"module", module}}
		for _, s := range slotSections {
			out = append(out, jsonKV{s, get(data, s, []any{})})
		}
		fmt.Fprintln(e.Out, dumpsIndent2(out))
		return
	}
	fmt.Fprintf(e.Out, "SLOTS: %s (%s, %s)\n", class, pyStr(pkg), pyStr(module))
	props, actions, topics := objects(data, "properties"), objects(data, "actions"), objects(data, "topics")
	if len(props) > 0 {
		fmt.Fprintf(e.Out, "\nPROPERTIES (%d):\n\n", len(props))
		for _, p := range props {
			fmt.Fprintf(e.Out, "  %s: %s%s%s\n", getStr(p, "name", "?"), getStr(p, "type", "?"),
				defaultSuffix(p, 60), flagsSuffix(p))
		}
	}
	if len(actions) > 0 {
		fmt.Fprintf(e.Out, "\nACTIONS (%d):\n\n", len(actions))
		for _, a := range actions {
			param := "()"
			if pt := truthyStr(a, "parameterType"); pt != "" {
				param = " (" + pt + ")"
			}
			fmt.Fprintf(e.Out, "  %s%s\n", getStr(a, "name", "?"), param)
		}
	}
	if len(topics) > 0 {
		fmt.Fprintf(e.Out, "\nTOPICS (%d):\n\n", len(topics))
		for _, t := range topics {
			fmt.Fprintf(e.Out, "  %s%s\n", getStr(t, "name", "?"), eventSuffix(t))
		}
	}
	if len(props) == 0 && len(actions) == 0 && len(topics) == 0 {
		fmt.Fprintln(e.Out, "  (no annotated slots found)")
	}
}

// defaultSuffix is " = <defaultValue>" (shortened to max code points) when
// the default is truthy.
func defaultSuffix(slot *index.Object, max int) string {
	if dv := truthyStr(slot, "defaultValue"); dv != "" {
		return " = " + ellipsis(dv, max)
	}
	return ""
}

// flagsSuffix is " [<flags>]" when the flags are truthy.
func flagsSuffix(slot *index.Object) string {
	if f := truthyStr(slot, "flags"); f != "" {
		return " [" + f + "]"
	}
	return ""
}

// eventSuffix is " -> <eventType>" when the event type is truthy.
func eventSuffix(slot *index.Object) string {
	if et := truthyStr(slot, "eventType"); et != "" {
		return " -> " + et
	}
	return ""
}
