package inventory

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestPosixDirname(t *testing.T) {
	for in, want := range map[string]string{"a/b": "a", "a/b/": "a/b", "base": "", "/x": "/", "//x": "//", "/a//b": "/a"} {
		if got := posixDirname(in); got != want {
			t.Errorf("posixDirname(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestHostDirname: os.path.dirname is ntpath.dirname on Windows (either separator splits,
// the drive and root stay), posixpath.dirname elsewhere.
func TestHostDirname(t *testing.T) {
	want := map[string]string{"a/b": "a", "/x": "/", "base": ""}
	if runtime.GOOS == "windows" {
		want = map[string]string{`C:\a\b`: `C:\a`, `C:\x`: `C:\`, "fixture/stats/nav": "fixture/stats",
			`a/b\c`: "a/b", "base": "", `\\srv\share\x`: `\\srv\share\`}
	}
	for in, w := range want {
		if got := hostDirname(in); got != w {
			t.Errorf("hostDirname(%q) = %q, want %q", in, got, w)
		}
	}
}
