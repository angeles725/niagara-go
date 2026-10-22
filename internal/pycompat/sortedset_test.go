package pycompat

import (
	"slices"
	"testing"
)

func TestSortedSet(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   []string
		want []string
	}{
		{"nil is an empty list", nil, []string{}},
		{"empty stays empty", []string{}, []string{}},
		{"duplicates collapse", []string{"b", "a", "b", "a"}, []string{"a", "b"}},
		{"code-point order", []string{"é", "Z", "a", "_"}, []string{"Z", "_", "a", "é"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := slices.Clone(tt.in)
			got := SortedSet(in)
			if got == nil || !slices.Equal(got, tt.want) {
				t.Errorf("SortedSet(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
			if !slices.Equal(in, tt.in) {
				t.Errorf("SortedSet modified its input: %q", in)
			}
		})
	}
}
