package domain

import (
	"fmt"
	"io"
	"sort"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/lib/drivers"
	"github.com/angeles725/niagara-go/internal/nav/lib/license"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func runDrivers(env *registry.Env, ns *argparse.Namespace) int {
	module, given := ns.Get("module").(string)
	cmdDrivers(env, module, given, ns.Bool("drivers_json"))
	return env.ExitCode()
}

// networkRows loads the indexes, the corpus root and the license features
// Drivers reads, then returns its rows and scope, or ok=false (after an
// exit-2 error, or silently when an index is empty).
func networkRows(env *registry.Env) (rows []*drivers.DriverRow, scope string, ok bool) {
	ci := env.Load("class-index.json", core.LoadOpts{})
	var inh, inv *index.Object
	if ci != nil && ci.Len() > 0 {
		inh = env.Load("inheritance.json", core.LoadOpts{})
	}
	if inh != nil && inh.Len() > 0 {
		inv = env.Load("module-inventory.json", core.LoadOpts{})
	}
	if inv == nil || inv.Len() == 0 {
		return nil, "", false
	}
	organized := env.SourceRoot(ci.Object("_meta"))
	if organized == "" {
		return nil, "", false
	}
	report, failure := license.Scan(env.BaseDir)
	if failure != nil {
		env.Error(failure.Message, failure.Hint)
		return nil, "", false
	}
	if report.Warning != "" {
		fmt.Fprintf(env.Err, "WARNING: %s\n", report.Warning)
	}
	rows, scope = drivers.Drivers(drivers.DriverInputs{ClassIndex: ci, Inheritance: inh, Inventory: inv,
		Organized: organized, Features: drivers.FeaturesByModule(report.Calls)})
	return rows, scope, true
}

// driversJSON is json.dumps({"module", "scope", "networks"}, indent=2).
func driversJSON(module string, given bool, scope string, rows []*drivers.DriverRow) string {
	o := index.NewObject()
	if given {
		o.Set("module", module)
	} else {
		o.Set("module", nil)
	}
	o.Set("scope", scope)
	networks := []any{}
	for _, r := range rows {
		networks = append(networks, r.Object())
	}
	o.Set("networks", networks)
	return core.Dumps(o, 2)
}

// cmdDrivers is cmd_drivers; given is false for a module argument of None.
func cmdDrivers(env *registry.Env, module string, given, asJSON bool) {
	rows, scope, ok := networkRows(env)
	if !ok {
		return
	}
	if module != "" {
		wanted := drivers.DriversIn(rows, module)
		if len(wanted) == 0 {
			driversMiss(env, module, rows, scope, asJSON)
			return
		}
		rows = wanted
	}
	switch {
	case asJSON:
		fmt.Fprintln(env.Out, driversJSON(module, given, scope, rows))
	case module != "":
		for _, r := range rows {
			for _, line := range drivers.FormatAnatomy(r) {
				fmt.Fprintln(env.Out, line)
			}
		}
	default:
		printTable(env.Out, rows, scope)
	}
}

// driversMiss is _miss: with --json, stdout stays pure JSON and the miss line
// goes to stderr.
func driversMiss(env *registry.Env, module string, rows []*drivers.DriverRow, scope string, asJSON bool) {
	if asJSON {
		fmt.Fprintln(env.Out, driversJSON(module, true, scope, nil))
		out := env.Runtime.Out
		env.Runtime.Out = env.Err
		env.Miss(fmt.Sprintf("driver network in module '%s'", module), scope, "")
		env.Runtime.Out = out
		return
	}
	known := false
	if inv := env.Load("module-inventory.json", core.LoadOpts{Quiet: true}); inv != nil {
		if mods := inv.Object("modules"); mods != nil {
			known = mods.Has(module)
			for _, v := range mods.Values() {
				if info, ok := v.(*index.Object); ok && !known {
					m, _ := info.Get("module")
					known = m == any(module)
				}
			}
		}
	}
	subject := fmt.Sprintf("driver network in module '%s'", module)
	if !known {
		subject += " (no such module in module-inventory.json)"
	}
	var mods []string
	for _, r := range rows {
		mods = append(mods, r.Module)
	}
	env.Report(subject, scope, module, sortedSet(mods), "drivers")
}

func printTable(w io.Writer, rows []*drivers.DriverRow, scope string) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  DRIVER NETWORKS: %s\n", scope)
	fmt.Fprintln(w)
	for _, line := range drivers.DriverTableHeader() {
		fmt.Fprintln(w, line)
	}
	for _, r := range rows {
		fmt.Fprintln(w, drivers.FormatDriverRow(r))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  * abstract. DOC: the doc module help.guide.base names, else the one named like")
	fmt.Fprintln(w, "  the module directory (by name). Full anatomy: drivers <module>   (e.g. drivers modbusTcp-rt)")
	fmt.Fprintln(w)
}

// sortedSet is sorted(set(xs)).
func sortedSet(xs []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}
