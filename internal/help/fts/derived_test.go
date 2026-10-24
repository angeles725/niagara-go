package fts

import (
	"reflect"
	"testing"
)

// TestDerivedNames: the full-text sources but the raw ones, then indexes/.
func TestDerivedNames(t *testing.T) {
	got := DerivedNames([]string{"bajadoc", "source", "guides", "devguide", "docs"})
	want := []string{"bajadoc-clean", "devguide-clean", "guides-clean", "docs-text", "course-text",
		"devguide-rc", "devguide-examples", "package-text", "lexicon-text", "indexes"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DerivedNames = %q, want %q", got, want)
	}
}
