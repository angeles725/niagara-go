package tagdict

import (
	"errors"
	"fmt"
	"strings"
)

// Niagara is the namespace of the Niagara dictionary (NiagaraTagDictionary).
const Niagara = "n"

// readNiagara reads a tag dictionary bog (tagdictionary-rt's Niagara.bog):
// every component holding tagDefinitions is a dictionary, with its tag,
// tag group and relation definitions and its tag rules.
func readNiagara(data []byte) (*File, error) {
	top, err := parseGraph(data)
	if err != nil {
		return nil, err
	}
	f := &File{Kind: "niagara"}
	found := false
	var walk func(n *node) error
	walk = func(n *node) error {
		if n.kid("tagDefinitions") != nil {
			found = true
			return f.dictionary(n)
		}
		for _, k := range n.kids {
			if err := walk(k); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(top); err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("no tag dictionary: no component holds tagDefinitions")
	}
	return f, nil
}

// dictionary adds the definitions and rules of dictionary d.
func (f *File) dictionary(d *node) error {
	ns := ""
	if v := d.kid("namespace"); v != nil {
		ns = v.value
	} else if d.typ == "tagdictionary:NiagaraTagDictionary" {
		ns = Niagara
	}
	if ns == "" {
		return fmt.Errorf("tag dictionary %q (%s) names no namespace", d.name, d.typ)
	}
	for _, t := range d.kid("tagDefinitions").kidsOrNil() {
		kind := "Marker"
		if v := t.kid("defValue"); v != nil {
			kind = v.typ
		}
		tag := Tag{Dict: ns, Name: t.name, Kind: kind}
		if c := condition(t.kid("validity")); c != "" {
			tag.Validity = []Field{{"condition", c}}
		}
		f.Tags = append(f.Tags, tag)
	}
	for _, g := range d.kid("tagGroupDefinitions").kidsOrNil() {
		f.Groups = append(f.Groups, Group{Dict: ns, Name: g.name, Tags: names(g.kid("tagList"))})
	}
	for _, r := range d.kid("relationDefinitions").kidsOrNil() {
		f.Relations = append(f.Relations, Relation{ns, r.name, r.typ})
	}
	for _, r := range d.kid("tagRules").kidsOrNil() {
		f.Rules = append(f.Rules, Rule{Dict: ns, Name: r.name, Condition: condition(r.kid("condition")),
			Tags: ruleTags(r.kid("tagList")), Groups: names(r.kid("tagGroupList")),
			Relations: ruleTags(r.kid("relationList"))})
	}
	return nil
}

// kid is n's child called name, nil when it has none.
func (n *node) kid(name string) *node {
	if n == nil {
		return nil
	}
	for _, k := range n.kids {
		if k.name == name {
			return k
		}
	}
	return nil
}

// local is the type name without its module.
func (n *node) local() string {
	_, name, _ := strings.Cut(n.typ, ":")
	return name
}

func names(list *node) []string {
	var out []string
	for _, k := range list.kidsOrNil() {
		out = append(out, k.name)
	}
	return out
}

func ruleTags(list *node) []RuleTag {
	var out []RuleTag
	for _, k := range list.kidsOrNil() {
		out = append(out, RuleTag{k.name, k.typ, condition(k.kid("validity"))})
	}
	return out
}

func (n *node) kidsOrNil() []*node {
	if n == nil {
		return nil
	}
	return n.kids
}

// condition is a validity or rule condition as text: "always",
// "isType <type>", "filter <bql>", and/or of its parts in parentheses,
// "not <part>"; another condition is its lower-cased type name followed by
// its properties.
func condition(c *node) string {
	if c == nil {
		return ""
	}
	switch c.local() {
	case "Always":
		return "always"
	case "IsTypeCondition":
		return "isType " + c.kid("objectType").value
	case "BooleanFilter":
		return "filter " + c.kid("filter").value
	case "And", "Or":
		var parts []string
		for _, k := range c.kids {
			parts = append(parts, condition(k))
		}
		return "(" + strings.Join(parts, " "+strings.ToLower(c.local())+" ") + ")"
	case "Not":
		if len(c.kids) > 0 {
			return "not " + condition(c.kids[0])
		}
	}
	name := c.local()
	if name == "" {
		return c.value
	}
	out := []string{strings.ToLower(name[:1]) + name[1:]}
	for _, k := range c.kids {
		if k.typ == "" || k.value != "" {
			out = append(out, k.name+"="+k.value)
		} else {
			out = append(out, k.name+"="+condition(k))
		}
	}
	return strings.Join(out, " ")
}
