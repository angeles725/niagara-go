package inventory

import (
	"reflect"
	"testing"
)

// Expected values from CPython 3.15: random.Random(0).
func TestPyRandomMatchesCPython(t *testing.T) {
	r := newPyRandom(0)
	got := []uint32{r.uint32(), r.uint32(), r.uint32()}
	if want := []uint32{3626764237, 1654615998, 3255389356}; !reflect.DeepEqual(got, want) {
		t.Errorf("getrandbits(32) x3 = %v, want %v", got, want)
	}
	upto := func(n int) []int {
		out := make([]int, n)
		for i := range out {
			out[i] = i
		}
		return out
	}
	cases := []struct {
		n, k int
		want []int
	}{
		{1000, 8, []int{864, 394, 776, 911, 430, 41, 265, 988}}, // set method
		{10, 4, []int{6, 9, 0, 2}},                              // pool method
		{300000, 40, []int{201979, 220500, 21225, 135746, 268055, 254766}},
	}
	for _, c := range cases {
		got := sample(newPyRandom(0), upto(c.n), c.k)
		if !reflect.DeepEqual(got[:len(c.want)], c.want) {
			t.Errorf("sample(range(%d), %d) = %v, want prefix %v", c.n, c.k, got, c.want)
		}
	}
}
