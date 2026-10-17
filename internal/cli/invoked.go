package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// InvokedPath is the executable as it was started, the way a Python script sees its own
// __file__: argv[0] made absolute, or looked up on $PATH when it has no path separator;
// os.Executable when that lookup finds nothing. Symlinks are not resolved.
func InvokedPath() (string, error) {
	return invokedPath(os.Args[0], os.Getenv("PATH"), os.Executable)
}

func invokedPath(arg0, pathEnv string, executable func() (string, error)) (string, error) {
	if !strings.ContainsRune(arg0, filepath.Separator) && !strings.ContainsRune(arg0, '/') {
		found, err := lookPath(arg0, pathEnv)
		if err != nil {
			return executable()
		}
		arg0 = found
	}
	return filepath.Abs(arg0)
}

// lookPath finds name as the shell would: the first $PATH entry holding an executable,
// non-directory file of that name ("" in the list is the current directory).
func lookPath(name, pathEnv string) (string, error) {
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			dir = "."
		}
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return p, nil
		}
	}
	return "", fs.ErrNotExist
}
