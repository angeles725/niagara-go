package help

// The rows of indexes/bytecode-index.json (bytecode_index_builder): every
// registered class by name, the supers of one as far as the index holds
// them, and the slots it declares and inherits, each as one line.

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

const bytecodeIndexName = "bytecode-index.json"

// bytecodeRows are the classes rows of bytecode-index.json by dotted class
// name (the first row of a name wins) and the index itself.
func (e *Env) bytecodeRows() (map[string]*index.Object, *index.Object, error) {
	idx, err := e.loadIndex(bytecodeIndexName, false)
	if err != nil {
		return nil, nil, err
	}
	rows := map[string]*index.Object{}
	for _, r := range objList(idx, "classes") {
		if c := getStr(r, "class", ""); rows[c] == nil {
			rows[c] = r
		}
	}
	return rows, idx, nil
}

// simpleName is the class name after its package.
func simpleName(class string) string { return class[strings.LastIndex(class, ".")+1:] }

// ancestors are the rows of the supers of r the index holds, nearest first,
// and the first super it does not hold ("" when the chain ends at
// java.lang.Object or loops).
func ancestors(rows map[string]*index.Object, r *index.Object) (chain []*index.Object, missing string) {
	seen := map[string]bool{getStr(r, "class", ""): true}
	for sup := getStr(r, "super", ""); sup != "" && sup != "java.lang.Object" && !seen[sup]; sup = getStr(r, "super", "") {
		seen[sup] = true
		if r = rows[sup]; r == nil {
			return chain, sup
		}
		chain = append(chain, r)
	}
	return chain, ""
}

// superChain is the SUPER line: the direct super and each of its supers the
// index holds, then where the chain leaves it.
func superChain(rows map[string]*index.Object, r *index.Object) string {
	sup := getStr(r, "super", "")
	if sup == "" {
		return "(none)"
	}
	chain, missing := ancestors(rows, r)
	var parts []string
	for _, a := range chain {
		parts = append(parts, getStr(a, "class", ""))
	}
	if missing != "" {
		parts = append(parts, missing+" (not in "+bytecodeIndexName+")")
	}
	if len(parts) == 0 {
		return sup
	}
	return strings.Join(parts, " > ")
}

// printBytecodeSlots prints the slots r declares, then those each super of it
// in the index declares that no nearer class re-declares.
func (e *Env) printBytecodeSlots(rows map[string]*index.Object, r *index.Object) {
	chain, _ := ancestors(rows, r)
	declared := objList(r, "slots")
	shown := map[string]bool{}
	for _, s := range declared {
		shown[getStr(s, "name", "")] = true
	}
	kept, total := make([][]*index.Object, len(chain)), 0
	for i, a := range chain {
		for _, s := range objList(a, "slots") {
			if n := getStr(s, "name", ""); !shown[n] {
				shown[n] = true
				kept[i] = append(kept[i], s)
				total++
			}
		}
	}
	fmt.Fprintf(e.Out, "SLOTS (%d declared, %d inherited) [bytecode]:\n", len(declared), total)
	for _, s := range declared {
		fmt.Fprintf(e.Out, "  %s\n", slotLine(s))
	}
	for i, own := range kept {
		if len(own) > 0 {
			fmt.Fprintf(e.Out, "  inherited from %s:\n", simpleName(getStr(chain[i], "class", "")))
		}
		for _, s := range own {
			fmt.Fprintf(e.Out, "    %s\n", slotLine(s))
		}
	}
}

// slotLine is one slot: kind, name, type (an action's parameter and return
// type), default, flag letters and whether it overrides an inherited slot or
// is known only by its field.
func slotLine(s *index.Object) string {
	kind, name, typ := getStr(s, "kind", ""), getStr(s, "name", ""), getStr(s, "type", "")
	line := pycompat.LJust(kind, 9, ' ') + name
	switch {
	case kind == "action":
		line += "(" + typ + ")"
		if rt := getStr(s, "return_type", ""); rt != "" {
			line += " -> " + rt
		}
	case typ != "":
		line += ": " + typ
	}
	if d := getStr(s, "default", ""); d != "" {
		line += " = " + d
	}
	if f := getStr(s, "flag_letters", ""); f != "" {
		line += "  flags " + f
	}
	if index.Truthy(get(s, "override", false)) {
		line += "  (override)"
	}
	if getStr(s, "via", "") == "field" {
		line += "  (field only: no slot annotation)"
	}
	return line
}
