package fulltext

import (
	"reflect"
	"testing"
)

func TestSplitHeader(t *testing.T) {
	tests := []struct {
		name   string
		lines  []string
		header map[string]string
		start  int
	}{
		{"no header", []string{"plain", ""}, nil, 0},
		{"header then blank line", []string{"TITLE: A", "", "body"}, map[string]string{"TITLE": "A"}, 2},
		{"header without blank line", []string{"TITLE: A", "body"}, map[string]string{"TITLE": "A"}, 1},
		{"first repeated key wins", []string{"TITLE: A", "TITLE: B"}, map[string]string{"TITLE": "A"}, 2},
		{"key with a space, value stripped", []string{"MY KEY: v \x1f", "　", "b"}, map[string]string{"MY KEY": "v"}, 2},
		{"lower-case key is body", []string{"title: x"}, nil, 0},
		{"key must reach a colon", []string{"TITLE x: y"}, nil, 0},
		{"empty value", []string{"TITLE:", ""}, map[string]string{"TITLE": ""}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header, start := splitHeader(tt.lines)
			if !reflect.DeepEqual(header, tt.header) || start != tt.start {
				t.Errorf("splitHeader = %v, %d; want %v, %d", header, start, tt.header, tt.start)
			}
		})
	}
}

func TestTitles(t *testing.T) {
	tests := []struct {
		name, source, rel string
		header            map[string]string
		title, titleText  string
	}{
		{"title header", "guides-clean", "a/page.txt", map[string]string{"TITLE": "Page"}, "Page", "Page page"},
		{"class and package", "bajadoc-clean", "x/BFoo.txt", map[string]string{"CLASS": "BFoo", "PACKAGE": "x"}, "BFoo (x)", "BFoo"},
		{"class only", "bajadoc-clean", "BFoo.txt", map[string]string{"CLASS": "BFoo"}, "BFoo", "BFoo"},
		{"PDF text stem", "docs-text", "Modbus_Guide.txt", nil, "Modbus Guide", "Modbus Guide"},
		{"course text path", "course-text", "day1/lab_1.txt", nil, "Course: day1/lab_1", "Course: day1/lab_1"},
		{"source stem", "source", "javax/baja/Foo.java", nil, "Foo", "Foo"},
		{"devguide-rc stem over title", "devguide-rc", "notes.txt", map[string]string{"TITLE": "RC"}, "RC", "notes"},
		{"dot file keeps its name", "guides-clean", ".hidden.txt", nil, ".hidden", ".hidden .hidden"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title := docTitle(tt.source, tt.rel, tt.header)
			if text := indexTitle(tt.source, tt.rel, tt.header, title); title != tt.title || text != tt.titleText {
				t.Errorf("titles = %q, %q; want %q, %q", title, text, tt.title, tt.titleText)
			}
		})
	}
}

func TestDocumentsPagesAndLines(t *testing.T) {
	type got struct {
		body       string
		line, page int
	}
	tests := []struct {
		name, source, text string
		want               []got
	}{
		{"one document after the header", "guides-clean", "TITLE: T\n\nbody\n", []got{{"body\n", 3, 0}}},
		{"blank pages skipped, numbering kept", "docs-text", "p1\n\f\n \n\fp3\nx\n\fp4",
			[]got{{"p1\n", 1, 1}, {"p3\nx\n", 4, 3}, {"p4", 6, 4}}},
		{"page 1 keeps the header", "course-text", "TITLE: T\n\nslide\n\fnext",
			[]got{{"TITLE: T\n\nslide\n", 1, 1}, {"next", 4, 2}}},
		{"paged source without form feed", "docs-text", "TITLE: T\n\nonly\n", []got{{"only\n", 3, 0}}},
		{"form feed outside a paged source", "guides-clean", "a\fb", []got{{"a\fb", 1, 0}}},
		{"empty file", "guides-clean", "", []got{{"", 1, 0}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out []got
			for _, d := range documents(tt.source, "f.txt", tt.text) {
				out = append(out, got{d.body, d.line, d.page})
			}
			if !reflect.DeepEqual(out, tt.want) {
				t.Errorf("documents = %+v, want %+v", out, tt.want)
			}
		})
	}
}
