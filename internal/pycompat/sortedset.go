package pycompat

import "slices"

// SortedSet is sorted(set(xs)): the distinct strings of xs in code-point (byte) order,
// a new non-nil list (Python's [] for no items) that leaves xs untouched.
func SortedSet(xs []string) []string {
	out := append([]string{}, xs...)
	slices.Sort(out)
	return slices.Compact(out)
}
