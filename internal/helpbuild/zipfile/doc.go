// Package zipfile is the Python-faithful jar reader of the helpbuild
// builders: the central directory as Python's zipfile lists it
// (ReadCentralDir, Open; zipdir.go) and entry reading like ZipFile.open
// (File.ReadEntry; zipread.go). It is a leaf: no other helpbuild package is
// imported. docs/helpbuild.md says why archive/zip is not used.
package zipfile
