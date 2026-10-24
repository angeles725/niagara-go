package helpbuild

import (
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/helpbuild/classfile"
	"github.com/angeles725/niagara-go/internal/niagaraannot"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// The rows of bytecode-index.json (bytecodeindex.go): one per parsed class,
// one per slot it declares.

// bytecodeIndex is the index being built.
type bytecodeIndex struct {
	jarCensus
	classes                []pyjson.Value
	registered, unreadable int // registered classes, and those not indexed
	types, covered         int // registered types, and those whose class is indexed
	byAnnotation, byField  int
}

// dotted is the binary name of an internal class name.
func dotted(internal string) string { return strings.ReplaceAll(internal, "/", ".") }

// accessWords are the class access flags bytecode-index.json names, in
// Java modifier order.
var accessWords = []struct {
	flag uint16
	word string
}{{classfile.AccPublic, "public"}, {classfile.AccFinal, "final"}, {classfile.AccAbstract, "abstract"},
	{classfile.AccInterface, "interface"}, {classfile.AccEnum, "enum"}}

// classRow is the classes row of one parsed class.
func (x *bytecodeIndex) classRow(r registration, jar string, cf *classfile.ClassFile) pyjson.Value {
	str := pyjson.String
	var access, ifaces, fields, methods []string
	for _, a := range accessWords {
		if cf.Access&a.flag != 0 {
			access = append(access, a.word)
		}
	}
	for _, i := range cf.Interfaces {
		ifaces = append(ifaces, dotted(i))
	}
	for _, f := range cf.Fields {
		if f.Access&classfile.AccPublic != 0 {
			fields = append(fields, f.Name)
		}
	}
	for _, m := range cf.Methods {
		if m.Access&classfile.AccPublic != 0 {
			methods = append(methods, m.Name+m.Descriptor)
		}
	}
	slots := []pyjson.Value{}
	for _, s := range classfile.NiagaraSlots(cf) {
		slots = append(slots, x.slotRow(s))
	}
	return pyjson.ObjectOf(pyjson.Field{Key: "class", Value: str(r.class)}, pyjson.Field{Key: "jar", Value: str(jar)},
		pyjson.Field{Key: "types", Value: pyjson.StringList(r.types)},
		pyjson.Field{Key: "access", Value: str(strings.Join(access, " "))},
		pyjson.Field{Key: "super", Value: str(dotted(cf.SuperClass))},
		pyjson.Field{Key: "interfaces", Value: pyjson.StringList(ifaces)},
		pyjson.Field{Key: "fields", Value: pyjson.StringList(fields)},
		pyjson.Field{Key: "methods", Value: pyjson.StringList(methods)},
		pyjson.Field{Key: "slots", Value: pyjson.Value{Kind: pyjson.Array, A: slots}})
}

// slotRow is the slots row of one slot, counted by how it was found.
func (x *bytecodeIndex) slotRow(s classfile.Slot) pyjson.Value {
	if s.Via == "field" {
		x.byField++
	} else {
		x.byAnnotation++
	}
	letters := ""
	if n, err := strconv.ParseUint(s.Flags, 10, 32); err == nil {
		letters = niagaraannot.EncodeFlags(uint32(n))
	}
	str := pyjson.String
	return pyjson.ObjectOf(pyjson.Field{Key: "name", Value: str(s.Name)}, pyjson.Field{Key: "kind", Value: str(s.Kind)},
		pyjson.Field{Key: "type", Value: str(s.Type)}, pyjson.Field{Key: "return_type", Value: str(s.ReturnType)},
		pyjson.Field{Key: "default", Value: str(s.Default)}, pyjson.Field{Key: "flags", Value: str(s.Flags)},
		pyjson.Field{Key: "flag_letters", Value: str(letters)}, pyjson.Field{Key: "via", Value: str(s.Via)})
}
