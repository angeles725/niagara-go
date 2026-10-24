package helpbuild

import (
	"fmt"
	"os"
	"strings"

	"github.com/angeles725/niagara-go/internal/helpbuild/modulexml"
	"github.com/angeles725/niagara-go/internal/helpbuild/zipfile"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// jarMember is one member of a jar and its bytes, or why they cannot be read.
type jarMember struct {
	name string
	data []byte
	err  error
}

// readMembers reads the members of jar whose name match accepts (membersOf);
// err is for a jar that cannot be opened.
func readMembers(jar string, match func(name string) bool) ([]jarMember, error) {
	z, err := zipfile.Open(jar)
	if err != nil {
		return nil, err
	}
	defer z.Close()
	return membersOf(z, match), nil
}

// membersOf reads the members of the open jar z whose name match accepts, in
// the order they first appear; a repeated name reads its last copy, as
// ZipFile.open does. A member that cannot be inflated carries its error.
func membersOf(z *zipfile.File, match func(name string) bool) []jarMember {
	last := map[string]int{}
	var order []string
	for i := range z.Infos {
		name := z.Infos[i].Name
		if !match(name) {
			continue
		}
		if _, seen := last[name]; !seen {
			order = append(order, name)
		}
		last[name] = i
	}
	out := make([]jarMember, len(order))
	for i, name := range order {
		data, err := z.ReadEntry(&z.Infos[last[name]], true)
		out[i] = jarMember{name, data, err}
	}
	return out
}

// moduleOf is the module of a jar: its descriptor's moduleName, else the jar
// name before its last "-".
func moduleOf(jar string, mod *modulexml.Module) string {
	if mod != nil && mod.Part != "" {
		return mod.Part
	}
	stem := strings.TrimSuffix(jar, ".jar")
	if i := strings.LastIndexByte(stem, '-'); i > 0 {
		stem = stem[:i]
	}
	return stem
}

// jarCensus is what an install-fed index counts of every jar of modules/
// (jars and jar_bytes, which freshness compares with the install) and the
// "errors" rows of what it could not read, each also a warning of script.
type jarCensus struct {
	script      string
	jars, bytes int
	errors      []pyjson.Value
}

// count counts one jar and returns its file name.
func (c *jarCensus) count(jar string) string {
	c.jars++
	if st, err := os.Stat(jar); err == nil {
		c.bytes += int(st.Size())
	}
	return pyBasename(jar)
}

// fail records why jar (or its member path, when not "") cannot be read.
func (c *jarCensus) fail(res *textResult, jar, path string, err error) {
	str := pyjson.String
	c.errors = append(c.errors, pyjson.ObjectOf(pyjson.Field{Key: "jar", Value: str(jar)},
		pyjson.Field{Key: "path", Value: str(path)}, pyjson.Field{Key: "error", Value: str(err.Error())}))
	where := jar
	if path != "" {
		where += "!" + path
	}
	res.Warnings = append(res.Warnings, fmt.Sprintf("WARNING: %s: %s: %v", c.script, where, err))
}
