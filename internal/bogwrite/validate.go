package bogwrite

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/bog"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyxml"
)

func errorf(format string, a ...any) finding { return finding{"ERROR", fmt.Sprintf(format, a...)} }

// validateXML is validate_xml over the document bytes.
func validateXML(data []byte) []finding {
	root, err := pyxml.Parse(data, pyxml.Mode{ETree: true})
	if err != nil {
		return []finding{errorf("XML not well-formed: %s", err)}
	}
	v := &validator{handles: map[string]string{}, declared: map[string]string{"b": "baja"}}
	if root.Tag != "bajaObjectGraph" {
		v.add("root element is %s, expected bajaObjectGraph", pycompat.StrRepr(root.Tag))
	}
	for _, c := range root.Children {
		if c.Tag == "p" {
			v.walk(c, orUnknown(c))
		}
	}
	for _, r := range v.refs {
		if _, ok := v.handles[r[0]]; !ok {
			v.add("unresolved sourceOrd h:%s at %s", r[0], r[1])
		}
	}
	return v.findings
}

func orUnknown(e *pyxml.Element) string {
	if n, _ := e.Get("n"); n != "" {
		return n
	}
	return "?"
}

type validator struct {
	findings []finding
	handles  map[string]string // handle -> first path
	declared map[string]string
	refs     [][2]string // (handle, link path)
}

func (v *validator) add(format string, a ...any) {
	v.findings = append(v.findings, errorf(format, a...))
}

func (v *validator) walk(el *pyxml.Element, path string) {
	m, _ := el.Get("m")
	for _, decl := range pycompat.Fields(m) {
		if a, b, ok := strings.Cut(decl, "="); ok {
			v.declared[pycompat.Strip(a)] = pycompat.Strip(b)
		}
	}
	if h, _ := el.Get("h"); h != "" {
		if first, dup := v.handles[h]; dup {
			v.add("duplicate handle h=%s at %s (also %s)", h, path, first)
		} else {
			v.handles[h] = path
		}
	}
	t, _ := el.Get("t")
	n, _ := el.Get("n")
	if n != "" && !slotNameOK(n) {
		v.add("illegal slot name %s at %s", pycompat.StrRepr(n), path)
	}
	if pfx, _, ok := strings.Cut(t, ":"); ok {
		if _, known := v.declared[pfx]; !known {
			v.add("type prefix %s used before declared at %s", pycompat.StrRepr(pfx), path)
		}
	}
	if lower := strings.ToLower(t); strings.HasSuffix(lower, "password") {
		v.add("password-typed value at %s (secrets must never be generated)", path)
	}
	if strings.HasSuffix(t, ":Link") || strings.HasSuffix(t, ":ConversionLink") {
		if f, _ := el.Get("f"); strings.Contains(f, "t") {
			v.add("transient link flag at %s (links must be persistent, B1168 D2)", path)
		}
		kids := map[string]string{}
		for _, c := range el.Children {
			if c.Tag == "p" {
				if cn, ok := c.Get("n"); ok {
					kids[cn], _ = c.Get("v")
				}
			}
		}
		for _, req := range []string{"sourceOrd", "sourceSlotName", "targetSlotName"} {
			if kids[req] == "" {
				v.add("link at %s missing child %s", path, req)
			}
		}
		if so := kids["sourceOrd"]; strings.HasPrefix(so, "h:") {
			v.refs = append(v.refs, [2]string{so[2:], path})
		}
	}
	if n == "wsAnnotation" {
		if val, _ := el.Get("v"); !wsOK(val) {
			v.add("bad wsAnnotation value %s at %s (expected x,y,w[,h])", pycompat.StrRepr(val), path)
		}
	}
	for _, c := range el.Children {
		if c.Tag == "p" {
			v.walk(c, path+"/"+orUnknown(c))
		}
	}
}

// wsOK is re.match(r"^-?\d+,-?\d+,\d+(,\d+)?(\|.*)?$", v) with Unicode \d (str.isdecimal);
// '.' stops at a newline and $ also matches before one final newline.
func wsOK(s string) bool {
	rs := []rune(s)
	i := 0
	num := func(signed bool) bool {
		if signed && i < len(rs) && rs[i] == '-' {
			i++
		}
		start := i
		for i < len(rs) && pycompat.IsDecimal(rs[i]) {
			i++
		}
		return i > start
	}
	comma := func() bool {
		if i < len(rs) && rs[i] == ',' {
			i++
			return true
		}
		return false
	}
	if !num(true) || !comma() || !num(true) || !comma() || !num(false) {
		return false
	}
	if save := i; comma() && !num(false) {
		i = save
	}
	if i < len(rs) && rs[i] == '|' {
		for i++; i < len(rs) && rs[i] != '\n'; i++ {
		}
	}
	return i == len(rs) || i == len(rs)-1 && rs[i] == '\n'
}

// validatePath is validate_path: an unreadable input is one ERROR finding.
func validatePath(path string) []finding {
	data, err := bog.ReadBytes(path)
	if err != nil {
		return []finding{{"ERROR", err.Error()}}
	}
	return validateXML(data)
}
