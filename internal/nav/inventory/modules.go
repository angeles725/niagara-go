package inventory

import (
	"fmt"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/lib/modlist"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

// The module inventory views of inventory/modules.py: inventory, module and modules.

func loadInventory(env *registry.Env) *index.Object {
	return env.Load("module-inventory.json", core.LoadOpts{})
}

// javaLabel is the "Java N" label of a bytecode version (45 = Java 1).
func javaLabel(bc int64) string {
	if bc >= 45 {
		return fmt.Sprintf("Java %d", bc-44)
	}
	return fmt.Sprint(bc)
}

func runInventory(env *registry.Env, _ *argparse.Namespace) int {
	data := loadInventory(env)
	if data == nil || data.Len() == 0 {
		return env.ExitCode()
	}
	w := env.Out
	inv := data.Object("modules")
	meta := data.Object("_meta")
	var mods []*index.Object
	var names []string
	for i, v := range inv.Values() {
		if o, ok := v.(*index.Object); ok {
			mods, names = append(mods, o), append(names, inv.Keys()[i])
		}
	}
	var withCode, javaFiles, classCount, zkm, withVF int64
	types := map[string]int64{}
	bytecodes := map[int64]int64{}
	modules := map[string]bool{}
	thirdParty := map[string]bool{}
	for _, v := range mods {
		if core.Truthy(v, "has_code") {
			withCode++
		}
		javaFiles += core.Int(v, "java_files")
		classCount += core.Int(v, "class_count")
		if core.Truthy(v, "zkm") {
			zkm++
		}
		if core.Truthy(v, "has_vineflower") {
			withVF++
		}
		types[modlist.TypeOrStandalone(v)]++
		if bc := core.Int(v, "bytecode"); bc > 0 {
			bytecodes[bc]++
		}
		modules[core.Str(v, "module")] = true
		for _, p := range core.Strs(v, "third_party") {
			thirdParty[p] = true
		}
	}
	total := int64(len(mods))
	line := strings.Repeat("=", 60)
	fmt.Fprintf(w, "%s\nMODULE INVENTORY\n%s\n\n", line, line)
	for _, row := range []struct {
		label string
		n     int64
	}{
		{"Total submodules (JARs):  ", total}, {"Unique modules:           ", int64(len(modules))},
		{"With Java code:           ", withCode}, {"Without code (docs/res):  ", total - withCode},
		{"With vineflower/:         ", withVF}, {"ZKM obfuscated:           ", zkm},
		{"Total .java files:        ", javaFiles}, {"Total .class files:       ", classCount},
		{"Unique 3rd-party libs:    ", int64(len(thirdParty))},
	} {
		fmt.Fprintf(w, "  %s %s\n", row.label, core.RJust(core.Commas(row.n), 6))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Type breakdown:")
	typeNames := make([]string, 0, len(types))
	for t := range types {
		typeNames = append(typeNames, t)
	}
	sort.Strings(typeNames)
	for _, t := range typeNames {
		fmt.Fprintf(w, "    %s %s\n", core.LJust(t, 12), core.RJust(core.Commas(types[t]), 5))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Bytecode versions:")
	bcs := make([]int64, 0, len(bytecodes))
	for bc := range bytecodes {
		bcs = append(bcs, bc)
	}
	sort.Slice(bcs, func(a, b int) bool { return bcs[a] < bcs[b] })
	for _, bc := range bcs {
		fmt.Fprintf(w, "    v%d (%s): %s\n", bc, core.LJust(javaLabel(bc), 8), core.RJust(core.Commas(bytecodes[bc]), 5))
	}
	fmt.Fprintln(w)
	if meta != nil && meta.Len() > 0 {
		ts := "unknown"
		if v, ok := meta.Get("timestamp"); ok {
			ts = core.PyStr(v)
		}
		fmt.Fprintf(w, "  Index built: %s\n", ts)
		fmt.Fprintf(w, "  Build time:  %ss\n", core.Fixed(core.Float(meta, "build_time_sec"), 1))
	}
	fmt.Fprintln(w)
	order := make([]int, len(mods))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return core.Int(mods[order[a]], "java_files") > core.Int(mods[order[b]], "java_files")
	})
	fmt.Fprintln(w, "  Top 10 by .java files:")
	for _, i := range order[:min(10, len(order))] {
		v := mods[i]
		fmt.Fprintf(w, "    %s %s files  %s classes\n", core.LJust(names[i], 40),
			core.RJust(core.Commas(core.Int(v, "java_files")), 5), core.RJust(core.Commas(core.Int(v, "class_count")), 5))
	}
	fmt.Fprintln(w)
	return env.ExitCode()
}

func runModules(env *registry.Env, ns *argparse.Namespace) int {
	data := loadInventory(env)
	if data == nil || data.Len() == 0 {
		return env.ExitCode()
	}
	w := env.Out
	bytecode, byBytecode := ns.Get("bytecode").(int)
	f := modlist.ModuleFilter{Type: ns.String("mod_type"), ZKM: ns.Bool("zkm"), NoCode: ns.Bool("no_code"),
		HasCode: ns.Bool("has_code"), ByBytecode: byBytecode, Bytecode: bytecode, Top: ns.Int("top")}
	results := modlist.Modules(data, f)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Modules%s: %d results\n", f.Label(), len(results))
	for _, line := range modlist.ModuleTableHeader() {
		fmt.Fprintln(w, line)
	}
	var java, classes int64
	for _, r := range results {
		fmt.Fprintln(w, modlist.FormatModuleRow(r))
		java += r.JavaFiles
		classes += r.ClassCount
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Total: %d modules, %s java files, %s classes\n", len(results), core.Commas(java), core.Commas(classes))
	fmt.Fprintln(w)
	return env.ExitCode()
}
