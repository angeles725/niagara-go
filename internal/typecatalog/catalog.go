package typecatalog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

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
	m    map[string]*Record
}

// Get returns the record of key, nil when absent.
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

func (c *Catalog) set(key string, r *Record) {
	if c.m == nil {
		c.m = map[string]*Record{}
	}
	if _, ok := c.m[key]; !ok {
		c.Keys = append(c.Keys, key)
	}
	c.m[key] = r
}

// Build is build_catalog: every type with at least one declaration under roots (the first file
// of a pkg.Class wins), and the number of .java files read.
func Build(roots []string) (*Catalog, int) {
	cat := &Catalog{}
	nfiles := 0
	for _, root := range roots {
		slotomatic.Walk(root, false, func(dir string, _ *[]string, files []string) {
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

// JSON is json.dumps(cat, indent=1, sort_keys=True).
func (c *Catalog) JSON() string {
	keys := append([]string(nil), c.Keys...)
	sort.Strings(keys)
	top := make([]kv, len(keys))
	for i, k := range keys {
		r := c.m[k]
		top[i] = kv{k, []kv{
			{"actions", decls(r.Actions)},
			{"class", r.Class},
			{"extends", r.Extends},
			{"package", r.Package},
			{"properties", decls(r.Properties)},
			{"source", r.Source},
			{"topics", decls(r.Topics)},
		}}
	}
	return dumps(top, 1)
}

func decls(ds []slotomatic.Decl) []any {
	out := make([]any, len(ds))
	for i, d := range ds {
		args := d.Args
		if args == nil {
			args = []string{}
		}
		out[i] = []kv{{"args", args}, {"default", d.Default}, {"facets", d.Facets},
			{"flagLetters", d.FlagLetters}, {"flags", d.Flags}, {"name", d.Name}}
	}
	return out
}

// loadJSON reads a catalog file written by build, keeping its key order. Only the fields show
// prints are kept.
func loadJSON(r io.Reader) (*Catalog, error) {
	dec := json.NewDecoder(bufio.NewReader(r))
	dec.UseNumber()
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, fmt.Errorf("not a JSON object")
	}
	cat := &Catalog{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		var raw struct {
			Package, Class, Extends, Source string
			Properties, Actions, Topics     []struct {
				Name, FlagLetters, Default, Facets string
				Flags                              json.Number
			}
		}
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		rec := &Record{Source: raw.Source}
		rec.Package, rec.Class, rec.Extends = raw.Package, raw.Class, raw.Extends
		conv := func(in []struct {
			Name, FlagLetters, Default, Facets string
			Flags                              json.Number
		}) []slotomatic.Decl {
			out := make([]slotomatic.Decl, len(in))
			for i, d := range in {
				f, _ := d.Flags.Int64()
				out[i] = slotomatic.Decl{Name: d.Name, Flags: f, FlagLetters: d.FlagLetters, Default: d.Default, Facets: d.Facets}
			}
			return out
		}
		rec.Properties, rec.Actions, rec.Topics = conv(raw.Properties), conv(raw.Actions), conv(raw.Topics)
		cat.set(tok.(string), rec)
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
