package fts

import (
	"bytes"
	"encoding/gob"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/angeles725/niagara-go/internal/atomicfile"
)

// Equivalence with the code before internal/statcache (PR #179): the legacy*
// functions are that code verbatim, and every case must decide as it did.

func legacyStatFile(path string) fileStat {
	st, err := os.Stat(path)
	if err != nil {
		return fileStat{}
	}
	return fileStat{st.Size(), st.ModTime().UnixNano(), true}
}

func legacyStatFiles(files []string) []fileStat {
	out := make([]fileStat, len(files))
	var wg sync.WaitGroup
	chunk := (len(files) + fpStatWorkers - 1) / fpStatWorkers
	for lo := 0; lo < len(files); lo += chunk {
		hi := min(lo+chunk, len(files))
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := lo; i < hi; i++ {
				out[i] = legacyStatFile(files[i])
			}
		}()
	}
	wg.Wait()
	return out
}

func legacyLoadFpMemo(path string) *fpMemo {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var m fpMemo
	if gob.NewDecoder(bytes.NewReader(data)).Decode(&m) != nil || m.Format != fpFormat || m.Engine != EngineVersion {
		return nil
	}
	for _, s := range m.Sources {
		if len(s.Dirs) != len(s.DirMtimes) || len(s.Files) != len(s.Stats) {
			return nil
		}
	}
	return &m
}

func legacyWriteFpMemo(path string, m *fpMemo) {
	var buf bytes.Buffer
	if gob.NewEncoder(&buf).Encode(m) != nil || os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	_ = atomicfile.WriteStream(path, 0o600, func(w io.Writer) error {
		_, err := w.Write(buf.Bytes())
		return err
	})
}

// TestStatFilesMatchesLegacy: every file set, at every worker count and with
// files that cannot be stat'ed, gets the stats it got before (fpcache.go
// statFiles), and Fingerprint's sequential stats equal them.
func TestStatFilesMatchesLegacy(t *testing.T) {
	base := fpTree(t)
	var files []string
	_ = filepath.Walk(base, func(p string, _ os.FileInfo, err error) error {
		files = append(files, p) // folders too: stat'ed like a file
		return err
	})
	broken := filepath.Join(base, "broken")
	if err := os.Symlink(filepath.Join(base, "nowhere"), broken); err == nil {
		files = append(files, broken)
	}
	files = append(files, filepath.Join(base, "missing.txt"))
	for n := 0; n <= len(files); n++ {
		set := files[:n]
		want := legacyStatFiles(set)
		for _, workers := range []int{0, 1, fpStatWorkers, len(files) + 3} {
			if got := statFiles(set, workers); !reflect.DeepEqual(got, want) {
				t.Fatalf("%d files, %d workers: %v, legacy %v", n, workers, got, want)
			}
		}
	}
	last := statFiles(files, 1)[len(files)-1]
	if last != (fileStat{}) {
		t.Fatalf("a missing file's stat is %+v, want the zero fileStat", last)
	}
}

func TestFpMemoMatchesLegacy(t *testing.T) {
	good := &fpMemo{Format: fpFormat, Engine: EngineVersion, Sources: []fpSource{{
		Label: "guides", Dir: "/d", Real: "/d", Trusted: true, Dirs: []string{"/d"}, DirMtimes: []int64{5},
		Files: []string{"a.txt"}, Stats: []fileStat{{1, 2, true}}, Part: "p",
	}}}
	variant := func(f func(m *fpMemo)) *fpMemo {
		m := *good
		m.Sources = []fpSource{good.Sources[0]}
		f(&m)
		return &m
	}
	dir := t.TempDir()
	cases := map[string]any{
		"valid":          good,
		"other format":   variant(func(m *fpMemo) { m.Format++ }),
		"other engine":   variant(func(m *fpMemo) { m.Engine += "x" }),
		"dirs mismatch":  variant(func(m *fpMemo) { m.Sources[0].DirMtimes = nil }),
		"stats mismatch": variant(func(m *fpMemo) { m.Sources[0].Stats = nil }),
		"garbage":        []byte("garbage"),
		"missing":        nil,
	}
	for name, v := range cases {
		p := filepath.Join(dir, name+".gob")
		switch v := v.(type) {
		case *fpMemo:
			legacyWriteFpMemo(p, v)
		case []byte:
			if err := os.WriteFile(p, v, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		got, want := loadFpMemo(p), legacyLoadFpMemo(p)
		if !reflect.DeepEqual(got, want) || (name == "valid") != (got != nil) {
			t.Errorf("%s: %+v, legacy %+v", name, got, want)
		}
	}
	// writeFpMemo writes the legacy bytes, mode 0600, in a directory it creates.
	newPath, oldPath := filepath.Join(dir, "n", "fingerprint.gob"), filepath.Join(dir, "o", "fingerprint.gob")
	writeFpMemo(newPath, good)
	legacyWriteFpMemo(oldPath, good)
	got, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := os.ReadFile(oldPath); !bytes.Equal(got, want) {
		t.Fatalf("memo bytes differ from the legacy writer:\n%x\n%x", got, want)
	}
	if st, err := os.Stat(newPath); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("memo mode %v, %v", st.Mode(), err)
	}
	for _, name := range []string{"fpMemo", "fpSource", "fileStat", "DirMtimes"} {
		if !bytes.Contains(got, []byte(name)) {
			t.Errorf("memo bytes do not name %s", name)
		}
	}
}
