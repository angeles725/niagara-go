package help

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// module-index.json, palette-index.json and tag-index.json are built from the
// install's modules/, which lies outside the data root the manifest
// fingerprints: each records the jar count and bytes it read
// (module_index_builder, palette_index_builder, tag_index_builder), and
// freshness compares them with the install now.

// installIndexes are the indexes built from the install's modules/.
var installIndexes = []string{moduleIndexName, paletteIndexName, tagIndexName}

// installJars counts the jars of dir (*.jar in any letter case, as the builder
// lists them) and their bytes; ok is false when dir cannot be listed.
func installJars(dir string) (jars, bytes int64, ok bool) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, false
	}
	for _, en := range ents {
		if !strings.HasSuffix(textfold.Lower(en.Name()), ".jar") {
			continue
		}
		jars++
		if st, err := os.Stat(filepath.Join(dir, en.Name())); err == nil {
			bytes += st.Size()
		}
	}
	return jars, bytes, true
}

// installIndexLine prints how the index name (one of installIndexes) compares with the modules/ of
// installDir ("" when none is registered); true when it is stale.
func (e *Env) installIndexLine(name, installDir string) bool {
	path := index.Path(e.BaseDir, name)
	if !pycompat.IsFile(path) {
		fmt.Fprintf(e.Out, "  %s: absent (rebuild-all builds it from the install's modules/)\n", name)
		return false
	}
	if installDir == "" {
		fmt.Fprintf(e.Out, "  %s: not compared — no install is registered for this data root\n", name)
		return false
	}
	dir := filepath.Join(installDir, "modules")
	jars, size, ok := installJars(dir)
	if !ok {
		fmt.Fprintf(e.Out, "  %s: not compared — %s is not reachable\n", name, dir)
		return false
	}
	idx, err := index.LoadObject(path)
	var was, wasBytes int64
	if err == nil {
		was, _ = index.Int(get(idx, "jars", nil))
		wasBytes, _ = index.Int(get(idx, "jar_bytes", nil))
	}
	if err == nil && was == jars && wasBytes == size {
		fmt.Fprintf(e.Out, "  %s: matches %s (%s jars, %s bytes)\n", name, dir, pycompat.Commas(int(jars)), pycompat.Commas(int(size)))
		return false
	}
	fmt.Fprintf(e.Out, "  %s: STALE — built from %s jars (%s bytes); %s now holds %s jars (%s bytes)\n", name,
		pycompat.Commas(int(was)), pycompat.Commas(int(wasBytes)), dir, pycompat.Commas(int(jars)), pycompat.Commas(int(size)))
	return true
}

// installIndexLines prints installIndexLine for each of installIndexes
// against the registered install and returns the stale ones.
func (e *Env) installIndexLines() []string {
	var stale []string
	for _, name := range installIndexes {
		if e.installIndexLine(name, e.installDirOf()) {
			stale = append(stale, name)
		}
	}
	return stale
}

// installDirOf is the install folder registered for the selected version, ""
// when there is no registry or entry.
func (e *Env) installDirOf() string {
	if e.RepoRoot == "" || e.VersionID == "" {
		return ""
	}
	reg, err := index.LoadRegistry(e.RepoRoot)
	if err != nil || reg == nil {
		return ""
	}
	entry, ok := reg.Entry(e.VersionID)
	if !ok {
		return ""
	}
	return entry.InstallDir
}
