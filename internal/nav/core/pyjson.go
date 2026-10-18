package core

import "github.com/angeles725/niagara-go/internal/pyjson"

// Dumps is json.dumps(v, indent=indent) with ensure_ascii on; indent < 0 is
// indent=None (", " and ": " separators on one line). v holds what
// pyjson.FromAny reads: decoded index values (nil, bool, string, json.Number,
// []any, *index.Object), []string, or Go ints and floats.
func Dumps(v any, indent int) string {
	return pyjson.DumpsAny(v, pyjson.Defaults(indent))
}
