package blockfiles

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/angeles725/niagara-go/internal/atomicfile"
	"github.com/angeles725/niagara-go/internal/statcache"
)

// The SubBlocks walk cache. SubBlocks reads names and entry types only, never file
// content, and a directory's mtime (and, on Linux, ctime) changes whenever a direct
// child is added, removed or renamed; so when every walked directory still has the stamp
// it had when the walk read it, the walk would read the same names and return the same
// blocks. A warm call stats the walked directories instead of reading them. What a
// directory stamp does not cover is recorded and checked too:
//   - the mode (an unreadable directory turning readable, or the reverse);
//   - every symlink whose name could be a block: the walk skips it when its target is a
//     directory, and that target may live anywhere;
//   - the executable (the walk and label rules are this code), the user and the root.
//
// A stamp is racy when it is too close to the cache file's own mtime (same file system
// clock): a change made in the same clock tick as the walk's stat would leave the stamp
// equal. Such a cache is not trusted (the next call walks and rewrites it, and once the
// tree has been quiet for racyWindow the rewritten cache holds). A walk that hit an error
// other than an unreadable directory is not cached at all.
//
// The cache is <root>/tools/.corpus-index/subblocks-<executable>.cache, beside the find
// cache, written only when that directory already exists, never with
// CORPUS_NAV_FIND_INDEX off and never by a test binary (unless its test opts in): a test
// pointed at a real corpus leaves nothing behind. Anything that does not parse exactly
// is a corrupt cache: walked and rewritten. Little-endian; str is a u32 length and the
// bytes:
//
//	magic "blkwalk1" | u32 CRC-32C of the rest | str key | u64 walk nanoseconds
//	u32 n, n x (str dir, stamp) | u32 n, n x (str link, u8 target is a directory)
//	u32 n, n x (str label, str path, str corpus)

const (
	subMagic    = "blkwalk1"
	subSchema   = "subblocks-1"
	racyWindow  = 2 * time.Second
	minDirSize  = 4 + stampSize
	minLinkSize = 5
)

var (
	// cacheTestBinary lets this package's tests use the cache from a test binary.
	cacheTestBinary bool
	// walks counts full SubBlocks walks (read by the tests).
	walks atomic.Int64
	// exeStamp is the stat stamp of this executable ("" when unknown: no cache).
	exeStamp  = func() string { exe, err := os.Executable(); return fileStamp(exe, err) }
	subCRC    = crc32.MakeTable(crc32.Castagnoli)
	errSubBad = errors.New("corrupt sub-blocks cache")
)

// walkRecord is what a warm call checks: every walked directory's stamp and every
// block-named symlink's target kind.
type walkRecord struct {
	dirs    []dirEntryStamp
	links   []linkKind
	invalid bool // an error that a later walk may not repeat: do not cache
	dur     time.Duration
}

type dirEntryStamp struct {
	path  string
	stamp stamp
}

type linkKind struct {
	path  string
	isDir bool
}

// cachePath is the walk cache of root, "" when caching is off for this call.
func cachePath(root string) string {
	if statcache.Off("CORPUS_NAV_FIND_INDEX") {
		return ""
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	name := filepath.Base(exe)
	if statcache.IsTestName(exe) && !cacheTestBinary {
		return ""
	}
	dir := join(join(root, "tools"), ".corpus-index")
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return ""
	}
	return join(dir, "subblocks-"+name+".cache")
}

// cacheKey ties a cache to the code, the user and the root it was walked for.
func cacheKey(root string) (string, bool) {
	exe := exeStamp()
	return fmt.Sprintf("%s\x00%d\x00%s\x00%s", subSchema, os.Getuid(), exe, root), exe != ""
}

// cachedSubBlocks is SubBlocks through the cache at path.
func cachedSubBlocks(root, path string) []Block {
	key, ok := cacheKey(root)
	if !ok {
		bs, _ := walkSubBlocks(root, false)
		return bs
	}
	if bs, ok := readSubCache(path, key); ok {
		return bs
	}
	start := time.Now()
	bs, rec := walkSubBlocks(root, true)
	rec.dur = time.Since(start)
	if !rec.invalid {
		_ = writeSubCache(path, key, bs, rec) // the walk is the answer either way
	}
	return bs
}

// readSubCache is the cached result when the cache at path is intact, made for key, not
// racy, and every recorded stamp still holds.
func readSubCache(path, key string) ([]Block, bool) {
	data, mtime, err := readFileMtime(path)
	if err != nil {
		return nil, false
	}
	bs, rec, err := parseSubCache(data, key)
	if err != nil {
		return nil, false
	}
	limit := mtime.Add(-racyWindow - rec.dur).UnixNano()
	for _, d := range rec.dirs {
		if d.stamp.newest() >= limit {
			return nil, false
		}
	}
	return bs, rec.holds()
}

func readFileMtime(path string) ([]byte, time.Time, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer fh.Close()
	st, err := fh.Stat()
	if err != nil {
		return nil, time.Time{}, err
	}
	data := make([]byte, st.Size())
	if _, err := io.ReadFull(fh, data); err != nil {
		return nil, time.Time{}, err
	}
	return data, st.ModTime(), nil
}

// holds reports that every recorded directory stamp and symlink kind is unchanged; the
// directories are stat'ed by walkWorkers goroutines, stopping at the first change.
func (rec *walkRecord) holds() bool {
	for _, l := range rec.links {
		if targetIsDir(l.path) != l.isDir {
			return false
		}
	}
	var changed atomic.Bool
	var wg sync.WaitGroup
	n := walkWorkers()
	for w := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := w; i < len(rec.dirs) && !changed.Load(); i += n {
				if st, err := statStamp(rec.dirs[i].path); err != nil || st != rec.dirs[i].stamp {
					changed.Store(true)
				}
			}
		}()
	}
	wg.Wait()
	return !changed.Load()
}

// targetIsDir is the walk's test on a symlink: its target is a directory.
func targetIsDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

// stableErr is an error the walk may cache: a missing or unreadable entry.
func stableErr(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission)
}

func writeSubCache(path, key string, bs []Block, rec *walkRecord) error {
	le := binary.LittleEndian
	b := []byte(subMagic + "\x00\x00\x00\x00")
	str := func(s string) { b = append(le.AppendUint32(b, uint32(len(s))), s...) }
	str(key)
	b = le.AppendUint64(b, uint64(rec.dur))
	b = le.AppendUint32(b, uint32(len(rec.dirs)))
	for _, d := range rec.dirs {
		str(d.path)
		b = d.stamp.append(b)
	}
	b = le.AppendUint32(b, uint32(len(rec.links)))
	for _, l := range rec.links {
		str(l.path)
		if l.isDir {
			b = append(b, 1)
		} else {
			b = append(b, 0)
		}
	}
	b = le.AppendUint32(b, uint32(len(bs)))
	for _, blk := range bs {
		str(blk.Label)
		str(blk.Path)
		str(blk.Corpus)
	}
	le.PutUint32(b[len(subMagic):], crc32.Checksum(b[len(subMagic)+4:], subCRC))
	return atomicfile.WriteStream(path, 0o600, func(w io.Writer) error {
		_, err := w.Write(b)
		return err
	})
}

// subReader is a bounds-checked cursor: the first short read sets err.
type subReader struct {
	b   []byte
	err error
}

func (r *subReader) next(n int) []byte {
	if r.err != nil || n < 0 || n > len(r.b) {
		r.err = errSubBad
		return nil
	}
	out := r.b[:n:n]
	r.b = r.b[n:]
	return out
}

func (r *subReader) u32() int {
	if b := r.next(4); b != nil {
		return int(binary.LittleEndian.Uint32(b))
	}
	return 0
}

func (r *subReader) str() string { return string(r.next(r.u32())) }

// count reads a u32 count of records at least min bytes each (errSubBad when they
// cannot fit: nothing is allocated for them).
func (r *subReader) count(min int) int {
	n := r.u32()
	if n > len(r.b)/min {
		r.err = errSubBad
		return 0
	}
	return n
}

func parseSubCache(data []byte, key string) ([]Block, *walkRecord, error) {
	if len(data) < len(subMagic)+4 || string(data[:len(subMagic)]) != subMagic {
		return nil, nil, errSubBad
	}
	body := data[len(subMagic)+4:]
	if crc32.Checksum(body, subCRC) != binary.LittleEndian.Uint32(data[len(subMagic):]) {
		return nil, nil, errSubBad
	}
	r := &subReader{b: body}
	if !bytes.Equal(r.next(r.u32()), []byte(key)) {
		return nil, nil, errSubBad
	}
	rec := &walkRecord{}
	if d := r.next(8); d != nil {
		rec.dur = time.Duration(binary.LittleEndian.Uint64(d) & (1<<62 - 1))
	}
	rec.dirs = make([]dirEntryStamp, r.count(minDirSize))
	for i := range rec.dirs {
		rec.dirs[i] = dirEntryStamp{r.str(), parseStamp(r.next(stampSize))}
	}
	rec.links = make([]linkKind, r.count(minLinkSize))
	for i := range rec.links {
		path := r.str()
		kind := r.next(1)
		rec.links[i] = linkKind{path, len(kind) == 1 && kind[0] == 1}
		if len(kind) == 1 && kind[0] > 1 {
			r.err = errSubBad
		}
	}
	bs := make([]Block, r.count(12))
	for i := range bs {
		bs[i] = Block{Label: r.str(), Path: r.str(), Corpus: r.str()}
	}
	if r.err != nil || len(r.b) != 0 {
		return nil, nil, errSubBad
	}
	if len(bs) == 0 {
		bs = nil // what a walk that found nothing returns
	}
	return bs, rec, nil
}
