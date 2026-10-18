package audits

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/registry"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// lexiconFile is one *.lexicon of an artifact: its path relative to
// extracted/ (or its JAR member name) and its text.
type lexiconFile struct{ rel, text string }

// artifactLexicons is _lexicons: every *.lexicon under extracted/ (os.walk,
// symlinked directories not entered), else, when none is there, every
// *.lexicon member of the JAR; sorted by relative name. A name found but
// unreadable still keeps the JAR out, as in the script.
func artifactLexicons(extracted, jar string) []lexiconFile {
	var found []lexiconFile
	names := 0
	walkLexicons(extracted, "", func(rel, path string) {
		names++
		if text, ok := readExtracted(path); ok {
			found = append(found, lexiconFile{rel, text})
		}
	})
	if names == 0 {
		found = jarLexicons(jar)
	}
	sort.Slice(found, func(i, j int) bool { return found[i].rel < found[j].rel })
	return found
}

// walkLexicons calls fn for every non-directory entry named *.lexicon under
// dir, recursing into real directories only (os.walk with followlinks off,
// errors ignored).
func walkLexicons(dir, rel string, fn func(rel, path string)) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		path, name := filepath.Join(dir, e.Name()), e.Name()
		if rel != "" {
			name = rel + "/" + name
		}
		switch {
		case e.IsDir():
			walkLexicons(path, name, fn)
		case e.Type()&os.ModeSymlink != 0 && isDir(path):
			// os.walk lists a symlinked directory as a directory and does not enter it
		case strings.HasSuffix(e.Name(), ".lexicon"):
			fn(name, path)
		}
	}
}

// jarLexicons is the JAR half of _lexicons: each *.lexicon member name once,
// read from its last entry as zipfile does, until a read fails.
func jarLexicons(jar string) []lexiconFile {
	if st, err := os.Stat(jar); err != nil || !st.Mode().IsRegular() {
		return nil
	}
	zr, err := zip.OpenReader(jar)
	if err != nil {
		return nil
	}
	defer zr.Close()
	last := map[string]*zip.File{}
	var order []string
	for _, f := range zr.File {
		n := core.ZipName(f)
		if !strings.HasSuffix(n, ".lexicon") {
			continue
		}
		if last[n] == nil {
			order = append(order, n)
		}
		last[n] = f
	}
	var out []lexiconFile
	for _, n := range order {
		text, ok := readZipFile(last[n])
		if !ok {
			break
		}
		out = append(out, lexiconFile{n, text})
	}
	return out
}

// readZipFile is a member's bytes decoded as UTF-8 with replacement.
func readZipFile(f *zip.File) (string, bool) {
	rc, err := f.Open()
	if err != nil {
		return "", false
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		return "", false
	}
	return pycompat.DecodeReplace(b), true
}

// countDirs is the number of entries of dir that are directories (symlinks
// followed): the script's "module dirs" scope count.
func countDirs(dir string) int {
	f, err := os.Open(dir)
	if err != nil {
		return 0
	}
	names, _ := f.Readdirnames(-1)
	f.Close()
	n := 0
	for _, name := range names {
		if isDir(filepath.Join(dir, name)) {
			n++
		}
	}
	return n
}

// sortedKeys is sorted(o) for an object of duplicate keys.
func sortedKeys(o *index.Object) []string {
	keys := append([]string(nil), o.Keys()...)
	sort.Strings(keys)
	return keys
}

// dupCount is the occurrence count stored under k.
func dupCount(o *index.Object, k string) int64 {
	v, _ := o.Get(k)
	n, _ := v.(int64)
	return n
}

// censusRow is one module of the --all census.
type censusRow struct {
	module                  string
	palette, lexicon, agent int
	dups                    *index.Object // "<artifact>:<key>" -> count
}

// paletteCensus is _census: one summary row per module directory of base
// holding an artifact, and the duplicate-key roll-up.
func paletteCensus(env *registry.Env, base string, asJSON bool) int {
	f, err := os.Open(base)
	var names []string
	if err == nil {
		names, _ = f.Readdirnames(-1)
		f.Close()
	}
	sort.Strings(names)
	var rows []censusRow
	for _, m := range names {
		dir := pycompat.PosixJoin(base, m)
		if !isDir(dir) || len(findArtifacts(dir)) == 0 {
			continue
		}
		r := censusRow{module: m, dups: index.NewObject()}
		for _, d := range collectModule(base, m) {
			r.palette += len(d.palette)
			r.lexicon += d.lexiconKeys
			r.agent += len(d.agents)
			for _, k := range d.dups.Keys() {
				r.dups.Set(d.artifact+":"+k, dupCount(d.dups, k))
			}
		}
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		fmt.Fprintf(env.Err, "ERROR: 0 modules with artifacts; searched %s\n", base)
		return core.StatusMiss
	}
	if asJSON {
		list := make([]any, len(rows))
		for i, r := range rows {
			o := index.NewObject()
			o.Set("module", r.module)
			o.Set("palette", int64(r.palette))
			o.Set("lexicon_keys", int64(r.lexicon))
			o.Set("dup_keys", r.dups)
			o.Set("agents", int64(r.agent))
			list[i] = o
		}
		o := index.NewObject()
		o.Set("base_dir", base)
		o.Set("modules", list)
		fmt.Fprintln(env.Out, core.Dumps(o, 2))
		return env.ExitCode()
	}
	w := env.Out
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %-24s %8s %8s %8s %7s\n", "module", "palette", "lexKeys", "dupKeys", "agents")
	fmt.Fprintln(w, "  "+strings.Repeat("-", 60))
	var dirty []censusRow
	for _, r := range rows {
		fmt.Fprintf(w, "  %-24s %8d %8d %8d %7d\n", r.module, r.palette, r.lexicon, r.dups.Len(), r.agent)
		if r.dups.Len() > 0 {
			dirty = append(dirty, r)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Modules with DUPLICATE bare lexicon keys (B759 hazard): %d / %d\n", len(dirty), len(rows))
	for _, r := range dirty {
		for _, k := range sortedKeys(r.dups) {
			fmt.Fprintf(w, "    %s  %s  x%d\n", r.module, k, dupCount(r.dups, k))
		}
	}
	return env.ExitCode()
}
