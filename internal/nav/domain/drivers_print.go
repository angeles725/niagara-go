package domain

import (
	"fmt"
	"io"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func runDrivers(env *registry.Env, ns *argparse.Namespace) int {
	module, given := ns.Get("module").(string)
	cmdDrivers(env, module, given, ns.Bool("drivers_json"))
	return env.ExitCode()
}

// driversJSON is json.dumps({"module", "scope", "networks"}, indent=2).
func driversJSON(module string, given bool, scope string, rows []*DriverRow) string {
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
		wanted := DriversIn(rows, module)
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
			for _, line := range FormatAnatomy(r) {
				fmt.Fprintln(env.Out, line)
			}
		}
	default:
		printTable(env.Out, rows, scope)
	}
}

// driversMiss is _miss: with --json, stdout stays pure JSON and the miss line
// goes to stderr.
func driversMiss(env *registry.Env, module string, rows []*DriverRow, scope string, asJSON bool) {
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

func printTable(w io.Writer, rows []*DriverRow, scope string) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  DRIVER NETWORKS: %s\n", scope)
	fmt.Fprintln(w)
	for _, line := range DriverTableHeader() {
		fmt.Fprintln(w, line)
	}
	for _, r := range rows {
		fmt.Fprintln(w, FormatDriverRow(r))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  * abstract. DOC: the doc module help.guide.base names, else the one named like")
	fmt.Fprintln(w, "  the module directory (by name). Full anatomy: drivers <module>   (e.g. drivers modbusTcp-rt)")
	fmt.Fprintln(w)
}
