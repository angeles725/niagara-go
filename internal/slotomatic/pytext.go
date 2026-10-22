package slotomatic

import "github.com/angeles725/niagara-go/internal/pycompat"

// ReadSource reads a source file the way open(path, encoding="utf-8", errors="replace").read()
// does (pycompat.ReadTextReplace).
func ReadSource(path string) (string, error) { return pycompat.ReadTextReplace(path) }
