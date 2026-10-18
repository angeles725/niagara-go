package help

// Reader for Niagara .bajadoc XML (bajadoc_xml.py): the class and member
// model class --jdk renders and selftest --recall takes its ground truth
// from. Elements follow xml.etree.ElementTree: an element's text is the
// character data before its first child, comments and processing
// instructions dropped.

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// xmlElem is one parsed element (ElementTree Element).
type xmlElem struct {
	tag      string
	attrs    map[string]string
	text     *string // nil when the element has no text (el.text is None)
	children []*xmlElem
}

// get is el.get(name, def).
func (x *xmlElem) get(name, def string) string {
	if v, ok := x.attrs[name]; ok {
		return v
	}
	return def
}

// find is el.find(tag): the first direct child with that tag, or nil.
func (x *xmlElem) find(tag string) *xmlElem {
	if x == nil {
		return nil
	}
	for _, c := range x.children {
		if c.tag == tag {
			return c
		}
	}
	return nil
}

// findAll is el.findall(tag): every direct child with that tag.
func (x *xmlElem) findAll(tag string) []*xmlElem {
	var out []*xmlElem
	for _, c := range x.children {
		if c.tag == tag {
			out = append(out, c)
		}
	}
	return out
}

// parseXML builds the element tree of one document (ET.fromstring).
func parseXML(data []byte) (*xmlElem, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	var stack []*xmlElem
	var root *xmlElem
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			el := &xmlElem{tag: qname(t.Name), attrs: map[string]string{}}
			for _, a := range t.Attr {
				el.attrs[qname(a.Name)] = a.Value
			}
			if n := len(stack); n > 0 {
				stack[n-1].children = append(stack[n-1].children, el)
			} else if root != nil {
				return nil, errors.New("junk after document element")
			} else {
				root = el
			}
			stack = append(stack, el)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if n := len(stack); n > 0 && len(stack[n-1].children) == 0 {
				s := string(t)
				if prev := stack[n-1].text; prev != nil {
					s = *prev + s
				}
				stack[n-1].text = &s
			}
		}
	}
	if root == nil {
		return nil, errors.New("no element found")
	}
	return root, nil
}

func qname(n xml.Name) string {
	if n.Space != "" {
		return n.Space + ":" + n.Local
	}
	return n.Local
}

// bajadocModifiers is MODIFIERS, in the order a member lists them.
var bajadocModifiers = []string{"public", "protected", "private", "abstract", "static", "final",
	"synchronized", "native"}

// bajadocMember is the member dict of _member.
type bajadocMember struct {
	kind, name     string
	modifiers      []string
	params         [][2]string // (type, name)
	ret, typ, desc string
}

// bajadocClass is the class dict of parse_class.
type bajadocClass struct {
	qualifiedName, name, pkg, module, kind string
	modifiers                              []string
	extends                                string
	implements                             []string
	since, desc                            string
	members                                []bajadocMember
	file                                   string // iter_classes: the .bajadoc it came from
}

// renderType is render_type.
func renderType(el *xmlElem) string {
	if el == nil {
		return ""
	}
	switch el.tag {
	case "type":
		dim, err := pycompat.ParseInt(orDefault(el.get("dimension", "0"), "0"))
		if err != nil {
			dim = 0
		}
		return el.get("class", "?") + strings.Repeat("[]", int(max(dim, 0)))
	case "parameterizedType":
		var inner []string
		if args := el.find("args"); args != nil {
			for _, a := range args.children {
				inner = append(inner, renderType(a))
			}
		}
		return el.get("class", "?") + "<" + strings.Join(inner, ", ") + ">"
	case "typeVariable":
		if c := el.get("class", ""); c != "" {
			return c
		}
		return el.get("name", "?")
	case "wildcardType":
		if b := el.find("bounds"); b != nil && len(b.children) > 0 {
			return "? extends " + renderType(b.children[0])
		}
		return "?"
	}
	return el.get("class", el.tag)
}

// orDefault is `s or def`.
func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// firstType is _first_type: the first child that is a type element.
func firstType(el *xmlElem) *xmlElem {
	if el == nil {
		return nil
	}
	for _, c := range el.children {
		switch c.tag {
		case "type", "parameterizedType", "typeVariable", "wildcardType":
			return c
		}
	}
	return nil
}

func xmlMods(el *xmlElem) []string {
	var out []string
	for _, m := range bajadocModifiers {
		if el.attrs[m] == "true" {
			out = append(out, m)
		}
	}
	return out
}

var markupTag = regexp.MustCompile(`<[^>]+>`)

// xmlText is _text: the element text without markup, whitespace collapsed.
func xmlText(el *xmlElem) string {
	if el == nil || el.text == nil {
		return ""
	}
	return strings.Join(pycompat.Fields(markupTag.ReplaceAllString(*el.text, "")), " ")
}

func parseMember(el *xmlElem, kind string) bajadocMember {
	m := bajadocMember{kind: kind, name: el.get("name", ""), modifiers: xmlMods(el),
		desc: xmlText(el.find("description"))}
	for _, p := range el.findAll("parameter") {
		t := renderType(firstType(p))
		if p.attrs["isVarargs"] == "true" && strings.HasSuffix(t, "[]") {
			t = t[:len(t)-2] + "..."
		}
		m.params = append(m.params, [2]string{t, p.get("name", "")})
	}
	if ret := el.find("return"); ret != nil {
		m.ret = renderType(firstType(ret))
	}
	switch kind {
	case "topic":
		m.typ = renderType(firstType(el.find("eventType")))
	case "field", "property":
		m.typ = renderType(firstType(el))
	}
	return m
}

func classKind(el *xmlElem) string {
	category := el.get("category", "")
	switch {
	case el.attrs["enum"] == "true" || category == "enum":
		return "enum"
	case el.attrs["annotation"] == "true" || category == "annotation":
		return "annotation_type"
	case el.attrs["interface"] == "true":
		return "interface"
	}
	return "class"
}

// parseBajadocClass is parse_class.
func parseBajadocClass(el *xmlElem) bajadocClass {
	c := bajadocClass{qualifiedName: el.get("qualifiedName", ""), name: el.get("name", ""),
		pkg: el.get("packageName", ""), kind: classKind(el), modifiers: xmlMods(el),
		desc: xmlText(el.find("description"))}
	for _, tag := range el.findAll("tag") {
		if tag.attrs["name"] == "@since" {
			c.since = xmlText(tag)
		}
	}
	c.module = el.get("module", "")
	if profile := el.get("runtimeProfile", ""); profile != "" {
		c.module += "-" + profile
	}
	if ext := el.find("extends"); ext != nil {
		c.extends = renderType(firstType(ext))
	}
	c.implements = []string{}
	if impl := el.find("implements"); impl != nil {
		for _, t := range impl.children {
			c.implements = append(c.implements, renderType(t))
		}
	}
	for _, child := range el.children {
		switch child.tag {
		case "method", "constructor", "field", "property", "action", "topic":
			c.members = append(c.members, parseMember(child, child.tag))
		}
	}
	return c
}

var charRef = regexp.MustCompile(`&#(x[0-9a-fA-F]+|[0-9]+);`)

func xmlChar(cp int64) bool {
	return cp == 9 || cp == 10 || cp == 13 || (0x20 <= cp && cp <= 0xD7FF) ||
		(0xE000 <= cp && cp <= 0xFFFD) || (0x10000 <= cp && cp <= 0x10FFFF)
}

// sanitizeXML is _sanitize: character references XML forbids become '?'.
func sanitizeXML(data []byte) []byte {
	return charRef.ReplaceAllFunc(data, func(m []byte) []byte {
		ref := string(m[2 : len(m)-1])
		var cp int64
		var err error
		if ref[0] == 'x' || ref[0] == 'X' {
			cp, err = strconv.ParseInt(ref[1:], 16, 64)
		} else {
			cp, err = strconv.ParseInt(ref, 10, 64)
		}
		if err == nil && xmlChar(cp) {
			return m
		}
		return []byte("?")
	})
}

// parseBajadocFile is parse_file: the classes of one .bajadoc file (none
// for an index file). ok is false when the file stayed unparseable after
// sanitizing (a PARSE_FAILURES entry).
func parseBajadocFile(path string) (classes []bajadocClass, ok bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	root, err := parseXML(data)
	if err != nil {
		if root, err = parseXML(sanitizeXML(data)); err != nil {
			return nil, false
		}
	}
	for _, el := range root.findAll("class") {
		if el.get("qualifiedName", "") != "" {
			classes = append(classes, parseBajadocClass(el))
		}
	}
	return classes, true
}

// simpleType is _simple: javax.baja.* and com.tridium.* names shortened to
// their simple name, java.* kept qualified like the javadoc text.
func simpleType(name string) string {
	parts := strings.Split(strings.ReplaceAll(name, ",", ", "), " ")
	for i, p := range parts {
		if (strings.HasPrefix(p, "javax.baja.") || strings.HasPrefix(p, "com.tridium.")) &&
			!strings.Contains(p, "<") {
			parts[i] = p[strings.LastIndex(p, ".")+1:]
		}
	}
	return strings.ReplaceAll(strings.Join(parts, " "), ",  ", ", ")
}

// cleanInfo is the dict of to_clean_info that format_output renders.
type cleanInfo struct {
	className, kind, pkg, since, extends, desc string
	implements, fields, ctors, methods         []string
}

// toCleanInfo is to_clean_info.
func toCleanInfo(c bajadocClass) cleanInfo {
	info := cleanInfo{className: c.name, kind: c.kind, pkg: c.pkg, since: c.since, desc: c.desc}
	if c.extends != "" {
		info.extends = simpleType(c.extends)
	}
	for _, i := range c.implements {
		info.implements = append(info.implements, simpleType(i))
	}
	for _, m := range c.members {
		var mods []string
		for _, x := range m.modifiers {
			if x == "static" || x == "abstract" {
				mods = append(mods, x)
			}
		}
		prefix := ""
		if len(mods) > 0 {
			prefix = strings.Join(mods, " ") + " "
		}
		ps := make([]string, len(m.params))
		for i, p := range m.params {
			ps[i] = simpleType(p[0]) + " " + p[1]
		}
		params := strings.Join(ps, ", ")
		switch m.kind {
		case "constructor":
			info.ctors = append(info.ctors, "  "+m.name+"("+params+")")
		case "method":
			info.methods = append(info.methods, "  "+prefix+simpleType(m.ret)+" "+m.name+"("+params+")")
		case "field":
			info.fields = append(info.fields, "  "+prefix+simpleType(m.typ)+" "+m.name)
		}
	}
	sort.Strings(info.fields)
	sort.SliceStable(info.methods, func(i, j int) bool {
		return methodSortKey(info.methods[i]) < methodSortKey(info.methods[j])
	})
	return info
}

// methodSortKey is s.split("(")[0].split()[-1]: the method name.
func methodSortKey(s string) string {
	f := pycompat.Fields(strings.SplitN(s, "(", 2)[0])
	if len(f) == 0 {
		return ""
	}
	return f[len(f)-1]
}

// formatCleanInfo is format_output: the bajadoc-clean text of a class.
func formatCleanInfo(info cleanInfo) string {
	lines := []string{
		strings.ReplaceAll(strings.ToUpper(info.kind), "_", " ") + ": " + info.className,
		"PACKAGE: " + info.pkg,
	}
	if info.since != "" {
		lines = append(lines, "SINCE: "+info.since)
	}
	if info.extends != "" {
		lines = append(lines, "EXTENDS: "+info.extends)
	}
	if len(info.implements) > 0 {
		lines = append(lines, "IMPLEMENTS: "+strings.Join(info.implements, ", "))
	}
	lines = append(lines, "")
	if info.desc != "" {
		lines = append(lines, "DESCRIPTION:")
		current := ""
		for _, w := range pycompat.Fields(info.desc) {
			if current != "" && runeLen(current)+1+runeLen(w) > 78 {
				lines = append(lines, current)
				current = w
			} else {
				current = pycompat.Strip(current + " " + w)
			}
		}
		if current != "" {
			lines = append(lines, current)
		}
		lines = append(lines, "")
	}
	for _, sec := range []struct {
		title string
		items []string
	}{{"FIELDS:", info.fields}, {"CONSTRUCTORS:", info.ctors}, {"METHODS:", info.methods}} {
		if len(sec.items) > 0 {
			lines = append(lines, sec.title)
			lines = append(lines, sec.items...)
			lines = append(lines, "")
		}
	}
	return strings.Join(lines, "\n")
}
