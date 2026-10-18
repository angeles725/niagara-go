package inventory

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestSummarizeMatchesJSONLoad: the _meta block a stats summary reports is
// json.load(f).get("_meta") whether it comes from the prefix or the whole
// document.
func TestSummarizeMatchesJSONLoad(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name, text, meta string
		loaded           bool
	}{
		{"prefix", `{ "_meta" : {"b": 1, "a": 2.5}, "x": [1]}`, `{"b": 1, "a": 2.5}`, true},
		{"meta not first", `{"x": 1, "_meta": {"a": 1}}`, `{"a": 1}`, true},
		{"NaN is Python JSON", `{"_meta": {"a": NaN}}`, `{"a": NaN}`, true},
		{"last _meta wins", `{"_meta": {"a": 1}, "_meta": {"a": 2}}`, `{"a": 2}`, true},
		{"no _meta", `{"x": "_meta"}`, "", true},
		{"_meta not an object", `{"_meta": [1]}`, "", true},
		{"top level list", `[1, 2]`, "", true},
		{"truncated", `{"_meta": {"a": 1}, "x": `, "", false},
		{"invalid UTF-8", "{\"_meta\": {\"a\": \"\xff\"}}", "", false},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := filepath.Join(dir, string(rune('a'+i))+".json")
			write(t, p, c.text)
			for _, full := range []bool{false, true} {
				s := summarize(p, full)
				if s.Loaded != c.loaded || string(s.Meta) != c.meta {
					t.Errorf("summarize(full=%v) = loaded %v meta %q, want %v %q", full, s.Loaded, s.Meta, c.loaded, c.meta)
				}
			}
		})
	}
}

func TestSummarizeCounts(t *testing.T) {
	p := filepath.Join(t.TempDir(), "class-index.json")
	write(t, p, `{"_meta": {}, "classes": {"A": [1, 2], "B": [3], "C": {"k": 1}}}`)
	s := summarize(p, true)
	if !s.Full || s.Len != 2 || !s.Truthy || s.Classes != 3 || s.Entries != 4 {
		t.Errorf("summary = %+v", s)
	}
}
