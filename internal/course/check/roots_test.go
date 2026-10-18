package check

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/course/key"
)

// writeTree writes files (path -> content, "/"-separated; a path ending in "/" is a directory)
// under a fresh directory and returns it.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, data := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if strings.HasSuffix(rel, "/") {
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestRootKinds: files and file-text read the install or the lab sources when the row names that
// root; {a,b} lists exact names, a name ending in "/" is a directory, ".." reaches beside the dev
// home (the course's ..\certificates), and Whole matches the regexp over the whole text.
func TestRootKinds(t *testing.T) {
	install := writeTree(t, map[string]string{
		"defaults/system.properties": "#niagara.moduleVerificationMode=low\r\nniagara.moduleVerificationMode=high\r\n",
	})
	labs := writeTree(t, map[string]string{
		"certificates/code_signing.xml":   "<properties>\n<entry key=\"niagara.signing.standardtsa\">\n  http://timestamp.digicert.com\n</entry>\n</properties>\n",
		"certificates/code_signing.jceks": "x",
		"dev/build.gradle.kts":            "",
		"dev/training/training-rt/src/":   "",
	})
	labsrc := filepath.Join(labs, "dev")
	st, err := Open(writeStation(t), install, false)
	if err != nil {
		t.Fatal(err)
	}
	st.labsrc = labsrc
	tsa := `<entry key="niagara\.signing\.standardtsa">\s*http://timestamp\.digicert\.com\s*</entry>`
	cases := []struct {
		ck       Check
		verdict  string
		evidence string
	}{
		{Check{Kind: kindFileText, Root: rootInstall, Glob: "defaults/system.properties", Match: `^\s*niagara\.moduleVerificationMode\s*[=:]\s*high\s*$`, Op: opEq, N: 1},
			Pass, "defaults/system.properties: 1 line matches"},
		{Check{Kind: kindFileText, Root: rootInstall, Glob: "defaults/system.properties", Match: `^niagara\.moduleVerificationMode=low$`, Op: opEq, N: 1},
			Fail, "0 of 1 files pass, want exactly 1 line =~ ^niagara\\.moduleVerificationMode=low$: defaults/system.properties: 0 lines match"},
		{Check{Kind: kindFileText, Root: rootLabsrc, Glob: "../certificates/code_signing.xml", Match: tsa, Whole: true, Op: opEq, N: 1},
			Pass, "../certificates/code_signing.xml: 1 match (first at line 2: <entry key=\"niagara.signing.standardtsa\"> …)"},
		// Line by line the entry and its value on the next line never match together.
		{Check{Kind: kindFileText, Root: rootLabsrc, Glob: "../certificates/code_signing.xml", Match: tsa, Op: opMin, N: 1},
			Fail, "0 of 1 files pass, want at least 1 line =~ "},
		{Check{Kind: kindFiles, Root: rootLabsrc, Glob: "../certificates/{code_signing.jceks,code_signing.xml}", Op: opEq, N: 2},
			Pass, "found 2 ../certificates/{code_signing.jceks,code_signing.xml}: code_signing.jceks … code_signing.xml"},
		{Check{Kind: kindFiles, Root: rootLabsrc, Glob: "{build.gradle.kts,settings.gradle.kts}", Op: opEq, N: 2},
			Fail, "found 1 {build.gradle.kts,settings.gradle.kts}, want exactly 2: build.gradle.kts"},
		{Check{Kind: kindFiles, Root: rootLabsrc, Glob: "training/training-rt/{src/,srcTest/}", Op: opEq, N: 1},
			Pass, "found 1 training/training-rt/{src/,srcTest/}: src/"},
		{Check{Kind: kindFiles, Root: rootLabsrc, Glob: "training/training-rt/src", Op: opMin, N: 1},
			Fail, "found 0 training/training-rt/src, want at least 1"}, // a directory only when the name ends in "/"
	}
	for _, c := range cases {
		r := st.Run(&c.ck)
		if r.Verdict != c.verdict || !strings.HasPrefix(r.Evidence, c.evidence) {
			t.Errorf("%s %s %s:\n got %s %q\nwant %s %q", c.ck.Kind, c.ck.Root, c.ck.Glob, r.Verdict, r.Evidence, c.verdict, c.evidence)
		}
	}
}

// A root the run was not given is SKIP, as a jar check without --install is; a root that is not a
// readable directory is SKIP too (a configuration error), never FAIL.
func TestRootMissing(t *testing.T) {
	st, err := Open(writeStation(t), "", false)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		install, labsrc string
		ck              Check
		evidence        string
	}{
		{"", "", Check{Kind: kindFileText, Root: rootInstall, Glob: "defaults/system.properties", Match: `.`, Op: opMin, N: 1},
			"no --install DIR: defaults/system.properties not checked"},
		{"", "", Check{Kind: kindFiles, Root: rootLabsrc, Glob: "build.gradle.kts", Op: opMin, N: 1},
			"no --labsrc DIR: build.gradle.kts not checked"},
		{"", file, Check{Kind: kindFiles, Root: rootLabsrc, Glob: "build.gradle.kts", Op: opMin, N: 1},
			"cannot read " + file + ": not a directory"},
		{filepath.Join(file, "none"), "", Check{Kind: kindFileText, Root: rootInstall, Glob: "a", Match: `.`, Line: 1},
			"cannot read " + filepath.Join(file, "none") + ":"},
	}
	for _, c := range cases {
		st.install, st.labsrc = c.install, c.labsrc
		r := st.Run(&c.ck)
		if r.Verdict != Skip || !strings.HasPrefix(r.Evidence, c.evidence) {
			t.Errorf("%s %s: %s %q, want SKIP %q", c.ck.Root, c.ck.Glob, r.Verdict, r.Evidence, c.evidence)
		}
	}
}

// A files or file-text row over the install or the lab sources is offline evidence: no station
// holds it. The default root (the station directory) stays live.
func TestRootEvidence(t *testing.T) {
	for _, c := range []struct {
		root, want string
	}{{"", key.EvidenceLive}, {rootInstall, key.EvidenceOffline}, {rootLabsrc, key.EvidenceOffline}} {
		for _, kind := range []string{kindFiles, kindFileText} {
			res := []LinkedResult{{Result: Result{Check: &Check{Kind: kind, Root: c.root}}}}
			if got := Evidence(res); got != c.want {
				t.Errorf("%s root %q: %s, want %s", kind, c.root, got, c.want)
			}
		}
	}
	if !NeedsLabSources([]*Check{{Kind: kindFiles, Root: rootLabsrc}}) || NeedsLabSources([]*Check{{Kind: kindFiles}}) {
		t.Error("NeedsLabSources: a lab-source files row must name the tree, a station one must not")
	}
}

func TestExpandBraces(t *testing.T) {
	for _, c := range []struct {
		in   string
		want []string
	}{
		{"a/b", []string{"a/b"}},
		{"a/{b,c}", []string{"a/b", "a/c"}},
		{"{x,y}/{b,c/}", []string{"x/b", "x/c/", "y/b", "y/c/"}},
		{"a/{b", []string{"a/{b"}}, // an unclosed brace is literal
	} {
		if got := expandBraces(c.in); !slices.Equal(got, c.want) {
			t.Errorf("expandBraces(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// The Lab 5 profile rows over a profile left as createProfile writes it before the lab's edits: an
// OU, CN not ${alias}, no standardtsa, a local profile; and an install still at low.
func TestLab5ProfileRowsFail(t *testing.T) {
	labs := writeTree(t, map[string]string{
		"certificates/code_signing.xml": "<properties>\n" +
			"<entry key=\"niagara.signing.dname\">CN=Name,OU=Unit,O=Company,L=City,ST=State,C=XX</entry>\n" +
			"<entry key=\"niagara.signing.profileType\">com.tridium.gradle.plugins.signing.profile.LocalSigningProfile</entry>\n" +
			"</properties>\n",
		"dev/": "",
	})
	install := writeTree(t, map[string]string{"defaults/system.properties": "niagara.moduleVerificationMode=low\nniagara.moduleVerificationMode=high\n"})
	st, err := Open(writeStation(t), install, false)
	if err != nil {
		t.Fatal(err)
	}
	st.labsrc = filepath.Join(labs, "dev")
	for _, id := range []string{"C5.5", "C5.6", "C5.7", "C5.8", "C5.9", "C5.10"} {
		if r := st.Run(specRow(id)); r.Verdict != Fail {
			t.Errorf("%s: %s %q, want FAIL", id, r.Verdict, r.Evidence)
		}
	}
	if r := st.Run(specRow("C5.4")); r.Verdict != Pass { // high is set, but C5.5 sees the second setting
		t.Errorf("C5.4: %s %q", r.Verdict, r.Evidence)
	}
}
