package testkit

import (
	"bytes"
	"encoding/binary"
)

// Class is the shape of a Niagara class file ClassBytes writes: its internal
// name (a/b/BFoo), superclass and interfaces, its public methods as
// name+descriptor pairs, the public static final Property fields it declares
// and the @NiagaraProperty slots of its RuntimeInvisibleAnnotations (CLASS
// retention, as javac writes them) after a visible @NiagaraType.
type Class struct {
	Name, Super string
	Interfaces  []string
	Methods     [][2]string
	Fields      []string
	Props       []Prop
}

// Prop is one @NiagaraProperty: name, type, defaultValue and flags (omitted
// when Flags is 0).
type Prop struct {
	Name, Type, Default string
	Flags               int32
}

// classPool is a constant pool under construction.
type classPool struct {
	b    bytes.Buffer
	next uint16
}

func u2(b *bytes.Buffer, v uint16) { b.Write([]byte{byte(v >> 8), byte(v)}) }

func (p *classPool) entry(tag byte, body []byte) uint16 {
	i := p.next
	p.b.WriteByte(tag)
	p.b.Write(body)
	p.next++
	return i
}

func (p *classPool) utf8(s string) uint16 {
	var b bytes.Buffer
	u2(&b, uint16(len(s)))
	b.WriteString(s)
	return p.entry(1, b.Bytes())
}

func (p *classPool) class(name string) uint16 {
	var b bytes.Buffer
	u2(&b, p.utf8(name))
	return p.entry(7, b.Bytes())
}

// annotation encodes an annotation of type desc with string or int elements.
func (p *classPool) annotation(desc string, elems ...any) []byte {
	var b bytes.Buffer
	u2(&b, p.utf8(desc))
	u2(&b, uint16(len(elems)/2))
	for i := 0; i < len(elems); i += 2 {
		u2(&b, p.utf8(elems[i].(string)))
		switch v := elems[i+1].(type) {
		case string:
			b.WriteByte('s')
			u2(&b, p.utf8(v))
		case int32:
			b.WriteByte('I')
			u2(&b, p.entry(3, binary.BigEndian.AppendUint32(nil, uint32(v))))
		}
	}
	return b.Bytes()
}

// attribute encodes an annotations attribute holding anns.
func (p *classPool) attribute(name string, anns ...[]byte) []byte {
	var body bytes.Buffer
	u2(&body, uint16(len(anns)))
	for _, a := range anns {
		body.Write(a)
	}
	var b bytes.Buffer
	u2(&b, p.utf8(name))
	b.Write(binary.BigEndian.AppendUint32(nil, uint32(body.Len())))
	b.Write(body.Bytes())
	return b.Bytes()
}

// ClassBytes is the class file of c (version 52, public).
func ClassBytes(c Class) []byte {
	p := &classPool{next: 1}
	this, super := p.class(c.Name), p.class(c.Super)
	var ifaces []uint16
	for _, i := range c.Interfaces {
		ifaces = append(ifaces, p.class(i))
	}
	member := func(access uint16, name, desc string) []byte {
		var b bytes.Buffer
		u2(&b, access)
		u2(&b, p.utf8(name))
		u2(&b, p.utf8(desc))
		u2(&b, 0)
		return b.Bytes()
	}
	var fields, methods [][]byte
	for _, f := range c.Fields {
		fields = append(fields, member(0x0019, f, "Ljavax/baja/sys/Property;"))
	}
	for _, m := range c.Methods {
		methods = append(methods, member(0x0001, m[0], m[1]))
	}
	var props [][]byte
	for _, s := range c.Props {
		elems := []any{"name", s.Name, "type", s.Type, "defaultValue", s.Default}
		if s.Flags != 0 {
			elems = append(elems, "flags", s.Flags)
		}
		props = append(props, p.annotation("Ljavax/baja/nre/annotations/NiagaraProperty;", elems...))
	}
	attrs := [][]byte{p.attribute("RuntimeVisibleAnnotations", p.annotation("Ljavax/baja/nre/annotations/NiagaraType;"))}
	if len(props) > 0 {
		attrs = append(attrs, p.attribute("RuntimeInvisibleAnnotations", props...))
	}
	var b bytes.Buffer
	b.Write([]byte{0xCA, 0xFE, 0xBA, 0xBE, 0, 0, 0, 52})
	u2(&b, p.next)
	b.Write(p.b.Bytes())
	for _, v := range []uint16{0x0021, this, super, uint16(len(ifaces))} {
		u2(&b, v)
	}
	for _, i := range ifaces {
		u2(&b, i)
	}
	for _, list := range [][][]byte{fields, methods, attrs} {
		u2(&b, uint16(len(list)))
		for _, m := range list {
			b.Write(m)
		}
	}
	return b.Bytes()
}
