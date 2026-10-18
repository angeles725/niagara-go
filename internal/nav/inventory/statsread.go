package inventory

import (
	"bytes"
	"encoding/json"
	"os"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
)

// What stats reads from a JSON index (summary), the way json.load sees the
// file. When only the _meta block is needed, a well-formed UTF-8 JSON document
// whose one "_meta" key opens the top-level object has that value decoded from
// the prefix; any other file is decoded whole (index.LoadPython), so the result
// is Python's.

// summary is what stats reads from one JSON index.
type summary struct {
	Loaded  bool   // json.load succeeded
	Full    bool   // decoded whole: Len, Truthy, Classes and Entries are set
	Meta    []byte // the top-level "_meta" object as JSON; nil when absent or not an object
	Len     int64  // len() of the top-level value
	Truthy  bool   // bool() of the top-level value
	Classes int64  // len(value["classes"]) when it is an object
	Entries int64  // sum(len(v) for v in value["classes"].values())
}

// meta is the decoded _meta block (nil when absent).
func (s summary) meta() *index.Object {
	if s.Meta == nil {
		return nil
	}
	v, err := index.Decode(s.Meta)
	if err != nil {
		return nil
	}
	o, _ := v.(*index.Object)
	return o
}

// summarize reads one JSON file as json.load would.
func summarize(path string, full bool) summary {
	if !full {
		if s, ok := metaFromPrefix(path); ok {
			return s
		}
	}
	v, err := index.LoadPython(path)
	if err != nil {
		return summary{}
	}
	s := summary{Loaded: true, Full: true, Truthy: truthy(v)}
	switch t := v.(type) {
	case *index.Object:
		s.Len = int64(t.Len())
		if m := t.Object("_meta"); m != nil {
			s.Meta = []byte(core.Dumps(m, -1))
		}
		if classes := t.Object("classes"); classes != nil {
			s.Classes = int64(classes.Len())
			for _, e := range classes.Values() {
				switch l := e.(type) {
				case []any:
					s.Entries += int64(len(l))
				case *index.Object:
					s.Entries += int64(l.Len())
				case string:
					s.Entries += int64(utf8.RuneCountInString(l))
				}
			}
		}
	case []any:
		s.Len = int64(len(t))
	case string:
		s.Len = int64(utf8.RuneCountInString(t))
	}
	return s
}

// metaFromPrefix is the fast path of summarize for the _meta block alone;
// ok=false sends the file to the whole-document decode.
func metaFromPrefix(path string) (summary, bool) {
	data, err := os.ReadFile(path)
	if err != nil || index.CheckUTF8(data) != nil || !json.Valid(data) {
		return summary{}, false
	}
	key := []byte(`"_meta"`)
	switch bytes.Count(data, key) {
	case 0:
		return summary{Loaded: true}, bytes.HasPrefix(bytes.TrimLeft(data, " \t\r\n"), []byte("{"))
	case 1:
	default:
		return summary{}, false
	}
	rest := bytes.TrimLeft(data, " \t\r\n")
	if !bytes.HasPrefix(rest, []byte("{")) {
		return summary{}, false
	}
	rest = bytes.TrimLeft(rest[1:], " \t\r\n")
	if !bytes.HasPrefix(rest, key) {
		return summary{}, false
	}
	rest = bytes.TrimLeft(rest[len(key):], " \t\r\n")
	if !bytes.HasPrefix(rest, []byte(":")) {
		return summary{}, false
	}
	var raw json.RawMessage
	if json.NewDecoder(bytes.NewReader(rest[1:])).Decode(&raw) != nil {
		return summary{}, false
	}
	v, err := index.Decode(raw)
	if err != nil {
		return summary{}, false
	}
	s := summary{Loaded: true}
	if m, ok := v.(*index.Object); ok {
		s.Meta = []byte(core.Dumps(m, -1))
	}
	return s, true
}

// truthy is bool(v) for a decoded value.
func truthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		return t != ""
	case int64:
		return t != 0
	case *index.Object:
		return t != nil && t.Len() > 0
	case []any:
		return len(t) > 0
	}
	f, _ := index.Float(v)
	return f != 0
}

// directReader reads every summary from its file.
type directReader struct{}

func (directReader) summary(path string, full bool) summary {
	if st, err := os.Stat(path); err != nil || !st.Mode().IsRegular() {
		return summary{}
	}
	return summarize(path, full)
}

func (directReader) save() {}
