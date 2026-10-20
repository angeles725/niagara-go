// Package helpbuild holds the niagara-help builders ported from niagara-help
// tools/ that the nh maintenance commands run. The root keeps the doc-jar
// layer (install rules with the embedded install_docs.json: rules.go; the
// doc_types predicates: doctypes.go; the jar-listing cache: jarlisting.go;
// the jar-entry mapping: docjars.go; extract / fill-missing / fidelity:
// extract.go), the guides and devguide builders (guidesparser.go,
// guidetoc.go, guidesindex.go, devguidetoc.go), the developer course
// (course.go), rebuild-all (rebuildall.go, rebuildall_stamp.go), the
// coverage reports and the whole-install sweep (coveragereport*.go,
// doccoverage*.go, sweep*.go) and versions setup / add (versionsetup*.go,
// versionreport.go, manifest.go). The sweep reads the course and the
// versions code reads coverage, so these stay together here.
//
// The leaves it builds on are subpackages: zipfile (the Python zipfile
// reader), pyfs (os.walk, shutil.copyfile, json load/write), registry
// (versions/registry.json lookups), htmltext (html_text.convert), pdftext
// (the PDF text builder), indexes (bajadoc-clean and the class, inheritance,
// method, source, slots and xref indexes) and fulltext (fulltext.sqlite).
// internal/testkit is the test harness they share.
//
// It is a non-printing library: every function returns typed values or
// errors and writes nothing to stdout or stderr. Behaviour matches the
// Python code byte for byte where its output is observable (listing JSON,
// cache files, rule and predicate results); the *frozen_test.go tests replay
// the output of the retired Python twin, frozen under each package's
// testdata/frozen, and require empty diffs. See docs/helpbuild.md for the
// slice plan and the intended differences.
package helpbuild
