package argparse

import (
	"sort"
	"strings"
)

// Namespace is argparse.Namespace: the parsed values by dest. Values are nil (None), bool,
// int, float64, string, []any (nargs and append actions) or whatever a default, const or
// Type.Convert supplied.
type Namespace struct {
	vals map[string]any
	// defaulted marks dests whose value is still the action default placed before
	// parsing (Python's `action.default is getattr(namespace, dest)` identity test).
	defaulted map[string]bool
	// unrecognized collects a command parser's leftover strings for the top parser
	// (Python's _UNRECOGNIZED_ARGS_ATTR).
	unrecognized []string
}

// NewNamespace returns an empty namespace.
func NewNamespace() *Namespace {
	return &Namespace{vals: map[string]any{}, defaulted: map[string]bool{}}
}

// Has reports whether dest is set (Python `dest in namespace`).
func (n *Namespace) Has(dest string) bool { _, ok := n.vals[dest]; return ok }

// Get returns the value of dest, nil when unset.
func (n *Namespace) Get(dest string) any { return n.vals[dest] }

// Set assigns dest (setattr).
func (n *Namespace) Set(dest string, v any) {
	n.vals[dest] = v
	delete(n.defaulted, dest)
}

func (n *Namespace) setDefault(dest string, v any) {
	n.vals[dest] = v
	n.defaulted[dest] = true
}

// Delete removes dest (delattr).
func (n *Namespace) Delete(dest string) {
	delete(n.vals, dest)
	delete(n.defaulted, dest)
}

// Keys returns the set dests in sorted order.
func (n *Namespace) Keys() []string {
	keys := make([]string, 0, len(n.vals))
	for k := range n.vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// String returns dest as a string: "" when unset or None.
func (n *Namespace) String(dest string) string {
	switch v := n.vals[dest].(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		return pyStr(v)
	}
}

// Bool returns dest as Python truthiness.
func (n *Namespace) Bool(dest string) bool { return truthy(n.vals[dest]) }

// Int returns dest as an int: 0 when unset, None or not a number.
func (n *Namespace) Int(dest string) int {
	switch v := n.vals[dest].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case bool:
		if v {
			return 1
		}
	}
	return 0
}

// Float returns dest as a float64: 0 when unset, None or not a number.
func (n *Namespace) Float(dest string) float64 {
	switch v := n.vals[dest].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}
	return 0
}

// Strings returns a list dest as strings (nil when unset or None); a scalar becomes a
// one-element list.
func (n *Namespace) Strings(dest string) []string {
	switch v := n.vals[dest].(type) {
	case nil:
		return nil
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, len(v))
		for i, x := range v {
			if s, ok := x.(string); ok {
				out[i] = s
			} else {
				out[i] = pyStr(x)
			}
		}
		return out
	default:
		return []string{n.String(dest)}
	}
}

// Repr is Python's repr(dict(sorted(vars(namespace).items()))).
func (n *Namespace) Repr() string {
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range n.Keys() {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(pyRepr(k))
		b.WriteString(": ")
		b.WriteString(pyRepr(n.vals[k]))
	}
	b.WriteByte('}')
	return b.String()
}

func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case int:
		return x != 0
	case float64:
		return x != 0
	case string:
		return x != ""
	case []any:
		return len(x) > 0
	case []string:
		return len(x) > 0
	}
	return true
}
