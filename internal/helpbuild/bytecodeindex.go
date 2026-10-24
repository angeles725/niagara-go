package helpbuild

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/helpbuild/classfile"
	"github.com/angeles725/niagara-go/internal/helpbuild/modulexml"
	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/helpbuild/zipfile"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// indexes/bytecode-index.json: the structure of every class a module.xml of
// an install's modules/ registers as a type, read from its class file
// (classfile), for the types bajadoc does not document:
//
//	{"jars": N, "jar_bytes": B,
//	 "classes": [{"class", "jar", "types", "access", "super", "interfaces",
//	              "fields", "methods", "slots": [{"name", "kind", "type",
//	              "return_type", "default", "flags", "flag_letters", "override",
//	              "via"}]}],
//	 "errors": [{"jar", "path", "error"}]}
//
// Only registered classes are read, each from the jar whose descriptor
// registers it; types are the "module:Type" names registering it. Class
// names are dotted; super is the direct superclass only (nh walks the chain
// across classes), "" for java.lang.Object. fields and methods are the
// public ones the class declares (a method as name + JVM descriptor). slots
// are classfile.NiagaraSlots: inherited slots are not included, flags is
// the decimal number javac folds the flags element into and flag_letters
// its Slot Sheet letters, override true for a slot that overrides an
// inherited one (an @NiagaraProperty override); a slot known only by its
// field (via "field") has its kind and name. A registered class the jar lacks or that cannot be
// parsed, like an unreadable jar or descriptor, is an "errors" row and a
// warning, never a silent skip.
const (
	bytecodeIndexName   = "bytecode-index.json"
	bytecodeIndexScript = "bytecode_index_builder"
)

// registration is one registered class of a jar and the types naming it.
type registration struct {
	class string
	types []string
}

// registrations are the classes a descriptor registers, in first-type order.
func registrations(mod *modulexml.Module, module string) []registration {
	at := map[string]int{}
	var out []registration
	for _, t := range mod.Types {
		i, ok := at[t.Class]
		if !ok {
			i = len(out)
			at[t.Class] = i
			out = append(out, registration{class: t.Class})
		}
		out[i].types = append(out[i].types, module+":"+t.Name)
	}
	return out
}

// classMember is the jar path of a dotted class name.
func classMember(class string) string { return strings.ReplaceAll(class, ".", "/") + ".class" }

// add indexes the registered classes of one jar, or records why it cannot.
func (x *bytecodeIndex) add(jar string, res *textResult) {
	name := x.count(jar)
	z, err := zipfile.Open(jar)
	if err != nil {
		x.fail(res, name, "", err)
		return
	}
	defer z.Close()
	desc := membersOf(z, func(m string) bool { return m == modulexml.Member })
	if len(desc) == 0 {
		return
	}
	mod, err := desc[0].data, desc[0].err
	var m *modulexml.Module
	if err == nil {
		m, err = modulexml.Parse(mod)
	}
	if err != nil {
		x.fail(res, name, modulexml.Member, err)
		return
	}
	regs := registrations(m, moduleOf(name, m))
	want := map[string]bool{}
	for _, r := range regs {
		want[classMember(r.class)] = true
	}
	got := map[string]jarMember{}
	for _, c := range membersOf(z, func(m string) bool { return want[m] }) {
		got[c.name] = c
	}
	for _, r := range regs {
		x.registered++
		x.types += len(r.types)
		path := classMember(r.class)
		c, ok := got[path]
		switch {
		case !ok:
			err = fmt.Errorf("registered in module.xml (%s) but not in the jar", strings.Join(r.types, ", "))
		case c.err != nil:
			err = c.err
		default:
			var cf *classfile.ClassFile
			if cf, err = classfile.Parse(c.data); err == nil {
				x.covered += len(r.types)
				x.classes = append(x.classes, x.classRow(r, name, cf))
				continue
			}
		}
		x.unreadable++
		x.fail(res, name, path, err)
	}
}

// buildBytecodeIndexFrom writes root/indexes/bytecode-index.json from the
// jars of installDir/modules.
func buildBytecodeIndexFrom(root, installDir string) (*textResult, error) {
	jars, err := lexiconJars(installDir)
	if err != nil {
		return nil, err
	}
	res := &textResult{}
	x := &bytecodeIndex{jarCensus: jarCensus{script: bytecodeIndexScript}}
	for _, jar := range jars {
		x.add(jar, res)
	}
	arr := func(a []pyjson.Value) pyjson.Value { return pyjson.Value{Kind: pyjson.Array, A: a} }
	v := pyjson.ObjectOf(pyjson.Field{Key: "jars", Value: pyjson.IntValue(int64(x.jars))},
		pyjson.Field{Key: "jar_bytes", Value: pyjson.IntValue(int64(x.bytes))},
		pyjson.Field{Key: "classes", Value: arr(x.classes)}, pyjson.Field{Key: "errors", Value: arr(x.errors)})
	out := pycompat.Join(root, "indexes", bytecodeIndexName)
	if err := pyfs.WriteJSON(out, v, lexiconIndexDump); err != nil {
		return nil, err
	}
	res.Files = len(x.classes)
	res.Log = append(res.Log, fmt.Sprintf("%s: %d of %d registered classes (%d of %d registered types; %d registered classes unreadable),"+
		" %d slots (%d by annotation, %d by field) from %d jars (%d errors) -> %s",
		bytecodeIndexName, res.Files, x.registered, x.covered, x.types, x.unreadable,
		x.byAnnotation+x.byField, x.byAnnotation, x.byField, x.jars, len(x.errors), out))
	return res, nil
}

// buildBytecodeIndex is the rebuild-all step: bytecode-index.json from the
// modules/ of the install registered for root.
var buildBytecodeIndex = fromInstall(bytecodeIndexName, buildBytecodeIndexFrom)
