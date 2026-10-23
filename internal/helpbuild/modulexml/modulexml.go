// Package modulexml reads the module descriptor every Niagara module jar ships
// (META-INF/module.xml) into the record nh keeps in indexes/module-index.json:
// the module's identity, its dependencies, the types it registers, the agents
// (views and editors) those types register on other types, its java-permissions,
// its defs and its adapters. It is a leaf: it reads bytes, not jars.
//
// The reader walks the elements wherever they sit, so an element indented or
// nested off its usual place still counts; a document that is not well-formed
// XML, whose root is not a named <module>, or that puts an <agent> outside a
// <type> (or an <on> outside an <agent>) is an error, never an empty module.
package modulexml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
)

// Member is the path of the descriptor inside a module jar.
const Member = "META-INF/module.xml"

// Module is one module descriptor. Each list keeps document order.
type Module struct {
	Name, Part, Profile, Vendor, Version, Description string

	Dependencies    []Dependency
	Types           []Type
	Agents          []Agent
	JavaPermissions []JavaPermission
	Defs            []Def
	Adapters        []Adapter
}

// Dependency is one <dependency>: the module required and its minimum version.
type Dependency struct{ Name, Vendor, Version string }

// Type is one <type>: the type name registered in this module and its class.
type Type struct{ Name, Class string }

// Agent is one <agent> of a type: the agent type (a view, editor or type
// extension), its class and the "module:Type" names it is an agent on.
type Agent struct {
	Type, Class         string
	On                  []string
	App, Default, Perms string
}

// JavaPermission is one <java-permission> under <java-permissions type=profile>.
type JavaPermission struct{ Profile, Class, Name, Action string }

// Def is one <def name value>.
type Def struct{ Name, Value string }

// Adapter is one <adapter from to> of a type.
type Adapter struct{ Type, From, To string }

// Counts are how many of each indexed element a descriptor holds.
type Counts struct{ Dependencies, Types, Agents, JavaPermissions, Defs, Adapters int }

// Counts counts the elements of m.
func (m *Module) Counts() Counts {
	return Counts{len(m.Dependencies), len(m.Types), len(m.Agents), len(m.JavaPermissions), len(m.Defs), len(m.Adapters)}
}

func attrs(se xml.StartElement) map[string]string {
	out := make(map[string]string, len(se.Attr))
	for _, a := range se.Attr {
		out[a.Name.Local] = a.Value
	}
	return out
}

// parser holds the elements open while the document is walked: the <type>
// and <agent> an element may belong to (-1 when none) and the profile of the
// open <java-permissions>.
type parser struct {
	m          *Module
	typ, agent int
	profile    string
}

// Parse reads one module.xml document.
func Parse(data []byte) (*Module, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	p := parser{typ: -1, agent: -1}
	for depth := 0; ; {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.EndElement:
			depth--
			switch t.Name.Local {
			case "type":
				p.typ = -1
			case "agent":
				p.agent = -1
			case "java-permissions":
				p.profile = ""
			}
		case xml.StartElement:
			depth++
			if err := p.start(depth, t.Name.Local, attrs(t)); err != nil {
				return nil, err
			}
		}
	}
	if p.m == nil {
		return nil, errors.New("no root element")
	}
	return p.m, nil
}

// start records one element at depth (1 is the root).
func (p *parser) start(depth int, name string, a map[string]string) error {
	if depth == 1 {
		if name != "module" {
			return fmt.Errorf("root element is <%s>, not <module>", name)
		}
		if a["name"] == "" {
			return errors.New("<module> has no name")
		}
		p.m = &Module{Name: a["name"], Part: a["moduleName"], Profile: a["runtimeProfile"],
			Vendor: a["vendor"], Version: a["vendorVersion"], Description: a["description"]}
		return nil
	}
	m := p.m
	switch name {
	case "dependency":
		m.Dependencies = append(m.Dependencies, Dependency{a["name"], a["vendor"], a["vendorVersion"]})
	case "type":
		m.Types = append(m.Types, Type{a["name"], a["class"]})
		p.typ = len(m.Types) - 1
	case "agent":
		if p.typ < 0 {
			return errors.New("<agent> outside a <type>")
		}
		perms := a["requiredPermissions"]
		if perms == "" {
			perms = a["requiredPermission"]
		}
		t := m.Types[p.typ]
		m.Agents = append(m.Agents, Agent{Type: t.Name, Class: t.Class, App: a["app"], Default: a["default"], Perms: perms})
		p.agent = len(m.Agents) - 1
	case "on":
		if p.agent < 0 {
			return errors.New("<on> outside an <agent>")
		}
		m.Agents[p.agent].On = append(m.Agents[p.agent].On, a["type"])
	case "adapter":
		if p.typ < 0 {
			return errors.New("<adapter> outside a <type>")
		}
		m.Adapters = append(m.Adapters, Adapter{m.Types[p.typ].Name, a["from"], a["to"]})
	case "def":
		m.Defs = append(m.Defs, Def{a["name"], a["value"]})
	case "java-permissions":
		p.profile = a["type"]
	case "java-permission":
		m.JavaPermissions = append(m.JavaPermissions, JavaPermission{p.profile, a["class"], a["name"], a["action"]})
	}
	return nil
}
