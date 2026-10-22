package helpbuild

import (
	"os"
	"path/filepath"

	"github.com/angeles725/niagara-go/internal/atomicfile"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// textPage is one clean-text file a derived full-text source writes: its
// path under the source folder ("/"-separated) and its text.
type textPage struct{ rel, text string }

// writeTextSource replaces baseDir/<dir>/ with pages, built in a sibling
// staging tree and swapped in (atomicfile.RebuiltDir), so a failed build
// keeps the previous tree. With no pages the folder is removed instead: a
// version without that kind of documentation has no empty source folder
// (and the full-text fingerprint does not list one). A symlink in place of
// the folder is refused, like the course copies.
func writeTextSource(baseDir, dir string, pages []textPage) error {
	final := filepath.Join(baseDir, dir)
	if st, err := os.Lstat(final); err == nil && st.Mode()&os.ModeSymlink != 0 {
		return errRmtreeSymlink{final}
	}
	if len(pages) == 0 {
		if pycompat.IsDir(final) {
			return os.RemoveAll(final)
		}
		return nil
	}
	return atomicfile.RebuiltDir(final, func(tmp string) error {
		for _, p := range pages {
			path := filepath.Join(tmp, filepath.FromSlash(p.rel))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := atomicfile.WriteText(path, p.text); err != nil {
				return err
			}
		}
		return nil
	})
}
