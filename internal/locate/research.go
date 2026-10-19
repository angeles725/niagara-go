package locate

import (
	"os"
	"path/filepath"
	"strings"
)

// Files that mark a niagara-research checkout (see ResearchCheckout).
const (
	researchCatalog      = "CATALOG.md"
	researchBloquePrefix = "niagara-mental-model-bloque"
	researchBloqueSuffix = ".md"
)

// researchLegacy is a compatibility fallback marker: the tools/corpus-nav.py shim.
// The corpus files are the marker; this one can go once no checkout relies on it.
var researchLegacy = filepath.Join("tools", "corpus-nav.py")

// ResearchCheckout accepts a niagara-research checkout without depending on any
// Python file: dir holds CATALOG.md as a regular file and at least one regular
// file niagara-mental-model-bloque*.md at its top level (what corpus-nav reads).
// As a compatibility fallback only, tools/corpus-nav.py as any non-directory also
// marks a checkout (the Python seam's is_file; no checkout holds a device there).
// Symlinks are followed. A repository that is not the corpus but holds both a
// CATALOG.md and a bloque file at its top level is a false positive.
func ResearchCheckout(dir string) bool {
	if HasFile(researchCatalog)(dir) && hasBloque(dir) {
		return true
	}
	st, err := os.Stat(filepath.Join(dir, researchLegacy))
	return err == nil && !st.IsDir()
}

// hasBloque reports whether dir holds a regular file niagara-mental-model-bloque*.md.
// It reads the directory's names in batches and stops at the first match.
func hasBloque(dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return false
	}
	defer f.Close()
	for {
		names, err := f.Readdirnames(256)
		for _, n := range names {
			if strings.HasPrefix(n, researchBloquePrefix) && strings.HasSuffix(n, researchBloqueSuffix) &&
				HasFile(n)(dir) {
				return true
			}
		}
		if err != nil { // io.EOF at the end, or an unreadable directory
			return false
		}
	}
}
