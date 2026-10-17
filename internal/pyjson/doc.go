// Package pyjson is a JSON codec with Python 3 json semantics, shared by the
// ports whose output prints decoded JSON: Loads mirrors json.loads(str) with
// the C scanner (strict=True, CPython's JSONDecodeError texts), Dumps mirrors
// json.dumps with the keyword arguments the tools use, and Value.Repr/Str
// mirror repr()/str() of the decoded value.
//
// A Value keeps what Python keeps: dict insertion order, arbitrary-precision
// ints, the int/float split, NaN/Infinity, and strings that may hold lone
// surrogates (stored as WTF-8, see AppendRune, Runes and WTF8).
package pyjson
