package locate

import (
	"os"
	"path/filepath"
)

type startKind int

const (
	cwdStart startKind = iota + 1
	executableStart
)

// Start is one place Find searches from.
type Start struct {
	kind startKind
}

var (
	// FromCwd starts at the working directory.
	FromCwd = Start{kind: cwdStart}
	// FromExecutable starts at the executable's directory, symlinks resolved.
	FromExecutable = Start{kind: executableStart}
)

// Options say where a checkout may be and how to recognise it (see the package doc).
type Options struct {
	Env        []string               // variables naming the checkout, first non-empty wins
	Getenv     func(string) string    // nil: os.Getenv
	Starts     []Start                // searched in order
	Marker     func(dir string) bool  // true for a checkout; nil matches nothing
	Getwd      func() (string, error) // nil: os.Getwd
	Executable func() (string, error) // nil: os.Executable
}

// Find returns the checkout: the first non-empty Env value as given, else the
// first directory the Marker accepts walking up from each start in order.
func Find(o Options) (string, bool) {
	getenv := o.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	if _, v := FirstEnv(getenv, o.Env...); v != "" {
		return v, true
	}
	if o.Marker == nil {
		return "", false
	}
	for _, s := range o.Starts {
		dir, ok := o.startDir(s)
		if !ok {
			continue
		}
		if found, ok := WalkUp(dir, o.Marker); ok {
			return found, true
		}
	}
	return "", false
}

// startDir is the directory s names, or false when it cannot be read.
func (o Options) startDir(s Start) (string, bool) {
	switch s.kind {
	case cwdStart:
		getwd := o.Getwd
		if getwd == nil {
			getwd = os.Getwd
		}
		wd, err := getwd()
		return wd, err == nil && wd != ""
	case executableStart:
		executable := o.Executable
		if executable == nil {
			executable = os.Executable
		}
		exe, err := executable()
		if err != nil || exe == "" {
			return "", false
		}
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		return filepath.Dir(exe), true
	}
	return "", false
}

// FirstEnv returns the first of names whose value is not empty, and that value;
// ("", "") when none is set.
func FirstEnv(getenv func(string) string, names ...string) (name, value string) {
	for _, n := range names {
		if v := getenv(n); v != "" {
			return n, v
		}
	}
	return "", ""
}

// WalkUp returns the first of dir (made absolute) and its parents that marker
// accepts, or ("", false) at the file-system root.
func WalkUp(dir string, marker func(string) bool) (string, bool) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for {
		if marker(dir) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// HasFile accepts a directory holding the regular file rel (symlinks followed).
func HasFile(rel string) func(string) bool {
	return func(dir string) bool {
		st, err := os.Stat(filepath.Join(dir, rel))
		return err == nil && st.Mode().IsRegular()
	}
}

// HasDir accepts a directory holding the directory rel (symlinks followed).
func HasDir(rel string) func(string) bool {
	return func(dir string) bool {
		st, err := os.Stat(filepath.Join(dir, rel))
		return err == nil && st.IsDir()
	}
}

// Exists accepts a directory holding rel of any type (a .git file or folder).
func Exists(rel string) func(string) bool {
	return func(dir string) bool {
		_, err := os.Stat(filepath.Join(dir, rel))
		return err == nil
	}
}

// All accepts a directory every marker accepts.
func All(markers ...func(string) bool) func(string) bool {
	return func(dir string) bool {
		for _, m := range markers {
			if !m(dir) {
				return false
			}
		}
		return true
	}
}
