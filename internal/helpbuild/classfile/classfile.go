package classfile

import (
	"fmt"
	"io"
)

const magic = 0xCAFEBABE

// Access flags (JVMS §4.1, §4.5, §4.6) the package and its callers test.
const (
	AccPublic    = 0x0001
	AccPrivate   = 0x0002
	AccProtected = 0x0004
	AccStatic    = 0x0008
	AccFinal     = 0x0010
	AccInterface = 0x0200
	AccAbstract  = 0x0400
	AccSynthetic = 0x1000
	AccEnum      = 0x4000
)

// ClassFile is a parsed class file. Class names are internal names
// (a/b/C); SuperClass is "" only when super_class is 0 (java/lang/Object,
// module-info).
type ClassFile struct {
	Minor, Major uint16
	Pool         Pool
	Access       uint16
	ThisClass    string
	SuperClass   string
	Interfaces   []string
	Fields       []Member     // non-synthetic, in file order
	Methods      []Member     // non-synthetic, in file order
	Annotations  []Annotation // the class's own, in file order
}

// ParseReader reads r to the end and parses it as Parse does.
func ParseReader(r io.Reader) (*ClassFile, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

// Parse parses a whole class file. Any fault is a typed error (errors.go)
// and no ClassFile.
func Parse(b []byte) (*ClassFile, error) {
	r := &reader{b: b}
	if m := r.u4("magic"); r.err == nil && m != magic {
		return nil, &BadMagicError{Got: m}
	}
	cf := &ClassFile{Minor: r.u2("minor_version"), Major: r.u2("major_version")}
	cf.Pool = parsePool(r)
	cf.Access = r.u2("access_flags")
	this, super := r.u2("this_class"), r.u2("super_class")
	ifaces := make([]uint16, r.u2("interfaces_count"))
	for i := range ifaces {
		ifaces[i] = r.u2("interface")
	}
	cf.Fields = parseMembers(r, cf.Pool, "fields")
	cf.Methods = parseMembers(r, cf.Pool, "methods")
	cf.Annotations = parseAttributes(r, cf.Pool)
	if r.err == nil && r.off != len(b) {
		r.fail(&FormatError{Offset: r.pos(), Msg: fmt.Sprintf("%d bytes after the class attributes", len(b)-r.off)})
	}
	if r.err != nil {
		return nil, r.err
	}
	var err error
	if cf.ThisClass, err = cf.Pool.ClassName(this); err != nil {
		return nil, err
	}
	if super != 0 {
		if cf.SuperClass, err = cf.Pool.ClassName(super); err != nil {
			return nil, err
		}
	}
	for _, i := range ifaces {
		name, err := cf.Pool.ClassName(i)
		if err != nil {
			return nil, err
		}
		cf.Interfaces = append(cf.Interfaces, name)
	}
	return cf, nil
}
