package classfile

import (
	"fmt"
	"math"
	"unicode/utf16"
)

// Tag is a constant pool entry kind (JVMS §4.4); 0 marks index 0 and the
// unusable slot after each Long and Double.
type Tag uint8

const (
	TagUtf8               Tag = 1
	TagInteger            Tag = 3
	TagFloat              Tag = 4
	TagLong               Tag = 5
	TagDouble             Tag = 6
	TagClass              Tag = 7
	TagString             Tag = 8
	TagFieldref           Tag = 9
	TagMethodref          Tag = 10
	TagInterfaceMethodref Tag = 11
	TagNameAndType        Tag = 12
	TagMethodHandle       Tag = 15
	TagMethodType         Tag = 16
	TagDynamic            Tag = 17
	TagInvokeDynamic      Tag = 18
	TagModule             Tag = 19
	TagPackage            Tag = 20
)

var tagNames = map[Tag]string{
	0: "unusable", TagUtf8: "Utf8", TagInteger: "Integer", TagFloat: "Float", TagLong: "Long",
	TagDouble: "Double", TagClass: "Class", TagString: "String", TagFieldref: "Fieldref",
	TagMethodref: "Methodref", TagInterfaceMethodref: "InterfaceMethodref", TagNameAndType: "NameAndType",
	TagMethodHandle: "MethodHandle", TagMethodType: "MethodType", TagDynamic: "Dynamic",
	TagInvokeDynamic: "InvokeDynamic", TagModule: "Module", TagPackage: "Package",
}

func (t Tag) String() string {
	if s, ok := tagNames[t]; ok {
		return s
	}
	return fmt.Sprintf("Tag(%d)", uint8(t))
}

// Constant is one pool entry. Str is a Utf8's text; Int an Integer's or
// Long's value or a MethodHandle's reference kind; Float a Float's or
// Double's value. Ref1 and Ref2 are the indexes the entry points at, in JVMS
// order: Class/String/MethodType/Module/Package one name or descriptor;
// the refs class then NameAndType; NameAndType name then descriptor;
// MethodHandle its reference; Dynamic/InvokeDynamic bootstrap method then
// NameAndType.
type Constant struct {
	Tag        Tag
	Str        string
	Int        int64
	Float      float64
	Ref1, Ref2 uint16
}

// Pool is the constant pool indexed as the file indexes it: Pool[0] and the
// slot after each Long and Double hold the zero Constant.
type Pool []Constant

func parsePool(r *reader) Pool {
	start := r.pos()
	n := int(r.u2("constant_pool_count"))
	if r.err == nil && n == 0 {
		r.fail(&FormatError{Offset: start, Msg: "constant_pool_count is 0"})
	}
	if r.err != nil {
		return nil
	}
	p := make(Pool, n)
	for i := 1; i < n && r.err == nil; i++ {
		at := r.pos()
		c := Constant{Tag: Tag(r.u1("constant tag"))}
		switch c.Tag {
		case TagUtf8:
			raw := r.take(int(r.u2("Utf8 length")), "Utf8 bytes")
			s, err := decodeModifiedUTF8(raw)
			if err != nil && r.err == nil {
				r.fail(&FormatError{Offset: at, Msg: fmt.Sprintf("constant %d: %v", i, err)})
			}
			c.Str = s
		case TagInteger:
			c.Int = int64(int32(r.u4("Integer")))
		case TagFloat:
			c.Float = float64(math.Float32frombits(r.u4("Float")))
		case TagLong, TagDouble:
			v := uint64(r.u4(c.Tag.String()))<<32 | uint64(r.u4(c.Tag.String()))
			if c.Tag == TagLong {
				c.Int = int64(v)
			} else {
				c.Float = math.Float64frombits(v)
			}
			if i+1 >= n && r.err == nil {
				r.fail(&FormatError{Offset: at, Msg: fmt.Sprintf("constant %d: %v has no second slot", i, c.Tag)})
			}
		case TagClass, TagString, TagMethodType, TagModule, TagPackage:
			c.Ref1 = r.u2(c.Tag.String())
		case TagFieldref, TagMethodref, TagInterfaceMethodref, TagNameAndType, TagDynamic, TagInvokeDynamic:
			c.Ref1, c.Ref2 = r.u2(c.Tag.String()), r.u2(c.Tag.String())
		case TagMethodHandle:
			c.Int = int64(r.u1("MethodHandle kind"))
			c.Ref1 = r.u2("MethodHandle reference")
		default:
			if r.err == nil {
				r.fail(&UnknownTagError{Index: i, Offset: at, Tag: uint8(c.Tag)})
			}
		}
		p[i] = c
		if c.Tag == TagLong || c.Tag == TagDouble {
			i++
		}
	}
	return p
}

func (p Pool) entry(i uint16, want Tag) (Constant, error) {
	if int(i) == 0 || int(i) >= len(p) || p[i].Tag != want {
		var got Tag
		if int(i) < len(p) {
			got = p[i].Tag
		}
		return Constant{}, &IndexError{Index: int(i), Size: len(p), Want: want, Got: got}
	}
	return p[i], nil
}

// Utf8 is the text of the Utf8 entry at i.
func (p Pool) Utf8(i uint16) (string, error) {
	c, err := p.entry(i, TagUtf8)
	return c.Str, err
}

// ClassName is the internal name (a/b/C) of the Class entry at i.
func (p Pool) ClassName(i uint16) (string, error) {
	c, err := p.entry(i, TagClass)
	if err != nil {
		return "", err
	}
	return p.Utf8(c.Ref1)
}

// decodeModifiedUTF8 decodes JVMS §4.4.7 modified UTF-8: NUL is C0 80, a
// supplementary character is a surrogate pair of two 3-byte forms, and no
// byte is 0 or F0..FF. An unpaired surrogate becomes U+FFFD.
func decodeModifiedUTF8(b []byte) (string, error) {
	units := make([]uint16, 0, len(b))
	cont := func(i int) bool { return i < len(b) && b[i]&0xC0 == 0x80 }
	for i := 0; i < len(b); {
		switch c := b[i]; {
		case c >= 0x01 && c <= 0x7F:
			units = append(units, uint16(c))
			i++
		case c&0xE0 == 0xC0 && cont(i+1):
			units = append(units, uint16(c&0x1F)<<6|uint16(b[i+1]&0x3F))
			i += 2
		case c&0xF0 == 0xE0 && cont(i+1) && cont(i+2):
			units = append(units, uint16(c&0x0F)<<12|uint16(b[i+1]&0x3F)<<6|uint16(b[i+2]&0x3F))
			i += 3
		default:
			return "", fmt.Errorf("malformed modified UTF-8 byte 0x%02X at %d", c, i)
		}
	}
	return string(utf16.Decode(units)), nil
}
