package domain

import (
	"fmt"
	"io"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

// row is one driver network (the dict _anatomy returns).
type row struct {
	network, module, pkg, path string
	abstract                   bool
	extends, family            string
	devices                    []string
	devicesFrom, devicesVia    string
	pdx                        []string
	pdxFrom                    string
	proxy                      []string
	proxyFrom                  string
	managers, jobs, license    []string
	doc                        docTarget
}

func optStr(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// object is the row as Python's dict, in its key order (for --json).
func (r *row) object() *index.Object {
	o := index.NewObject()
	var exists any
	if r.doc.exists != nil {
		exists = *r.doc.exists
	}
	for _, kv := range []struct {
		k string
		v any
	}{
		{"network", r.network}, {"module", r.module}, {"package", r.pkg}, {"abstract", r.abstract},
		{"extends", r.extends}, {"family", r.family}, {"devices", r.devices},
		{"devices_from", r.devicesFrom}, {"devices_resolved_via", r.devicesVia},
		{"point_device_exts", r.pdx}, {"point_device_exts_from", r.pdxFrom},
		{"proxy_exts", r.proxy}, {"proxy_exts_from", r.proxyFrom}, {"managers", r.managers},
		{"discovery_jobs", r.jobs}, {"license_features", r.license},
		{"help_guide_base", optStr(r.doc.base)}, {"help_guide_base_others", r.doc.others},
		{"help_doc_module", optStr(r.doc.target)}, {"help_doc_module_exists", exists},
		{"doc_module", optStr(r.doc.doc)}, {"doc_module_via", optStr(r.doc.via)}, {"path", r.path},
	} {
		o.Set(kv.k, kv.v)
	}
	return o
}

func runDrivers(env *registry.Env, ns *argparse.Namespace) int {
	module, given := ns.Get("module").(string)
	cmdDrivers(env, module, given, ns.Bool("drivers_json"))
	return env.ExitCode()
}

// driversJSON is json.dumps({"module", "scope", "networks"}, indent=2).
func driversJSON(module string, given bool, scope string, rows []*row) string {
	o := index.NewObject()
	if given {
		o.Set("module", module)
	} else {
		o.Set("module", nil)
	}
	o.Set("scope", scope)
	networks := []any{}
	for _, r := range rows {
		networks = append(networks, r.object())
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
		var wanted []*row
		for _, r := range rows {
			if module == r.module || module == familyDir(r.path) {
				wanted = append(wanted, r)
			}
		}
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
			printAnatomy(env.Out, r)
		}
	default:
		printTable(env.Out, rows, scope)
	}
}

// driversMiss is _miss: with --json, stdout stays pure JSON and the miss line
// goes to stderr.
func driversMiss(env *registry.Env, module string, rows []*row, scope string, asJSON bool) {
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
		mods = append(mods, r.module)
	}
	env.Report(subject, scope, module, sortedSet(mods), "drivers")
}

// first is _first: the first value and how many more.
func first(values []string) string {
	switch len(values) {
	case 0:
		return "-"
	case 1:
		return values[0]
	}
	return fmt.Sprintf("%s (+%d)", values[0], len(values)-1)
}

func docCell(r *row) string {
	if r.doc.doc != nil && *r.doc.doc != "" {
		if r.doc.via != nil && *r.doc.via == "by name" {
			return *r.doc.doc + " (by name)"
		}
		return *r.doc.doc
	}
	if r.doc.target != nil && *r.doc.target != "" {
		return *r.doc.target + " (missing)"
	}
	return "-"
}

func docLine(r *row) string {
	if r.doc.base == nil {
		return docCell(r)
	}
	note := ""
	if r.doc.exists != nil && !*r.doc.exists {
		note = fmt.Sprintf("; %s is not in organized/", core.PyStr(optStr(r.doc.target)))
	}
	if len(r.doc.others) > 0 {
		note += "; other submodule lexicons: " + strings.Join(r.doc.others, ", ")
	}
	doc := "-"
	if r.doc.doc != nil && *r.doc.doc != "" {
		doc = *r.doc.doc
	}
	return fmt.Sprintf("%s  [help.guide.base=%s%s]", doc, *r.doc.base, note)
}

func printTable(w io.Writer, rows []*row, scope string) {
	line := func(cols ...string) {
		fmt.Fprintf(w, "  %s %s %s %s %s %s\n", core.LJust(cols[0], 30), core.LJust(cols[1], 24),
			core.LJust(cols[2], 11), core.LJust(cols[3], 30), core.LJust(cols[4], 24), cols[5])
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  DRIVER NETWORKS: %s\n", scope)
	fmt.Fprintln(w)
	line("NETWORK", "MODULE", "FAMILY", "DEVICE", "LICENSE", "DOC")
	fmt.Fprintln(w, "  "+strings.Repeat("-", 140))
	for _, r := range rows {
		name := r.network
		if r.abstract {
			name += " *"
		}
		line(name, r.module, r.family, first(r.devices), first(r.license), docCell(r))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  * abstract. DOC: the doc module help.guide.base names, else the one named like")
	fmt.Fprintln(w, "  the module directory (by name). Full anatomy: drivers <module>   (e.g. drivers modbusTcp-rt)")
	fmt.Fprintln(w)
}

func printAnatomy(w io.Writer, r *row) {
	abstract := ""
	if r.abstract {
		abstract = " (abstract)"
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  DRIVER %s%s  (%s, %s)\n", r.network, abstract, r.module, r.pkg)
	fmt.Fprintln(w, "  "+strings.Repeat("=", 70))
	fmt.Fprintf(w, "  %s %s (extends %s)\n", core.LJust("family", 18), r.family, r.extends)
	devicesFrom := r.devicesFrom
	if r.devicesVia == "getDeviceType()" {
		devicesFrom = "getDeviceType()"
	}
	for _, f := range []struct {
		label  string
		values []string
		origin string
	}{
		{"devices", r.devices, devicesFrom}, {"point-device-ext", r.pdx, r.pdxFrom},
		{"proxy-ext", r.proxy, r.proxyFrom}, {"managers", r.managers, ""},
		{"discovery jobs", r.jobs, ""}, {"license", r.license, ""},
	} {
		values, note := "-", ""
		if len(f.values) > 0 {
			values = strings.Join(f.values, ", ")
			if f.origin != "" && f.origin != r.module {
				note = "  [from " + f.origin + "]"
			}
		}
		fmt.Fprintf(w, "  %s %s%s\n", core.LJust(f.label, 18), values, note)
	}
	fmt.Fprintf(w, "  %s %s\n", core.LJust("doc module", 18), docLine(r))
	fmt.Fprintf(w, "  %s %s\n", core.LJust("source", 18), r.path)
	fmt.Fprintln(w)
}
