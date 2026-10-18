package main

import (
	"reflect"
	"testing"
)

func TestReindexArgs(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want []string
		ok   bool
	}{
		{[]string{"reindex", "--check"}, []string{"--check"}, true},
		{[]string{"--base-dir", "/n", "reindex", "--only", "x"}, []string{"--base-dir", "/n", "--only", "x"}, true},
		{[]string{"-d", "/n", "reindex"}, []string{"--base-dir", "/n"}, true},
		{[]string{"--base-dir=/n", "reindex"}, []string{"--base-dir", "/n"}, true},
		{[]string{"class", "reindex"}, nil, false},
		{[]string{"--base-dir", "/n", "class"}, nil, false},
		{nil, nil, false},
	} {
		got, ok := reindexArgs(tc.args)
		if ok != tc.ok || (ok && !reflect.DeepEqual(got, tc.want)) {
			t.Errorf("reindexArgs(%q) = %q, %v; want %q, %v", tc.args, got, ok, tc.want, tc.ok)
		}
	}
}
