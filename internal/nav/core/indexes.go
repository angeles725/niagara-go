package core

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// progressMB is the size from which loading an index prints a progress line.
const progressMB = 20

// IndexPath is indexes.path: <base>/indexes/<name>.
func IndexPath(baseDir, name string) string { return filepath.Join(baseDir, "indexes", name) }

// what is _what: "class-index.json" -> "class index" (error messages).
func what(name string) string {
	return strings.ReplaceAll(strings.TrimSuffix(name, filepath.Ext(name)), "-", " ")
}

func absPath(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

// absentOptional is _absent_optional: true, after a one-time NOTE, when the
// optional index at key does not exist.
func (r *Runtime) absentOptional(key, name string) bool {
	if pycompat.IsFile(key) {
		return false
	}
	if !r.noted[key] {
		r.noted[key] = true
		fmt.Fprintf(r.Err, "NOTE: optional %s not found (%s); the output omits what it adds. "+
			"Build it with ./reindex.sh\n", what(name), key)
	}
	return true
}

// LoadOpts are the keyword arguments of indexes.load.
type LoadOpts struct {
	Quiet    bool // no progress line (the REPL prints its own)
	Optional bool // absent: NOTE on stderr and nil, exit status unchanged
}

// Load is indexes.load(base_dir, name): the parsed indexes/<name> (cached for
// the process), or nil after an exit-2 error (or, when optional, a NOTE).
// Indexes of 20 MB or more print "  Loading <name> (N.N MB)... OK" on stderr;
// the class index also warns when it no longer describes the corpus.
func (r *Runtime) Load(name string, opts LoadOpts) *index.Object {
	key := absPath(IndexPath(r.BaseDir, name))
	if o, ok := r.json[key]; ok {
		return o
	}
	if opts.Optional && r.absentOptional(key, name) {
		return nil
	}
	progress := !opts.Quiet && r.progress(key, name)
	var data *index.Object
	if name == "class-index.json" {
		data = r.loadClassIndex(key, what(name))
	} else {
		data = r.LoadJSON(key, what(name))
	}
	if progress {
		if data != nil {
			fmt.Fprint(r.Err, " OK\n")
		} else {
			fmt.Fprint(r.Err, "\n")
		}
	}
	if data == nil {
		return nil
	}
	if name == "class-index.json" {
		for _, w := range r.Staleness(data.Object("_meta"), key) {
			fmt.Fprintf(r.Err, "WARNING: %s\n", w)
		}
	}
	r.json[key] = data
	return data
}

// progress prints Load's "  Loading <name> (N.N MB)..." line when the index
// at key is a file of 20 MB or more; true when it did.
func (r *Runtime) progress(key, name string) bool {
	st, err := os.Stat(key)
	if err != nil || !st.Mode().IsRegular() {
		return false
	}
	sizeMB := float64(st.Size()) / (1024 * 1024)
	if sizeMB < progressMB {
		return false
	}
	fmt.Fprintf(r.Err, "  Loading %s (%s MB)...", name, Fixed(sizeMB, 1))
	return true
}

// LoadedFromCache prints what Load prints on stderr when it parses
// indexes/<name> successfully (its progress line), for an index a command
// took from its own cache instead, so stderr is the same on a hit and on a
// miss. The class index, whose Load also warns about staleness, is not one.
func (r *Runtime) LoadedFromCache(name string) {
	if r.progress(absPath(IndexPath(r.BaseDir, name)), name) {
		fmt.Fprint(r.Err, " OK\n")
	}
}

// LoadJSON is outcome.load_json: the JSON object at path, or nil after an
// exit-2 error naming it (a truncated index must not look like an empty search).
func (r *Runtime) LoadJSON(path, what string) *index.Object {
	if !pycompat.IsFile(path) {
		r.Error(fmt.Sprintf("%s not found: %s", what, path), "Run: ./reindex.sh")
		return nil
	}
	v, err := index.LoadPython(path)
	if err == nil {
		if o, ok := v.(*index.Object); ok {
			return o
		}
		err = fmt.Errorf("the top-level JSON value is not an object")
	}
	r.Error(fmt.Sprintf("%s is unreadable: %s (%v)", what, path, err), "Run: ./reindex.sh")
	return nil
}

// Connect is indexes.connect: a read-only connection to indexes/<name>
// (cached), or nil after an exit-2 error (or, when optional, a NOTE).
func (r *Runtime) Connect(name string, optional bool) *sql.DB {
	key := absPath(IndexPath(r.BaseDir, name))
	if db, ok := r.db[key]; ok {
		return db
	}
	if optional && r.absentOptional(key, name) {
		return nil
	}
	if !pycompat.IsFile(key) {
		r.Error(fmt.Sprintf("%s not found: %s", what(name), key), "Run: ./reindex.sh")
		return nil
	}
	db, err := index.OpenSQLite(key)
	if err != nil {
		r.Unreadable(err)
		return nil
	}
	r.db[key] = db
	return db
}
