package help

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/help/fts/ftsfixture"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// rootSourceRepr is a quoted source file path, os.path.join(base_dir,
// "source", data["path"]) in source_search.py: on Windows only the joins are
// "\" (the index path keeps its "/").
var rootSourceRepr = regexp.MustCompile(`'<ROOT>/source/([^']*)'`)

// atRoot is ftsfixture.AtRoot after rootSourceRepr: a recorded Linux output
// with <ROOT> replaced by the checkout, its os.path.join paths for the host.
func atRoot(recorded, checkout string) string {
	s := rootSourceRepr.ReplaceAllStringFunc(recorded, func(m string) string {
		return pycompat.FSRepr(pycompat.Join(checkout, "source", rootSourceRepr.FindStringSubmatch(m)[1]))
	})
	return ftsfixture.AtRoot(s, checkout)
}

// TestFulltextCommandsMatchPython runs find, guide-search, devguide-search
// and pdf over the full-text fixture checkout and compares them with what
// tools/niagara_help.py printed there (testdata/fulltext-cli, written by
// the retired fulltext_gen.py; <ROOT> is the checkout).
func TestFulltextCommandsMatchPython(t *testing.T) {
	raw, err := os.ReadFile("testdata/fulltext-cli/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name   string   `json:"name"`
		Args   []string `json:"args"`
		Exit   int      `json:"exit"`
		Stderr string   `json:"stderr"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	testdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	checkout := ftsfixture.Checkout(t, testdata, false)
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join(testdata, "fulltext-cli", c.Name+".txt"))
			if err != nil {
				t.Fatal(err)
			}
			out, errs, code := run(t, checkout, c.Args...)
			wantOut, wantErr := atRoot(string(want), checkout), atRoot(c.Stderr, checkout)
			if code != c.Exit || errs != wantErr || out != wantOut {
				t.Errorf("exit %d (want %d) stderr %q (want %q) stdout:\n%s\nwant:\n%s",
					code, c.Exit, errs, wantErr, out, wantOut)
			}
		})
	}
}

// rebuildCase is one case of testdata/fulltext-rebuild/cases.json (written
// by the retired rebuild_gen.py) or of its intended.json, what nh prints where
// it differs from Python on purpose.
type rebuildCase struct {
	Name   string   `json:"name"`
	Args   []string `json:"args"`
	Setup  string   `json:"setup"`
	Exit   int      `json:"exit"`
	Stdout *string  `json:"stdout"` // intended.json only; else <case>.txt
	Stderr string   `json:"stderr"`
	Built  string   `json:"built"`
	Reason string   `json:"reason"`
}

// TestFulltextRebuildMatchesPython: with a missing or stale full-text index
// the search commands build it in process, print Python's lines on stderr
// and answer (decision D38); a read-only indexes/ builds it in the cache
// directory. Each case runs on a fresh fixture copy prepared like
// rebuild_gen.py did; intended.json replaces the cases nh answers otherwise.
func TestFulltextRebuildMatchesPython(t *testing.T) {
	var cases, intended []rebuildCase
	for path, into := range map[string]*[]rebuildCase{
		"testdata/fulltext-rebuild/cases.json": &cases, "testdata/fulltext-rebuild/intended.json": &intended} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, into); err != nil {
			t.Fatal(err)
		}
	}
	override := map[string]rebuildCase{}
	for _, c := range intended {
		if c.Reason == "" || c.Stdout == nil {
			t.Fatalf("intended case %s needs a reason and a stdout", c.Name)
		}
		override[c.Name] = c
	}
	testdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, py := range cases {
		t.Run(py.Name, func(t *testing.T) {
			c, isIntended := override[py.Name]
			if !isIntended {
				c = py
				want, err := os.ReadFile(filepath.Join(testdata, "fulltext-rebuild", c.Name+".txt"))
				if err != nil {
					t.Fatal(err)
				}
				s := string(want)
				c.Stdout = &s
			}
			if strings.HasPrefix(c.Setup, "ro-") && (runtime.GOOS == "windows" || os.Geteuid() == 0) {
				t.Skip("read-only directories need a non-root POSIX user")
			}
			checkout := ftsfixture.Checkout(t, testdata, true)
			base := filepath.Join(checkout, ftsfixture.Versions[c.Args[1]])
			if base == filepath.Join(checkout, ".") {
				base = checkout
			}
			cache := filepath.Join(filepath.Dir(checkout), "cache")
			t.Setenv("XDG_CACHE_HOME", cache)
			for _, d := range rebuildSetup(t, testdata, checkout, base, cache, c.Setup) {
				defer os.Chmod(d, 0o755)
			}
			out, errs, code := run(t, checkout, c.Args...)
			cacheDB := fts.CachePath(base)
			norm := strings.NewReplacer("<CACHEDB>", cacheDB, "<CACHEDIR>", filepath.Dir(cacheDB),
				"<CACHE>", cache)
			if wantErr := atRoot(norm.Replace(c.Stderr), checkout); code != c.Exit || errs != wantErr {
				t.Errorf("exit %d (want %d) stderr:\n%s\nwant:\n%s", code, c.Exit, errs, wantErr)
			}
			if wantOut := atRoot(norm.Replace(*c.Stdout), checkout); out != wantOut {
				t.Errorf("stdout:\n%s\nwant:\n%s", out, wantOut)
			}
			if got := builtAt(base); got != c.Built {
				t.Errorf("up-to-date index in %q, want %q", got, c.Built)
			}
		})
	}
}

// rebuildSetup prepares a fixture copy without indexes as rebuild_gen.py
// does for setup, returning the directories it made read-only.
func rebuildSetup(t *testing.T, testdata, checkout, base, cache, setup string) []string {
	t.Helper()
	var locked []string
	switch setup {
	case "missing":
	case "stale":
		vid := "1.0"
		if base != checkout {
			vid = "2.0"
		}
		db := fts.DBPath(base)
		ftsfixture.Install(t, filepath.Join(testdata, "fulltext-db", vid+".sqlite"), db, checkout)
		ftsfixture.SetFingerprint(t, db, "stale", "", "")
	case "ro-everywhere":
		if err := os.MkdirAll(cache, 0o755); err != nil {
			t.Fatal(err)
		}
		locked = append(locked, cache)
		fallthrough
	case "ro-indexes":
		locked = append(locked, filepath.Join(base, "indexes"))
	default:
		t.Fatalf("unknown setup %q", setup)
	}
	for _, d := range locked {
		if err := os.Chmod(d, 0o500); err != nil {
			t.Fatal(err)
		}
	}
	return locked
}

// builtAt is where an up-to-date index of base exists: "indexes", "cache"
// or "none" (rebuild_gen.py where).
func builtAt(base string) string {
	switch path, _ := fts.FreshPath(base, fts.Fingerprint(fts.SourceDirs(base))); path {
	case "":
		return "none"
	case fts.DBPath(base):
		return "indexes"
	default:
		return "cache"
	}
}

func TestFtHeadIsPythonSliceEnd(t *testing.T) {
	for _, c := range [][3]int{{5, 3, 3}, {5, 9, 5}, {5, -1, 4}, {5, -9, 0}, {0, -1, 0}, {5, 0, 0}} {
		if got := ftHead(c[0], c[1]); got != c[2] {
			t.Errorf("ftHead(%d, %d) = %d, want %d", c[0], c[1], got, c[2])
		}
	}
}

func TestFormatF0RoundsHalfToEven(t *testing.T) {
	for v, want := range map[any]string{json.Number("0.5"): "0", json.Number("1.5"): "2", json.Number("2.5"): "2",
		json.Number("120.5"): "120", json.Number("3"): "3", -0.4: "-0", 0: "0", "x": "x"} {
		if got := formatF0(v); got != want {
			t.Errorf("formatF0(%v) = %q, want %q", v, got, want)
		}
	}
}
