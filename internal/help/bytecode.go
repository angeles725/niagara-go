package help

// The structure of a registered class read from its class file, over
// indexes/bytecode-index.json (built by rebuild-all's bytecode_index_builder
// from every class a module.xml registers): class falls back to it for a
// class with neither javadoc nor Java source (or shows it with --bytecode),
// and type says whether a type without javadoc has it. Every line read from
// it is tagged [bytecode]; inherited slots come from walking the direct
// supers the index records, as far as it holds them.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// bytecodeMatches are the rows of a qualified class name, or of every class
// with that simple name (sorted).
func bytecodeMatches(rows map[string]*index.Object, name string) []*index.Object {
	if r := rows[name]; r != nil {
		return []*index.Object{r}
	}
	var names []string
	for c := range rows {
		if !strings.Contains(name, ".") && simpleName(c) == name {
			names = append(names, c)
		}
	}
	sort.Strings(names)
	out := make([]*index.Object, len(names))
	for i, c := range names {
		out[i] = rows[c]
	}
	return out
}

// classFromBytecode is class's fallback for a name neither class-index.json
// nor source-index.json holds (fallback true), or class --bytecode: the class
// file of the first class of that name in bytecode-index.json. ok is false
// when it holds none.
func (e *Env) classFromBytecode(name string, fallback bool) (ok bool, err error) {
	rows, _, err := e.bytecodeRows()
	if err != nil {
		return false, err
	}
	hits := bytecodeMatches(rows, name)
	if len(hits) == 0 {
		return false, nil
	}
	out := e.Out
	if len(hits) > 1 {
		fmt.Fprintf(out, "%d classes named '%s' in %s:\n", len(hits), name, bytecodeIndexName)
		for _, h := range hits {
			fmt.Fprintf(out, "  %s (%s)\n", getStr(h, "class", ""), getStr(h, "jar", ""))
		}
		fmt.Fprintln(out, "Showing the first; pass a qualified name for another.")
		fmt.Fprintln(out)
	}
	r := hits[0]
	class := getStr(r, "class", "")
	pkg := strings.TrimSuffix(strings.TrimSuffix(class, simpleName(class)), ".")
	fmt.Fprintf(out, "CLASS: %s [bytecode]\nPACKAGE: %s [bytecode]\nTYPES: %s (%s) [bytecode]\n", simpleName(class), pkg,
		strings.Join(strList(r, "types"), ", "), getStr(r, "jar", ""))
	if fallback {
		fmt.Fprintln(out, "No javadoc (.bajadoc) or Java source for this class; its class file is indexed:")
	}
	fmt.Fprintf(out, "BYTECODE: %s (%s!%s.class)\n", bytecodeIndexName, getStr(r, "jar", ""), strings.ReplaceAll(class, ".", "/"))
	fmt.Fprintf(out, "ACCESS: %s [bytecode]\n", getStr(r, "access", ""))
	fmt.Fprintf(out, "SUPER: %s [bytecode]\n", superChain(rows, r))
	ifaces := strings.Join(strList(r, "interfaces"), ", ")
	if ifaces == "" {
		ifaces = "(none)"
	}
	fmt.Fprintf(out, "INTERFACES: %s [bytecode]\n", ifaces)
	e.printBytecodeSlots(rows, r)
	methods := strList(r, "methods")
	var names []string
	for _, m := range methods {
		names = append(names, m[:strings.IndexByte(m+"(", '(')])
	}
	fmt.Fprintf(out, "METHODS (%d public, declared) [bytecode]: %s\n", len(methods), strings.Join(names, ", "))
	return true, nil
}

// typeBytecode is the BYTECODE line of a type whose class has no javadoc,
// and whether bytecode-index.json has its class file.
func (e *Env) typeBytecode(class string) (line string, ok bool, err error) {
	rows, idx, err := e.bytecodeRows()
	if err != nil {
		return "", false, err
	}
	if r := rows[class]; r != nil {
		return fmt.Sprintf("BYTECODE: %s has %s: %d slots declared (nh class %s --bytecode)", bytecodeIndexName, class,
			listLen(r, "slots"), class), true, nil
	}
	path := strings.ReplaceAll(class, ".", "/") + ".class"
	for _, x := range objList(idx, "errors") {
		if getStr(x, "path", "") == path {
			return fmt.Sprintf("BYTECODE: %s could not read %s!%s: %s", bytecodeIndexName, getStr(x, "jar", ""), path,
				getStr(x, "error", "")), false, nil
		}
	}
	return "BYTECODE: not in " + bytecodeIndexName + " (rebuild-all builds it from the install's modules/)", false, nil
}

// bytecodeStats counts bytecode-index.json, nil when it is absent: its
// classes, the registered types they cover, their slots by how each was
// found, and its errors rows as registered classes and as jars or
// descriptors it could not read.
func (e *Env) bytecodeStats() (pyjson.Obj, error) {
	if !pycompat.IsFile(index.Path(e.BaseDir, bytecodeIndexName)) {
		return nil, nil
	}
	idx, err := e.loadIndex(bytecodeIndexName, false)
	if err != nil {
		return nil, err
	}
	classes := objList(idx, "classes")
	types, byAnnotation, byField, badClasses := 0, 0, 0, 0
	for _, c := range classes {
		types += listLen(c, "types")
		for _, s := range objList(c, "slots") {
			if getStr(s, "via", "") == "field" {
				byField++
			} else {
				byAnnotation++
			}
		}
	}
	errs := objList(idx, "errors")
	for _, x := range errs {
		if strings.HasSuffix(getStr(x, "path", ""), ".class") {
			badClasses++
		}
	}
	return pyjson.Obj{{"classes", len(classes)}, {"types", types}, {"slots", byAnnotation + byField},
		{"by_annotation", byAnnotation}, {"by_field", byField}, {"unreadable_classes", badClasses},
		{"unreadable_jars", len(errs) - badClasses}}, nil
}

// bytecodeStatsLine is the stats text line of the bytecodeStats counts c.
func bytecodeStatsLine(c pyjson.Obj, w func(int) string) string {
	n := func(i int) string { return pycompat.Commas(c[i].Val.(int)) }
	return fmt.Sprintf("%s classes (%s registered types; %s slots: %s by annotation, %s by field;"+
		" %s registered classes unreadable, %s jars or descriptors unreadable)", w(c[0].Val.(int)), n(1), n(2), n(3), n(4), n(5), n(6))
}
