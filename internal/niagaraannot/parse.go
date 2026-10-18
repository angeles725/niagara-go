package niagaraannot

import (
	"fmt"
	"math"
	"os"
	"strings"
)

// Annotation is one annotation as written: its simple name (NiagaraProperty also for
// @javax.baja.nre.annotations.NiagaraProperty) and its elements; a single unnamed element is
// "value".
type Annotation struct {
	Name  string
	Line  int
	Elems map[string]Value
}

// Value is an element value: a string (one literal or literals joined with +), a nested
// annotation, an array, or any other expression (Text only).
type Value struct {
	Text    string // the source text, whitespace runs collapsed to one space
	Str     string // IsStr: the string value
	IsStr   bool
	Annot   *Annotation
	Array   []Value
	IsArray bool
	Line    int
}

// Slot kinds.
const (
	Property = "property"
	Action   = "action"
	Topic    = "topic"
)

// Slot is one declared frozen slot.
type Slot struct {
	Kind       string   // Property, Action or Topic
	Name       string   // name
	Type       string   // property: type; action: parameterType; topic: eventType ("" when absent)
	ReturnType string   // action: returnType
	Default    string   // property, action: the defaultValue string ("" when absent)
	Flags      string   // the flags expression as written ("" when absent: no flags)
	Facets     []string // each @Facet: its value, "name=value" when it also has a name
	Override   bool     // override = true
	Line       int
}

// Range is one tag of an enum range: its value and the ordinal written for it. An explicit
// ordinal is kept as written, -1 included (the lonworks enums' *Nul tags).
type Range struct {
	Tag         string
	Ordinal     int    // the int literal; unresolved when OrdinalText is not one (a constant)
	OrdinalText string // the ordinal as written
	HasOrdinal  bool   // false: ordinal omitted, Constants numbers it
}

// unresolved marks an ordinal written as an expression the parser does not evaluate.
const unresolved = math.MinInt

// Enum is a @NiagaraEnum.
type Enum struct {
	Range   []Range
	Default string // defaultValue as written ("" when absent)
	Line    int
}

// Type is what the annotations before a file's top-level type declaration say.
type Type struct {
	Name        string        // the declared class (interface, enum) name
	Line        int           // line of the class keyword
	NiagaraType bool          // has @NiagaraType
	Slots       []Slot        // properties, actions and topics in source order
	Enum        *Enum         // the @NiagaraEnum, nil when absent
	Annotations []*Annotation // every annotation before the declaration, in source order
}

// Slot returns the declared slot named name, or nil.
func (t *Type) Slot(name string) *Slot {
	for i := range t.Slots {
		if t.Slots[i].Name == name {
			return &t.Slots[i]
		}
	}
	return nil
}

// ParseFile reads and parses one Java source file.
func ParseFile(path string) (*Type, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(string(b))
}

// Parse reads the annotations of the top-level type declared in src.
func Parse(src string) (*Type, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{src: src, toks: toks}
	t := &Type{}
	for p.i < len(p.toks) {
		tk := p.toks[p.i]
		if tk.kind == tPunct && tk.text == "@" && !p.isIdent(p.i+1, "interface") {
			a, err := p.annotation()
			if err != nil {
				return nil, err
			}
			t.Annotations = append(t.Annotations, a)
			continue
		}
		if tk.kind == tIdent && (tk.text == "class" || tk.text == "interface" || tk.text == "enum" || tk.text == "record") {
			if p.i+1 < len(p.toks) && p.toks[p.i+1].kind == tIdent {
				t.Name, t.Line = p.toks[p.i+1].text, tk.line
				return t, t.build()
			}
		}
		p.i++
	}
	return nil, fmt.Errorf("no type declaration")
}

type parser struct {
	src  string
	toks []token
	i    int
}

func (p *parser) isIdent(i int, text string) bool {
	return i < len(p.toks) && p.toks[i].kind == tIdent && p.toks[i].text == text
}

func (p *parser) isPunct(i int, text string) bool {
	return i < len(p.toks) && p.toks[i].kind == tPunct && p.toks[i].text == text
}

func (p *parser) errorf(format string, args ...any) error {
	line := 0
	if p.i < len(p.toks) {
		line = p.toks[p.i].line
	} else if len(p.toks) > 0 {
		line = p.toks[len(p.toks)-1].line
	}
	return fmt.Errorf("line %d: %s", line, fmt.Sprintf(format, args...))
}

// annotation parses @Name, @a.b.Name, @Name(value) or @Name(k = v, ...) at p.i (the '@').
func (p *parser) annotation() (*Annotation, error) {
	a := &Annotation{Line: p.toks[p.i].line, Elems: map[string]Value{}}
	p.i++
	for {
		if p.i >= len(p.toks) || p.toks[p.i].kind != tIdent {
			return nil, p.errorf("annotation name expected")
		}
		a.Name = p.toks[p.i].text
		p.i++
		if !p.isPunct(p.i, ".") {
			break
		}
		p.i++
	}
	if !p.isPunct(p.i, "(") {
		return a, nil
	}
	p.i++
	if p.isPunct(p.i, ")") {
		p.i++
		return a, nil
	}
	if p.i+1 < len(p.toks) && p.toks[p.i].kind == tIdent && p.isPunct(p.i+1, "=") && !p.isPunct(p.i+2, "=") {
		for {
			if p.i+1 >= len(p.toks) || p.toks[p.i].kind != tIdent || !p.isPunct(p.i+1, "=") {
				return nil, p.errorf("@%s: element name = value expected", a.Name)
			}
			name := p.toks[p.i].text
			p.i += 2
			v, err := p.value()
			if err != nil {
				return nil, err
			}
			if _, dup := a.Elems[name]; dup {
				return nil, p.errorf("@%s: element %s given twice", a.Name, name)
			}
			a.Elems[name] = v
			if p.isPunct(p.i, ",") {
				p.i++
				continue
			}
			break
		}
	} else {
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		a.Elems["value"] = v
	}
	if !p.isPunct(p.i, ")") {
		return nil, p.errorf("@%s: ')' expected", a.Name)
	}
	p.i++
	return a, nil
}

// value parses an element value: a nested annotation, an array or an expression, ending before
// the ',' ')' or '}' that closes it.
func (p *parser) value() (Value, error) {
	if p.i >= len(p.toks) {
		return Value{}, p.errorf("value expected")
	}
	start := p.toks[p.i]
	switch {
	case p.isPunct(p.i, "@"):
		a, err := p.annotation()
		if err != nil {
			return Value{}, err
		}
		return Value{Text: p.text(start, p.toks[p.i-1]), Annot: a, Line: start.line}, nil
	case p.isPunct(p.i, "{"):
		p.i++
		v := Value{IsArray: true, Line: start.line}
		for !p.isPunct(p.i, "}") {
			e, err := p.value()
			if err != nil {
				return Value{}, err
			}
			v.Array = append(v.Array, e)
			if p.isPunct(p.i, ",") {
				p.i++
			} else if !p.isPunct(p.i, "}") {
				return Value{}, p.errorf("',' or '}' expected in an array")
			}
		}
		p.i++
		v.Text = p.text(start, p.toks[p.i-1])
		return v, nil
	}
	depth, from := 0, p.i
	for p.i < len(p.toks) {
		tk := p.toks[p.i]
		if tk.kind == tPunct {
			switch tk.text {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				if depth == 0 {
					return p.expr(from)
				}
				depth--
			case ",":
				if depth == 0 {
					return p.expr(from)
				}
			}
		}
		p.i++
	}
	return Value{}, p.errorf("unterminated annotation value")
}

// expr is the Value of the tokens from..p.i-1: a string when they are string literals joined
// with +.
func (p *parser) expr(from int) (Value, error) {
	if from == p.i {
		return Value{}, p.errorf("empty annotation value")
	}
	toks := p.toks[from:p.i]
	v := Value{Text: p.text(toks[0], toks[len(toks)-1]), Line: toks[0].line}
	var sb strings.Builder
	v.IsStr = true
	for k, tk := range toks {
		if k%2 == 0 && tk.kind == tString {
			sb.WriteString(tk.text)
		} else if k%2 == 1 && tk.kind == tPunct && tk.text == "+" {
			continue
		} else {
			v.IsStr = false
			break
		}
	}
	if len(toks)%2 == 0 {
		v.IsStr = false
	}
	if v.IsStr {
		v.Str = sb.String()
	}
	return v, nil
}

// text is the source from a to b, whitespace runs collapsed.
func (p *parser) text(a, b token) string {
	return strings.Join(strings.Fields(p.src[a.pos:b.end]), " ")
}
