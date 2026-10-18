package niagaraannot

import (
	"fmt"
	"strconv"
)

// slotKinds maps the slot annotations and their containers to the slot kind.
var slotKinds = map[string]string{
	"NiagaraProperty": Property, "NiagaraProperties": Property,
	"NiagaraAction": Action, "NiagaraActions": Action,
	"NiagaraTopic": Topic, "NiagaraTopics": Topic,
}

// containerOf maps each repeatable container to the annotation it holds.
var containerOf = map[string]string{
	"NiagaraProperties": "NiagaraProperty", "NiagaraActions": "NiagaraAction", "NiagaraTopics": "NiagaraTopic",
}

// build fills the slots and the enum from the annotations.
func (t *Type) build() error {
	for _, a := range t.Annotations {
		switch a.Name {
		case "NiagaraType":
			t.NiagaraType = true
		case "NiagaraProperty", "NiagaraAction", "NiagaraTopic":
			if err := t.addSlot(slotKinds[a.Name], a); err != nil {
				return err
			}
		case "NiagaraProperties", "NiagaraActions", "NiagaraTopics":
			if err := t.addSlots(slotKinds[a.Name], containerOf[a.Name], a, "value"); err != nil {
				return err
			}
		case "NiagaraSlots":
			for _, el := range []struct{ elem, kind, name string }{
				{"properties", Property, "NiagaraProperty"},
				{"actions", Action, "NiagaraAction"},
				{"topics", Topic, "NiagaraTopic"},
			} {
				if err := t.addSlots(el.kind, el.name, a, el.elem); err != nil {
					return err
				}
			}
		case "NiagaraEnum":
			if t.Enum != nil {
				return fmt.Errorf("line %d: a second @NiagaraEnum", a.Line)
			}
			e, err := enumOf(a)
			if err != nil {
				return err
			}
			t.Enum = e
		}
	}
	seen := map[string]bool{}
	for _, s := range t.Slots {
		if seen[s.Name] {
			return fmt.Errorf("line %d: slot %s declared twice", s.Line, s.Name)
		}
		seen[s.Name] = true
	}
	return nil
}

// addSlots adds the slot annotations named name in the array (or single annotation) elem of a.
func (t *Type) addSlots(kind, name string, a *Annotation, elem string) error {
	v, ok := a.Elems[elem]
	if !ok {
		return nil
	}
	for _, e := range elements(v) {
		if e.Annot == nil || e.Annot.Name != name {
			return fmt.Errorf("line %d: @%s %s: @%s expected, got %s", e.Line, a.Name, elem, name, e.Text)
		}
		if err := t.addSlot(kind, e.Annot); err != nil {
			return err
		}
	}
	return nil
}

// elements is an array's values, or the one value of a single-element shorthand.
func elements(v Value) []Value {
	if v.IsArray {
		return v.Array
	}
	return []Value{v}
}

// addSlot adds one @NiagaraProperty, @NiagaraAction or @NiagaraTopic.
func (t *Type) addSlot(kind string, a *Annotation) error {
	s := Slot{Kind: kind, Line: a.Line}
	var err error
	str := func(elem string, required bool) string {
		v, ok := a.Elems[elem]
		if !ok {
			if required && err == nil {
				err = fmt.Errorf("line %d: @%s has no %s", a.Line, a.Name, elem)
			}
			return ""
		}
		if !v.IsStr && err == nil {
			err = fmt.Errorf("line %d: @%s %s is not a string literal: %s", v.Line, a.Name, elem, v.Text)
		}
		return v.Str
	}
	s.Name = str("name", true)
	switch kind {
	case Property:
		s.Type = str("type", true)
		s.Default = str("defaultValue", false)
	case Action:
		s.Type = str("parameterType", false)
		s.ReturnType = str("returnType", false)
		s.Default = str("defaultValue", false)
	case Topic:
		s.Type = str("eventType", false)
	}
	if err != nil {
		return err
	}
	if v, ok := a.Elems["flags"]; ok {
		s.Flags = v.Text
	}
	if v, ok := a.Elems["override"]; ok {
		s.Override = v.Text == "true"
	}
	if v, ok := a.Elems["facets"]; ok {
		for _, e := range elements(v) {
			f, err := facet(e)
			if err != nil {
				return err
			}
			s.Facets = append(s.Facets, f)
		}
	}
	t.Slots = append(t.Slots, s)
	return nil
}

// facet is one @Facet as "value", or "name=value" when it names its key.
func facet(v Value) (string, error) {
	if v.Annot == nil || v.Annot.Name != "Facet" {
		return "", fmt.Errorf("line %d: @Facet expected, got %s", v.Line, v.Text)
	}
	val := v.Annot.Elems["value"]
	out := val.Str
	if !val.IsStr {
		out = val.Text
	}
	if n, ok := v.Annot.Elems["name"]; ok {
		name := n.Str
		if !n.IsStr {
			name = n.Text
		}
		out = name + "=" + out
	}
	return out, nil
}

// enumOf reads a @NiagaraEnum: its range of @Range tags and its defaultValue.
func enumOf(a *Annotation) (*Enum, error) {
	e := &Enum{Line: a.Line}
	rv, ok := a.Elems["range"]
	if !ok {
		return nil, fmt.Errorf("line %d: @NiagaraEnum has no range", a.Line)
	}
	for _, v := range elements(rv) {
		if v.Annot == nil || v.Annot.Name != "Range" {
			return nil, fmt.Errorf("line %d: @Range expected, got %s", v.Line, v.Text)
		}
		tag, ok := v.Annot.Elems["value"]
		if !ok || !tag.IsStr {
			return nil, fmt.Errorf("line %d: @Range value is not a string literal", v.Line)
		}
		r := Range{Tag: tag.Str}
		if o, ok := v.Annot.Elems["ordinal"]; ok {
			r.OrdinalText, r.HasOrdinal = o.Text, true
			if n, err := strconv.ParseInt(o.Text, 0, 32); err == nil {
				r.Ordinal = int(n)
			} else {
				r.Ordinal = unresolved
			}
		}
		e.Range = append(e.Range, r)
	}
	if d, ok := a.Elems["defaultValue"]; ok {
		if !d.IsStr {
			return nil, fmt.Errorf("line %d: @NiagaraEnum defaultValue is not a string literal: %s", d.Line, d.Text)
		}
		e.Default = d.Str
	}
	return e, nil
}
