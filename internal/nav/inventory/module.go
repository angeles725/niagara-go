package inventory

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/lib/modlist"
	"github.com/angeles725/niagara-go/internal/nav/registry"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// runModule is cmd_module: one submodule by exact name or unique substring,
// or only its module.xml dependencies / permissions.
func runModule(env *registry.Env, ns *argparse.Namespace) int {
	if ns.Bool("mod_show_sma") {
		// The license cross-reference needs license_inspect (slice S25).
		fmt.Fprintln(env.Err, "mnav: 'module --sma' is not ported yet")
		return 2
	}
	data := loadInventory(env)
	if data == nil || data.Len() == 0 {
		return env.ExitCode()
	}
	w := env.Out
	inv := data.Object("modules")
	name := ns.String("name")
	resolved := name
	if !inv.Has(name) {
		lower := textfold.Lower(name)
		var matches []string
		for _, k := range inv.Keys() {
			if strings.Contains(textfold.Lower(k), lower) {
				matches = append(matches, k)
			}
		}
		switch len(matches) {
		case 0:
			env.ModuleMiss(name, "modules")
			return env.ExitCode()
		case 1:
			resolved = matches[0]
		default:
			sort.Strings(matches)
			fmt.Fprintf(w, "Multiple matches for '%s':\n", name)
			for _, m := range matches[:min(20, len(matches))] {
				v := inv.Object(m)
				typ := "-"
				if core.Truthy(v, "type") {
					typ = core.Str(v, "type")
				}
				fmt.Fprintf(w, "  %s %s  %s java  %s\n", core.LJust(m, 40), core.RJust(typ, 4),
					core.RJust(core.Commas(core.Int(v, "java_files")), 5), yesBlankZKM(core.Truthy(v, "zkm")))
			}
			if len(matches) > 20 {
				fmt.Fprintf(w, "  ... and %d more\n", len(matches)-20)
			}
			return env.ExitCode()
		}
	}
	v := inv.Object(resolved)
	perms, deps := ns.Bool("mod_show_perms"), ns.Bool("mod_show_deps")
	if perms || deps {
		printModuleXMLView(w, resolved, v, perms, deps)
		return env.ExitCode()
	}
	printModuleDetail(w, resolved, v)
	return env.ExitCode()
}

func yesBlankZKM(b bool) string {
	if b {
		return "ZKM"
	}
	return ""
}

func printModuleDetail(w io.Writer, name string, v *index.Object) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Module:      %s\n", name)
	fmt.Fprintf(w, "  Parent:      %s\n", core.Str(v, "module"))
	fmt.Fprintf(w, "  Type:        %s\n", modlist.TypeOrStandalone(v))
	fmt.Fprintf(w, "  JAR:         %s\n", core.Str(v, "jar"))
	fmt.Fprintf(w, "  Has code:    %s\n", yesNo(core.Truthy(v, "has_code")))
	fmt.Fprintf(w, "  Class count: %s\n", core.Commas(core.Int(v, "class_count")))
	fmt.Fprintf(w, "  Java files:  %s\n", core.Commas(core.Int(v, "java_files")))
	zkm := "no"
	if core.Truthy(v, "zkm") {
		zkm = "YES"
	}
	fmt.Fprintf(w, "  ZKM:         %s\n", zkm)
	java := "?"
	if bc := core.Int(v, "bytecode"); bc >= 45 {
		java = fmt.Sprint(bc - 44)
	}
	fmt.Fprintf(w, "  Bytecode:    v%s (Java %s)\n", core.Str(v, "bytecode"), java)
	fmt.Fprintf(w, "  Vineflower:  %s\n", yesNo(core.Truthy(v, "has_vineflower")))
	fmt.Fprintf(w, "  CFR:         %s\n", yesNo(core.Truthy(v, "has_cfr")))
	fmt.Fprintln(w)
	for _, sec := range []struct{ key, title string }{{"packages", "Packages"}, {"third_party", "Third-party libs"}} {
		if !core.Truthy(v, sec.key) {
			continue
		}
		items := core.Strs(v, sec.key)
		fmt.Fprintf(w, "  %s (%d):\n", sec.title, len(items))
		for _, p := range items {
			fmt.Fprintf(w, "    %s\n", p)
		}
		fmt.Fprintln(w)
	}
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// findModuleXML is _find_module_xml: <corpus>/<parent or *>/<sub>/extracted/
// META-INF/module.xml, then any module directory; the first sorted hit.
func findModuleXML(sub, parent string) string {
	root, _ := core.ResolveOrganized()
	if root == "" {
		return ""
	}
	if parent == "" {
		parent = "*"
	}
	for _, mod := range []string{parent, "*"} {
		hits, _ := filepath.Glob(filepath.Join(root, mod, sub, "extracted", "META-INF", "module.xml"))
		hits = visible(hits, root, mod)
		sort.Strings(hits)
		if len(hits) > 0 {
			return hits[0]
		}
	}
	return ""
}

// visible drops the hits whose wildcard module directory is hidden: Python's
// glob "*" does not match a leading dot.
func visible(hits []string, root, mod string) []string {
	if mod != "*" {
		return hits
	}
	var out []string
	for _, h := range hits {
		rel, err := filepath.Rel(root, h)
		if err == nil && !strings.HasPrefix(rel, ".") {
			out = append(out, h)
		}
	}
	return out
}

func printModuleXMLView(w io.Writer, name string, v *index.Object, perms, deps bool) {
	parent := ""
	if core.Truthy(v, "module") {
		parent = core.Str(v, "module")
	}
	path := findModuleXML(name, parent)
	if path == "" {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  module.xml not found for '%s' (parent='%s')\n", name, core.Str(v, "module"))
		fmt.Fprintln(w)
		return
	}
	root, err := parseXMLFile(path)
	if err != nil {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  module.xml parse failed: %v\n", err)
		fmt.Fprintln(w)
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Module:      %s\n", name)
	fmt.Fprintf(w, "  module.xml:  %s\n", path)
	fmt.Fprintln(w)
	if deps {
		if d := root.find("dependencies"); d == nil {
			fmt.Fprintln(w, "  No <dependencies> block in module.xml.")
		} else {
			fmt.Fprintf(w, "  DEPENDENCIES (%d)\n", len(d.children))
			fmt.Fprintf(w, "  %-30s %-12s %s\n", "NAME", "VENDOR", "VERSION")
			fmt.Fprintln(w, "  "+strings.Repeat("-", 64))
			for _, e := range d.children {
				fmt.Fprintf(w, "  %s %s %s\n", core.LJust(core.Cut(e.attr("name"), 30), 30),
					core.LJust(core.Cut(e.attr("vendor"), 12), 12), e.attr("vendorVersion"))
			}
		}
		fmt.Fprintln(w)
	}
	if perms {
		if p := root.find("permissions"); p == nil {
			fmt.Fprintln(w, "  No <permissions> block in module.xml.")
		} else {
			total := 0
			fmt.Fprintln(w, "  DECLARED PERMISSIONS")
			for _, jps := range p.findAll("java-permissions") {
				entries := jps.findAll("java-permission")
				total += len(entries)
				fmt.Fprintf(w, "    type=%s  (%d entries)\n", jps.attr("type"), len(entries))
				for _, jp := range entries {
					fmt.Fprintf(w, "      %s %s %s\n", core.LJust(core.Cut(jp.attr("class"), 45), 45),
						core.LJust(core.Cut(jp.attr("name"), 25), 25), jp.attr("action"))
				}
			}
			fmt.Fprintln(w)
			fmt.Fprintf(w, "  Total java-permission entries: %d\n", total)
		}
		fmt.Fprintln(w)
	}
}
