package bognav

import (
	"fmt"

	"github.com/angeles725/niagara-go/internal/bog"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// bog-nav prints json.dumps through the shared pyjson builder: a row is a pyjson.Dict.

// dumps is json.dumps(v, indent=indent) with ensure_ascii; indent < 0 is indent=None (one line,
// ", " and ": " separators). v is a row or a []any of output values (see row); any other value
// type is an error, never a panic.
func dumps(v any, indent int) (string, error) {
	val, err := pyjson.FromAny(jsonValue(v))
	if err != nil {
		return "", err
	}
	return pyjson.Dumps(val, pyjson.Defaults(indent)), nil
}

// printJSON prints dumps(v, indent) and a newline; on an encoder error it prints one stderr
// line instead and marks the run failed (Main exits 2).
func (e *env) printJSON(v any, indent int) {
	s, err := dumps(v, indent)
	if err != nil {
		fmt.Fprintf(e.err, "bog-nav: %v\n", err)
		e.failed = true
		return
	}
	e.println(s)
}

// Keys and Values make a row a pyjson.Dict, its members in output order.
func (r row) Keys() []string {
	keys := make([]string, len(r))
	for i, m := range r {
		keys[i] = m.K
	}
	return keys
}

func (r row) Values() []any {
	vals := make([]any, len(r))
	for i, m := range r {
		vals[i] = jsonValue(m.V)
	}
	return vals
}

// jsonValue is v in a type pyjson.FromAny reads: a *string is its str or None, a []row a list.
func jsonValue(v any) any {
	switch x := v.(type) {
	case *string:
		if x == nil {
			return nil
		}
		return *x
	case []row:
		items := make([]any, len(x))
		for i, r := range x {
			items[i] = r
		}
		return items
	}
	return v
}

// slotJSON is a stored slot as the Python dict {type, value, flags[, child]}.
func slotJSON(s *bog.Slot) row {
	r := row{{"type", s.Type}, {"value", s.Value}, {"flags", s.Flags}}
	if s.Child.Len() > 0 {
		var child row
		for k, v := range s.Child.All() {
			child = append(child, kv{k, v})
		}
		r = append(r, kv{"child", child})
	}
	return r
}

// orderedStrings is a bog.Ordered[string] as a JSON object.
func orderedStrings(o *bog.Ordered[string]) row {
	r := row{}
	for k, v := range o.All() {
		r = append(r, kv{k, v})
	}
	return r
}
