package navbuild

import (
	"strings"
	"testing"
)

// laterBuilders are the builders that read the class index: each runs after
// the inventory and the class index of its base dir, in this order (the call
// graph reads the inheritance index).
type laterBuilder struct {
	index string
	build func(base, organized string) error
}

var laterBuilders = []laterBuilder{
	{InheritanceFile, func(base, _ string) error { _, err := BuildInheritance(base); return err }},
	{XrefFile, func(base, organized string) error { _, err := BuildXref(base, organized); return err }},
	{TokenFile, func(base, organized string) error {
		_, err := BuildTokenIndex(base, organized, DefaultMinTokenLen)
		return err
	}},
	{MethodFile, func(base, organized string) error {
		_, err := BuildMethodIndex(base, organized)
		return err
	}},
	{CallgraphFile, func(base, organized string) error {
		_, err := BuildCallgraph(base, organized)
		return err
	}},
}

func TestReadClassIndex(t *testing.T) {
	for _, tc := range []struct {
		name, doc string
		entries   int
		fails     bool
	}{
		// build_inheritance/build_xref read ci_data.get("classes", {}): a missing key is empty.
		{"no classes key", `{"_meta": {}}`, 0, false},
		{"null members", `{"classes": {"A": [{"outer_class": null, "extends": null, "implements": null, "path": "a/A.java"}]}}`, 1, false},
		{"not JSON", `{"classes": `, 0, true},
		{"not an object", `[]`, 0, true},
		{"classes not an object", `{"classes": []}`, 0, true},
		{"entries not a list", `{"classes": {"A": {}}}`, 0, true},
		{"entry not an object", `{"classes": {"A": [1]}}`, 0, true},
		{"extends not a string", `{"classes": {"A": [{"extends": ["B"]}]}}`, 0, true},
		{"implements not a list", `{"classes": {"A": [{"implements": "I"}]}}`, 0, true},
		{"implements holds a non-string", `{"classes": {"A": [{"implements": [1]}]}}`, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := newBase(t)
			mustWrite(t, IndexPath(base, ClassIndexFile), tc.doc)
			got, err := readClassIndex(base)
			if (err != nil) != tc.fails || len(got) != tc.entries {
				t.Errorf("got %d entries, err %v; want %d, fails=%v", len(got), err, tc.entries, tc.fails)
			}
		})
	}
	if _, err := readClassIndex(newBase(t)); err == nil || !strings.Contains(err.Error(), "Run ./reindex.sh") {
		t.Errorf("missing class index: %v", err)
	}
}
