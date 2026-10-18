package core

import (
	"fmt"
	"os"
	"path/filepath"
)

// NavMarker is the file that identifies a module-navigator checkout: its module
// inventory index.
var NavMarker = filepath.Join("indexes", "module-inventory.json")

// Executable is os.Executable, replaceable in tests.
var Executable = os.Executable

// NavDirVars name the module-navigator checkout, first set wins: an installed
// binary (outside any checkout) is pointed at its indexes with them.
// NAV_DIR is an alias.
var NavDirVars = []string{"MODULE_NAVIGATOR_DIR", "NAV_DIR"}

// LocateNavDir returns the module-navigator checkout: the directory the first
// set NavDirVars variable names (made absolute), which must hold indexes/ (else an error
// naming the variable, "VAR=value: ...", and "" - never a fallback to the
// search); with none set, the search of FindNavDir.
func LocateNavDir() (string, error) {
	for _, name := range NavDirVars {
		v := os.Getenv(name)
		if v == "" {
			continue
		}
		if st, err := os.Stat(filepath.Join(v, "indexes")); err != nil || !st.IsDir() {
			return "", fmt.Errorf("%s=%s: not a module-navigator checkout (no indexes/ directory)", name, v)
		}
		return absPath(v), nil
	}
	return searchNavDir(), nil
}

// FindNavDir is LocateNavDir without its error: "" when a NavDirVars variable
// names no checkout.
func FindNavDir() string {
	dir, _ := LocateNavDir()
	return dir
}

// searchNavDir returns the first of the working directory, the executable's
// directory and their parents that holds indexes/module-inventory.json, or ""
// when none does. Python derives the checkout from the script's own path
// (NAV_DIR); a Go binary has no such path, so it is searched from where it runs.
func searchNavDir() string {
	var starts []string
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}
	if exe, err := Executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		starts = append(starts, filepath.Dir(exe))
	}
	for _, start := range starts {
		for dir := filepath.Clean(start); ; dir = filepath.Dir(dir) {
			if st, err := os.Stat(filepath.Join(dir, NavMarker)); err == nil && st.Mode().IsRegular() {
				return dir
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	return ""
}

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
