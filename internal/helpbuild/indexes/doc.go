// Package indexes holds the corpus index builders of niagara-help: the
// bajadoc-clean/ text from the javadoc pages and the .bajadoc XML (S7a,
// S7b: bajadoc.go, bajadocrun.go, bajadocxmlrun.go), the class, inheritance
// and method indexes (S8: classindex.go, classindexxml.go, inheritance.go,
// inheritread.go, methodindex.go) and the source, slots and xref indexes
// (S9: javasrc.go, sourceindex.go, slotsparse.go, slotsindex.go,
// xrefindex.go). Each Build* function is one rebuild-all step; it imports
// only pyfs and htmltext of the helpbuild packages.
package indexes
