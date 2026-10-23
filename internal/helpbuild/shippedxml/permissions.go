// Package shippedxml reads XML files module jars ship besides module.xml:
// module-permissions.xml templates (the permissions a new module declares,
// as devkit and wbutil hand them out) and the .xsd/.dtd schemas some jars
// carry. It is a leaf: it reads bytes, not jars.
package shippedxml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Permissions is one module-permissions.xml template: its java-permissions
// and permission groups, or, for a Velocity template that only includes
// another, its directives.
type Permissions struct {
	Path            string
	JavaPermissions []JavaPermission
	Groups          []Group
	Velocity        []string
}

// JavaPermission is one <java-permission> under <java-permissions type=profile>.
type JavaPermission struct{ Profile, Class, Name, Action string }

// Group is one <niagara-permission-groups type=...> and its requests.
type Group struct {
	Type     string
	Requests []Request
}

// Request is one <req-permission>; Commented when the template ships it
// inside comments, as an example to uncomment.
type Request struct {
	Name, PurposeKey string
	Parameters       [][2]string
	Commented        bool
}

// IsPermissionsTemplate reports whether a jar member is a module-permissions template.
func IsPermissionsTemplate(member string) bool {
	base := member[strings.LastIndex(member, "/")+1:]
	return base == "module-permissions.xml" || base == "module-permissions.xml.vm"
}

// ParsePermissions reads the template at jar member path.
func ParsePermissions(path string, data []byte) (*Permissions, error) {
	p := &Permissions{Path: path}
	if text := strings.TrimSpace(string(data)); strings.HasSuffix(path, ".vm") && strings.HasPrefix(text, "#") {
		for _, l := range strings.Split(text, "\n") {
			if l = strings.TrimSpace(l); l == "" {
				continue
			}
			if !strings.HasPrefix(l, "#") {
				return nil, fmt.Errorf("line %q of a Velocity template is neither XML nor a directive", l)
			}
			p.Velocity = append(p.Velocity, l)
		}
		return p, nil
	}
	return p, p.read(data, false)
}

// read walks one <permissions> document; commented marks the requests it adds.
func (p *Permissions) read(data []byte, commented bool) error {
	d := xml.NewDecoder(bytes.NewReader(data))
	var stack []string
	var req *Request
	var text strings.Builder
	var comments []byte
	profile, root := "", false
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			a := map[string]string{}
			for _, at := range t.Attr {
				a[at.Name.Local] = at.Value
			}
			name := t.Name.Local
			stack, root = append(stack, name), true
			switch {
			case len(stack) == 1 && name != "permissions":
				return fmt.Errorf("root element is <%s>, not <permissions>", name)
			case name == "java-permissions":
				profile = a["type"]
			case name == "java-permission":
				p.JavaPermissions = append(p.JavaPermissions, JavaPermission{profile, a["class"], a["name"], a["action"]})
			case name == "niagara-permission-groups":
				p.Groups = append(p.Groups, Group{Type: a["type"]})
				comments = comments[:0]
			case name == "req-permission":
				if len(p.Groups) == 0 {
					return errors.New("<req-permission> outside a <niagara-permission-groups>")
				}
				req = &Request{Commented: commented}
			case name == "parameter" && req != nil:
				req.Parameters = append(req.Parameters, [2]string{a["name"], a["value"]})
			}
			text.Reset()
		case xml.CharData:
			text.Write(t)
		case xml.Comment:
			comments = append(comments, t...)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
			g := len(p.Groups) - 1
			switch t.Name.Local {
			case "name":
				if req != nil {
					req.Name = strings.TrimSpace(text.String())
				}
			case "purposeKey":
				if req != nil {
					req.PurposeKey = strings.TrimSpace(text.String())
				}
			case "req-permission":
				p.Groups[g].Requests = append(p.Groups[g].Requests, *req)
				req = nil
			case "niagara-permission-groups":
				if !commented {
					p.commentedRequests(g, comments)
				}
			}
		}
	}
	if !root {
		return errors.New("no root element")
	}
	return nil
}

// commentedRequests adds to group g the requests its comments hold, read as
// one document: the template comments a request out one line at a time.
// Comments that are prose, not XML, add nothing.
func (p *Permissions) commentedRequests(g int, comments []byte) {
	doc := "<permissions><niagara-permission-groups>" + string(comments) + "</niagara-permission-groups></permissions>"
	sub := &Permissions{}
	if sub.read([]byte(doc), true) != nil || len(sub.Groups) != 1 {
		return
	}
	reqs := sub.Groups[0].Requests
	p.Groups[g].Requests = append(reqs, p.Groups[g].Requests...)
}
