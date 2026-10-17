package help

// contract (contract_search.cmd_contract): what an interface requires (its
// methods, constants and slots), who implements it and its description.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// ContractParams are the arguments of `contract`.
type ContractParams struct{ Name string }

// member is a method or field of the interface: its name and signature.
type member struct{ name, sig string }

// Contract is contract_search.cmd_contract.
func (e *Env) Contract(p ContractParams) error {
	var idx [4]*index.Object
	for i, n := range []string{"class-index.json", "inheritance.json", "method-index.json", "slots-index.json"} {
		o, err := e.loadIndex(n, true)
		if err != nil {
			return err
		}
		idx[i] = o
	}
	classes, inh, methods, slots := idx[0], idx[1], idx[2], idx[3]
	name, ok := resolveClassKey(classes, p.Name)
	if !ok {
		return e.reportMiss("contract", p.Name, "classes")
	}
	ci := classes.Object(name)
	typ, pkg := getStr(ci, "type", ""), getStr(ci, "package", "")
	if typ != "interface" && typ != "class" {
		fmt.Fprintf(e.Out, "'%s' is a %s — expected interface.\n", name, typ)
	}
	fmt.Fprintf(e.Out, "CONTRACT: %s (%s)\nTYPE: %s\n%s\n", name, pkg, typ, strings.Repeat("=", 60))
	entry := inh.Object(name)
	if ext := truthyStr(entry, "extends"); ext != "" && ext != "java.lang.Object" {
		fmt.Fprintf(e.Out, "EXTENDS: %s\n", ext)
	}
	if impl := strList(entry, "implements"); len(impl) > 0 {
		fmt.Fprintf(e.Out, "ALSO EXTENDS: %s\n", strings.Join(impl, ", "))
	}

	var ms, fs []member
	for _, mname := range methods.Keys() {
		entries, _ := get(methods, mname, nil).([]any)
		for _, v := range entries {
			en, _ := v.(*index.Object)
			if cls, _ := get(en, "class", nil).(string); cls != name {
				continue
			}
			switch pyStr(get(en, "section", nil)) {
			case "methods":
				ms = append(ms, member{mname, getStr(en, "signature", "")})
			case "fields":
				fs = append(fs, member{mname, getStr(en, "signature", "")})
			}
		}
	}
	e.printMembers("METHODS TO IMPLEMENT", ms)
	e.printMembers("CONSTANTS/FIELDS", fs)

	data := slots.Object(name)
	lists, err := slotLists(name, data, "properties", "actions")
	if err != nil {
		return err
	}
	props, actions := lists[0], lists[1]
	if len(props) > 0 || len(actions) > 0 {
		fmt.Fprintln(e.Out, "\nSLOTS:")
		for _, s := range props {
			fmt.Fprintf(e.Out, "  [P] %s: %s\n", getStr(s, "name", "?"), getStr(s, "type", "?"))
		}
		for _, a := range actions {
			fmt.Fprintf(e.Out, "  [A] %s%s\n", getStr(a, "name", "?"), paramSuffix(a))
		}
	}

	impls := contractImplementors(inh, name)
	if len(impls) > 0 {
		fmt.Fprintf(e.Out, "\nIMPLEMENTORS (%d):\n\n", len(impls))
		for _, im := range impls {
			fmt.Fprintf(e.Out, "  %s (%s)\n", im.name, im.pkg)
		}
	} else {
		fmt.Fprintln(e.Out, "\nIMPLEMENTORS: (none found)")
	}
	return e.printContractDescription(pkg, name)
}

// printMembers prints a titled list of members sorted by name (stable).
func (e *Env) printMembers(title string, ms []member) {
	if len(ms) == 0 {
		return
	}
	sort.SliceStable(ms, func(i, j int) bool { return ms[i].name < ms[j].name })
	fmt.Fprintf(e.Out, "\n%s (%d):\n\n", title, len(ms))
	for _, m := range ms {
		fmt.Fprintf(e.Out, "  %s\n", m.sig)
	}
}

// contractImplementors lists the inheritance keys (cut at "&lt;") whose
// implements names the interface case-insensitively, sorted by name.
func contractImplementors(inh *index.Object, iface string) []implementor {
	want := lowerStr(iface)
	var out []implementor
	for _, key := range inh.Keys() {
		entry := inh.Object(key)
		for _, i := range strList(entry, "implements") {
			if lowerStr(i) == want {
				name, _, _ := strings.Cut(key, "&lt;")
				out = append(out, implementor{name, getStr(entry, "package", "")})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// printContractDescription prints the DESCRIPTION part of the class's
// bajadoc-clean text: the lines before the first FIELDS:/METHODS:/
// CONSTRUCTORS: line, from "DESCRIPTION:" on.
func (e *Env) printContractDescription(pkg, name string) error {
	text, ok, err := e.readCleanClass(pkg, name)
	if err != nil || !ok || text == "" {
		return err
	}
	var head []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "FIELDS:") || strings.HasPrefix(line, "METHODS:") ||
			strings.HasPrefix(line, "CONSTRUCTORS:") {
			break
		}
		head = append(head, line)
	}
	desc := pycompat.Strip(strings.Join(head, "\n"))
	if i := strings.Index(desc, "DESCRIPTION:"); i >= 0 {
		fmt.Fprintf(e.Out, "\n%s\n", desc[i:])
	}
	return nil
}
