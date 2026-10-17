package nav

import (
	"os"
	"path/filepath"
)

// navMarker is the file that identifies a module-navigator checkout: its class
// inventory index.
var navMarker = filepath.Join("indexes", "module-inventory.json")

// FindNavDir returns the module-navigator checkout: the first of the working directory,
// the executable's directory and their parents that holds indexes/module-inventory.json,
// or "" when none does. Python derives it from the script's own path (NAV_DIR); a Go
// binary has no such path, so the checkout is searched from where it runs.
func FindNavDir() string {
	var starts []string
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}
	if exe, err := executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		starts = append(starts, filepath.Dir(exe))
	}
	for _, start := range starts {
		for dir := filepath.Clean(start); ; dir = filepath.Dir(dir) {
			if st, err := os.Stat(filepath.Join(dir, navMarker)); err == nil && st.Mode().IsRegular() {
				return dir
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	return ""
}

// DetectBaseDir is detect_base_dir(): the checkout holding indexes/, or "".
func DetectBaseDir() string { return FindNavDir() }

// OrganizedDir is corpus.organized_dir(): $NAV_CORPUS_BASE when set (the only path
// tried, so a wrong override is an error, never a silent fallback), else the
// organized/ directory next to the module-navigator checkout. It returns "" when
// neither is known. The path may not exist.
func OrganizedDir() string {
	if v := os.Getenv("NAV_CORPUS_BASE"); v != "" {
		return v
	}
	nav := FindNavDir()
	if nav == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(nav), "organized")
}
