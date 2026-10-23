package help

// Module descriptor lookups over indexes/module-index.json (built by rebuild-all's
// module_index_builder from every module jar's META-INF/module.xml): `module`
// resolves a module name and prints its summary or a listing.

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/fnmatch"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

const moduleIndexName = "module-index.json"

// ModuleParams are the arguments of `module`.
// Detail is "", or the one list to print: "types", "agents", "permissions",
// "defs" or "schemas"; JSON prints the record (or that list) as JSON.
type ModuleParams struct {
	Name   string
	Detail string
	JSON   bool
}

// modules is the "modules" object of module-index.json (required: a data root
// without it cannot answer).
func (e *Env) modules() (*index.Object, error) {
	idx, err := e.loadIndex(moduleIndexName, true)
	if err != nil {
		return nil, err
	}
	if mods := idx.Object("modules"); mods != nil {
		return mods, nil
	}
	return index.NewObject(), nil
}

// resolveModules is the modules a query names, in index order: the exact name,
// else the name in any letter case, else every runtime profile of a module part
// name, else a glob (as written, then in any letter case).
func resolveModules(mods *index.Object, query string) []string {
	if mods.Has(query) {
		return []string{query}
	}
	pick := func(match func(name string, rec *index.Object) bool) []string {
		var out []string
		for _, k := range mods.Keys() {
			if match(k, mods.Object(k)) {
				out = append(out, k)
			}
		}
		return out
	}
	want := textfold.Lower(query)
	for _, match := range []func(string, *index.Object) bool{
		func(k string, _ *index.Object) bool { return textfold.Lower(k) == want },
		func(_ string, r *index.Object) bool { return textfold.Lower(getStr(r, "module", "")) == want },
		func(k string, _ *index.Object) bool { return fnmatch.Fnmatch(k, query) },
		func(k string, _ *index.Object) bool { return lowerGlob(k, query) },
	} {
		if out := pick(match); len(out) > 0 {
			return out
		}
	}
	return nil
}

// Module is `module`: the summary of the one module a name resolves to, or the
// listing of several, or the miss block.
func (e *Env) Module(p ModuleParams) error {
	mods, err := e.modules()
	if err != nil {
		return err
	}
	names := resolveModules(mods, p.Name)
	switch len(names) {
	case 0:
		return e.reportMiss("module", p.Name, "module-xml")
	case 1:
		e.moduleOne(mods, names[0], p)
	default:
		e.moduleMany(mods, names, p)
	}
	return nil
}

// moduleListing lists the modules a query matched, one line each.
func (e *Env) moduleListing(mods *index.Object, query string, names []string) {
	fmt.Fprintf(e.Out, "%d modules matching '%s':\n\n", len(names), query)
	width := 0
	for _, n := range names {
		width = max(width, runeLen(n))
	}
	for _, n := range names {
		r := mods.Object(n)
		fmt.Fprintf(e.Out, "  %s  %s %s  %s\n", pycompat.LJust(n, width, ' '), getStr(r, "vendor", ""),
			getStr(r, "version", ""), getStr(r, "description", ""))
	}
}

// reverseDependencies are the modules that declare a dependency on name, with
// the version each requires.
func reverseDependencies(mods *index.Object, name string) [][2]string {
	var out [][2]string
	for _, k := range mods.Keys() {
		for _, d := range objList(mods.Object(k), "dependencies") {
			if getStr(d, "name", "") == name {
				out = append(out, [2]string{k, getStr(d, "version", "")})
				break
			}
		}
	}
	return out
}

// moduleSummary prints one module: identity, dependencies both ways, element
// counts with the command that lists them, adapters and java-permissions.
func (e *Env) moduleSummary(mods *index.Object, name string) {
	r, out := mods.Object(name), e.Out
	fmt.Fprintf(out, "MODULE: %s\nJAR: %s\n", name, getStr(r, "jar", ""))
	fmt.Fprintf(out, "PART: %s (runtime profile %s)\n", getStr(r, "module", ""), orNone(getStr(r, "runtime_profile", "")))
	fmt.Fprintf(out, "VENDOR: %s %s\n", getStr(r, "vendor", ""), getStr(r, "version", ""))
	fmt.Fprintf(out, "DESCRIPTION: %s\n", getStr(r, "description", ""))
	deps := objList(r, "dependencies")
	rev := reverseDependencies(mods, name)
	width := 0
	for _, d := range deps {
		width = max(width, runeLen(getStr(d, "name", "")))
	}
	for _, d := range rev {
		width = max(width, runeLen(d[0]))
	}
	fmt.Fprintf(out, "\nDEPENDENCIES (%d):\n", len(deps))
	for _, d := range deps {
		fmt.Fprintf(out, "  %s  %s %s\n", pycompat.LJust(getStr(d, "name", ""), width, ' '), getStr(d, "vendor", ""), getStr(d, "version", ""))
	}
	fmt.Fprintf(out, "\nREVERSE DEPENDENCIES (%d):\n", len(rev))
	for _, d := range rev {
		fmt.Fprintf(out, "  %s  requires %s\n", pycompat.LJust(d[0], width, ' '), d[1])
	}
	fmt.Fprintln(out)
	for _, c := range []struct{ label, key, flag string }{
		{"TYPES", "types", "--types"}, {"AGENTS", "agents", "--agents"}, {"DEFS", "defs", "--defs"},
	} {
		label := fmt.Sprintf("%s: %d", c.label, listLen(r, c.key))
		fmt.Fprintf(out, "%s  (nh module %s %s)\n", pycompat.LJust(label, 9, ' '), name, c.flag)
	}
	for _, c := range [][3]string{{"SCHEMAS", "schemas", "--schemas"}, {"PERMISSION TEMPLATES", "permission_templates", "--permissions"}} {
		if n := listLen(r, c[1]); n > 0 { // only a module whose jar ships some
			fmt.Fprintf(out, "%s: %d  (nh module %s %s)\n", c[0], n, name, c[2])
		}
	}
	adapters := objList(r, "adapters")
	fmt.Fprintf(out, "ADAPTERS: %d\n", len(adapters))
	for _, a := range adapters {
		fmt.Fprintf(out, "  %s: %s -> %s\n", getStr(a, "type", ""), getStr(a, "from", ""), getStr(a, "to", ""))
	}
	e.printPermissions(r)
}

// printPermissions lists the java-permissions of a module record.
func (e *Env) printPermissions(r *index.Object) {
	perms := objList(r, "java_permissions")
	fmt.Fprintf(e.Out, "\nJAVA PERMISSIONS (%d):\n", len(perms))
	e.permissionLines(perms)
}

// permissionLines prints one java-permission per line, profiles aligned.
func (e *Env) permissionLines(perms []*index.Object) {
	for _, line := range permissionLineTexts(perms) {
		fmt.Fprintln(e.Out, line)
	}
}

// permissionLineTexts are the lines permissionLines prints.
func permissionLineTexts(perms []*index.Object) []string {
	width := 0
	for _, p := range perms {
		width = max(width, runeLen(getStr(p, "profile", "")))
	}
	var out []string
	for _, p := range perms {
		line := fmt.Sprintf("  %s  %s %q", pycompat.LJust(getStr(p, "profile", ""), width, ' '), getStr(p, "class", ""),
			getStr(p, "name", ""))
		if a := getStr(p, "action", ""); a != "" {
			line += " " + a
		}
		out = append(out, line)
	}
	return out
}

// orNone is s, or "none" when it is empty.
func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "none"
	}
	return s
}
