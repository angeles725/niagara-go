package inventory

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
)

// The library half of `modules` (decision D29): Modules takes the decoded
// module-inventory.json, prints nothing and returns typed rows;
// ModuleTableHeader and FormatModuleRow yield the exact lines the command
// prints, so the command and nsearch show the same text. Loading the index
// and its stderr belong to the command (modules.go).

// ModuleFilter is the filters of `modules`; the zero value keeps every
// submodule, sorted by name.
type ModuleFilter struct {
	Type       string // rt, wb, ux, doc, se or standalone (no type); "" any
	ZKM        bool   // ZKM-obfuscated only
	NoCode     bool   // without Java code only
	HasCode    bool   // with Java code only
	ByBytecode bool   // filter on Bytecode
	Bytecode   int    // the bytecode version kept when ByBytecode (52 = Java 8)
	Top        int    // non-zero: the first Top (Python seq[:Top]) by .java files, most first
}

// Label is the " (type=rt, ZKM, ...)" the results line names the filters
// with; "" without filters (Top is not a filter).
func (f ModuleFilter) Label() string {
	var parts []string
	if f.Type != "" {
		parts = append(parts, "type="+f.Type)
	}
	if f.ZKM {
		parts = append(parts, "ZKM")
	}
	if f.NoCode {
		parts = append(parts, "no-code")
	}
	if f.HasCode {
		parts = append(parts, "has-code")
	}
	if f.ByBytecode {
		parts = append(parts, fmt.Sprintf("bytecode=v%d", f.Bytecode))
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

// ModuleRow is one submodule (JAR) of the inventory.
type ModuleRow struct {
	Name       string
	Type       string // "" when the submodule has no type
	JavaFiles  int64
	ClassCount int64
	ZKM        bool
	Bytecode   int64  // 0 when unknown
	bytecode   string // str(v["bytecode"]) as the command prints it, when known
}

// Modules is the submodules of inventory (module-inventory.json) that pass f.
func Modules(inventory *index.Object, f ModuleFilter) []ModuleRow {
	inv := inventory.Object("modules")
	if inv == nil {
		return nil
	}
	var rows []ModuleRow
	for i, name := range inv.Keys() {
		v, ok := inv.Values()[i].(*index.Object)
		if !ok {
			continue
		}
		switch {
		case f.Type != "" && typeOrStandalone(v) != f.Type,
			f.ZKM && !core.Truthy(v, "zkm"),
			f.NoCode && core.Truthy(v, "has_code"),
			f.HasCode && !core.Truthy(v, "has_code"):
			continue
		}
		if f.ByBytecode {
			raw, _ := v.Get("bytecode")
			if n, isInt := index.Int(raw); !isInt || n != int64(f.Bytecode) {
				continue
			}
		}
		r := ModuleRow{Name: name, JavaFiles: core.Int(v, "java_files"), ClassCount: core.Int(v, "class_count"),
			ZKM: core.Truthy(v, "zkm"), Bytecode: max(core.Int(v, "bytecode"), 0)}
		if core.Truthy(v, "type") {
			r.Type = core.Str(v, "type")
		}
		if r.Bytecode > 0 {
			r.bytecode = core.Str(v, "bytecode")
		}
		rows = append(rows, r)
	}
	if f.Top != 0 {
		sort.SliceStable(rows, func(a, b int) bool { return rows[a].JavaFiles > rows[b].JavaFiles })
		return rows[:pySliceEnd(f.Top, len(rows))]
	}
	sort.SliceStable(rows, func(a, b int) bool { return rows[a].Name < rows[b].Name })
	return rows
}

// ModuleTableHeader is the column line and the rule of the `modules` table.
func ModuleTableHeader() [2]string {
	return [2]string{fmt.Sprintf("  %-40s %4s  %6s  %6s  %3s  %3s", "NAME", "TYPE", "JAVA", "CLASS", "ZKM", "BC"),
		"  " + strings.Repeat("-", 70)}
}

// FormatModuleRow is the `modules` table row of r: name, type ("-" when
// none), .java and .class counts, YES when ZKM, bytecode ("v?" when unknown).
func FormatModuleRow(r ModuleRow) string {
	typ := "-"
	if r.Type != "" {
		typ = r.Type
	}
	bc := "?"
	if r.Bytecode > 0 {
		bc = r.bytecode
		if bc == "" { // a row built by the caller
			bc = strconv.FormatInt(r.Bytecode, 10)
		}
	}
	return fmt.Sprintf("  %s %s  %s  %s  %s  v%s", core.LJust(r.Name, 40), core.RJust(typ, 4),
		core.RJust(core.Commas(r.JavaFiles), 6), core.RJust(core.Commas(r.ClassCount), 6),
		core.RJust(yesBlank(r.ZKM), 3), bc)
}
