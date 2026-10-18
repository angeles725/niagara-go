package searchdoor

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/help"
	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/help/fts/ftsfixture"
)

// The docs source answers every niagara_help find call in process. Its answers are
// compared with what tools/niagara_help.py printed on the full-text fixture checkout
// (help/testdata/fulltext-cli and fulltext-rebuild, the oracles of nh's own tests)
// and, for the cases those recordings do not cover, with nh itself (help.Main).

type ftCase struct {
	Name   string   `json:"name"`
	Args   []string `json:"args"`
	Setup  string   `json:"setup"`
	Exit   int      `json:"exit"`
	Stdout *string  `json:"stdout"`
	Stderr string   `json:"stderr"`
}

func loadFtCases(t *testing.T, path string) []ftCase {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cases []ftCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

func helpTestdata(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("../help/testdata")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func recorded(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestDocsFindMatchesNiagaraHelp: hits, the honest-zero block (exit 1) and an
// unsearchable query (exit 2), byte for byte.
func TestDocsFindMatchesNiagaraHelp(t *testing.T) {
	testdata := helpTestdata(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	checkout := ftsfixture.Checkout(t, testdata, false)
	script := stubScript(t, filepath.Join(checkout, "tools", "niagara_help.py"))
	n := 0
	for _, c := range loadFtCases(t, filepath.Join(testdata, "fulltext-cli", "cases.json")) {
		if !strings.HasPrefix(c.Name, "find") {
			continue
		}
		n++
		t.Run(c.Name, func(t *testing.T) {
			norm := strings.NewReplacer(ftsfixture.Root, checkout)
			want := norm.Replace(recorded(t, filepath.Join(testdata, "fulltext-cli", c.Name+".txt")))
			checkAnswer(t, docsFind(envOf(nil), append([]string{script}, docsArgv(c.Args)...)),
				oracle{c.Exit, want, norm.Replace(c.Stderr)})
		})
	}
	if n < 8 {
		t.Fatalf("only %d find cases in cases.json", n)
	}
}

// TestDocsFindRebuildsTheFulltextIndex (D38): a missing or stale fulltext.sqlite is
// built first, with Python's notices on stderr, then the query is answered; a
// read-only indexes/ builds it in the cache directory. The rebuild recordings of nh's
// tests are the oracle (intended.json where nh answers otherwise on purpose).
func TestDocsFindRebuildsTheFulltextIndex(t *testing.T) {
	testdata := helpTestdata(t)
	dir := filepath.Join(testdata, "fulltext-rebuild")
	override := map[string]ftCase{}
	for _, c := range loadFtCases(t, filepath.Join(dir, "intended.json")) {
		override[c.Name] = c
	}
	n := 0
	for _, py := range loadFtCases(t, filepath.Join(dir, "cases.json")) {
		if len(py.Args) < 3 || py.Args[2] != "find" {
			continue
		}
		n++
		t.Run(py.Name, func(t *testing.T) {
			c, ok := override[py.Name]
			if !ok {
				c = py
				s := recorded(t, filepath.Join(dir, c.Name+".txt"))
				c.Stdout = &s
			}
			if strings.HasPrefix(c.Setup, "ro-") && (runtime.GOOS == "windows" || os.Geteuid() == 0) {
				t.Skip("read-only directories need a non-root POSIX user")
			}
			checkout := ftsfixture.Checkout(t, testdata, true)
			base := filepath.Join(checkout, ftsfixture.Versions[c.Args[1]])
			cache := filepath.Join(filepath.Dir(checkout), "cache")
			t.Setenv("XDG_CACHE_HOME", cache)
			var locked []string
			switch c.Setup {
			case "stale":
				db := fts.DBPath(base)
				ftsfixture.Install(t, filepath.Join(testdata, "fulltext-db", c.Args[1]+".sqlite"), db, checkout)
				ftsfixture.SetFingerprint(t, db, "stale", "", "")
			case "ro-everywhere":
				if err := os.MkdirAll(cache, 0o755); err != nil {
					t.Fatal(err)
				}
				locked = append(locked, cache)
				fallthrough
			case "ro-indexes":
				locked = append(locked, filepath.Join(base, "indexes"))
			}
			for _, d := range locked {
				if err := os.Chmod(d, 0o500); err != nil {
					t.Fatal(err)
				}
				defer os.Chmod(d, 0o755)
			}
			script := stubScript(t, filepath.Join(checkout, "tools", "niagara_help.py"))
			cacheDB := fts.CachePath(base)
			norm := strings.NewReplacer("<CACHEDB>", cacheDB, "<CACHEDIR>", filepath.Dir(cacheDB),
				"<CACHE>", cache)
			at := func(s string) string { return ftsfixture.AtRoot(norm.Replace(s), checkout) }
			checkAnswer(t, docsFind(envOf(nil), append([]string{script}, docsArgv(c.Args)...)),
				oracle{c.Exit, at(*c.Stdout), at(c.Stderr)})
			if p, _ := fts.FreshPath(base, fts.Fingerprint(fts.SourceDirs(base))); c.Exit != 2 && p == "" {
				t.Error("no up-to-date index after the answer")
			}
		})
	}
	if n < 5 {
		t.Fatalf("only %d find rebuild cases", n)
	}
}

// TestDocsFindMatchesNhOnVersionErrors: a version that cannot be resolved or is
// registered but not extracted is nh's ERROR lines on stdout, exit 2.
func TestDocsFindMatchesNhOnVersionErrors(t *testing.T) {
	testdata := helpTestdata(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("NIAGARA_HELP_VERSION", "") // for nh (runNh), which reads the process
	checkout := ftsfixture.Checkout(t, testdata, false)
	script := stubScript(t, filepath.Join(checkout, "tools", "niagara_help.py"))
	reg := filepath.Join(checkout, "versions", "registry.json")
	raw, err := os.ReadFile(reg)
	if err != nil {
		t.Fatal(err)
	}
	var registry map[string]any
	if err := json.Unmarshal(raw, &registry); err != nil {
		t.Fatal(err)
	}
	registry["versions"].(map[string]any)["3.0"] = map[string]any{"label": "Three", "install_dir": ".",
		"data_root": "versions/3.0"}
	if raw, err = json.Marshal(registry); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reg, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"--niagara-version", "9.9", "find", "alarm", "-n", "5"},
		{"--niagara-version", "3.0", "find", "alarm", "-n", "5"},
		{"--niagara-version", "-x", "find", "alarm", "-n", "5", "--or"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			got := docsFind(envOf(nil), append([]string{script}, args...))
			want := runNh(t, checkout, args)
			if want.rc != 2 {
				t.Fatalf("nh exit %d, want a version error (2): %q", want.rc, want.stdout)
			}
			checkAnswer(t, got, want)
		})
	}
}

// runNh runs nh in process on the checkout.
func runNh(t *testing.T, checkout string, args []string) oracle {
	t.Helper()
	t.Setenv("NIAGARA_HELP_DIR", checkout) // restored by the test's cleanup
	var out, errb bytes.Buffer
	rc := help.Main(args, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	return oracle{rc, out.String(), errb.String()}
}

// TestDocsFindSearchesADashQuery: a query that starts with "-" is searched as text
// (nh's argparse would read it as an option); an empty query never reaches a
// backend (the door exits 3 first: goldens blank and blank-spaces).
func TestDocsFindSearchesADashQuery(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	checkout := ftsfixture.Checkout(t, helpTestdata(t), false)
	script := stubScript(t, filepath.Join(checkout, "tools", "niagara_help.py"))
	a := docsFind(envOf(nil), []string{script, "--niagara-version", "1.0", "find", "-zzqq", "-n", "5"})
	if !a.ok || a.rc != 1 || !strings.HasPrefix(a.stdout, "No match for '-zzqq' (find) in this scope.") {
		t.Errorf("%+v", a)
	}
}
