package classfile

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestParseHeaderAndInterfaces(t *testing.T) {
	c := newClass()
	c.ifaces = []uint16{c.pool.class("java/io/Serializable"), c.pool.class("a/I")}
	cf, err := Parse(c.bytes())
	if err != nil {
		t.Fatal(err)
	}
	if cf.Major != 52 || cf.Access != 0x0021 || cf.ThisClass != "a/B" || cf.SuperClass != "java/lang/Object" {
		t.Fatalf("header = %d %#x %q %q", cf.Major, cf.Access, cf.ThisClass, cf.SuperClass)
	}
	if got := strings.Join(cf.Interfaces, ","); got != "java/io/Serializable,a/I" {
		t.Fatalf("interfaces = %q", got)
	}
}

func TestParseNoSuperClass(t *testing.T) {
	c := newClass()
	c.super = 0
	cf, err := Parse(c.bytes())
	if err != nil || cf.SuperClass != "" {
		t.Fatalf("super = %q, err %v", cf.SuperClass, err)
	}
}

// Every constant pool tag through Java 21, with Long and Double taking two
// slots so the entry after each lands two indexes on.
func TestParsePoolEveryTag(t *testing.T) {
	c := newClass()
	p := c.pool
	nat := p.raw(12, append(be2(p.utf8("f")), be2(p.utf8("I"))...)...)
	cls := p.class("a/C")
	idx := map[string]uint16{
		"int":    p.raw(3, 0xFF, 0xFF, 0xFF, 0xFB),
		"float":  p.raw(4, 0x3F, 0xC0, 0, 0),
		"long":   p.raw(5, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02),
		"after":  p.utf8("after-long"),
		"double": p.raw(6, 0x40, 0x09, 0x21, 0xFB, 0x54, 0x44, 0x2D, 0x18),
		"string": p.raw(8, be2(p.utf8("s"))...),
		"field":  p.raw(9, append(be2(cls), be2(nat)...)...),
		"method": p.raw(10, append(be2(cls), be2(nat)...)...),
		"imeth":  p.raw(11, append(be2(cls), be2(nat)...)...),
		"handle": p.raw(15, 6, 0, 0),
		"mtype":  p.raw(16, be2(p.utf8("()V"))...),
		"dyn":    p.raw(17, 0, 0, 0, 0),
		"indy":   p.raw(18, 0, 1, 0, 0),
		"module": p.raw(19, be2(p.utf8("m"))...),
		"pkg":    p.raw(20, be2(p.utf8("a/p"))...),
		"mutf8":  p.utf8("a\xC0\x80b\xC3\xA9\xED\xA0\xBD\xED\xB8\x80"),
	}
	cf, err := Parse(c.bytes())
	if err != nil {
		t.Fatal(err)
	}
	pl := cf.Pool
	if v := pl[idx["int"]]; v.Tag != TagInteger || v.Int != -5 {
		t.Errorf("Integer = %+v", v)
	}
	if v := pl[idx["float"]]; v.Tag != TagFloat || v.Float != 1.5 {
		t.Errorf("Float = %+v", v)
	}
	if v := pl[idx["long"]]; v.Tag != TagLong || v.Int != 1<<32|2 {
		t.Errorf("Long = %+v", v)
	}
	if idx["after"] != idx["long"]+2 || pl[idx["long"]+1].Tag != 0 {
		t.Errorf("Long does not take two slots: after=%d long=%d", idx["after"], idx["long"])
	}
	if _, err := pl.Utf8(idx["long"] + 1); err == nil {
		t.Error("the slot after a Long reads as an entry")
	}
	if s, _ := pl.Utf8(idx["after"]); s != "after-long" {
		t.Errorf("entry after the Long = %q", s)
	}
	if v := pl[idx["double"]]; v.Tag != TagDouble || math.Abs(v.Float-math.Pi) > 1e-15 {
		t.Errorf("Double = %+v", v)
	}
	if v := pl[idx["handle"]]; v.Tag != TagMethodHandle || v.Int != 6 {
		t.Errorf("MethodHandle = %+v", v)
	}
	for name, tag := range map[string]Tag{"string": TagString, "field": TagFieldref, "method": TagMethodref,
		"imeth": TagInterfaceMethodref, "mtype": TagMethodType, "dyn": TagDynamic, "indy": TagInvokeDynamic,
		"module": TagModule, "pkg": TagPackage} {
		if pl[idx[name]].Tag != tag {
			t.Errorf("%s tag = %v, want %v", name, pl[idx[name]].Tag, tag)
		}
	}
	if pl[idx["field"]].Ref1 != cls || pl[idx["field"]].Ref2 != nat || pl[idx["indy"]].Ref1 != 1 {
		t.Errorf("refs = %+v %+v", pl[idx["field"]], pl[idx["indy"]])
	}
	if s, _ := pl.Utf8(idx["mutf8"]); s != "a\x00bé😀" {
		t.Errorf("modified UTF-8 = %q", s)
	}
}

func TestParseErrors(t *testing.T) {
	good := newClass().bytes()
	badMagic := append([]byte{0xCA, 0xFE, 0xBA, 0xBF}, good[4:]...)

	unknown := newClass()
	unknown.pool.raw(2, 0, 0)

	notClass := newClass()
	notClass.this = notClass.pool.utf8("a/B")

	badUTF := newClass()
	badUTF.pool.utf8("a\x00b")

	lastLong := newClass()
	lastLong.pool.raw(5, 0, 0, 0, 0, 0, 0, 0, 0)
	lastLong.pool.next-- // the count claims the Long's second slot is not there

	_, err := Parse(badMagic)
	var bm *BadMagicError
	if !errors.As(err, &bm) || bm.Got != 0xCAFEBABF {
		t.Errorf("bad magic: %v", err)
	}
	_, err = Parse(unknown.bytes())
	var ut *UnknownTagError
	if !errors.As(err, &ut) || ut.Tag != 2 || ut.Index != 5 {
		t.Errorf("unknown tag: %v", err)
	}
	_, err = Parse(notClass.bytes())
	var ie *IndexError
	if !errors.As(err, &ie) || ie.Want != TagClass || ie.Got != TagUtf8 {
		t.Errorf("this_class on a Utf8: %v", err)
	}
	for name, b := range map[string][]byte{"NUL byte in Utf8": badUTF.bytes(), "Long in the last slot": lastLong.bytes()} {
		var fe *FormatError
		if _, err := Parse(b); !errors.As(err, &fe) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// Every proper prefix of a class file is truncated, at the offset it ends.
func TestParseTruncated(t *testing.T) {
	c := newClass()
	c.pool.raw(6, 1, 2, 3, 4, 5, 6, 7, 8)
	c.ifaces = []uint16{c.pool.class("a/I")}
	full := c.bytes()
	for n := 0; n < len(full); n++ {
		_, err := Parse(full[:n])
		var te *TruncatedError
		if !errors.As(err, &te) {
			t.Fatalf("prefix %d: err = %v", n, err)
		}
		if te.Offset > n || te.Need < 1 || !strings.Contains(err.Error(), "truncated at offset") {
			t.Fatalf("prefix %d: %+v (%v)", n, te, err)
		}
	}
}
