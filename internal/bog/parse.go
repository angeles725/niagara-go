package bog

import (
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// frameKind is the kind of an open element on the parser stack.
type frameKind uint8

const (
	frameComp frameKind = iota
	frameLink
	frameOther
)

type frame struct {
	kind      frameKind
	path      string
	comp      *Comp // frameComp
	slotOwner *Comp // frameOther of a composite property: the component that owns the slot
	slotName  string
}

// platformSlots are not recorded as value slots (bognav/model.py _PLATFORM_SLOTS).
var platformSlots = map[string]bool{"wsAnnotation": true, "value": true, "status": true, "displayName": true}

var writableTypes = map[string]bool{
	"BooleanWritable": true, "NumericWritable": true, "FloatWritable": true, "IntegerWritable": true,
}

// Parse builds the graph of a BOG XML text, as bognav/model.py Bog(path, lines=text.splitlines())
// does: a tolerant, line-oriented tag reader, so a truncated or malformed document yields a
// smaller graph rather than an error. srcPath is only recorded. Text that is not valid UTF-8 is
// first decoded as ReadText does.
func Parse(srcPath, text string) *Bog {
	if !utf8.ValidString(text) {
		text = pycompat.DecodeReplace([]byte(text))
	}
	p := parser{b: &Bog{SrcPath: srcPath}}
	sc := tagScanner{s: text}
	for {
		t, ok := sc.next()
		if !ok {
			break
		}
		p.tag(t)
	}
	return p.b
}

type parser struct {
	b      *Bog
	stack  []frame
	inLink bool
	link   *Link
}

func (p *parser) top() *frame {
	if len(p.stack) == 0 {
		return nil
	}
	return &p.stack[len(p.stack)-1]
}

func (p *parser) topPath() string {
	if f := p.top(); f != nil {
		return f.path
	}
	return ""
}

func (p *parser) nearestComp() *Comp {
	for i := len(p.stack) - 1; i >= 0; i-- {
		if p.stack[i].kind == frameComp {
			return p.stack[i].comp
		}
	}
	return nil
}

func (p *parser) push(f frame) { p.stack = append(p.stack, f) }

func optional(s string, ok bool) *string {
	if !ok {
		return nil
	}
	return &s
}

func (p *parser) tag(t tag) {
	if t.closing {
		if (t.name == "p" || t.name == "a") && len(p.stack) > 0 {
			popped := p.stack[len(p.stack)-1]
			p.stack = p.stack[:len(p.stack)-1]
			if popped.kind == frameLink {
				p.b.Links = append(p.b.Links, p.link)
				p.inLink = false
				p.link = nil
			}
		}
		return
	}
	switch t.name {
	case "a":
		n, _ := attr(t.full, attrN)
		f, _ := attr(t.full, attrF)
		if c := p.nearestComp(); c != nil {
			c.Actions.Set(n, f)
		}
	case "p":
		p.property(t)
	}
}

func (p *parser) property(t tag) {
	rawN, _ := attr(t.full, attrN)
	n := UnescapeName(rawN)
	h, hasH := attr(t.full, attrH)
	typ, _ := attr(t.full, attrT)
	v, hasV := attr(t.full, attrV)
	m, _ := attr(t.full, attrM)
	flags, _ := attr(t.full, attrF)

	for _, part := range pycompat.Fields(m) {
		if k, mod, ok := strings.Cut(part, "="); ok {
			p.b.Prefixes.Set(k, mod)
		}
	}
	if f := p.top(); f != nil && f.kind == frameComp {
		if f.comp.Present == nil {
			f.comp.Present = map[string]struct{}{}
		}
		f.comp.Present[n] = struct{}{}
	}

	if p.inLink {
		p.linkChild(t, n, v, hasV, hasH)
		return
	}
	if isLinkType(typ) && !t.selfClose {
		c := p.nearestComp()
		p.inLink = true
		p.link = &Link{LinkName: n}
		if c != nil {
			p.link.ContainerH = &c.Handle
			p.link.ContainerPath = c.Path
		}
		p.push(frame{kind: frameLink, path: p.topPath()})
		return
	}
	if hasH {
		p.component(t, n, h, typ)
		return
	}
	if n == "fallback" {
		if c := p.nearestComp(); c != nil {
			c.HasFallback = true
		}
		if !t.selfClose {
			p.push(frame{kind: frameOther, path: p.topPath() + "/fallback"})
		}
		return
	}
	if t.selfClose && (hasV || flags != "") {
		p.valueSlot(n, typ, v, hasV, flags)
		return
	}
	if !t.selfClose {
		// composite property (no handle), e.g. a setpoint StatusNumeric: a value slot on the
		// component, and a frame tagged with its owner so the nested value/status is captured.
		parent := p.top()
		var owner *Comp
		if parent != nil && parent.kind == frameComp && !platformSlots[n] {
			parent.comp.Slots.Set(n, &Slot{Type: typ, Flags: flags})
			owner = parent.comp
		}
		ppath := p.topPath()
		path := ppath
		if n != "" {
			path = strings.TrimLeft(ppath+"/"+n, "/")
		}
		p.push(frame{kind: frameOther, path: path, slotOwner: owner, slotName: n})
	}
}

// isLinkType is _LINK_TYPES: the type ends in ":Link" or ":ConversionLink".
func isLinkType(t string) bool {
	return strings.HasSuffix(t, ":Link") || strings.HasSuffix(t, ":ConversionLink")
}

func (p *parser) linkChild(t tag, n, v string, hasV, hasH bool) {
	if t.selfClose {
		switch {
		case n == "sourceOrd" && v != "" && strings.HasPrefix(v, "h:"):
			src := v[2:]
			p.link.SrcH = &src
		case n == "sourceOrd" && v != "":
			p.link.SrcOrd = optional(v, hasV)
		case n == "sourceSlotName" && v != "":
			s := UnescapeName(v)
			p.link.SrcSlot = &s
		case n == "targetSlotName" && v != "":
			s := UnescapeName(v)
			p.link.TgtSlot = &s
		}
		return
	}
	if !hasH {
		p.push(frame{kind: frameOther, path: p.topPath()})
	}
}

func (p *parser) component(t tag, n, h, typ string) {
	pfx := ""
	if i := strings.IndexByte(typ, ':'); i >= 0 {
		pfx = typ[:i]
	}
	module, _ := p.b.Prefixes.Get(pfx)
	parentComp := p.nearestComp()
	var parentH *string
	if parentComp != nil {
		parentH = &parentComp.Handle
	}
	path := strings.TrimLeft(p.topPath()+"/"+n, "/")

	count, _ := p.b.HandleCount.Get(h)
	p.b.HandleCount.Set(h, count+1)
	c := &Comp{Name: n, Handle: h, Type: typ, Pfx: pfx, Module: module, Path: path, ParentH: parentH}
	c.IsWritable = writableTypes[c.SimpleType()]
	p.b.Comps.Set(h, c)
	if parentH == nil {
		p.b.Roots = append(p.b.Roots, h)
	} else if parent, ok := p.b.Comps.Get(*parentH); ok {
		parent.Children = append(parent.Children, h)
	}
	if !t.selfClose {
		p.push(frame{kind: frameComp, path: path, comp: c})
	}
}

func (p *parser) valueSlot(n, typ, v string, hasV bool, flags string) {
	parent := p.top()
	if parent == nil {
		return
	}
	switch {
	case parent.kind == frameComp && !platformSlots[n]:
		parent.comp.Slots.Set(n, &Slot{Type: typ, Value: optional(v, hasV), Flags: flags})
	case parent.kind == frameComp && n == "value" && typ != "":
		// typed `value` wrapper (bac:BacnetBinaryPv, b:Float...): the point payload of a leaf.
		parent.comp.ValueType = typ
		parent.comp.ValueVal = optional(v, hasV)
	case parent.kind == frameOther && parent.slotOwner != nil && (n == "value" || n == "status"):
		// nested struct child of a composite property (StatusNumeric.value): surfaced on the slot.
		slot, ok := parent.slotOwner.Slots.Get(parent.slotName)
		if ok && hasV {
			slot.Child.Set(n, v)
			if n == "value" {
				slot.Value = &v
			}
		}
	}
}
