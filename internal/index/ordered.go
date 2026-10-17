package index

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// Object is a decoded JSON object that remembers key insertion order, like a
// Python dict: a duplicated key keeps its first position and its last value.
type Object struct {
	keys []string
	vals map[string]any
}

// NewObject returns an empty object.
func NewObject() *Object { return &Object{vals: map[string]any{}} }

// Set stores v under k, appending k when it is new.
func (o *Object) Set(k string, v any) {
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

// Get returns the value under k and whether k is present.
func (o *Object) Get(k string) (any, bool) {
	v, ok := o.vals[k]
	return v, ok
}

// Has reports whether k is present (Python `k in d`).
func (o *Object) Has(k string) bool { _, ok := o.vals[k]; return ok }

// Len is the number of keys.
func (o *Object) Len() int { return len(o.keys) }

// Keys returns the keys in insertion order. The slice is shared: do not modify it.
func (o *Object) Keys() []string { return o.keys }

// Object returns the value under k when it is a JSON object, else nil.
func (o *Object) Object(k string) *Object {
	v, _ := o.vals[k].(*Object)
	return v
}

// String returns the value under k when it is a JSON string.
func (o *Object) String(k string) (string, bool) {
	v, ok := o.vals[k].(string)
	return v, ok
}

// Decode parses one JSON document. Values are nil (null), bool, json.Number,
// string, []any (array) and *Object; trailing data after the value is an error.
func Decode(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := decodeValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("extra data after the JSON value")
	}
	return v, nil
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

func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			o := NewObject()
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				v, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				o.Set(kt.(string), v)
			}
			_, err := dec.Token() // '}'
			return o, err
		case '[':
			arr := []any{}
			for dec.More() {
				v, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			_, err := dec.Token() // ']'
			return arr, err
		}
		return nil, fmt.Errorf("unexpected delimiter %q", t)
	default:
		return t, nil // nil, bool, json.Number or string
	}
}
