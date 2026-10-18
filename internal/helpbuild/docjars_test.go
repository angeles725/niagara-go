package helpbuild

import (
	"reflect"
	"strings"
	"testing"
)

func dest(t *testing.T, jar, entry string) string {
	t.Helper()
	rel, ok := DestPath(jar, entry, mustDefault(t))
	if !ok {
		return "<none>"
	}
	return rel
}

// test_extraction.TestDestPath, case by case.
func TestDestPathMapsEveryJarKind(t *testing.T) {
	cases := [][3]string{
		// test_developer_doc_goes_to_devguide, test_developer_rc_goes_to_devguide_rc
		{"docDeveloper-doc.jar", "doc/web-rt/x.bajadoc", "devguide/web-rt/x.bajadoc"},
		{"docDeveloper-doc.jar", "rc/a/B.java", "devguide-rc/a/B.java"},
		// test_source_keeps_full_path, test_jdk, test_javadoc_jar_goes_to_bajadoc
		{"docSource-doc.jar", "web-rt/javax/W.java", "source/web-rt/javax/W.java"},
		{"docSource-doc.jar", "doc/index.html", "source/doc/index.html"},
		{"docJdk-doc.jar", "doc/jdk-se/a.bajadoc", "jdk/jdk-se/a.bajadoc"},
		{"niagaraJavadoc.jar", "doc/index.html", "bajadoc/index.html"},
		// test_guides_strip_doc_prefix, test_non_doc_entry_kept_relative
		{"docAlarms-doc.jar", "doc/AboutAlarms.html", "guides/Alarms/AboutAlarms.html"},
		{"docHoneywell.jar", "doc/x.html", "guides/Honeywell/x.html"},
		{"clHVAC-doc.jar", "doc/bajadoc.index", "guides/clHVAC/bajadoc.index"},
		{"docDeveloperAnalytics-doc.jar", "docDeveloperAnalytics-doc.lexicon",
			"guides/DeveloperAnalytics/docDeveloperAnalytics-doc.lexicon"},
		// test_meta_inf_and_dirs_skipped
		{"docAlarms-doc.jar", "META-INF/MANIFEST.MF", "<none>"},
		{"docAlarms-doc.jar", "doc/", "<none>"},
		// test_developer_examples_and_module_help_pages_map
		{"docDeveloper-ux.jar", "examples/bajaux/A.js", "devguide-examples/examples/bajaux/A.js"},
		{"docDeveloper-ux.jar", "com/x/A.class", "<none>"},
		{"docDeveloper-rt.jar", "module.palette", "<none>"},
		{"docDeveloper-rt.jar", "rc/com/x/BVal.java", "devguide-rc/com/x/BVal.java"},
		{"ascCommon.jar", "doc/About-Asc.html", "guides/ascCommon/About-Asc.html"},
		{"ascCommon.jar", "com/x/BAsc.class", "<none>"},
	}
	// test_module_docs_outside_doc_folder_map_by_extract_rules
	for _, je := range [][2]string{{"bajaScript-ux.jar", "rc/env/bajaScriptCommunications.md"},
		{"electronicSignature-ux.jar", "rc/ReadMe.txt"}, {"batchJob-ux.jar", "jsdoc/RetentionPolicyFE.html"},
		{"workbench-wb.jar", "com/tridium/workbench/media/about.html"}, {"galileoSupervisor-wb.jar", "res/alertonHome.html"},
		{"micros-wb.jar", "help.html"}, {"micros-ux.jar", "rc/html/altHelp.html"}} {
		cases = append(cases, [3]string{je[0], je[1], "guides/" + strings.TrimSuffix(je[0], ".jar") + "/" + je[1]})
	}
	for _, je := range [][2]string{{"easyBinding-ux.jar", "rc/node_modules/x/README.md"},
		{"DashboardFord-ux.jar", "rc/vendor/THIRD-PARTY.md"}, {"batchJob-ux.jar", "jsdoc/scripts/URI.js"},
		{"batchJob-ux.jar", "rc/batchJob.htm"}, {"other-wb.jar", "com/tridium/workbench/media/about.html"}} {
		cases = append(cases, [3]string{je[0], je[1], "<none>"})
	}
	for _, c := range cases {
		if got := dest(t, c[0], c[1]); got != c[2] {
			t.Errorf("DestPath(%s, %s) = %s, want %s", c[0], c[1], got, c[2])
		}
	}
}

// test_extraction.test_is_doc_jar.
func TestIsDocJar(t *testing.T) {
	for name, want := range map[string]bool{"docAlarms-doc.jar": true, "docHoneywell.jar": true, "clHVAC-doc.jar": true,
		"alarm-rt.jar": false, "docDeveloper-rt.jar": true, "docDeveloper-ux.jar": true, "ascCommon.jar": false,
		"docX-rt.jar": false, JavadocJar: false} {
		if IsDocJar(name) != want {
			t.Errorf("IsDocJar(%s) = %v", name, !want)
		}
	}
	if IsModuleJar(JavadocJar) || IsModuleJar("docAlarms-doc.jar") || !IsModuleJar("alarm-rt.jar") {
		t.Error("IsModuleJar")
	}
	if JarStem("x") != "" || GuideFolder("doc.jar") != "doc" || GuideFolder("docñ-doc.jar") != "ñ" {
		t.Error("JarStem / GuideFolder slice code points like Python")
	}
}

// An entry repeated in the zip, or two entries mapping to one path: the
// first wins, the later one collides with it.
func TestMappedEntriesFirstWins(t *testing.T) {
	got := MappedEntries("thing-ux.jar", []string{"rc/notes.md", "doc/rc/notes.md", "rc/", "doc/x.tar.gz", "rc/notes.md"},
		mustDefault(t))
	want := []Mapped{{Pos: 0, Entry: "rc/notes.md", Rel: "guides/thing-ux/rc/notes.md"},
		{Pos: 1, Entry: "doc/rc/notes.md", Rel: "guides/thing-ux/rc/notes.md", First: "rc/notes.md", Collides: true},
		{Pos: 4, Entry: "rc/notes.md", Rel: "guides/thing-ux/rc/notes.md", First: "rc/notes.md", Collides: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v", got)
	}
}
