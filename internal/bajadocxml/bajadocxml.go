package bajadocxml

// Reader for Niagara .bajadoc XML (bajadoc_xml.py): the class and member
// model class --jdk renders and selftest --recall takes its ground truth
// from. Elements follow xml.etree.ElementTree: an element's text is the
// character data before its first child, comments and processing
// instructions dropped.

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
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
	d.CharsetReader = charsetReader
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

// charsetReader decodes the two encodings expat knows besides UTF-8 and
// UTF-16, as ET.fromstring does: ISO-8859-1 (every byte is its code point)
// and US-ASCII (a byte above 0x7F is not well-formed). Any other name stays
// an error, so such a document is unparseable (Python decodes a single-byte
// codec it knows and raises LookupError for an unknown name).
func charsetReader(label string, in io.Reader) (io.Reader, error) {
	enc := strings.ToLower(label)
	if enc != "iso-8859-1" && enc != "us-ascii" {
		return nil, fmt.Errorf("unknown encoding %q", label)
	}
	raw, err := io.ReadAll(in)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(raw))
	for _, b := range raw {
		if b >= utf8.RuneSelf && enc == "us-ascii" {
			return nil, errors.New("not well-formed (invalid token)")
		}
		out = utf8.AppendRune(out, rune(b))
	}
	return bytes.NewReader(out), nil
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

// Member is the member dict of _member.
type Member struct {
	Kind, Name     string
	Modifiers      []string
	Params         [][2]string // (type, name)
	Ret, Typ, Desc string
}

// Class is the class dict of parse_class.
type Class struct {
	QualifiedName, Name, Pkg, Module, Kind string
	Modifiers                              []string
	Extends                                string
	Implements                             []string
	Since, Desc                            string
	Members                                []Member
	File                                   string // iter_classes: the .bajadoc it came from
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
	return strings.Join(textfold.Fields(markupTag.ReplaceAllString(*el.text, "")), " ")
}

func parseMember(el *xmlElem, kind string) Member {
	m := Member{Kind: kind, Name: el.get("name", ""), Modifiers: xmlMods(el),
		Desc: xmlText(el.find("description"))}
	for _, p := range el.findAll("parameter") {
		t := renderType(firstType(p))
		if p.attrs["isVarargs"] == "true" && strings.HasSuffix(t, "[]") {
			t = t[:len(t)-2] + "..."
		}
		m.Params = append(m.Params, [2]string{t, p.get("name", "")})
	}
	if ret := el.find("return"); ret != nil {
		m.Ret = renderType(firstType(ret))
	}
	switch kind {
	case "topic":
		m.Typ = renderType(firstType(el.find("eventType")))
	case "field", "property":
		m.Typ = renderType(firstType(el))
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
func parseBajadocClass(el *xmlElem) Class {
	c := Class{QualifiedName: el.get("qualifiedName", ""), Name: el.get("name", ""),
		Pkg: el.get("packageName", ""), Kind: classKind(el), Modifiers: xmlMods(el),
		Desc: xmlText(el.find("description"))}
	for _, tag := range el.findAll("tag") {
		if tag.attrs["name"] == "@since" {
			c.Since = xmlText(tag)
		}
	}
	c.Module = el.get("module", "")
	if profile := el.get("runtimeProfile", ""); profile != "" {
		c.Module += "-" + profile
	}
	if ext := el.find("extends"); ext != nil {
		c.Extends = renderType(firstType(ext))
	}
	c.Implements = []string{}
	if impl := el.find("implements"); impl != nil {
		for _, t := range impl.children {
			c.Implements = append(c.Implements, renderType(t))
		}
	}
	for _, child := range el.children {
		switch child.tag {
		case "method", "constructor", "field", "property", "action", "topic":
			c.Members = append(c.Members, parseMember(child, child.tag))
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

// ParseFile is parse_file: the classes of one .bajadoc file (none
// for an index file). ok is false when the file stayed unparseable after
// sanitizing (a PARSE_FAILURES entry) or could not be read.
func ParseFile(path string) (classes []Class, ok bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return Parse(data)
}

// Parse is parse_file on the bytes of a file already read (a builder that
// stops on a read error, as ET.parse raises it, reads the file itself): ok
// is false when the document stayed unparseable after sanitizing.
func Parse(data []byte) (classes []Class, ok bool) {
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

// Simple is _simple for the builders (the S9 slots index types its
// .bajadoc slots with it).
func Simple(name string) string { return simpleType(name) }

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

// CleanInfo is the dict of to_clean_info that format_output renders.
// Subclasses is the caller's to fill (to_clean_info leaves it empty;
// bajadoc_xml_parser.build derives it from the extends relation).
type CleanInfo struct {
	ClassName, Kind, Pkg, Since, Extends, Desc string
	Implements, Fields, Ctors, Methods         []string
	Subclasses                                 []string
}

// ToCleanInfo is to_clean_info.
func ToCleanInfo(c Class) CleanInfo {
	info := CleanInfo{ClassName: c.Name, Kind: c.Kind, Pkg: c.Pkg, Since: c.Since, Desc: c.Desc}
	if c.Extends != "" {
		info.Extends = simpleType(c.Extends)
	}
	for _, i := range c.Implements {
		info.Implements = append(info.Implements, simpleType(i))
	}
	for _, m := range c.Members {
		var mods []string
		for _, x := range m.Modifiers {
			if x == "static" || x == "abstract" {
				mods = append(mods, x)
			}
		}
		prefix := ""
		if len(mods) > 0 {
			prefix = strings.Join(mods, " ") + " "
		}
		ps := make([]string, len(m.Params))
		for i, p := range m.Params {
			ps[i] = simpleType(p[0]) + " " + p[1]
		}
		params := strings.Join(ps, ", ")
		switch m.Kind {
		case "constructor":
			info.Ctors = append(info.Ctors, "  "+m.Name+"("+params+")")
		case "method":
			info.Methods = append(info.Methods, "  "+prefix+simpleType(m.Ret)+" "+m.Name+"("+params+")")
		case "field":
			info.Fields = append(info.Fields, "  "+prefix+simpleType(m.Typ)+" "+m.Name)
		}
	}
	sort.Strings(info.Fields)
	sort.SliceStable(info.Methods, func(i, j int) bool {
		return methodSortKey(info.Methods[i]) < methodSortKey(info.Methods[j])
	})
	return info
}

// methodSortKey is s.split("(")[0].split()[-1]: the method name.
func methodSortKey(s string) string {
	f := textfold.Fields(strings.SplitN(s, "(", 2)[0])
	if len(f) == 0 {
		return ""
	}
	return f[len(f)-1]
}

// FormatCleanInfo is format_output: the bajadoc-clean text of a class.
func FormatCleanInfo(info CleanInfo) string {
	lines := []string{
		strings.ReplaceAll(strings.ToUpper(info.Kind), "_", " ") + ": " + info.ClassName,
		"PACKAGE: " + info.Pkg,
	}
	if info.Since != "" {
		lines = append(lines, "SINCE: "+info.Since)
	}
	if info.Extends != "" {
		lines = append(lines, "EXTENDS: "+info.Extends)
	}
	if len(info.Implements) > 0 {
		lines = append(lines, "IMPLEMENTS: "+strings.Join(info.Implements, ", "))
	}
	if len(info.Subclasses) > 0 {
		lines = append(lines, "SUBCLASSES: "+strings.Join(info.Subclasses, ", "))
	}
	lines = append(lines, "")
	if info.Desc != "" {
		lines = append(lines, "DESCRIPTION:")
		current := ""
		for _, w := range textfold.Fields(info.Desc) {
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
	}{{"FIELDS:", info.Fields}, {"CONSTRUCTORS:", info.Ctors}, {"METHODS:", info.Methods}} {
		if len(sec.items) > 0 {
			lines = append(lines, sec.title)
			lines = append(lines, sec.items...)
			lines = append(lines, "")
		}
	}
	return strings.Join(lines, "\n")
}

// runeLen is len(s) for a Python str.
func runeLen(s string) int { return utf8.RuneCountInString(s) }
