// Package classfile reads Java .class files far enough to recover a Niagara
// type's structure without its source or bajadoc: the header and constant
// pool (classfile.go, constpool.go), fields, methods and their annotations
// (members.go, annotations.go), and the slots the Niagara annotations and
// static Property/Action/Topic fields declare (slots.go). It parses bytes
// only; reading a jar member is the caller's job. A malformed file is a
// typed error (errors.go), never a partial result.
package classfile
