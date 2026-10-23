package indexes

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The Java sources the extraction took from module jars (see
// SourceOriginsFile). docSource wins: the source and slots builders index
// every docSource file first and the module-jar files after, so a class
// in both keeps its plain key for docSource and the module-jar copy gets
// the "Name (module)" alternate key the builders already give a second
// same-name class. Without the file every source is docSource and the
// indexes are what they were.

// sourceOrigins is the origins file of a data root: path under source/
// ("/" separators) -> jar name.
type sourceOrigins map[string]string

// readSourceOrigins reads source/<SourceOriginsFile>; a data root without
// it (or without a source/ folder) has no module-jar source. A file that is not a JSON object of strings
// is an error: a wrong origin must not be indexed as docSource silently.
func readSourceOrigins(baseDir string) (sourceOrigins, error) {
	if !pycompat.IsDir(pycompat.Join(baseDir, "source")) {
		return sourceOrigins{}, nil
	}
	path := pycompat.Join(baseDir, "source", SourceOriginsFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return sourceOrigins{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: not an object of path -> jar: %w", path, err)
	}
	return m, nil
}

// jar is the module jar of the file at path (sourceDir absolute), "" for
// docSource.
func (o sourceOrigins) jar(sourceDir, path string) string {
	if len(o) == 0 {
		return ""
	}
	return o[relToSource(sourceDir, path)]
}

// absSourceDir is the absolute source/ folder of baseDir.
func absSourceDir(baseDir string) (string, error) {
	return filepath.Abs(pycompat.Join(baseDir, "source"))
}
