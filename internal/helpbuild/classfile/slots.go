package classfile

import (
	"strconv"
	"strings"
)

// Slot kinds, the names internal/niagaraannot uses for the same slots.
const (
	KindProperty = "property"
	KindAction   = "action"
	KindTopic    = "topic"
)

// Slot is one frozen slot a class declares, shaped like niagaraannot.Slot
// so the source and bytecode readings compare field by field. Strings are
// the annotation elements as compiled; Flags is the flags element as a
// decimal number ("" when omitted), since javac folds Flags.SUMMARY | ...
// into a constant. Via is "annotation" for a slot from a Niagara slot
// annotation and "field" for one known only by its public static final
// Property, Action or Topic field (only its Kind and Name are known then).
type Slot struct {
	Kind       string
	Name       string
	Type       string // property: type; action: parameterType; topic: eventType
	ReturnType string // action: returnType
	Default    string // property, action: defaultValue
	Flags      string
	Facets     []string // each @Facet as value, or name=value when it names its key
	Override   bool
	Via        string
}

const nreAnnotations = "Ljavax/baja/nre/annotations/"

// annotationKinds maps each slot annotation to its kind.
var annotationKinds = map[string]string{
	"NiagaraProperty": KindProperty, "NiagaraAction": KindAction, "NiagaraTopic": KindTopic,
}

// containers maps each container annotation to its slot-holding elements.
var containers = map[string][]string{
	"NiagaraProperties": {"value"}, "NiagaraActions": {"value"}, "NiagaraTopics": {"value"},
	"NiagaraSlots": {"properties", "actions", "topics"},
}

// fieldKinds maps a slot field's descriptor to its kind.
var fieldKinds = map[string]string{
	"Ljavax/baja/sys/Property;": KindProperty, "Ljavax/baja/sys/Action;": KindAction, "Ljavax/baja/sys/Topic;": KindTopic,
}

// NiagaraSlots returns the slots cf declares: first those of its Niagara
// slot annotations (single or in a container) in annotation order, then
// those of its public static final Property/Action/Topic fields that no
// annotation names, in field order. Inherited slots are not included.
func NiagaraSlots(cf *ClassFile) []Slot {
	var out []Slot
	seen := map[string]bool{}
	add := func(a *Annotation) {
		if kind, ok := annotationKinds[a.Name()]; ok && isNre(a) {
			s := slotOf(kind, a)
			seen[s.Name] = true
			out = append(out, s)
		}
	}
	for i := range cf.Annotations {
		a := &cf.Annotations[i]
		if !isNre(a) {
			continue
		}
		add(a)
		for _, elem := range containers[a.Name()] {
			v, _ := a.Get(elem)
			for _, item := range v.Array {
				if item.Annotation != nil {
					add(item.Annotation)
				}
			}
		}
	}
	const psf = AccPublic | AccStatic | AccFinal
	for _, f := range cf.Fields {
		if kind, ok := fieldKinds[f.Descriptor]; ok && f.Access&psf == psf && !seen[f.Name] {
			seen[f.Name] = true
			out = append(out, Slot{Kind: kind, Name: f.Name, Via: "field"})
		}
	}
	return out
}

// isNre is an annotation of the javax.baja.nre.annotations package.
func isNre(a *Annotation) bool { return strings.HasPrefix(a.Type, nreAnnotations) }

func slotOf(kind string, a *Annotation) Slot {
	str := func(name string) string { v, _ := a.Get(name); return v.Str }
	s := Slot{Kind: kind, Name: str("name"), Default: str("defaultValue"), Via: "annotation"}
	switch kind {
	case KindProperty:
		s.Type = str("type")
	case KindAction:
		s.Type, s.ReturnType = str("parameterType"), str("returnType")
	case KindTopic:
		s.Type = str("eventType")
	}
	if v, ok := a.Get("flags"); ok {
		s.Flags = strconv.FormatInt(v.Int, 10)
	}
	if v, ok := a.Get("override"); ok {
		s.Override = v.Int != 0
	}
	facets, _ := a.Get("facets")
	for _, f := range facets.Array {
		if f.Annotation == nil {
			continue
		}
		name, _ := f.Annotation.Get("name")
		value, _ := f.Annotation.Get("value")
		if name.Str != "" {
			s.Facets = append(s.Facets, name.Str+"="+value.Str)
		} else {
			s.Facets = append(s.Facets, value.Str)
		}
	}
	return s
}
