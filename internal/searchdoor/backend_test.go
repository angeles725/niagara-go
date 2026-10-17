package searchdoor

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name          string
		rc            int
		out, err      string
		state, reason string
	}{
		{"rows", 0, "B1 · x\n", "", OK, ""},
		{"miss with exit 1", 1, "(no matches; searched 3)\n", "", Empty, ""},
		{"exit 1 without a miss line", 1, "x", "a\nValueError: boom\n", Unavailable,
			"error: exit 1 without a miss/scope line (crashed?): ValueError: boom"},
		{"exit 1 silent", 1, "", "", Unavailable, "error: exit 1 without a miss/scope line (crashed?): no output"},
		{"exit 2 names stderr's last line", 2, "", "x\n  no index  \n", Unavailable, "error: no index (exit 2)"},
		{"exit 2 falls back to stdout", 2, "\n  first\nsecond", "", Unavailable, "error: first (exit 2)"},
		{"exit 3 silent", 3, "", "", Unavailable, "error: exit 3 (exit 3)"},
		{"exit 0 silent", 0, "\n \n", "", Unavailable,
			"ambiguous: exit 0 with no output (neither rows nor a miss/scope line)"},
		{"exit 0 miss line", 0, "\n  0 matches for x\n", "", Unavailable,
			"ambiguous: exit 0 but the output is a miss line: 0 matches for x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.rc, tt.out, tt.err, 0)
			if got.State != tt.state || got.Reason != tt.reason {
				t.Errorf("Classify = %s %q, want %s %q", got.State, got.Reason, tt.state, tt.reason)
			}
		})
	}
}

func TestPyPath(t *testing.T) {
	for in, want := range map[string]string{"a//b/./c/": "a/b/c", "./x": "x", "/a/../b": "/a/../b", "": ".",
		"//srv/x": "//srv/x", "///x": "/x", "missing/corpus-nav.py": "missing/corpus-nav.py"} {
		if got := pyPath(in); got != want {
			t.Errorf("pyPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatG3(t *testing.T) {
	for in, want := range map[float64]string{1: "1", 60: "60", 0.9994: "0.999", 0.99951: "1", 1000: "1e+03", 0.25: "0.25"} {
		if got := formatG3(in); got != want {
			t.Errorf("formatG3(%v) = %q, want %q", in, got, want)
		}
	}
}
