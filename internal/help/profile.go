package help

// One-shot class profile (profile_search.py): identity, inheritance chain,
// own slots and methods, import references and source file.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
)

// profileChain is _get_chain: the class and its extends ancestors, root
// first, following generic-stripped keys and stopping at a cycle.
func profileChain(inh *index.Object, name string) []string {
	chain := []string{name}
	seen := map[string]bool{}
	for cur := name; cur != "" && !seen[cur]; {
		seen[cur] = true
		entry := getObj(inh, cur)
		if !truthy(entry) {
			for _, k := range inh.Keys() {
				if stripGenerics(k) == cur {
					entry = inh.Object(k)
					break
				}
			}
		}
		parent := truthyStr(entry, "extends")
		if !truthy(entry) || parent == "" {
			break
		}
		chain = append(chain, parent)
		cur = parent
	}
	return reversed(chain)
}

// sortedHead prints "<label> (N): a, b" sorted, the first ten and " ..."
// when there are more.
func (e *Env) sortedHead(label string, names []string) {
	names = append([]string(nil), names...)
	sort.Strings(names)
	tail := ""
	if len(names) > 10 {
		names, tail = names[:10], " ..."
	}
	fmt.Fprintf(e.Out, "%s: %s%s\n", label, strings.Join(names, ", "), tail)
}

// Profile is cmd_profile; its indexes are optional (a missing one is empty).
func (e *Env) Profile(name string) error {
	idx := map[string]*index.Object{}
	for _, n := range []string{"class-index.json", "inheritance.json", "method-index.json",
		"slots-index.json", "xref-index.json", "source-index.json"} {
		o, err := e.loadIndex(n, false)
		if err != nil {
			return err
		}
		idx[n] = o
	}
	classes := idx["class-index.json"]
	cn, ok := resolveClassKey(classes, name)
	if !ok {
		return e.reportMiss("profile", name, "classes")
	}
	ci, inh := classes.Object(cn), idx["inheritance.json"]
	ih, x := getObj(inh, cn), getObj(idx["xref-index.json"], cn)
	src, sl := getObj(idx["source-index.json"], cn), getObj(idx["slots-index.json"], cn)
	out := e.Out
	fmt.Fprintf(out, "PROFILE: %s\n%s\n", cn, strings.Repeat("=", 60))
	fmt.Fprintf(out, "  Package:  %s\n  Type:     %s\n", getStr(ci, "package", ""), getStr(ci, "type", ""))
	if truthy(src) {
		fmt.Fprintf(out, "  Module:   %s\n  Source:   %s lines\n", getStr(src, "module", "?"), getStr(src, "lines", "?"))
	}
	fmt.Fprintf(out, "\nHIERARCHY:\n  Chain: %s\n", strings.Join(profileChain(inh, cn), " > "))
	if truthy(get(ih, "implements", nil)) {
		fmt.Fprintf(out, "  Implements: %s\n", strings.Join(strList(ih, "implements"), ", "))
	}
	if subs := strList(ih, "subclasses"); len(subs) > 0 {
		e.sortedHead(fmt.Sprintf("  Subclasses (%d)", len(subs)), subs)
	}

	props, actions, topics := objList(sl, "properties"), objList(sl, "actions"), objList(sl, "topics")
	if len(props)+len(actions)+len(topics) > 0 {
		fmt.Fprintf(out, "\nSLOTS (%d props, %d actions, %d topics):\n",
			listLen(sl, "properties"), listLen(sl, "actions"), listLen(sl, "topics"))
		for _, p := range props {
			flags := ""
			if f := truthyStr(p, "flags"); f != "" {
				flags = " [" + f + "]"
			}
			fmt.Fprintf(out, "  [P] %s: %s%s\n", getStr(p, "name", "?"), getStr(p, "type", "?"), flags)
		}
		for _, a := range actions {
			param := "()"
			if t := truthyStr(a, "parameterType"); t != "" {
				param = "(" + t + ")"
			}
			fmt.Fprintf(out, "  [A] %s%s\n", getStr(a, "name", "?"), param)
		}
		for _, t := range topics {
			evt := ""
			if v := truthyStr(t, "eventType"); v != "" {
				evt = " -> " + v
			}
			fmt.Fprintf(out, "  [T] %s%s\n", getStr(t, "name", "?"), evt)
		}
	}

	var sigs []string
	methods := idx["method-index.json"]
	for _, k := range methods.Keys() {
		for _, m := range objList(methods, k) {
			if getStr(m, "class", "") == cn && getStr(m, "section", "") == "methods" {
				sigs = append(sigs, getStr(m, "signature", ""))
			}
		}
	}
	if len(sigs) > 0 {
		sort.Strings(sigs)
		fmt.Fprintf(out, "\nMETHODS (%d):\n", len(sigs))
		for _, s := range sigs {
			fmt.Fprintf(out, "  %s\n", s)
		}
	}

	uses, usedBy := strList(x, "uses"), strList(x, "used_by")
	if len(uses)+len(usedBy) > 0 {
		fmt.Fprintln(out, "\nXREF:")
		if len(uses) > 0 {
			e.sortedHead(fmt.Sprintf("  Uses (%d)", len(uses)), uses)
		}
		if len(usedBy) > 0 {
			e.sortedHead(fmt.Sprintf("  Used by (%d)", len(usedBy)), usedBy)
		}
	}
	if truthy(src) {
		fmt.Fprintf(out, "\nSOURCE: source/%s\n", getStr(src, "path", "?"))
	}
	fmt.Fprintf(out, "\nTip: Use 'snippet %s <method>' to read a specific method\n", cn)
	return nil
}
