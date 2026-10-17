// Package index reads the niagara-help corpus metadata the Python tools write:
// versions/registry.json (which Niagara versions are extracted and where) and
// the JSON indexes under <data root>/indexes/.
//
// Every JSON document is decoded in order (Decode, LoadOrdered): objects keep
// the key insertion order of Python's dict, because the Python tools iterate
// their indexes in that order and stable sorts over it decide the output.
// A Go map is never iterated to produce output.
//
// Known gaps against Python's json module: the non-standard literals NaN,
// Infinity and -Infinity are rejected (no niagara-help index contains them),
// and numbers are kept as json.Number text for the caller to convert.
package index
