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

// module-index.json is built from the install's modules/, which lies outside
// the data root the manifest fingerprints: the index records the jar count and
// bytes it read (module_index_builder), and freshness compares them with the
// install now.

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

// moduleIndexLine prints how module-index.json compares with the modules/ of
// installDir ("" when none is registered); true when it is stale.
func (e *Env) moduleIndexLine(installDir string) bool {
	path := index.Path(e.BaseDir, moduleIndexName)
	if !pycompat.IsFile(path) {
		fmt.Fprintln(e.Out, "  module-index.json: absent (rebuild-all builds it from the install's modules/)")
		return false
	}
	if installDir == "" {
		fmt.Fprintln(e.Out, "  module-index.json: not compared — no install is registered for this data root")
		return false
	}
	dir := filepath.Join(installDir, "modules")
	jars, size, ok := installJars(dir)
	if !ok {
		fmt.Fprintf(e.Out, "  module-index.json: not compared — %s is not reachable\n", dir)
		return false
	}
	idx, err := index.LoadObject(path)
	var was, wasBytes int64
	if err == nil {
		was, _ = index.Int(get(idx, "jars", nil))
		wasBytes, _ = index.Int(get(idx, "jar_bytes", nil))
	}
	if err == nil && was == jars && wasBytes == size {
		fmt.Fprintf(e.Out, "  module-index.json: matches %s (%s jars, %s bytes)\n", dir, pycompat.Commas(int(jars)), pycompat.Commas(int(size)))
		return false
	}
	fmt.Fprintf(e.Out, "  module-index.json: STALE — built from %s jars (%s bytes); %s now holds %s jars (%s bytes)\n",
		pycompat.Commas(int(was)), pycompat.Commas(int(wasBytes)), dir, pycompat.Commas(int(jars)), pycompat.Commas(int(size)))
	return true
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
