package helpbuild

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts"
	ftbuild "github.com/angeles725/niagara-go/internal/helpbuild/fulltext"
	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// alarmPackageIndex is a package-index.bajadoc as the 4.15 doclet writes
// it: the description is escaped HTML with &#xa; line breaks.
const alarmPackageIndex = `<?xml version="1.0" encoding="UTF-8"?>
<bajadoc version="2.0">
<package module="alarm" runtimeProfile="rt" name="javax.baja.alarm">
<description>
&lt;p&gt;&#xa;Every alarm has an &lt;i&gt;alarm class&lt;/i&gt; which defines how and where the alarm is&#xa;routed.&#xa;&lt;/p&gt;&#xa;&lt;ul&gt;&lt;li&gt;HVAC alarms&lt;/li&gt;&lt;/ul&gt;
</description>
<class packageName="javax.baja.alarm" name="BAlarmClass"><description>A class description.</description></class>
</package>
</bajadoc>
`

const emptyPackageIndex = `<?xml version="1.0"?><bajadoc><package module="m" runtimeProfile="rt" name="p.empty"><description/>` +
	`<class packageName="p.empty" name="BX"><description>Not the package prose.</description></class></package></bajadoc>`

func TestBuildPackageText(t *testing.T) {
	base := t.TempDir()
	w := func(rel, data string) {
		testkit.WriteFile(t, filepath.Join(base, filepath.FromSlash(rel)), []byte(data))
	}
	w("devguide/alarm-rt/javax/baja/alarm/package-index.bajadoc", alarmPackageIndex)
	w("devguide/alarm-wb/javax/baja/alarm/package-index.bajadoc",
		strings.Replace(alarmPackageIndex, `runtimeProfile="rt"`, `runtimeProfile="wb"`, 1))
	w("guides/alarm/javax/baja/alarm/package-index.bajadoc", alarmPackageIndex) // the same text again
	w("devguide/m-rt/p/empty/package-index.bajadoc", emptyPackageIndex)
	w("devguide/bad-rt/p/bad/package-index.bajadoc", "<bajadoc><package")
	res, err := buildPackageText(base)
	if err != nil {
		t.Fatal(err)
	}
	want := "TITLE: javax.baja.alarm package\nPACKAGE: javax.baja.alarm\nMODULE: alarm-rt, alarm-wb\n\n" +
		"DESCRIPTION:\nEvery alarm has an alarm class which defines how and where the alarm is routed.\n\n- HVAC alarms\n"
	got, _ := os.ReadFile(filepath.Join(base, "package-text", "javax.baja.alarm.txt"))
	if string(got) != want {
		t.Errorf("package page:\n%q\nwant\n%q", got, want)
	}
	if files := copiedFiles(t, filepath.Join(base, "package-text")); !reflect.DeepEqual(files, []string{"javax.baja.alarm.txt"}) {
		t.Errorf("package-text/ holds %q (an empty description writes no page)", files)
	}
	if res.Files != 1 || len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "cannot parse") ||
		!reflect.DeepEqual(res.Log, []string{"package-text/: 1 package descriptions (from 4 package-index.bajadoc files)"}) {
		t.Errorf("result %+v", res)
	}
	if got := PackageDescription(base, "javax.baja.alarm"); len(got) < 2 || got[0] != "DESCRIPTION:" {
		t.Errorf("PackageDescription = %q", got)
	}
	if got := PackageDescription(base, "p.empty"); got != nil {
		t.Errorf("no page, no description: %q", got)
	}
	// Without any description the folder goes away: no stale pages.
	for _, rel := range []string{"devguide", "guides"} {
		if err := os.RemoveAll(filepath.Join(base, rel)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := buildPackageText(base); err != nil || pathExists(filepath.Join(base, "package-text")) {
		t.Errorf("an empty build kept package-text/ (%v)", err)
	}
}

func pathExists(p string) bool { _, err := os.Lstat(p); return err == nil }

// A package-index.bajadoc of the Niagara API is indexed through its
// package-text/ page (the empty listing has nothing beyond its classes);
// the JDK's and module-index.bajadoc stay listings.
func TestPackageIndexIsIndexed(t *testing.T) {
	for _, c := range []struct{ rel, verdict, detail string }{
		{"devguide/alarm-rt/javax/baja/alarm/package-index.bajadoc", verdictIndex, "bajadoc-package"},
		{"guides/A/p/package-index.bajadoc", verdictIndex, "bajadoc-package"},
		{"jdk/java/io/package-index.bajadoc", verdictExclude, reasons["bajadoc-listing"]},
		{"devguide/m/module-index.bajadoc", verdictExclude, reasons["bajadoc-listing"]},
	} {
		if v, d := classify(c.rel); v != c.verdict || d != c.detail {
			t.Errorf("classify(%s) = %s %q; want %s %q", c.rel, v, d, c.verdict, c.detail)
		}
	}
	root := t.TempDir()
	w := func(rel, data string) {
		testkit.WriteFile(t, filepath.Join(root, filepath.FromSlash(rel)), []byte(data))
	}
	w("devguide/alarm-rt/javax/baja/alarm/package-index.bajadoc", alarmPackageIndex)
	w("devguide/m-rt/p/empty/package-index.bajadoc", emptyPackageIndex)
	w("devguide/n-rt/p/gone/package-index.bajadoc", strings.ReplaceAll(alarmPackageIndex, "javax.baja.alarm", "p.gone"))
	if _, err := buildPackageText(root); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "package-text", "p.gone.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := ftbuild.Build(root, ftbuild.Options{}); err != nil {
		t.Fatal(err)
	}
	x := newIndexedViews(root, func(string) (map[string]bool, error) { return fulltextDocsAt(fts.DBPath(root)) })
	for rel, want := range map[string]bool{
		"devguide/alarm-rt/javax/baja/alarm/package-index.bajadoc": true,
		"devguide/m-rt/p/empty/package-index.bajadoc":              true,
		"devguide/n-rt/p/gone/package-index.bajadoc":               false,
	} {
		if got, err := x.check("bajadoc-package", rel); err != nil || got != want {
			t.Errorf("check(bajadoc-package, %s) = %v, %v; want %v", rel, got, err, want)
		}
	}
}

// The package descriptions are a rebuild-all step.
func TestRebuildPlanHasPackageText(t *testing.T) {
	for _, b := range rebuildBuilders {
		if b.Script == packageTextScript && reflect.DeepEqual(b.Produces, []string{"package-text/"}) {
			return
		}
	}
	t.Error("rebuild-all has no package_text_builder step")
}
