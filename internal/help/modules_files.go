package help

// The module-permissions templates and schemas a module's jar ships, as
// module-index.json records them (module_index_builder, shippedxml):
// `module --permissions` adds the templates, `module --schemas` lists the
// schemas. A record without the keys ships none.

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// modulePermissions prints a module's java-permissions, then the templates it ships.
func (e *Env) modulePermissions(name string, r *index.Object) {
	perms := objList(r, "java_permissions")
	fmt.Fprintf(e.Out, "JAVA PERMISSIONS of %s (%d):\n", name, len(perms))
	e.permissionLines(perms)
	templates := objList(r, "permission_templates")
	if len(templates) == 0 {
		return
	}
	fmt.Fprintf(e.Out, "\nPERMISSION TEMPLATES of %s (%d):\n", name, len(templates))
	for _, tp := range templates {
		fmt.Fprintf(e.Out, "  %s\n", getStr(tp, "path", ""))
		for _, v := range strList(tp, "velocity") {
			fmt.Fprintf(e.Out, "    velocity: %s\n", v)
		}
		for _, line := range permissionLineTexts(objList(tp, "java_permissions")) {
			fmt.Fprintf(e.Out, "  %s\n", line)
		}
		for _, g := range objList(tp, "groups") {
			reqs := objList(g, "requests")
			if len(reqs) == 0 {
				fmt.Fprintf(e.Out, "    group %s: no requests\n", getStr(g, "type", ""))
				continue
			}
			fmt.Fprintf(e.Out, "    group %s:\n", getStr(g, "type", ""))
			for _, rq := range reqs {
				fmt.Fprintf(e.Out, "      %s\n", requestLine(rq))
			}
		}
	}
}

// requestLine is one req-permission: name, purpose key, parameters and
// whether the template ships it commented out.
func requestLine(rq *index.Object) string {
	line := fmt.Sprintf("%s  %q", getStr(rq, "name", ""), getStr(rq, "purpose_key", ""))
	var params []string
	for _, p := range objList(rq, "parameters") {
		params = append(params, getStr(p, "name", "")+"="+getStr(p, "value", ""))
	}
	if len(params) > 0 {
		line += "  " + strings.Join(params, " ")
	}
	if c, _ := get(rq, "commented", false).(bool); c {
		line += "  (commented out)"
	}
	return line
}

// moduleSchemas lists the schemas a module ships.
func (e *Env) moduleSchemas(name string, r *index.Object) {
	schemas := objList(r, "schemas")
	fmt.Fprintf(e.Out, "SCHEMAS of %s (%d):\n", name, len(schemas))
	width := 0
	for _, s := range schemas {
		width = max(width, runeLen(getStr(s, "path", "")))
	}
	for _, s := range schemas {
		parts := []string{getStr(s, "kind", "")}
		if root := getStr(s, "root", ""); root != "" {
			parts = append(parts, "root "+root)
		}
		if ns := getStr(s, "target_namespace", ""); ns != "" {
			parts = append(parts, "targetNamespace "+ns)
		}
		n, _ := index.Int(get(s, "elements", nil))
		parts = append(parts, fmt.Sprintf("%d elements", n))
		fmt.Fprintf(e.Out, "  %s  %s\n", pycompat.LJust(getStr(s, "path", ""), width, ' '), strings.Join(parts, "; "))
	}
}
