package corpusnav

import "testing"

func TestJSONDumps(t *testing.T) {
	tests := []struct {
		name string
		v    any
		want string
	}{
		{"empty containers", obj{{"a", []any{}}, {"b", obj{}}}, `{"a": [], "b": {}}`},
		{"nested object keeps key order", obj{{"z", 1}, {"a", []any{"B1", 2}}}, `{"z": 1, "a": ["B1", 2]}`},
		{"non-ASCII is kept raw, controls escaped", "ñ \"\\\n\x01", "\"ñ \\\"\\\\\\n\\u0001\""},
		{"an undecodable byte is one U+FFFD", "\xffabc", "\"�abc\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := jsonDumps(tt.v); got != tt.want {
				t.Errorf("jsonDumps = %q; want %q", got, tt.want)
			}
		})
	}
}
