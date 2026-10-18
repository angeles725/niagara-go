package helpbuild

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// test_install_sweep.TestGlobs.test_glob_semantics, plus the non-ASCII
// letters Python's re.IGNORECASE folds onto an ASCII letter (İ ı K ſ).
func TestGlobSemantics(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"**/node_modules/**", "node_modules/a.md", true},
		{"**/node_modules/**", "rc/node_modules/a/b", true},
		{"rc/**.md", "rc/env/x.md", true},
		{"rc/*.md", "rc/env/x.md", false},
		{"res/*Home.html", "res/alertonHome.html", true},
		{"*.jar", "sw/x.jar", false},
		{"doc/x.html", "doc/x.htmlx", false},
		{"META-INF/**", "meta-inf/x", true},
		{"META-INF/**", "META-İNF/x", true}, // İ: simple lower-case is i
		{"META-INF/**", "META-ıNF/x", true}, // ı: sre's case fix for i
		{"k.jar", "\u212a.JAR", true},       // Kelvin sign
		{"jsdoc/**.html", "jſdoc/a.html", true},
		{"a?c", "a/c", false},
		{"a?c", "a\nc", true},
		{"**", "a\nb", false}, // '.' stops at a newline, as in Python
		{"a+b(c)", "a+b(c)", true},
	}
	for _, c := range cases {
		if got := GlobRe(c.pattern).MatchString(c.path); got != c.want {
			t.Errorf("GlobRe(%q).match(%q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

func mustDefault(t *testing.T) *Rules {
	t.Helper()
	r, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// test_shipped_rules_are_well_formed.
func TestShippedRulesAreWellFormed(t *testing.T) {
	rules := mustDefault(t)
	for _, r := range append(append([]*Rule{}, rules.Extract...), rules.Exclude...) {
		if strings.TrimSpace(r.Reason) == "" {
			t.Errorf("rule %s has no reason", r.ID)
		}
	}
	for _, d := range rules.SkipDirs {
		if strings.TrimSpace(d.Reason) == "" {
			t.Errorf("skip dir %s has no reason", d.Dir)
		}
	}
	if !rules.IsDocLike("doc/A.HTML") || rules.IsDocLike("com/a/B.class") {
		t.Fatal("IsDocLike")
	}
	if len(rules.Extract) == 0 || len(rules.Exclude) == 0 {
		t.Fatal("no rules parsed")
	}
}

// TestEmbeddedRulesMatchCanonical is the drift test: the embedded copy must
// equal niagara-help tools/niagara_help_lib/install_docs.json byte for byte.
// $NIAGARA_HELP_DIR set to a folder without the file is a failure, not a
// skip; only when neither the variable nor a checkout exists does it skip.
func TestEmbeddedRulesMatchCanonical(t *testing.T) {
	tools := niagaraHelpTools()
	if tools == "" {
		t.Skip("no niagara-help checkout ($NIAGARA_HELP_DIR unset, none under $NIAGARA_RESEARCH or walking up): drift not checked")
	}
	canon, err := os.ReadFile(filepath.Join(filepath.Dir(tools), canonicalRules))
	if err != nil {
		t.Fatalf("canonical install_docs.json: %v", err)
	}
	t.Logf("canonical %s", filepath.Join(filepath.Dir(tools), canonicalRules))
	if !bytes.Equal(canon, embeddedRules) {
		t.Fatal("internal/helpbuild/install_docs.json drifted from niagara-help " +
			"tools/niagara_help_lib/install_docs.json: copy it again")
	}
}

// test_install_sweep.RULES: the sweep fixture's own rules.
const sweepRules = `{
 "doc_like": {"extensions": [".html", ".htm", ".md", ".txt", ".pdf"], "names": ["toc.xml"]},
 "skip_dirs": [{"dir": "jre", "reason": "bundled Java runtime"}, {"dir": "x", "reason": "first"},
               {"dir": "jre", "reason": "later wins"}],
 "globs": {"web": ["**/node_modules/**", "rc/vendor/**"]},
 "extract": [{"id": "rc-markdown", "entry": ["rc/**.md"], "unless": ["@web"], "reason": "md"}],
 "exclude": [
  {"id": "node-modules", "entry": "**/node_modules/**", "reason": "bundled npm package"},
  {"id": "third-party-java", "entry": "org/**", "reason": "third-party library resource"},
  {"id": "any-txt", "entry": "**.txt", "reason": "second match never wins"},
  {"id": "docs-menu", "jar": "(docs)", "entry": "menu/**", "reason": "PDF library menu frame"},
  {"id": "wb-only", "jar": ["*-wb.jar", "x.jar"], "entry": "res/**", "unless": ["res/keep/**"], "reason": "wb"}
 ],
 "course": {"text": ["text/*.txt"], "code": ["**.java"], "pdf": ["course/**.pdf"]}
}`

func sweepFixture(t *testing.T) *Rules {
	t.Helper()
	r, err := Parse([]byte(sweepRules), "rules.json")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func ruleID(r *Rule) string {
	if r == nil {
		return "-"
	}
	return r.ID
}

func TestRulesClassifyLikePython(t *testing.T) {
	r := sweepFixture(t)
	checks := []struct{ got, want string }{
		{ruleID(r.ExtractRule("thing-ux.jar", "rc/notes.md")), "rc-markdown"},
		{ruleID(r.ExtractRule("thing-ux.jar", "rc/node_modules/pkg/README.md")), "-"},
		{ruleID(r.ExtractRule("thing-ux.jar", "rc/vendor/a.md")), "-"},
		{ruleID(r.ExcludeRule("thing-ux.jar", "rc/node_modules/pkg/README.md")), "node-modules"},
		{ruleID(r.ExcludeRule("lib-rt.jar", "org/apache/words.txt")), "third-party-java"},
		{ruleID(r.ExcludeRule("lib-rt.jar", "doc/a.txt")), "any-txt"},
		{ruleID(r.ExcludeRule("(docs)", "menu/tree.html")), "docs-menu"},
		{ruleID(r.ExcludeRule("other.jar", "menu/tree.html")), "-"},
		{ruleID(r.ExcludeRule("A-WB.JAR", "res/a.png")), "wb-only"},
		{ruleID(r.ExcludeRule("x.jar", "res/keep/a.png")), "-"},
		{r.CourseKind("text/a.txt"), "text"},
		{r.CourseKind("text/sub/a.txt"), ""},
		{r.CourseKind("course/Labs/A.java"), "code"},
		{r.CourseKind("course/x.PDF"), "pdf"},
	}
	for i, c := range checks {
		if c.got != c.want {
			t.Errorf("check %d: got %q, want %q", i, c.got, c.want)
		}
	}
	if reason, ok := r.SkipReason("jre"); !ok || reason != "later wins" {
		t.Errorf("SkipReason(jre) = %q %v", reason, ok)
	}
	if len(r.SkipDirs) != 2 || r.SkipDirs[0].Dir != "jre" || r.SkipDirs[1].Dir != "x" {
		t.Errorf("SkipDirs %+v, want dict order jre, x", r.SkipDirs)
	}
	if !r.IsDocLike("a/toc.XML") || r.IsDocLike("a/b.css") {
		t.Error("IsDocLike names/extensions")
	}
}

func TestDocLikeDirs(t *testing.T) {
	r := mustDefault(t)
	if !r.IsDocLike("x/DOCS/a.bin") || r.IsDocLike("docs") || !r.IsDocLike("Help/x") {
		t.Error("doc_like dirs: any folder segment, case-insensitive, never the file name")
	}
}

func TestBadRulesAreErrors(t *testing.T) {
	bad := map[string]string{
		"unknown glob list": `{"doc_like": {"extensions": [], "names": []}, "skip_dirs": [], "extract": [],
			"exclude": [{"id": "a", "entry": "x", "unless": ["@nope"], "reason": "r"}]}`,
		"duplicate ids": `{"doc_like": {"extensions": [], "names": []}, "skip_dirs": [],
			"extract": [{"id": "a", "entry": "x", "reason": "r"}], "exclude": [{"id": "a", "entry": "y", "reason": "r"}]}`,
		"missing reason": `{"doc_like": {"extensions": [], "names": []}, "skip_dirs": [], "extract": [],
			"exclude": [{"id": "a", "entry": "x"}]}`,
		"missing doc_like": `{"skip_dirs": [], "extract": [], "exclude": []}`,
		"skip_dirs not a list": `{"doc_like": {"extensions": [], "names": []}, "skip_dirs": {"jre": "r"},
			"extract": [], "exclude": []}`,
		"skip_dirs a string": `{"doc_like": {"extensions": [], "names": []}, "skip_dirs": "jre",
			"extract": [], "exclude": []}`,
		"globs a list": `{"doc_like": {"extensions": [], "names": []}, "skip_dirs": [], "globs": ["web"],
			"extract": [], "exclude": []}`,
		"course a string": `{"doc_like": {"extensions": [], "names": []}, "skip_dirs": [], "course": "text",
			"extract": [], "exclude": []}`,
		"not JSON": `{`,
	}
	for name, data := range bad {
		if _, err := Parse([]byte(data), "rules.json"); err == nil {
			t.Errorf("%s: parsed without error", name)
		}
	}
	// data.get(...) or {}: a falsy globs or course of any type is no data.
	for _, falsy := range []string{`[]`, `""`, `0`, `null`, `{}`} {
		data := `{"doc_like": {"extensions": [], "names": []}, "skip_dirs": [], "globs": ` + falsy +
			`, "course": ` + falsy + `, "extract": [], "exclude": []}`
		if _, err := Parse([]byte(data), "rules.json"); err != nil {
			t.Errorf("falsy %s: %v", falsy, err)
		}
	}
	_, err := Parse([]byte(bad["unknown glob list"]), "rules.json")
	if err == nil || !strings.Contains(err.Error(), `rule a: unknown glob list @nope (define it under "globs")`) {
		t.Errorf("message %v", err)
	}
}

// canonicalRules is the data file inside a niagara-help checkout.
var canonicalRules = filepath.Join("tools", "niagara_help_lib", "install_docs.json")

// niagaraHelpTools is niagara-help tools/: $NIAGARA_HELP_DIR/tools when the
// variable is set; else $NIAGARA_RESEARCH/niagara-help/tools when that
// checkout holds the data file; else the first checkout met walking up
// from the test directory, at each level as <dir>/niagara-help or
// <dir>/niagara-research/niagara-help (a worktree under
// ~/niagara-go-worktrees finds ~/niagara-research/niagara-help). "" when
// there is none.
func niagaraHelpTools() string {
	if env := os.Getenv("NIAGARA_HELP_DIR"); env != "" {
		return filepath.Join(env, "tools")
	}
	if env := os.Getenv("NIAGARA_RESEARCH"); env != "" {
		help := filepath.Join(env, "niagara-help")
		if st, err := os.Stat(filepath.Join(help, canonicalRules)); err == nil && st.Mode().IsRegular() {
			return filepath.Join(help, "tools")
		}
	}
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		for _, sub := range []string{"niagara-help", filepath.Join("niagara-research", "niagara-help")} {
			if st, err := os.Stat(filepath.Join(dir, sub, canonicalRules)); err == nil && st.Mode().IsRegular() {
				return filepath.Join(dir, sub, "tools")
			}
		}
		up := filepath.Dir(dir)
		if up == dir {
			return ""
		}
		dir = up
	}
}

// The checkout lookup: $NIAGARA_HELP_DIR wins; $NIAGARA_RESEARCH (the
// variable the glossary and version drift tests read) is used when
// <it>/niagara-help holds the data file, else the walk up decides.
func TestNiagaraHelpToolsLookup(t *testing.T) {
	help := filepath.Join(t.TempDir(), "nh")
	t.Setenv("NIAGARA_HELP_DIR", help)
	t.Setenv("NIAGARA_RESEARCH", "")
	if got := niagaraHelpTools(); got != filepath.Join(help, "tools") {
		t.Fatalf("NIAGARA_HELP_DIR: %q", got)
	}
	research := t.TempDir()
	writeFile(t, filepath.Join(research, "niagara-help", canonicalRules), []byte("{}"))
	t.Setenv("NIAGARA_HELP_DIR", "")
	t.Setenv("NIAGARA_RESEARCH", research)
	if got := niagaraHelpTools(); got != filepath.Join(research, "niagara-help", "tools") {
		t.Fatalf("NIAGARA_RESEARCH: %q", got)
	}
	empty := t.TempDir()
	t.Setenv("NIAGARA_RESEARCH", empty)
	if got := niagaraHelpTools(); strings.HasPrefix(got, empty) {
		t.Fatalf("NIAGARA_RESEARCH without a checkout: %q", got)
	}
}
