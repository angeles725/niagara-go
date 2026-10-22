package scope

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/help/fts/ftsfixture"
	"github.com/angeles725/niagara-go/internal/index"
)

// helpTestdata is the nh testdata folder, whose recorded Python outputs these
// tests reuse: the library must produce nh's text without an Env.
const helpTestdata = "../testdata"

func abs(t *testing.T, path string) string {
	t.Helper()
	p, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// recorded is a recorded nh stdout with its root placeholder replaced.
// The outputs were recorded on Linux: placeholder/a/b is root joined with a
// and b by os.path.join, so on Windows its "/" become "\".
func recorded(t *testing.T, file, placeholder, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(helpTestdata, file))
	if err != nil {
		t.Fatal(err)
	}
	rest := regexp.MustCompile(regexp.QuoteMeta(placeholder) + `(/[^\s)'"]*)`)
	s := rest.ReplaceAllStringFunc(string(raw), func(m string) string {
		return root + filepath.FromSlash(m[len(placeholder):])
	})
	return strings.ReplaceAll(s, placeholder, root)
}

func joined(lines []string) string { return strings.Join(lines, "\n") + "\n" }

// TestMissOnFulltextCheckout: the blocks of find, guide-search,
// devguide-search and pdf, including a borrowed docs-text/ ("shared from
// Niagara 1.0") and an absent index, equal what tools/niagara_help.py printed.
func TestMissOnFulltextCheckout(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	checkout := ftsfixture.Checkout(t, abs(t, helpTestdata), false)
	tests := []struct {
		file, version, command, query, scope string
	}{
		{"find-miss", "1.0", "find", "zzqq wwvv", "fulltext"},
		{"guide-search-miss", "2.0", "guide-search", "zzqq", "guides"},
		{"devguide-search-miss", "1.0", "devguide-search", "zzqq", "devguide"},
		{"pdf-shared-miss", "2.0", "pdf", "zzqq", "pdf"},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			root := filepath.Join(checkout, ftsfixture.Versions[tt.version])
			root = filepath.Clean(root)
			lines, err := Miss(root, tt.command, tt.query, tt.scope, nil)
			want := recorded(t, filepath.Join("fulltext-cli", tt.file+".txt"), ftsfixture.Root, checkout)
			if err != nil || joined(lines) != want {
				t.Errorf("err %v, got:\n%s\nwant:\n%s", err, joined(lines), want)
			}
		})
	}
}

// TestMissCountsClassIndexAndUsesTheLoader: the class index line counts names
// and classes (others included), and an existing index is read through the
// given loader, so nh's cache serves it.
func TestMissCountsClassIndexAndUsesTheLoader(t *testing.T) {
	root := abs(t, helpTestdata)
	want := recorded(t, filepath.Join("misses", "class-qualified-miss.txt"), "<TESTDATA>", root)
	var asked []string
	load := func(name string) (*index.Object, error) {
		asked = append(asked, name)
		return index.LoadObject(index.Path(root, name))
	}
	lines, err := Miss(root, "class", "nope.BZzz", "classes", load)
	if err != nil || joined(lines) != want {
		t.Errorf("err %v, got:\n%s\nwant:\n%s", err, joined(lines), want)
	}
	if strings.Join(asked, ",") != "class-index.json" {
		t.Errorf("loader asked for %v", asked)
	}
}

func TestMissPropagatesLoaderErrorAndRejectsUnknownScope(t *testing.T) {
	root := abs(t, helpTestdata)
	boom := errors.New("boom")
	if _, err := Miss(root, "class", "x", "classes", func(string) (*index.Object, error) { return nil, boom }); err != boom {
		t.Errorf("loader error: %v", err)
	}
	var unknown UnknownScopeError
	if _, err := Miss(root, "x", "y", "nope", nil); !errors.As(err, &unknown) ||
		err.Error() != "unknown search scope 'nope'" {
		t.Errorf("unknown scope: %v", err)
	}
}

func TestMissUnregisteredRootWithAbsentIndexes(t *testing.T) {
	root := t.TempDir()
	lines, err := Miss(root, "guide", "q", "guides", nil)
	want := "  searched: Niagara unregistered (" + root + ") -- guides-index.json: absent; guides-clean/: absent"
	if err != nil || len(lines) != 4 || lines[1] != want {
		t.Errorf("err %v lines %q", err, lines)
	}
}

const twoVersions = `{"default": "2.0.0.1", "versions": {
 "2.0.0.1": {"label": "Two", "install_dir": "/nonexistent-install", "data_root": "versions/2.0.0.1"},
 "1.0.0.1": {"label": "One", "install_dir": ".", "data_root": "."}}}`

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestResolveAndVersionLines: the texts nh prints on stdout (exit 2) for an
// unknown version and a registered one that is not extracted, and a clean
// resolution of an extracted one.
func TestResolveAndVersionLines(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "versions", "registry.json"), twoVersions)
	write(t, filepath.Join(dir, "indexes", "class-index.json"), "{}")
	noEnv := func(string) string { return "" }

	_, _, err := Resolve(dir, "7", noEnv)
	var uv *index.UnknownVersionError
	if !errors.As(err, &uv) || joined(VersionLines(err)) !=
		"ERROR: Unknown Niagara version '7'. Known versions: 1.0.0.1, 2.0.0.1\n" {
		t.Errorf("unknown version: %v %q", err, VersionLines(err))
	}

	_, _, err = Resolve(dir, "", noEnv) // the default 2.0.0.1 has no indexes
	want := "ERROR: Niagara version 2.0.0.1 is registered but not extracted at " +
		filepath.Join(dir, "versions", "2.0.0.1") + ".\nRun: nh versions extract 2.0.0.1\n"
	if joined(VersionLines(err)) != want {
		t.Errorf("not extracted: %q", VersionLines(err))
	}

	if id, root, err := Resolve(dir, "1.0", noEnv); err != nil || id != "1.0.0.1" || root != dir {
		t.Errorf("extracted: %q %q %v", id, root, err)
	}
}

// TestErrorLineForMissingFulltextIndex: the stderr line nh prints (exit 2)
// when fulltext.sqlite is missing, from the fts error the search returns.
func TestErrorLineForMissingFulltextIndex(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	checkout := ftsfixture.Checkout(t, abs(t, helpTestdata), true)
	_, err := ftsfixture.Search(checkout, "1.0", "alarm", nil, "and")
	var stale *fts.StaleIndexError
	want := "ERROR: full-text index indexes/fulltext.sqlite under " + checkout + " is missing; this reader " +
		"never builds it, rebuild it with: nh --niagara-version 1.0 find index"
	if !errors.As(err, &stale) || ErrorLine(err) != want {
		t.Errorf("err %v line %q", err, ErrorLine(err))
	}
}
