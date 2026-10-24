package classfile

import (
	"errors"
	"strings"
	"testing"
)

func memberNames(ms []Member) string {
	var s []string
	for _, m := range ms {
		s = append(s, m.Name+" "+m.Descriptor)
	}
	return strings.Join(s, "; ")
}

// Fields and methods keep file order, synthetic ones are dropped, and
// attributes the reader does not know are skipped by their length.
func TestParseMembers(t *testing.T) {
	c := newClass()
	p := c.pool
	c.fields = [][]byte{
		p.member(AccPublic|AccStatic|AccFinal, "in", "Ljavax/baja/sys/Property;", p.attr("ConstantValue", []byte{0, 1})),
		p.member(AccSynthetic|AccFinal, "this$0", "La/O;"),
		p.member(AccPrivate, "x", "I"),
	}
	c.methods = [][]byte{
		p.member(AccPublic, "<init>", "()V", p.attr("Code", make([]byte, 40))),
		p.member(AccPublic|AccSynthetic, "access$000", "()V"),
	}
	c.attrs = [][]byte{p.attr("SourceFile", be2(p.utf8("B.java")))}
	cf, err := Parse(c.bytes())
	if err != nil {
		t.Fatal(err)
	}
	if got := memberNames(cf.Fields); got != "in Ljavax/baja/sys/Property;; x I" {
		t.Errorf("fields = %q", got)
	}
	if got := memberNames(cf.Methods); got != "<init> ()V" {
		t.Errorf("methods = %q", got)
	}
	if cf.Fields[0].Access != AccPublic|AccStatic|AccFinal {
		t.Errorf("access = %#x", cf.Fields[0].Access)
	}
}

// Annotations on the class, a field and a method, visible and invisible, in
// file order, with every element value kind and nested annotation arrays.
func TestParseAnnotations(t *testing.T) {
	c := newClass()
	p := c.pool
	inner := p.ann("La/Inner;", "v", evArray(p.evStr("x"), p.evStr("y")))
	c.attrs = [][]byte{
		p.anns(true, p.ann("La/Vis;")),
		p.anns(false, p.ann("La/Kinds;",
			"s", p.evStr("text"), "i", p.evInt(-7), "z", evConst('Z', p.raw(3, 0, 0, 0, 1)),
			"j", evConst('J', p.raw(5, 0, 0, 0, 1, 0, 0, 0, 0)), "d", evConst('D', p.raw(6, 0x3F, 0xF8, 0, 0, 0, 0, 0, 0)),
			"f", evConst('F', p.raw(4, 0x40, 0x20, 0, 0)), "e", p.evEnum("La/E;", "ON"), "c", p.evClass("Ljava/lang/String;"),
			"nest", evArray(evAnn(inner), evAnn(p.ann("La/Inner;", "v", evArray()))))),
	}
	c.fields = [][]byte{p.member(AccPublic, "f", "I", p.anns(true, p.ann("La/F;")))}
	c.methods = [][]byte{p.member(AccPublic, "m", "()V", p.anns(false, p.ann("La/M;")))}
	cf, err := Parse(c.bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(cf.Annotations) != 2 || cf.Annotations[0].Type != "La/Vis;" || !cf.Annotations[0].Visible ||
		cf.Annotations[1].Visible || cf.Annotations[1].Name() != "Kinds" {
		t.Fatalf("class annotations = %+v", cf.Annotations)
	}
	a := cf.Annotations[1]
	var order []string
	for _, e := range a.Elements {
		order = append(order, e.Name)
	}
	if got := strings.Join(order, ","); got != "s,i,z,j,d,f,e,c,nest" {
		t.Errorf("element order = %q", got)
	}
	get := func(n string) Value { v, _ := a.Get(n); return v }
	if get("s").Str != "text" || get("i").Int != -7 || get("z").Int != 1 || get("j").Int != 1<<32 ||
		get("d").Float != 1.5 || get("f").Float != 2.5 || get("e").EnumType != "La/E;" || get("e").Str != "ON" ||
		get("c").Str != "Ljava/lang/String;" || get("c").Tag != 'c' {
		t.Errorf("values = %+v", a.Elements)
	}
	nest := get("nest").Array
	if len(nest) != 2 || nest[0].Annotation == nil || nest[0].Annotation.Type != "La/Inner;" {
		t.Fatalf("nest = %+v", nest)
	}
	if v, ok := nest[0].Annotation.Get("v"); !ok || len(v.Array) != 2 || v.Array[1].Str != "y" {
		t.Errorf("nested array = %+v", v)
	}
	if v, _ := nest[1].Annotation.Get("v"); v.Tag != '[' || len(v.Array) != 0 {
		t.Errorf("empty nested array = %+v", v)
	}
	if _, ok := a.Get("missing"); ok {
		t.Error("Get found a missing element")
	}
	if cf.Fields[0].Annotations[0].Type != "La/F;" || cf.Methods[0].Annotations[0].Type != "La/M;" {
		t.Errorf("member annotations = %+v / %+v", cf.Fields[0].Annotations, cf.Methods[0].Annotations)
	}
}

func TestParseMemberErrors(t *testing.T) {
	long := newClass() // the attribute claims one byte more than its annotations use
	info := long.pool.anns(true, long.pool.ann("La/A;"))
	info[5]++
	long.attrs = [][]byte{append(info, 0)}

	short := newClass() // the annotations run past the attribute's end
	info = short.pool.anns(true, short.pool.ann("La/A;"))
	info[5]--
	short.attrs = [][]byte{info[:len(info)-1]}

	tag := newClass()
	tag.attrs = [][]byte{tag.pool.anns(true, tag.pool.ann("La/A;", "x", []byte{'?', 0, 1}))}

	trailing := newClass()
	trailing.tail = []byte{0}

	badName := newClass()
	badName.fields = [][]byte{{0, 1, 0, 2, 0, 1, 0, 0}} // name index 2 is a Class

	for name, tc := range map[string]struct {
		c    *class
		want any
	}{
		"attribute longer than its annotations": {long, new(*FormatError)},
		"annotations past the attribute":        {short, new(*TruncatedError)},
		"unknown element_value tag":             {tag, new(*FormatError)},
		"bytes after the class":                 {trailing, new(*FormatError)},
		"member name not a Utf8":                {badName, new(*IndexError)},
	} {
		if _, err := Parse(tc.c.bytes()); err == nil || !errors.As(err, tc.want) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

// Every proper prefix of a class with members and annotations is truncated.
func TestParseMembersTruncated(t *testing.T) {
	c := newClass()
	p := c.pool
	c.fields = [][]byte{p.member(AccPublic, "f", "I", p.anns(true, p.ann("La/A;", "v", evArray(p.evStr("x")))))}
	c.methods = [][]byte{p.member(AccPublic, "m", "()V", p.attr("Code", []byte{1, 2, 3}))}
	c.attrs = [][]byte{p.anns(false, p.ann("La/B;", "n", evAnn(p.ann("La/C;", "e", p.evEnum("La/E;", "X")))))}
	full := c.bytes()
	if _, err := Parse(full); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < len(full); n++ {
		var te *TruncatedError
		if _, err := Parse(full[:n]); !errors.As(err, &te) {
			t.Fatalf("prefix %d: err = %v", n, err)
		}
	}
}
