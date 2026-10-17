// Package index reads the niagara-help corpus metadata the Python tools write:
// versions/registry.json (which Niagara versions are extracted and where) and
// the JSON indexes under <data root>/indexes/.
//
// Every JSON document is decoded in order (Decode, LoadOrdered): objects keep
// the key insertion order of Python's dict, because the Python tools iterate
// their indexes in that order and stable sorts over it decide the output.
// A Go map is never iterated to produce output.
//
// LoadPython reads an index exactly as json.load(open(path, encoding="utf-8"))
// does, and its errors print Python's exception text. OpenSQLite opens a SQLite
// index read-only with the pure-Go driver modernc.org/sqlite (no cgo).
//
// Numbers are kept as json.Number text (Int, Float and IsInt convert them), so
// an integer literal stays distinct from a float like in Python. NaN, Infinity
// and -Infinity are accepted as Python's json module accepts them. Known gap: a
// lone UTF-16 surrogate escape ("\ud800") decodes to U+FFFD, which a Go string
// cannot otherwise hold.
package index
