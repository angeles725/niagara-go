package helpbuild

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Extraction, fill-missing and the sha256 fidelity check of an install's
// doc jars (doc_jars.extract, fill_missing, fidelity_jar, fidelity). Every
// jar of ListDocJars(install, true, ls) is opened like zipfile.ZipFile, its
// entries mapped by MappedEntries with the default rules; an unreadable
// doc jar or entry is an error, as Python raises.

// ErrUnsafeEntry is a mapped entry whose corpus path could leave the
// data root on some OS: a ".." segment (split on "/" and on "\"), a
// leading "/" or "\", or a drive letter ("C:"). Python joins it to the
// root and writes wherever it points; Extract and FillMissing refuse it
// instead, on every OS (vendor jars never hold one).
var ErrUnsafeEntry = errors.New("jar entry path could leave the data root")

// Collision is one corpus path two entries map to: Owner's entry was
// written first, By's replaced it (extract), or By's was skipped in favour
// of an earlier entry of the same jar (Owner == By).
type Collision struct{ Rel, Owner, By string }

// ExtractStats is doc_jars.extract's result.
type ExtractStats struct {
	Files      int
	Bytes      uint64 // sum of the central-directory file sizes
	Collisions []Collision
}

// FidelityReport is doc_jars.fidelity's result: entries checked, sha256
// matches, and the corpus paths that differ (also after CRLF -> LF),
// differ only in line ends (EOLOnly) or are absent (Missing).
type FidelityReport struct {
	Checked, Matched             int
	Mismatched, Missing, EOLOnly []string
	Collisions                   []Collision
	Jars                         int
}

func newReport() *FidelityReport {
	return &FidelityReport{Mismatched: []string{}, Missing: []string{}, EOLOnly: []string{}, Collisions: []Collision{}}
}

// mappedJar is _iter_mapped: jar opened, its extracted entries (with the
// zipInfo each one reads) and its same-jar collisions in jar order.
type mappedJar struct {
	z    *zipFile
	name string
	rows []Mapped
}

func openMapped(jar string) (*mappedJar, error) {
	rules, err := Default()
	if err != nil {
		return nil, err
	}
	z, err := openZip(jar)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", jar, err)
	}
	names := make([]string, len(z.infos))
	for i, x := range z.infos {
		names[i] = x.name
	}
	name := pyBasename(jar)
	return &mappedJar{z: z, name: name, rows: MappedEntries(name, names, rules)}, nil
}

// each calls f for every extracted entry; a skipped colliding entry is
// passed to skip (when non-nil) as Collision{rel, jar, jar} first, in order.
func (m *mappedJar) each(skip func(Collision), f func(x *zipInfo, rel string) error) error {
	defer m.z.f.Close()
	for _, r := range m.rows {
		if r.Collides {
			if skip != nil {
				skip(Collision{r.Rel, m.name, m.name})
			}
			continue
		}
		if err := f(&m.z.infos[r.Pos], r.Rel); err != nil {
			return err
		}
	}
	return nil
}

func eachDocJar(installDir string, ls *Listings, f func(jar string) error) error {
	jars, err := ListDocJars(installDir, true, ls)
	if err != nil {
		return err
	}
	for _, jar := range jars {
		if err := f(jar); err != nil {
			return err
		}
	}
	return nil
}

// outPath is os.path.join(root, rel) for a write, refusing what could
// leave root. Both separators are checked on every OS: filepath.Join on
// Windows splits on "\", and the refusal must not depend on where the
// corpus is built. A "\" in a safe name is kept (on Linux the file is
// named with it, as Python writes it). The joined path is checked to stay
// under root as well.
func outPath(root, rel string) (string, error) {
	unsafe := fmt.Errorf("%w: %q", ErrUnsafeEntry, rel)
	if strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, `\`) || hasDriveLetter(rel) ||
		filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" {
		return "", unsafe
	}
	for _, seg := range strings.FieldsFunc(rel, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return "", unsafe
		}
	}
	out := pycompat.Join(root, rel)
	if r, err := filepath.Rel(filepath.Clean(root), filepath.Clean(out)); err != nil ||
		r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", unsafe
	}
	return out, nil
}

// hasDriveLetter is a rel starting like a Windows volume: "C:", "C:x",
// "C:\x".
func hasDriveLetter(rel string) bool {
	if len(rel) < 2 || rel[1] != ':' {
		return false
	}
	c := rel[0] | 0x20
	return c >= 'a' && c <= 'z'
}

// Extract writes every mapped entry of every doc jar of installDir under
// root (doc_jars.extract): parent folders 0777 and files 0666 less the
// umask, an existing file overwritten; a path two jars map to is written
// by each in turn and recorded as a collision. ls (nil = in memory)
// supplies the module-jar listings for HelpJars.
func Extract(installDir, root string, ls *Listings) (*ExtractStats, error) {
	st := &ExtractStats{Collisions: []Collision{}}
	owners := map[string]string{}
	err := eachDocJar(installDir, ls, func(jar string) error {
		m, err := openMapped(jar)
		if err != nil {
			return err
		}
		skip := func(c Collision) { st.Collisions = append(st.Collisions, c) }
		return m.each(skip, func(x *zipInfo, rel string) error {
			if prev, ok := owners[rel]; ok {
				st.Collisions = append(st.Collisions, Collision{rel, prev, m.name})
			}
			owners[rel] = m.name
			out, err := outPath(root, rel)
			if err != nil {
				return err
			}
			if err := writeEntry(m.z, x, out); err != nil {
				return err
			}
			st.Files++
			st.Bytes += x.size
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return st, nil
}

// writeEntry is makedirs(dirname(out)); with z.open(info) as src,
// open(out, "wb") as dst: copyfileobj(src, dst). The folder is made before
// the entry is opened and the file created before it is read, so a failed
// read leaves the empty file Python's single-chunk copy leaves. The read is
// copyfileobj's bounded one (readEntry whole=false), which is verified like
// read(): a bad CRC-32 or a truncated entry is an error and nothing is
// written to the file.
func writeEntry(z *zipFile, x *zipInfo, out string) error {
	if err := os.MkdirAll(filepath.Dir(out), 0o777); err != nil {
		return err
	}
	if _, err := z.dataStart(x); err != nil {
		return err
	}
	f, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return err
	}
	data, err := z.readEntry(x, false) // bounded, CRC-checked
	if err == nil {
		_, err = f.Write(data)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// FillMissing writes only the mapped entries whose destination is absent
// (doc_jars.fill_missing): an existing path, even a dangling symlink, is
// never touched; each file is written atomically (WriteBytes, 0644).
// It returns the corpus paths written, in jar order.
func FillMissing(installDir, root string, ls *Listings) ([]string, error) {
	written := []string{}
	err := eachDocJar(installDir, ls, func(jar string) error {
		m, err := openMapped(jar)
		if err != nil {
			return err
		}
		return m.each(nil, func(x *zipInfo, rel string) error {
			out, err := outPath(root, rel)
			if err != nil {
				return err
			}
			if _, err := os.Lstat(out); err == nil { // os.path.lexists
				return nil
			}
			data, err := m.z.readEntry(x, true)
			if err != nil {
				return err
			}
			if err := WriteBytes(out, data); err != nil {
				return err
			}
			written = append(written, rel)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return written, nil
}

// FidelityJar compares one jar's mapped entries with the files under root
// (doc_jars.fidelity_jar), adding to rep.
func FidelityJar(jarPath, root string, rep *FidelityReport) error {
	m, err := openMapped(jarPath)
	if err != nil {
		return err
	}
	skip := func(c Collision) { rep.Collisions = append(rep.Collisions, c) }
	return m.each(skip, func(x *zipInfo, rel string) error {
		rep.Checked++
		path := pycompat.Join(root, rel)
		if !pycompat.IsFile(path) {
			rep.Missing = append(rep.Missing, rel)
			return nil
		}
		data, err := m.z.readEntry(x, true)
		if err != nil {
			return err
		}
		local, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		switch {
		case sha256.Sum256(data) == sha256.Sum256(local):
			rep.Matched++
		case bytes.Equal(eolLF(data), eolLF(local)):
			// Same bytes once CRLF -> LF: git text normalization of a
			// tracked corpus (the 4.14 root), not a content change.
			rep.EOLOnly = append(rep.EOLOnly, rel)
		default:
			rep.Mismatched = append(rep.Mismatched, rel)
		}
		return nil
	})
}

func eolLF(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }

// Fidelity sha256-compares every mapped entry of every doc jar of
// installDir with root (doc_jars.fidelity).
func Fidelity(installDir, root string, ls *Listings) (*FidelityReport, error) {
	rep := newReport()
	err := eachDocJar(installDir, ls, func(jar string) error {
		if err := FidelityJar(jar, root, rep); err != nil {
			return err
		}
		rep.Jars++
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rep, nil
}

// fileSHA256 is _sha256: the hex digest of a file, read in 1 MiB chunks.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.CopyBuffer(h, f, make([]byte, 1<<20)); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
