package bog

import "iter"

// Ordered is a string-keyed map that iterates in first-insertion order, like a Python dict:
// setting an existing key replaces its value and keeps its position. The zero value is empty
// and ready to use.
type Ordered[V any] struct {
	keys []string
	m    map[string]V
}

// Set stores v under k.
func (o *Ordered[V]) Set(k string, v V) {
	if o.m == nil {
		o.m = make(map[string]V)
	}
	if _, ok := o.m[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.m[k] = v
}

// Get returns the value under k and whether it is present.
func (o *Ordered[V]) Get(k string) (V, bool) {
	v, ok := o.m[k]
	return v, ok
}

// Len is the number of keys.
func (o *Ordered[V]) Len() int { return len(o.keys) }

// Keys returns the keys in insertion order. The slice is shared: do not modify it.
func (o *Ordered[V]) Keys() []string { return o.keys }

// All yields the entries in insertion order.
func (o *Ordered[V]) All() iter.Seq2[string, V] {
	return func(yield func(string, V) bool) {
		for _, k := range o.keys {
			if !yield(k, o.m[k]) {
				return
			}
		}
	}
}
