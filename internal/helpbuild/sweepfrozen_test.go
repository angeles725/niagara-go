package helpbuild

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/index"
)

// The whole-install sweep (install_sweep.cmd_sweep) on the fixture the
// retired testdata/parity/sweepdriver.py built, against its frozen Python
// output (frozen_test.go). sweepFrozenFixture builds the same tree with the
// Go ports of the steps the driver ran in Python (Extract, RebuildAll, the
// full-text build); the frozen reports were checked byte-identical to the
// Python sweep over the Python-built fixture.

// sweepLines is the report as lines: ["out", line] per report line, then
// ["exit", status].
func sweepLines(report string, status int) []string {
	var out []string
	if report != "" {
		for _, line := range strings.Split(strings.TrimSuffix(report, "\n"), "\n") {
			out = append(out, testkit.DumpLine(testkit.Str("out"), testkit.Str(line)))
		}
	}
	return append(out, fmt.Sprintf(`["exit", %d]`, status))
}

// goSweep runs RunSweep with the rules file rulesPath ("-": the shipped
// rules).
func goSweep(repo, base, vid, rulesPath string) ([]string, error) {
	var opts SweepOptions
	if rulesPath != "-" {
		r, err := Load(rulesPath)
		if err != nil {
			return nil, err
		}
		opts.Rules = r
	}
	var buf bytes.Buffer
	status, err := RunSweep(repo, base, vid, &buf, opts)
	if err != nil {
		return nil, err
	}
	return sweepLines(buf.String(), status), nil
}

// sweepRepo makes a repository holding a copy of the registry at src.
func sweepRepo(t *testing.T, src string) string {
	t.Helper()
	reg, err := os.ReadFile(filepath.Join(src, index.RegistryRel))
	if err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	testkit.WriteFile(t, filepath.Join(repo, index.RegistryRel), reg)
	return repo
}

// lockDir makes dir unreadable for the test and reports whether that
// worked (root reads mode-0 folders).
func lockDir(t *testing.T, dir string) bool {
	t.Helper()
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	_, err := os.ReadDir(dir)
	return err != nil
}

const sweepFixturePage = "<html><head><title>%s</title></head><body><p>%s</p></body></html>"

func sweepFixtureJar(t *testing.T, path string, entries ...[2]string) {
	testkit.WriteJar(t, path, entries...)
}

func fpage(title, body string) string { return fmt.Sprintf(sweepFixturePage, title, body) }

// sweepFixtureBajadoc is the driver's BAJADOC document of class pkg.name.
func sweepFixtureBajadoc(pkg, name string) string {
	return "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<bajadoc>\n<class qualifiedName=\"" + pkg + "." + name +
		"\" name=\"" + name + "\" packageName=\"" + pkg + "\"><description>d</description></class>\n</bajadoc>\n"
}

// sweepFixtureInstall is the driver's make_install: the test_install_sweep
// fixture plus every content kind, the installed-copy and in-jar-duplicate
// rules, empty, corrupt, upper-case and non-ASCII jars, a root jar, a
// location longer than its column, more than 25 gaps under sw/, loose docs
// and the folders the test locks. The entries are text, encoded as UTF-8
// (the "Êþ" of a class file included), as zipfile.writestr wrote them.
func sweepFixtureInstall(t *testing.T, root string) {
	t.Helper()
	mods := filepath.Join(root, "modules")
	help := [][2]string{{"doc/About-Help.html", fpage("About Help", "helpzork page")}, {"com/acme/BHelp.class", "Êþ"}}
	sweepFixtureJar(t, filepath.Join(mods, "docGuide-doc.jar"),
		[2]string{"doc/index.html", fpage("Guide", "guide home")}, [2]string{"doc/Intro.html", fpage("Intro", "intro text")},
		[2]string{"doc/LICENSE.txt", "licence text\n"},
		[2]string{"doc/toc.xml", "<toc><tocitem text=\"Intro\" target=\"Intro.html\"/></toc>\n"},
		[2]string{"doc/notes.text", "a .text file\n"}, [2]string{"doc/odd.weird", "a file in a doc folder\n"})
	sweepFixtureJar(t, filepath.Join(mods, "helpMod-rt.jar"), help...)
	sweepFixtureJar(t, filepath.Join(mods, "thing-ux.jar"),
		[2]string{"rc/notes.md", "# Notes\n\nquibbleNotes of the thing module\n"},
		[2]string{"rc/node_modules/pkg/README.md", "npm readme\n"}, [2]string{"rc/thing.js", "var x;"})
	sweepFixtureJar(t, filepath.Join(mods, "lib-rt.jar"),
		[2]string{"org/apache/words.txt", "a\nb\n"}, [2]string{"com/acme/BLib.class", "Êþ"},
		[2]string{"com/acme/readme.txt", "a module readme\n"})
	sweepFixtureJar(t, filepath.Join(mods, "docDeveloper-doc.jar"),
		[2]string{"doc/index.html", fpage("Dev", "developer guide")},
		[2]string{"doc/toc.xml", "<toc><tocitem text=\"Dev\" target=\"index.html\"/></toc>\n"},
		[2]string{"doc/module-index.bajadoc", "<bajadoc/>\n"},
		[2]string{"doc/javax/baja/dev/BDev.bajadoc", sweepFixtureBajadoc("javax.baja.dev", "BDev")},
		[2]string{"doc/broken.bajadoc", "<bajadoc><class"}, [2]string{"doc/Readme.TXT", "upper-case extension\n"},
		[2]string{"rc/Example.java", "public class Example {}\n"}, [2]string{"rc/readme.txt", "rc readme\n"})
	sweepFixtureJar(t, filepath.Join(mods, "docDeveloper-ux.jar"),
		[2]string{"examples/demo.js", "function demo() { return 1; }\n"}, [2]string{"examples/demo.min.js", "function a(){}\n"},
		[2]string{"examples/page.html", fpage("Example", "example page")}, [2]string{"examples/style.css", "p {}\n"},
		[2]string{"ext/babel/readme.md", "babel\n"}, [2]string{"examples/Thumbs.db", "x"}, [2]string{"examples/data.bin", "x"})
	sweepFixtureJar(t, filepath.Join(mods, "docSource-doc.jar"),
		[2]string{"doc/toc.xml", "<toc/>\n"}, [2]string{"doc/index.html", fpage("Src", "source browser")},
		[2]string{"javax/baja/sys/readme.txt", "source readme\n"})
	sweepFixtureJar(t, filepath.Join(mods, "docJdk-doc.jar"),
		[2]string{"doc/java/lang/Object.bajadoc", sweepFixtureBajadoc("java.lang", "Object")},
		[2]string{"doc/package-index.bajadoc", "<bajadoc/>\n"})
	sweepFixtureJar(t, filepath.Join(root, "javadoc", "niagaraJavadoc.jar"),
		[2]string{"doc/index.html", fpage("Javadoc", "frames")},
		[2]string{"doc/allclasses-noframe.html", "<html><body><ul>\n<li><a href=\"javax/baja/sys/BObject.html\" " +
			"title=\"class in javax.baja.sys\">BObject</a></li>\n</ul></body></html>\n"},
		[2]string{"doc/javax/baja/sys/BObject.html", fpage("BObject", "class")},
		[2]string{"doc/javax/baja/sys/BMissing.html", fpage("BMissing", "none")},
		[2]string{"doc/javax/baja/sys/class-use/BObject.html", "<p>use"},
		[2]string{"doc/javax/baja/sys/notes.txt", "javadoc text file\n"})
	sweepFixtureJar(t, filepath.Join(mods, "mmm-ux.jar"), [2]string{"doc/rc/x.md", "# first\n"}, [2]string{"rc/x.md", "# second\n"})
	sweepFixtureJar(t, filepath.Join(mods, "nnn-ux.jar"), [2]string{"doc/rc/y.md", "# same\n"}, [2]string{"rc/y.md", "# same\n"})
	sweepFixtureJar(t, filepath.Join(mods, "ffmpeg-wb.jar"),
		[2]string{"doc/index.html", fpage("ffmpeg", "licence offer")}, [2]string{"doc/ffmpeg-6.0.tar.xz", "xz"})
	sweepFixtureJar(t, filepath.Join(mods, "Upper.JAR"), [2]string{"doc/a.html", "<p>a"})
	testkit.WriteFile(t, filepath.Join(mods, "broken-rt.jar"), []byte("PK\x03\x04 not a zip"))
	testkit.WriteFile(t, filepath.Join(mods, "empty-rt.jar"), nil)
	sweepFixtureJar(t, filepath.Join(mods, "ünï-rt.jar"), [2]string{"doc/été.html", fpage("Ete", "été")})
	sweepFixtureJar(t, filepath.Join(root, "loose.jar"), [2]string{"doc/a.html", "<p>root"})
	sweepFixtureJar(t, filepath.Join(root, "a-very-long-top-level-folder-name-indeed", "x.jar"), [2]string{"doc/a.html", "<p>long"})
	// sw/: an identical copy, a changed copy and a copy with many pages
	// the installed jar does not have (more than FIRST unclassified).
	sweepFixtureJar(t, filepath.Join(root, "sw", "1.0", "helpMod-rt.jar"), help...)
	sweepFixtureJar(t, filepath.Join(root, "sw", "2.0", "helpMod-rt.jar"), [2]string{"doc/About-Help.html", fpage("About Help", "newer text")})
	var pages [][2]string
	for i := range 30 {
		pages = append(pages, [2]string{fmt.Sprintf("doc/p%02d.html", i), fmt.Sprintf("<p>%d", i)})
	}
	sweepFixtureJar(t, filepath.Join(root, "sw", "3.0", "helpMod-rt.jar"), pages...)
	sweepFixtureJar(t, filepath.Join(root, "sw", "locked", "x.jar"), [2]string{"doc/a.html", "<p>a"})
	sweepFixtureJar(t, filepath.Join(root, "jre", "lib", "rt.jar"), [2]string{"about.html", "x"})
	for rel, data := range map[string]string{
		"docs/menu/tree.html": fpage("Docs", "menu"), "docs/Manual.pdf": "%PDF-1.4 not really", "docs/sub/Manual2.PDF": "%PDF-1.4",
		"docs/Missing.pdf": "%PDF-1.4", "docs/notes.txt": "loose notes\n", "docs/image.png": "png", "docs/locked/Manual3.pdf": "%PDF-1.4",
	} {
		testkit.WriteFile(t, filepath.Join(root, filepath.FromSlash(rel)), []byte(data))
	}
}

// sweepFixtureCourse is the driver's COURSE_FILES.
var sweepFixtureCourse = map[string]string{
	"text/Review Assignment 1.txt":                      "Review Assignment 1\n\fPage two\n",
	"text/8 ORDs.txt":                                   "ORDs\n\fNMessage link layer\n",
	"course/Review Assignments/Review Assignment 1.pdf": "%PDF-1.4",
	"course/Slides/8 ORDs.pdf":                          "%PDF-1.4",
	"course/Slides/9 Orphan.pdf":                        "%PDF-1.4",
	"course/Labs/Lab1 First/src/BFirst.java":            "public class BFirst {}\n",
	"course/Labs/Lab5 Code Signing/rootca.pem":          "-----BEGIN CERT",
	"course/Labs/Lab1 First/module.palette":             "<palette/>",
	"niagara-4.15.6.20/dev/demo/readme.txt":             "Demo readme\n",
	"niagara-4.15.6.20/dev/demo/src/BDemo.java":         "public class BDemo {}\n",
	"solutions/cert-exam/BAnswer.java":                  "public class BAnswer {}\n",
	"MANIFEST.sha256":                                   "abc  x\n",
}

// sweepFrozenFixture is the driver's fixture: the install, the course tree,
// the repository (1.0 extracted and rebuilt, 2.0 registered on the same
// install but never extracted, 3.0 without an install), the test rules file
// (the test's exclusions with the shipped extract, globs and course rules),
// the PDF text and pdf-index.json of 1.0 and its full-text index.
func sweepFrozenFixture(t *testing.T, dir string) (install, repo, rules string) {
	t.Helper()
	install = filepath.Join(dir, "install")
	sweepFixtureInstall(t, install)
	src := filepath.Join(dir, "course-src")
	for rel, text := range sweepFixtureCourse {
		testkit.WriteFile(t, filepath.Join(src, filepath.FromSlash(rel)), []byte(text))
	}
	repo = filepath.Join(dir, "repo")
	for _, vid := range []string{"1.0", "2.0", "3.0"} {
		if err := os.MkdirAll(filepath.Join(repo, "versions", vid, "indexes"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	reg, err := json.Marshal(map[string]any{"default": "1.0", "versions": map[string]any{
		"1.0": map[string]string{"label": "fixture", "install_dir": install, "data_root": "versions/1.0", "course_dir": src},
		"2.0": map[string]string{"label": "same install, nothing extracted", "install_dir": install, "data_root": "versions/2.0"},
		"3.0": map[string]string{"label": "no install", "install_dir": "", "data_root": "versions/3.0"}}})
	if err != nil {
		t.Fatal(err)
	}
	testkit.WriteFile(t, filepath.Join(repo, "versions", "registry.json"), reg)
	var shipped map[string]any
	if err := json.Unmarshal(embeddedRules, &shipped); err != nil {
		t.Fatal(err)
	}
	ruleData, err := json.Marshal(map[string]any{
		"doc_like":  map[string]any{"extensions": []string{".html", ".htm", ".md", ".txt", ".pdf"}, "names": []string{"toc.xml"}},
		"skip_dirs": []any{map[string]string{"dir": "jre", "reason": "bundled Java runtime"}},
		"extract":   shipped["extract"],
		"exclude": []any{
			map[string]string{"id": "node-modules", "entry": "**/node_modules/**", "reason": "bundled npm package"},
			map[string]string{"id": "third-party-java", "entry": "org/**", "reason": "third-party library resource"},
			map[string]string{"id": "docs-menu", "jar": "(docs)", "entry": "menu/**", "reason": "PDF library menu frame"},
		},
		"globs": shipped["globs"], "course": shipped["course"],
	})
	if err != nil {
		t.Fatal(err)
	}
	rules = filepath.Join(dir, "rules.json")
	testkit.WriteFile(t, rules, ruleData)
	data := filepath.Join(repo, "versions", "1.0")
	if _, err := Extract(install, data, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := RebuildAll(data, RebuildOptions{Stdout: io.Discard, Stderr: io.Discard}); err != nil {
		t.Fatal(err)
	}
	testkit.WriteFile(t, filepath.Join(data, "docs-text", "Manual.txt"), []byte("manual text\n"))
	testkit.WriteFile(t, filepath.Join(data, "indexes", "pdf-index.json"), []byte(`{"Manual": {"source_pdf": "Manual.pdf"}, `+
		`"Manual2": {"source_pdf": "sub/manual2.pdf"}, "Manual3": {"source_pdf": "locked/Manual3.pdf"}, "NoSource": {}}`))
	if _, err := buildingDocs(data); err != nil {
		t.Fatal(err)
	}
	return install, repo, rules
}

// TestSweepFrozen: the gate on the fixture with the shipped rules and the
// test rules, identified from the data root, on a version sharing the
// install but never extracted (PDF fallback, course note), one without an
// install, an unknown version and an unregistered data root; each run
// cold, then again over its warm listing cache (recorded as the lineDelta
// from the cold report).
func TestSweepFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	dir := t.TempDir()
	install, repo, rules := sweepFrozenFixture(t, dir)
	if !lockDir(t, filepath.Join(install, "sw", "locked")) || !lockDir(t, filepath.Join(install, "docs", "locked")) {
		t.Skip("running with permissions that read mode-0 folders: the unreadable-folder rows cannot be checked")
	}
	for _, c := range []struct{ name, vid, base, rules string }{
		{"1.0-shipped-rules", "1.0", "1.0", "-"},
		{"1.0-test-rules", "1.0", "1.0", rules},
		{"identified-from-the-data-root", "", "1.0", "-"},
		{"2.0-nothing-extracted", "2.0", "2.0", "-"},
		{"3.0-no-install", "3.0", "3.0", "-"},
		{"unknown-version", "9.9", "1.0", "-"},
		{"unregistered-data-root", "", "none", "-"},
	} {
		base := filepath.Join(repo, "versions", c.base)
		r := sweepRepo(t, repo)
		var runs [2][]string
		for i := range runs {
			lines, err := goSweep(r, base, c.vid, c.rules)
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			runs[i] = testkit.Relabeled(lines, dir, "<dir>")
		}
		testkit.CheckFrozen(t, "sweep/"+c.name, append(append(runs[0], "warm:"), testkit.LineDelta(runs[0], runs[1])...))
	}
}
