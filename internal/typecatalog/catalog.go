package typecatalog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
	"github.com/angeles725/niagara-go/internal/slotomatic"
)

// Record is one catalog entry: a type's own declarations and the file they were parsed from.
type Record struct {
	slotomatic.Type
	Source string
}

// Catalog maps pkg.Class to its record, keeping the order entries were added (os.walk order for
// a built catalog, file order for a loaded one), like the Python dict.
type Catalog struct {
	Keys []string
	m    map[string]*Record // a built catalog
	raw  map[string]any     // a loaded catalog file: the json.load value of each entry
}

// Get returns the record of a built catalog's key, nil when absent or loaded from a file.
func (c *Catalog) Get(key string) *Record { return c.m[key] }

// Len is the number of types.
func (c *Catalog) Len() int { return len(c.Keys) }

func (c *Catalog) setDefault(key string, r *Record) {
	if c.m == nil {
		c.m = map[string]*Record{}
	}
	if _, ok := c.m[key]; !ok {
		c.Keys = append(c.Keys, key)
		c.m[key] = r
	}
}

// Build is build_catalog: every type with at least one declaration under roots (the first file
// of a pkg.Class wins), and the number of .java files read.
func Build(roots []string) (*Catalog, int) {
	cat := &Catalog{}
	nfiles := 0
	for _, root := range roots {
		pycompat.WalkWith(root, pycompat.WalkOptions{Unsorted: true}, func(dir string, dirs, files []string) []string {
			for _, fn := range files {
				if !strings.HasSuffix(fn, ".java") {
					continue
				}
				path := slotomatic.Join(dir, fn)
				nfiles++
				text, err := slotomatic.ReadSource(path)
				if err != nil {
					continue
				}
				t := slotomatic.ParseSource(text)
				if len(t.Properties)+len(t.Actions)+len(t.Topics) == 0 || t.Class == "" {
					continue
				}
				cat.setDefault(Key(t), &Record{Type: t, Source: path})
			}
			return dirs
		})
	}
	return cat, nfiles
}

// Key is the catalog key of a type: "<package>.<Class>", or the bare class without a package.
func Key(t slotomatic.Type) string {
	if t.Package == "" {
		return t.Class
	}
	return t.Package + "." + t.Class
}

// JSON is json.dumps(cat, indent=1, sort_keys=True): the keys are sorted here, so the dump keeps
// insertion order.
func (c *Catalog) JSON() string {
	keys := append([]string(nil), c.Keys...)
	sort.Strings(keys)
	top := make(pyjson.Obj, len(keys))
	for i, k := range keys {
		r := c.m[k]
		top[i] = pyjson.KV{Key: k, Val: pyjson.Obj{
			{"actions", decls(r.Actions)},
			{"class", r.Class},
			{"extends", r.Extends},
			{"package", r.Package},
			{"properties", decls(r.Properties)},
			{"source", r.Source},
			{"topics", decls(r.Topics)},
		}}
	}
	return pyjson.DumpsAny(top, pyjson.Defaults(1))
}

func decls(ds []slotomatic.Decl) []any {
	out := make([]any, len(ds))
	for i, d := range ds {
		args := d.Args
		if args == nil {
			args = []string{}
		}
		flags := pyjson.Value{Kind: pyjson.Int, I: d.FlagValue()} // a Python int: no bound
		out[i] = pyjson.Obj{{"args", args}, {"default", d.Default}, {"facets", d.Facets},
			{"flagLetters", d.FlagLetters}, {"flags", flags}, {"name", d.Name}}
	}
	return out
}

// loadJSON reads a catalog file as json.load does, keeping its key order (a repeated key keeps
// its first position and its last value). Entries stay Python values (see pyvalue.go).
func loadJSON(r io.Reader) (*Catalog, error) {
	dec := json.NewDecoder(bufio.NewReader(r))
	dec.UseNumber()
	v, err := decodeValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("extra data after the JSON value")
	}
	d, ok := v.(*pyDict)
	if !ok {
		return nil, fmt.Errorf("not a JSON object")
	}
	cat := &Catalog{raw: map[string]any{}}
	for _, k := range d.keys {
		cat.Keys = append(cat.Keys, k)
		cat.raw[k] = d.m[k]
	}
	return cat, nil
}

// Load is _load_catalog: a directory is built, anything else is read as a catalog JSON file.
func Load(arg string) (*Catalog, error) {
	if st, err := os.Stat(arg); err == nil && st.IsDir() {
		cat, _ := Build([]string{arg})
		return cat, nil
	}
	f, err := os.Open(arg)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return loadJSON(f)
}
