package classfile

import "fmt"

// Member is a field or a method: its access flags, name, descriptor and the
// annotations of its RuntimeVisibleAnnotations and
// RuntimeInvisibleAnnotations attributes, in file order.
type Member struct {
	Access      uint16
	Name        string
	Descriptor  string
	Annotations []Annotation
}

// parseMembers reads a fields or methods table, dropping ACC_SYNTHETIC
// members (compiler-made: outer-instance fields, accessors, bridges).
func parseMembers(r *reader, pool Pool, what string) []Member {
	n := int(r.u2(what + "_count"))
	var out []Member
	for i := 0; i < n && r.err == nil; i++ {
		m := Member{Access: r.u2(what + " access_flags")}
		m.Name = poolRef(r, pool, TagUtf8, what+" name_index").Str
		m.Descriptor = poolRef(r, pool, TagUtf8, what+" descriptor_index").Str
		m.Annotations = parseAttributes(r, pool)
		if m.Access&AccSynthetic == 0 {
			out = append(out, m)
		}
	}
	return out
}

// parseAttributes walks an attribute table by each attribute's length and
// returns the annotations of its RuntimeVisibleAnnotations and
// RuntimeInvisibleAnnotations attributes, in file order. Niagara's slot
// annotations (NiagaraProperty and kin) have CLASS retention, so they are in
// the invisible one. An annotations attribute must use exactly its length.
func parseAttributes(r *reader, pool Pool) []Annotation {
	n := int(r.u2("attributes_count"))
	var out []Annotation
	for i := 0; i < n && r.err == nil; i++ {
		name := poolRef(r, pool, TagUtf8, "attribute_name_index").Str
		size := r.u4("attribute_length")
		at := r.pos()
		info := r.take(int(size), name+" attribute")
		visible := name == "RuntimeVisibleAnnotations"
		if r.err != nil || !visible && name != "RuntimeInvisibleAnnotations" {
			continue
		}
		sub := &reader{b: info, base: at}
		anns := parseAnnotations(sub, pool, visible)
		if sub.err == nil && sub.off != len(info) {
			sub.fail(&FormatError{Offset: sub.pos(), Msg: fmt.Sprintf("%s has %d bytes past its annotations", name, len(info)-sub.off)})
		}
		r.fail(sub.err)
		out = append(out, anns...)
	}
	return out
}

// poolRef reads a u2 pool index and resolves it to an entry of kind want; a
// bad index is the reader's error.
func poolRef(r *reader, pool Pool, want Tag, what string) Constant {
	i := r.u2(what)
	if r.err != nil {
		return Constant{}
	}
	c, err := pool.entry(i, want)
	if err != nil {
		r.fail(err)
	}
	return c
}
