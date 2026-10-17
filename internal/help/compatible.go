package help

// compatible (compatible_search.cmd_compatible): the class of a type, then
// classes whose names suggest they convert, hold or extend it, and how many
// properties use it.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
)

// CompatibleParams are the arguments of `compatible`.
type CompatibleParams struct{ Type string }

// pointKeywords mark control point classes by name.
var pointKeywords = []string{"point", "writable", "schedule", "const"}

// typeBase is the point-name stem of a lower-cased term: "bstatus" and then
// "status" removed ("bstatusnumeric" -> "numeric").
func typeBase(termLow string) string {
	return strings.ReplaceAll(strings.ReplaceAll(termLow, "bstatus", ""), "status", "")
}

// Compatible is compatible_search.cmd_compatible.
func (e *Env) Compatible(p CompatibleParams) error {
	var idx [3]*index.Object
	for i, n := range []string{"class-index.json", "slots-index.json", "inheritance.json"} {
		o, err := e.loadIndex(n, true)
		if err != nil {
			return err
		}
		idx[i] = o
	}
	classes, slots, inh := idx[0], idx[1], idx[2]
	terms := []string{p.Type}
	if short, ok := strings.CutPrefix(p.Type, "B"); ok {
		terms = append(terms, short)
	} else {
		terms = append(terms, "B"+p.Type)
	}
	fmt.Fprintf(e.Out, "COMPATIBLE: %s\n%s\n", p.Type, strings.Repeat("=", 60))
	for _, t := range terms {
		resolved, ok := resolveClassKey(classes, t)
		if !ok {
			continue
		}
		ci, entry := classes.Object(resolved), inh.Object(resolved)
		fmt.Fprintf(e.Out, "\nTYPE: %s (%s, %s)\n", resolved, getStr(ci, "package", ""), getStr(ci, "type", ""))
		if ext := truthyStr(entry, "extends"); ext != "" {
			fmt.Fprintf(e.Out, "  Extends: %s\n", ext)
		}
		if subs := strList(entry, "subclasses"); len(subs) > 0 {
			sort.Strings(subs)
			fmt.Fprintf(e.Out, "  Subtypes: %s\n", strings.Join(subs, ", "))
		}
		break
	}

	var converters, points, extensions []string
	for _, cls := range classes.Keys() {
		low := lowerStr(cls)
		for _, t := range terms {
			if strings.Contains(low, lowerStr(t)) && (strings.Contains(low, "to") ||
				strings.Contains(low, "from") || strings.Contains(low, "converter")) {
				converters = append(converters, cls)
				break
			}
		}
		for _, t := range terms {
			if base := typeBase(lowerStr(t)); base != "" && strings.Contains(low, base) && containsAny(low, pointKeywords) {
				points = append(points, cls)
				break
			}
		}
		for _, t := range terms {
			if base := typeBase(lowerStr(t)); base != "" && strings.Contains(low, base) && strings.Contains(low, "ext") {
				extensions = append(extensions, cls)
				break
			}
		}
	}
	e.printClassList("CONVERTERS", converters, classes)
	e.printClassList("CONTROL POINTS", points, classes)
	e.printClassList("EXTENSIONS", extensions, classes)

	want := lowerStr(p.Type)
	slotCount, classCount := 0, 0
	for _, cls := range slots.Keys() {
		found := false
		props, err := objects(cls, slots.Object(cls), "properties")
		if err != nil {
			return err
		}
		for _, s := range props {
			if lowerStr(getStr(s, "type", "")) == want {
				slotCount++
				found = true
			}
		}
		if found {
			classCount++
		}
	}
	if slotCount > 0 {
		fmt.Fprintf(e.Out, "\nSLOT USAGE: %d properties of type '%s' across %d classes\n", slotCount, p.Type, classCount)
		fmt.Fprintf(e.Out, "  (use 'slots --by-type %s' for full list)\n", p.Type)
	}
	return nil
}

// containsAny reports whether s contains one of subs.
func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// printClassList prints a titled, sorted list of class keys with their
// packages (the (name, package) tuples sort by name: keys are unique).
func (e *Env) printClassList(title string, names []string, classes *index.Object) {
	if len(names) == 0 {
		return
	}
	sort.Strings(names)
	fmt.Fprintf(e.Out, "\n%s (%d):\n\n", title, len(names))
	for _, n := range names {
		fmt.Fprintf(e.Out, "  %s (%s)\n", n, getStr(classes.Object(n), "package", ""))
	}
}
