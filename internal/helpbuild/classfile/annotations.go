package classfile

import (
	"fmt"
	"strings"
)

// maxNesting bounds how deep annotations and arrays nest inside one
// element value, so a hostile file cannot exhaust the stack.
const maxNesting = 64

// Annotation is one annotation (JVMS §4.7.16). Type is its field descriptor
// (Ljavax/baja/nre/annotations/NiagaraType;); Visible is true for
// RuntimeVisibleAnnotations and false for RuntimeInvisibleAnnotations (CLASS
// retention). Elements keep file order; omitted elements take the
// annotation's declared default, which the class file does not carry.
type Annotation struct {
	Type     string
	Visible  bool
	Elements []Element
}

// Element is one element_value_pair.
type Element struct {
	Name  string
	Value Value
}

// Value is an element_value. Tag is its JVMS tag. B C I J S Z hold Int
// (Z: 0 or 1, C: the UTF-16 unit); D F hold Float; s holds Str; c holds the
// class literal's return descriptor in Str (Ljava/lang/String;, V); e holds
// the enum's type descriptor in EnumType and the constant's name in Str;
// @ holds Annotation; [ holds Array in order.
type Value struct {
	Tag        byte
	Int        int64
	Float      float64
	Str        string
	EnumType   string
	Annotation *Annotation
	Array      []Value
}

// Name is the annotation type's simple name: NiagaraType for
// Ljavax/baja/nre/annotations/NiagaraType;, Outer$Inner for a nested type.
func (a *Annotation) Name() string {
	s := strings.TrimSuffix(strings.TrimPrefix(a.Type, "L"), ";")
	return s[strings.LastIndexByte(s, '/')+1:]
}

// Get returns the value of the element named name, false when the file
// omits it.
func (a *Annotation) Get(name string) (Value, bool) {
	for _, e := range a.Elements {
		if e.Name == name {
			return e.Value, true
		}
	}
	return Value{}, false
}

func parseAnnotations(r *reader, pool Pool, visible bool) []Annotation {
	n := int(r.u2("num_annotations"))
	var out []Annotation
	for i := 0; i < n && r.err == nil; i++ {
		out = append(out, parseAnnotation(r, pool, visible, 0))
	}
	return out
}

func parseAnnotation(r *reader, pool Pool, visible bool, depth int) Annotation {
	a := Annotation{Visible: visible, Type: poolRef(r, pool, TagUtf8, "annotation type_index").Str}
	n := int(r.u2("num_element_value_pairs"))
	for i := 0; i < n && r.err == nil; i++ {
		name := poolRef(r, pool, TagUtf8, "element_name_index").Str
		a.Elements = append(a.Elements, Element{Name: name, Value: parseValue(r, pool, visible, depth)})
	}
	return a
}

// constTags maps a constant element_value tag to the pool entry it names.
var constTags = map[byte]Tag{
	'B': TagInteger, 'C': TagInteger, 'I': TagInteger, 'S': TagInteger, 'Z': TagInteger,
	'J': TagLong, 'D': TagDouble, 'F': TagFloat, 's': TagUtf8,
}

func parseValue(r *reader, pool Pool, visible bool, depth int) Value {
	at := r.pos()
	v := Value{Tag: r.u1("element_value tag")}
	if r.err != nil {
		return v
	}
	if (v.Tag == '@' || v.Tag == '[') && depth >= maxNesting {
		r.fail(&FormatError{Offset: at, Msg: fmt.Sprintf("element values nest deeper than %d", maxNesting)})
		return v
	}
	if want, ok := constTags[v.Tag]; ok {
		c := poolRef(r, pool, want, "const_value_index")
		v.Int, v.Float, v.Str = c.Int, c.Float, c.Str
		return v
	}
	switch v.Tag {
	case 'e':
		v.EnumType = poolRef(r, pool, TagUtf8, "enum type_name_index").Str
		v.Str = poolRef(r, pool, TagUtf8, "enum const_name_index").Str
	case 'c':
		v.Str = poolRef(r, pool, TagUtf8, "class_info_index").Str
	case '@':
		a := parseAnnotation(r, pool, visible, depth+1)
		v.Annotation = &a
	case '[':
		n := int(r.u2("num_values"))
		v.Array = []Value{}
		for i := 0; i < n && r.err == nil; i++ {
			v.Array = append(v.Array, parseValue(r, pool, visible, depth+1))
		}
	default:
		r.fail(&FormatError{Offset: at, Msg: fmt.Sprintf("unknown element_value tag %q", v.Tag)})
	}
	return v
}
