// Package pyre compiles Python `re` patterns (the dialect a user types on a tool's
// command line) and matches them like Python does, over github.com/dlclark/regexp2.
//
// A pattern is parsed by a port of CPython 3.15's re/_parser.py, so an invalid
// pattern fails with Python's re.error text ("unterminated character set at
// position 0"), then rewritten in .NET syntax for regexp2: named groups become
// numbered ones (Python numbering), \Z is the absolute end, {,n} is {0,n}, \w and \s
// use Python's str definitions, (?x) comments and whitespace are dropped, re.ASCII is
// applied by the rewrite. Positions are code-point (rune) indices, as in Python.
//
// Known differences, none hit by the ported tools: \b and \B use the .NET word
// definition (letters, Mn, Nd, Pc); IGNORECASE folds with .NET's
// simple lower-casing (Python also folds ſ/s and a few others); \N{name} escapes are
// rejected; an iteration resumes one character after an empty match (Python 3.7+
// may then match a non-empty string at the same position); FutureWarnings for
// nested sets are not printed. A pattern Python accepts but regexp2 rejects fails
// with "unsupported by the Go regex engine: ...".
package pyre
