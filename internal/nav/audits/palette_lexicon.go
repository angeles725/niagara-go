package audits

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/modjar"
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
	pycompat.Walk(extracted, func(dir string, dirs, files []string) []string {
		rel := strings.TrimPrefix(filepath.ToSlash(dir[len(extracted):]), "/")
		for _, f := range files {
			if !strings.HasSuffix(f, ".lexicon") {
				continue
			}
			names++
			if text, ok := readExtracted(pycompat.Join(dir, f)); ok {
				found = append(found, lexiconFile{strings.TrimPrefix(rel+"/"+f, "/"), text})
			}
		}
		return dirs
	})
	if names == 0 {
		found = jarLexicons(jar)
	}
	sort.Slice(found, func(i, j int) bool { return found[i].rel < found[j].rel })
	return found
}

// jarLexicons is the JAR half of _lexicons: each *.lexicon member name once,
// read from its last entry as zipfile does, until a read fails.
func jarLexicons(jar string) []lexiconFile {
	if st, err := os.Stat(jar); err != nil || !st.Mode().IsRegular() {
		return nil
	}
	j, err := modjar.Open(jar, modjar.WithNameFunc(core.ZipName))
	if err != nil {
		return nil
	}
	defer j.Close()
	last := map[string]int{}
	var order []string
	for i, n := range j.Names() {
		if !strings.HasSuffix(n, ".lexicon") {
			continue
		}
		if _, seen := last[n]; !seen {
			order = append(order, n)
		}
		last[n] = i
	}
	var out []lexiconFile
	for _, n := range order {
		b, err := j.Read(last[n], 0)
		if err != nil {
			break
		}
		out = append(out, lexiconFile{n, pycompat.DecodeReplace(b)})
	}
	return out
}

// countDirs is the number of entries of dir that are directories (symlinks
// followed): the script's "module dirs" scope count.
func countDirs(dir string) int {
	names, _ := pycompat.Listdir(dir)
	n := 0
	for _, name := range names {
		if pycompat.IsDir(filepath.Join(dir, name)) {
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
	names, _ := pycompat.SortedListdir(base)
	var rows []censusRow
	for _, m := range names {
		dir := pycompat.Join(base, m)
		if !pycompat.IsDir(dir) || len(findArtifacts(dir)) == 0 {
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
