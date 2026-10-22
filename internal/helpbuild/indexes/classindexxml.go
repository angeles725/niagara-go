package indexes

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/angeles725/niagara-go/internal/bajadocxml"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// bajadoc_xml.iter_classes for the class index: the walk, and parse_file
// through the shared reader internal/bajadocxml (the class index keeps the
// name, package, module and kind of each class).

// bajadocWalk is bajadoc_xml.iter_classes' walk: os.walk(root) with sorted
// folders and files, every *.bajadoc but module-index.bajadoc. visit gets
// the path (os.path.join from root) and the path relative to root with "/"
// separators.
func bajadocWalk(root string, visit func(path, rel string)) {
	pycompat.Walk(root, func(dir string, dirs, files []string) []string {
		sort.Strings(dirs)
		sort.Strings(files)
		sub, err := filepath.Rel(root, dir) // dir is root joined with folder names
		if err != nil {
			sub = "."
		}
		for _, name := range files {
			if !strings.HasSuffix(name, ".bajadoc") || name == "module-index.bajadoc" {
				continue
			}
			rel := name
			if sub != "." {
				rel = filepath.ToSlash(sub) + "/" + name
			}
			visit(pycompat.Join(dir, name), rel)
		}
		return dirs
	})
}

// parsedBajadoc is ReadBajadocClasses' result for one file.
type parsedBajadoc struct {
	classes []bajadocxml.Class
	failed  bool
	err     error
}

// parseBajadocFiles runs ReadBajadocClasses on every path, on half the CPUs
// (the files are independent; a large tree holds 115 MB of XML), and
// returns the results in the order of paths.
func parseBajadocFiles(paths []string) []parsedBajadoc {
	out := make([]parsedBajadoc, len(paths))
	workers := max(1, min(runtime.GOMAXPROCS(0)/2, len(paths)))
	next := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				f := &out[i]
				f.classes, f.failed, f.err = ReadBajadocClasses(paths[i])
			}
		}()
	}
	for i := range paths {
		next <- i
	}
	close(next)
	wg.Wait()
	return out
}

// ReadBajadocClasses is bajadoc_xml.parse_file through bajadocxml.Parse
// (failed reports a file that stays unparseable after sanitizing:
// PARSE_FAILURES, no class). err != nil is what ET.parse raises besides
// ParseError (the file cannot be read): Python does not catch it.
func ReadBajadocClasses(path string) (classes []bajadocxml.Class, failed bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		var pe *fs.PathError
		if errors.As(err, &pe) {
			return nil, false, errors.New(pycompat.OSErrorText(&fs.PathError{Op: pe.Op, Path: path, Err: pe.Err}))
		}
		return nil, false, err
	}
	classes, ok := bajadocxml.Parse(data)
	return classes, !ok, nil
}
