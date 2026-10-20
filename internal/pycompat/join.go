package pycompat

import (
	"runtime"

	"github.com/angeles725/niagara-go/internal/pypath"
)

// Join is os.path.join for the host: posixpath.join off Windows, ntpath.join on Windows.
// Nothing is normalised, so a "/" inside a part stays as given on Windows.
func Join(first string, rest ...string) string {
	if runtime.GOOS == "windows" {
		return pypath.NtJoin(first, rest...)
	}
	return pypath.PosixJoin(first, rest...)
}
