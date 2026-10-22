// Package modjar reads one member (META-INF/module.xml, usually) out of many module jars:
// ReadMember for one jar, ReadAll for a list through a bounded pool of workers, and a
// Cache that keeps each jar's answer under its size and modification time.
//
// An N4 install holds about a thousand jars, often on a slow file system (/mnt/c under
// WSL): opening them one at a time is what made station-modules and
// niagara-security-audit take tens of seconds. The pool overlaps the opens; the cache
// skips them on a later run when a jar has not changed.
package modjar

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
)

// DefaultWorkers is the pool size used when ReadAll is given none: enough to hide the
// latency of a slow file system without monopolising a shared host.
const DefaultWorkers = 8

// ErrNoMember means the jar opened but holds no entry with the member's name.
var ErrNoMember = errors.New("no such member")

// Result is what one jar gave.
type Result struct {
	Path string
	// Data is the member's bytes: of its last entry when the name repeats, as
	// zipfile.ZipFile.read picks it.
	Data []byte
	// MetaInf lists the jar's entry names under META-INF/ in archive order (signature
	// files, manifests): what a caller needs besides the member without opening the jar again.
	MetaInf []string
	// Err is the first failure for this jar: the open error (a missing file, not a zip),
	// ErrNoMember, or the error reading the member (a corrupt stream, a CRC mismatch).
	// Data and MetaInf are empty when it is set.
	Err error
}

// opens counts the jars actually opened (read by the tests to tell a cache hit from a miss).
var opens atomic.Int64

// ReadMember reads member out of the jar at path.
func ReadMember(path, member string) Result {
	opens.Add(1)
	r := Result{Path: path}
	z, err := zip.OpenReader(path)
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) { // an insecure name still gives a reader
		r.Err = err
		return r
	}
	defer z.Close()
	var hit *zip.File
	var meta []string
	for _, f := range z.File {
		if strings.HasPrefix(f.Name, "META-INF/") {
			meta = append(meta, f.Name)
		}
		if f.Name == member {
			hit = f
		}
	}
	if hit == nil {
		r.Err = fmt.Errorf("%s: %w: %s", path, ErrNoMember, member)
		return r
	}
	data, err := readFile(hit)
	if err != nil {
		r.Err = fmt.Errorf("%s: %s: %w", path, member, err)
		return r
	}
	r.Data, r.MetaInf = data, meta
	return r
}

func readFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// ReadAll reads member out of every jar in paths with at most workers jars open at once
// (DefaultWorkers when workers <= 0). The results are in the order of paths.
func ReadAll(paths []string, member string, workers int) []Result {
	return pool(paths, workers, func(p string) Result { return ReadMember(p, member) })
}

// pool runs fn over paths on at most workers goroutines, results in input order.
func pool(paths []string, workers int, fn func(string) Result) []Result {
	out := make([]Result, len(paths))
	if workers <= 0 {
		workers = DefaultWorkers
	}
	workers = min(workers, len(paths))
	var next atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for {
				i := int(next.Add(1)) - 1
				if i >= len(paths) {
					return
				}
				out[i] = fn(paths[i])
			}
		})
	}
	wg.Wait()
	return out
}
