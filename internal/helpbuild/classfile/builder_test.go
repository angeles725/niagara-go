package classfile

import (
	"bytes"
	"encoding/binary"
)

// buf is the byte builder the tests assemble class files with: big-endian
// u1/u2/u4 writes, chained.
type buf struct{ bytes.Buffer }

func (b *buf) u1(v ...byte) *buf { b.Write(v); return b }
func (b *buf) u2(v uint16) *buf  { return b.u1(byte(v>>8), byte(v)) }
func (b *buf) u4(v uint32) *buf  { return b.u1(binary.BigEndian.AppendUint32(nil, v)...) }

// pool builds a constant pool and hands out the index of each entry it adds.
type pool struct {
	b    buf
	next uint16 // the index the next entry gets (1 when empty)
}

func newPool() *pool { return &pool{next: 1} }

// raw adds an entry from its tag and body bytes; wide entries (Long, Double)
// take two indexes.
func (p *pool) raw(tag byte, body ...byte) uint16 {
	i := p.next
	p.b.u1(tag).u1(body...)
	p.next++
	if tag == 5 || tag == 6 {
		p.next++
	}
	return i
}

func be2(v uint16) []byte { return []byte{byte(v >> 8), byte(v)} }

// utf8 adds a Utf8 entry holding the given (already modified-UTF-8) bytes.
func (p *pool) utf8(s string) uint16 {
	return p.raw(1, append(be2(uint16(len(s))), s...)...)
}

func (p *pool) class(name string) uint16 { return p.raw(7, be2(p.utf8(name))...) }

// class is the skeleton of one class file: header, pool, access flags,
// this/super/interfaces, the encoded fields and methods (member), the
// encoded class attributes (attr) and any raw bytes after them.
type class struct {
	pool            *pool
	access          uint16
	this, super     uint16
	ifaces          []uint16
	fields, methods [][]byte
	attrs           [][]byte
	tail            []byte
}

// newClass is public class a/B extends java/lang/Object with no members.
func newClass() *class {
	p := newPool()
	return &class{pool: p, access: 0x0021, this: p.class("a/B"), super: p.class("java/lang/Object")}
}

func (c *class) bytes() []byte {
	var b buf
	b.u4(0xCAFEBABE).u2(0).u2(52).u2(c.pool.next)
	b.Write(c.pool.b.Bytes())
	b.u2(c.access).u2(c.this).u2(c.super).u2(uint16(len(c.ifaces)))
	for _, i := range c.ifaces {
		b.u2(i)
	}
	for _, list := range [][][]byte{c.fields, c.methods, c.attrs} {
		b.u2(uint16(len(list)))
		for _, m := range list {
			b.Write(m)
		}
	}
	b.Write(c.tail)
	return b.Bytes()
}

// attr encodes one attribute named name around info.
func (p *pool) attr(name string, info []byte) []byte {
	var b buf
	b.u2(p.utf8(name)).u4(uint32(len(info))).Write(info)
	return b.Bytes()
}

// member encodes a field or method.
func (p *pool) member(access uint16, name, desc string, attrs ...[]byte) []byte {
	var b buf
	b.u2(access).u2(p.utf8(name)).u2(p.utf8(desc)).u2(uint16(len(attrs)))
	for _, a := range attrs {
		b.Write(a)
	}
	return b.Bytes()
}

// anns encodes a RuntimeVisibleAnnotations or RuntimeInvisibleAnnotations
// attribute holding the encoded annotations.
func (p *pool) anns(visible bool, list ...[]byte) []byte {
	name := "RuntimeInvisibleAnnotations"
	if visible {
		name = "RuntimeVisibleAnnotations"
	}
	var b buf
	b.u2(uint16(len(list)))
	for _, a := range list {
		b.Write(a)
	}
	return p.attr(name, b.Bytes())
}

// ann encodes an annotation of type desc; pairs alternate element name
// (string) and encoded element value ([]byte).
func (p *pool) ann(desc string, pairs ...any) []byte {
	var b buf
	b.u2(p.utf8(desc)).u2(uint16(len(pairs) / 2))
	for i := 0; i < len(pairs); i += 2 {
		b.u2(p.utf8(pairs[i].(string))).Write(pairs[i+1].([]byte))
	}
	return b.Bytes()
}

// The element_value encoders: a constant (tag plus pool index), a string,
// an int, an enum constant, a class literal, a nested annotation, an array.
func evConst(tag byte, idx uint16) []byte { return append([]byte{tag}, be2(idx)...) }
func (p *pool) evStr(s string) []byte     { return evConst('s', p.utf8(s)) }
func (p *pool) evInt(v int32) []byte {
	return evConst('I', p.raw(3, binary.BigEndian.AppendUint32(nil, uint32(v))...))
}
func (p *pool) evEnum(typ, name string) []byte {
	return append(append([]byte{'e'}, be2(p.utf8(typ))...), be2(p.utf8(name))...)
}
func (p *pool) evClass(desc string) []byte { return evConst('c', p.utf8(desc)) }
func evAnn(a []byte) []byte                { return append([]byte{'@'}, a...) }
func evArray(vals ...[]byte) []byte {
	b := append([]byte{'['}, be2(uint16(len(vals)))...)
	for _, v := range vals {
		b = append(b, v...)
	}
	return b
}
