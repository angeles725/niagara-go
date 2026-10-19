package searchdoor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/help/fts/ftsfixture"
)

// The default backend paths are checkout DIRECTORIES (defaultPaths). These tests
// run each source through the real route (no fake pick) with a checkout
// directory as argv[0], on the fixtures the other in-process tests use, and
// compare the Raw with the backend's recorded answer: each source answers (hits
// and an honest zero), never "tool not found".

// realRun is b.Run through the real route with the timing zeroed.
func realRun(t *testing.T, b *inprocBackend, src source, argv []string) Raw {
	t.Helper()
	raw := b.Run(src, argv, 30)
	if strings.Contains(raw.Reason, "tool not found") {
		t.Fatalf("%v: %+v, want the checkout directory accepted", argv, raw)
	}
	raw.MS = 0
	return raw
}

func checkRaw(t *testing.T, got Raw, want oracle, state string) {
	t.Helper()
	if w := Classify(want.rc, want.stdout, want.stderr, 0); got != w || got.State != state {
		t.Errorf("got  %+v\nwant %+v (state %s)", got, w, state)
	}
}

func TestBlocksSourceAnswersOnACheckoutDirectory(t *testing.T) {
	t.Setenv("CORPUS_NAV_FIND_INDEX", "off")
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../corpusnav/testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	b := &inprocBackend{clock: time.Now}
	b.bindEnv(envOf(map[string]string{"CORPUS_NAV_INDEX_DIR": t.TempDir()}))
	for golden, state := range map[string]string{"find-alarm-histories-and": OK, "find-zzqq": Empty} {
		t.Run(golden, func(t *testing.T) {
			args := map[string][]string{
				"find-alarm-histories-and": {"find", "alarm histories", "--limit", "50", "--and"},
				"find-zzqq":                {"find", "zzqq", "--limit", "2000", "--or"},
			}[golden]
			checkRaw(t, realRun(t, b, blocksSource, append([]string{root}, args...)),
				readOracle(t, filepath.Join("..", "corpusnav", "testdata", "golden", golden)), state)
		})
	}
}

func TestDocsSourceAnswersOnACheckoutDirectory(t *testing.T) {
	testdata := helpTestdata(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	checkout := ftsfixture.Checkout(t, testdata, false)
	b := &inprocBackend{clock: time.Now}
	b.bindEnv(envOf(map[string]string{"XDG_CACHE_HOME": os.Getenv("XDG_CACHE_HOME")}))
	cases := map[string]ftCase{}
	for _, c := range loadFtCases(t, filepath.Join(testdata, "fulltext-cli", "cases.json")) {
		cases[c.Name] = c
	}
	for name, state := range map[string]string{"find": OK, "find-miss": Empty} {
		t.Run(name, func(t *testing.T) {
			c, ok := cases[name]
			if !ok {
				t.Fatalf("no case %q in cases.json", name)
			}
			norm := strings.NewReplacer(ftsfixture.Root, checkout)
			want := oracle{c.Exit, norm.Replace(recorded(t, filepath.Join(testdata, "fulltext-cli", c.Name+".txt"))),
				norm.Replace(c.Stderr)}
			checkRaw(t, realRun(t, b, docsSource, append([]string{checkout}, docsArgv(c.Args)...)), want, state)
		})
	}
}

func TestCodeSourceAnswersOnACheckoutDirectory(t *testing.T) {
	organized, err := filepath.Abs("../nav/testdata/fixture/search-organized")
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(t.TempDir(), "module-navigator")
	if err := os.CopyFS(filepath.Join(base, "indexes"), os.DirFS("../nav/testdata/fixture/search/indexes")); err != nil {
		t.Fatal(err)
	}
	b := &inprocBackend{clock: time.Now}
	b.bindEnv(envOf(map[string]string{"NAV_CORPUS_BASE": organized}))
	for golden, state := range map[string]string{"nav-token": OK, "nav-token-miss": Empty} {
		t.Run(golden, func(t *testing.T) {
			args := map[string][]string{
				"nav-token":      {"token", "ackAlarm", "-n", "100000"},
				"nav-token-miss": {"token", "zzNothing", "-n", "100000"},
			}[golden]
			checkRaw(t, realRun(t, b, codeSource, append([]string{base}, args...)),
				readOracle(t, filepath.Join("..", "nav", "testdata", "golden", golden)), state)
		})
	}
}
