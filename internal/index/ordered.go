package index

import (
	"fmt"
	"os"
)

// Object is a decoded JSON object that remembers key insertion order, like a
// Python dict: a duplicated key keeps its first position and its last value.
// Small objects are searched linearly; larger ones get a key index.
type Object struct {
	keys []string
	vals []any
	idx  map[string]int // nil while the object has at most indexFrom keys
}

// indexFrom is the key count above which an Object keeps a key -> position map.
const indexFrom = 12

// NewObject returns an empty object.
func NewObject() *Object { return &Object{} }

func (o *Object) find(k string) int {
	if o.idx != nil {
		if i, ok := o.idx[k]; ok {
			return i
		}
		return -1
	}
	for i, key := range o.keys {
		if key == k {
			return i
		}
	}
	return -1
}

// Set stores v under k, appending k when it is new.
func (o *Object) Set(k string, v any) {
	if i := o.find(k); i >= 0 {
		o.vals[i] = v
		return
	}
	o.keys = append(o.keys, k)
	o.vals = append(o.vals, v)
	if o.idx != nil {
		o.idx[k] = len(o.keys) - 1
	} else if len(o.keys) > indexFrom {
		o.idx = make(map[string]int, len(o.keys))
		for i, key := range o.keys {
			o.idx[key] = i
		}
	}
}

// Get returns the value under k and whether k is present.
func (o *Object) Get(k string) (any, bool) {
	if i := o.find(k); i >= 0 {
		return o.vals[i], true
	}
	return nil, false
}

// Has reports whether k is present (Python `k in d`).
func (o *Object) Has(k string) bool { return o.find(k) >= 0 }

// Len is the number of keys.
func (o *Object) Len() int { return len(o.keys) }

// Keys returns the keys in insertion order. The slice is shared: do not modify it.
func (o *Object) Keys() []string { return o.keys }

// Values returns the values in key order (Values()[i] belongs to Keys()[i]).
// The slice is shared: do not modify it.
func (o *Object) Values() []any { return o.vals }

// Object returns the value under k when it is a JSON object, else nil.
func (o *Object) Object(k string) *Object {
	v, _ := o.Get(k)
	ob, _ := v.(*Object)
	return ob
}

// String returns the value under k when it is a JSON string.
func (o *Object) String(k string) (string, bool) {
	v, _ := o.Get(k)
	s, ok := v.(string)
	return s, ok
}

// Decode parses one JSON document with Python json.loads semantics. Values are
// nil (null), bool, json.Number (integer and float literals keep their text, so
// an int stays distinct from a float; NaN, Infinity and -Infinity are accepted
// like Python's), string, []any (array) and *Object. A syntax error is a
// *DecodeError carrying Python's JSONDecodeError message.
func Decode(data []byte) (any, error) {
	p := newParser(data)
	return p.document()
}

// LoadOrdered reads and decodes the JSON file at path.
func LoadOrdered(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	v, err := Decode(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return v, nil
}

// LoadObject reads a JSON file whose top-level value must be an object.
func LoadObject(path string) (*Object, error) {
	v, err := LoadOrdered(path)
	if err != nil {
		return nil, err
	}
	o, ok := v.(*Object)
	if !ok {
		return nil, fmt.Errorf("%s: top-level JSON value is not an object", path)
	}
	return o, nil
}
