package helpbuild

import "os"

// Helpers only the tests use; production code has no caller for them.

// Len is the number of jars in the cache.
func (ls *Listings) Len() int { return len(ls.jars.Keys) }

// Builders returns the rebuild-all plan in run order (a copy).
func Builders() []builder { return append([]builder(nil), rebuildBuilders...) }

// Load reads the rules data file at path.
func Load(path string) (*Rules, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data, path)
}
