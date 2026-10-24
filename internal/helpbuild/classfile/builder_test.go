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
// this/super/interfaces and the raw bytes that follow them (members and
// attributes, appended by the tests that need them).
type class struct {
	pool        *pool
	access      uint16
	this, super uint16
	ifaces      []uint16
	tail        []byte
}

// newClass is public class a/B extends java/lang/Object with an empty tail.
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
	b.Write(c.tail)
	return b.Bytes()
}
