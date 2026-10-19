package helpbuild

import (
	"os"
	"strings"
)

// Helpers only the tests use; production code has no caller for them.

// Len is the number of jars in the cache.
func (ls *Listings) Len() int { return len(ls.jars.Keys) }

// Stdout is the text the run printed: the Log entries in order, each
// followed by a newline except a progress entry (one starting with "\r",
// written without a newline).
func (r *PDFResult) Stdout() string {
	var b strings.Builder
	for _, e := range r.Log {
		b.WriteString(e)
		if !strings.HasPrefix(e, "\r") {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// Builders returns the rebuild-all plan in run order (a copy).
func Builders() []Builder { return append([]Builder(nil), rebuildBuilders...) }

// Load reads the rules data file at path.
func Load(path string) (*Rules, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data, path)
}
