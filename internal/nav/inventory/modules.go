package inventory

import (
	"fmt"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

// The module inventory views of inventory/modules.py: inventory, module and modules.

func loadInventory(env *registry.Env) *index.Object {
	return env.Load("module-inventory.json", core.LoadOpts{})
}

// typeOrStandalone is `v["type"] if v["type"] else "standalone"`.
func typeOrStandalone(v *index.Object) string {
	if core.Truthy(v, "type") {
		return core.Str(v, "type")
	}
	return "standalone"
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
		types[typeOrStandalone(v)]++
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
	inv := data.Object("modules")
	typeFilter := ns.String("mod_type")
	zkmOnly, noCode, hasCode := ns.Bool("zkm"), ns.Bool("no_code"), ns.Bool("has_code")
	bytecode, filterBC := ns.Get("bytecode").(int)
	top := ns.Int("top")
	type row struct {
		name string
		v    *index.Object
	}
	var results []row
	for i, name := range inv.Keys() {
		v, ok := inv.Values()[i].(*index.Object)
		if !ok {
			continue
		}
		switch {
		case typeFilter != "" && typeOrStandalone(v) != typeFilter,
			zkmOnly && !core.Truthy(v, "zkm"),
			noCode && core.Truthy(v, "has_code"),
			hasCode && !core.Truthy(v, "has_code"):
			continue
		}
		if filterBC {
			if n, isInt := index.Int(func() any { x, _ := v.Get("bytecode"); return x }()); !isInt || n != int64(bytecode) {
				continue
			}
		}
		results = append(results, row{name, v})
	}
	if top != 0 {
		sort.SliceStable(results, func(a, b int) bool {
			return core.Int(results[a].v, "java_files") > core.Int(results[b].v, "java_files")
		})
		results = results[:pySliceEnd(top, len(results))]
	} else {
		sort.SliceStable(results, func(a, b int) bool { return results[a].name < results[b].name })
	}
	var parts []string
	if typeFilter != "" {
		parts = append(parts, "type="+typeFilter)
	}
	if zkmOnly {
		parts = append(parts, "ZKM")
	}
	if noCode {
		parts = append(parts, "no-code")
	}
	if hasCode {
		parts = append(parts, "has-code")
	}
	if filterBC {
		parts = append(parts, fmt.Sprintf("bytecode=v%d", bytecode))
	}
	label := ""
	if len(parts) > 0 {
		label = " (" + strings.Join(parts, ", ") + ")"
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Modules%s: %d results\n", label, len(results))
	fmt.Fprintf(w, "  %-40s %4s  %6s  %6s  %3s  %3s\n", "NAME", "TYPE", "JAVA", "CLASS", "ZKM", "BC")
	fmt.Fprintln(w, "  "+strings.Repeat("-", 70))
	var java, classes int64
	for _, r := range results {
		typ := "-"
		if core.Truthy(r.v, "type") {
			typ = core.Str(r.v, "type")
		}
		bc := "?"
		if core.Int(r.v, "bytecode") > 0 {
			bc = core.Str(r.v, "bytecode")
		}
		fmt.Fprintf(w, "  %s %s  %s  %s  %s  v%s\n", core.LJust(r.name, 40), core.RJust(typ, 4),
			core.RJust(core.Commas(core.Int(r.v, "java_files")), 6), core.RJust(core.Commas(core.Int(r.v, "class_count")), 6),
			core.RJust(yesBlank(core.Truthy(r.v, "zkm")), 3), bc)
		java += core.Int(r.v, "java_files")
		classes += core.Int(r.v, "class_count")
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Total: %d modules, %s java files, %s classes\n", len(results), core.Commas(java), core.Commas(classes))
	fmt.Fprintln(w)
	return env.ExitCode()
}

func yesBlank(b bool) string {
	if b {
		return "YES"
	}
	return ""
}

// pySliceEnd is the end index of Python's seq[:n] on a sequence of length l.
func pySliceEnd(n, l int) int {
	if n < 0 {
		return max(0, l+n)
	}
	return min(n, l)
}
