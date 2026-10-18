package bog

import (
	"path/filepath"
	"reflect"
	"testing"
)

const nestedFixture = "testdata/nested/composite.xml"

func loadNested(t *testing.T, opts Options) *Bog {
	t.Helper()
	b, err := LoadOptions(nestedFixture, opts)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func nestedPairs(c *Comp) [][2]any { return pairs(&c.Nested) }

func TestNestedSlotsRecordsCompositeChildrenByDotPath(t *testing.T) {
	b := loadNested(t, Options{NestedSlots: true})
	tests := []struct {
		name   string
		handle string
		want   [][2]any
	}{
		{
			name:   "deep composite children, decoded names, flags-only and link children skipped",
			handle: "3",
			want: [][2]any{
				{"tcpConfig.address.host", "10.0.0.5"},
				{"tcpConfig.address.port", "4911"},
				{"tcpConfig.address.alt port", "8080"},
				{"tcpConfig.timeout", "5000"},
			},
		},
		{
			name:   "component inside a network roots its own composites",
			handle: "5",
			want:   [][2]any{{"pollConfig.rate", "fast"}},
		},
		{
			name:   "value and status of a StatusNumeric, fallback and plain value slots excluded",
			handle: "4",
			want: [][2]any{
				{"setpoint.value", "21.5"},
				{"setpoint.status", "0;activeLevel=e:17@control:PriorityLevel"},
			},
		},
		{
			name:   "component without composites",
			handle: "2",
			want:   [][2]any{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, ok := b.Comps.Get(tt.handle)
			if !ok {
				t.Fatalf("h:%s missing", tt.handle)
			}
			if got := nestedPairs(c); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Nested = %v, want %v", got, tt.want)
			}
		})
	}
	web, _ := b.Comps.Get("3")
	if v, ok := web.Nested.Get("tcpConfig.address.port"); !ok || v != "4911" {
		t.Fatalf(`Nested.Get("tcpConfig.address.port") = %q, %v`, v, ok)
	}
}

func TestNestedSlotsIsOffByDefault(t *testing.T) {
	for _, b := range []*Bog{loadNested(t, Options{}), loadFixture(t, "../nested/composite.xml")} {
		for _, c := range b.Comps.All() {
			if c.Nested.Len() != 0 {
				t.Fatalf("h:%s Nested = %v without Options.NestedSlots", c.Handle, nestedPairs(c))
			}
		}
	}
}

// The option only adds Comp.Nested: every other part of the graph is the model.py one.
func TestNestedSlotsLeavesTheGraphUnchanged(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "model", "*"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, nestedFixture)
	for _, path := range files {
		if filepath.Ext(path) == ".json" {
			continue
		}
		t.Run(filepath.Base(path), func(t *testing.T) {
			text, err := ReadText(path)
			if err != nil {
				t.Fatal(err)
			}
			plain := dumpBog(Parse(path, text), -1)
			nested := dumpBog(ParseOptions(path, text, Options{NestedSlots: true}), -1)
			if !reflect.DeepEqual(plain, nested) {
				t.Fatal("Options{NestedSlots: true} changed the model.py graph")
			}
		})
	}
}
