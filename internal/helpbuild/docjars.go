package helpbuild

import (
	"os"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// Doc-jar discovery and the jar-entry -> corpus-path mapping of one Niagara
// install (niagara_help_lib/doc_jars.py is_doc_jar ... mapped_entries):
//
//	javadoc/niagaraJavadoc.jar     doc/<p>            -> bajadoc/<p>
//	modules/docDeveloper-doc.jar   doc/<p>            -> devguide/<p>
//	                               rc/<p>             -> devguide-rc/<p>
//	modules/docDeveloper-ux.jar    <p> (not .class)   -> devguide-examples/<p>
//	modules/docDeveloper-rt.jar    rc/<p> (not .class) -> devguide-rc/<p>
//	modules/docSource-doc.jar      <p>                -> source/<p>
//	modules/docJdk-doc.jar         doc/<p>            -> jdk/<p>
//	modules/doc<Name>[-doc].jar    doc/<p> | <p>      -> guides/<Name>/<p>
//	modules/<name>-doc.jar         doc/<p> | <p>      -> guides/<name>/<p>
//	modules/<module>.jar           doc/<p>, or <p> named by an install_docs.json
//	                               extract rule       -> guides/<module>/<p>
//
// META-INF/ entries and directory entries are never extracted. Every
// function taking *Rules needs non-nil rules (Python's None default is
// Default(); the extraction entry points load it themselves).

// javadocJar is the install's javadoc/niagaraJavadoc.jar.
const javadocJar = "niagaraJavadoc.jar"

var (
	specialDocs       = map[string]string{"docDeveloper": "devguide", "docSource": "source", "docJdk": "jdk"}
	developerCodeJars = []string{"docDeveloper-ux.jar", "docDeveloper-rt.jar"}
)

// isDocJar: *-doc.jar, an OEM doc<Name>.jar (docHoneywell.jar) without a
// runtime-profile suffix, or a docDeveloper example-code jar.
func isDocJar(name string) bool {
	return strings.HasSuffix(name, "-doc.jar") || name == developerCodeJars[0] || name == developerCodeJars[1] ||
		(strings.HasPrefix(name, "doc") && strings.HasSuffix(name, ".jar") && !strings.Contains(name, "-"))
}

// isModuleJar: neither a doc jar nor the javadoc jar; only its doc/ pages
// and extract-rule entries are documentation.
func isModuleJar(name string) bool { return !isDocJar(name) && name != javadocJar }

// dropRunes is s[:-n] in code points (Python slicing; "" when shorter).
func dropRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return ""
	}
	return string(r[:len(r)-n])
}

// jarStem is the jar name without ".jar" and a "-doc" suffix.
func jarStem(name string) string {
	stem := dropRunes(name, len(".jar"))
	return strings.TrimSuffix(stem, "-doc")
}

// guideFolder is the guides/ folder of a jar: the stem without a leading
// "doc" (docAlarms-doc.jar -> Alarms), the stem itself otherwise.
func guideFolder(name string) string {
	stem := jarStem(name)
	if strings.HasPrefix(stem, "doc") && len(stem) > 3 {
		return stem[3:]
	}
	return stem
}

// moduleDest is _module_dest: the guides/ path of a module-jar entry in
// its top-level doc/ folder or named by an extract rule.
func moduleDest(jarName, entry string, rules *Rules) (string, bool) {
	if strings.HasSuffix(entry, "/") || strings.HasPrefix(entry, "META-INF/") {
		return "", false
	}
	if strings.HasPrefix(entry, "doc/") {
		return "guides/" + guideFolder(jarName) + "/" + entry[4:], true
	}
	if rules.ExtractRule(jarName, entry) != nil {
		return "guides/" + guideFolder(jarName) + "/" + entry, true
	}
	return "", false
}

// isHelpEntry: a module-jar entry that makes it a help jar (any entry
// moduleDest maps, stylesheets aside).
func isHelpEntry(jarName, entry string, rules *Rules) bool {
	if strings.HasSuffix(textfold.Lower(entry), ".css") {
		return false
	}
	_, ok := moduleDest(jarName, entry, rules)
	return ok
}

// destPath is the corpus-relative destination of one jar entry; ok is
// false when the entry is not extracted (dest_path returning None).
func destPath(jarName, entry string, rules *Rules) (rel string, ok bool) {
	if strings.HasSuffix(entry, "/") || strings.HasPrefix(entry, "META-INF/") {
		return "", false
	}
	switch {
	case jarName == javadocJar:
		if strings.HasPrefix(entry, "doc/") {
			return "bajadoc/" + entry[4:], true
		}
		return "", false
	case jarName == "docDeveloper-ux.jar":
		if strings.HasSuffix(entry, ".class") {
			return "", false
		}
		return "devguide-examples/" + entry, true
	case jarName == "docDeveloper-rt.jar": // its rc/ Java examples only
		if strings.HasPrefix(entry, "rc/") && !strings.HasSuffix(entry, ".class") {
			return "devguide-rc/" + entry[3:], true
		}
		return "", false
	case isModuleJar(jarName):
		return moduleDest(jarName, entry, rules)
	}
	stem := jarStem(jarName)
	if stem == "docSource" {
		return "source/" + entry, true
	}
	if stem == "docDeveloper" && strings.HasPrefix(entry, "rc/") {
		return "devguide-rc/" + entry[3:], true
	}
	if dir, ok := specialDocs[stem]; ok {
		if !strings.HasPrefix(entry, "doc/") {
			return "", false
		}
		return dir + "/" + entry[4:], true
	}
	return "guides/" + guideFolder(jarName) + "/" + strings.TrimPrefix(entry, "doc/"), true
}

// sortedNames is sorted(os.listdir(dir)) (os.ReadDir sorts by name; byte
// order is code-point order for UTF-8 names).
func sortedNames(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(ents))
	for i, e := range ents {
		names[i] = e.Name()
	}
	return names, nil
}

// helpJars lists the module jars under modules/ that carry documentation
// (help_jars): a top-level doc/ page or an extract-rule entry. Every
// module jar is listed (through ls, nil = in memory); an unreadable jar
// has no entries.
func helpJars(installDir string, ls *Listings, rules *Rules) ([]string, error) {
	modules := pycompat.Join(installDir, "modules")
	if !pycompat.IsDir(modules) {
		return nil, nil
	}
	if ls == nil {
		ls = newListings("")
	}
	names, err := sortedNames(modules)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, n := range names {
		if !strings.HasSuffix(n, ".jar") || isDocJar(n) {
			continue
		}
		path := pycompat.Join(modules, n)
		entries, _ := ls.Names(path)
		for _, e := range entries {
			if isHelpEntry(n, e, rules) {
				out = append(out, path)
				break
			}
		}
	}
	return out, nil
}

// listDocJars is list_doc_jars: the modules/ doc jars, the javadoc jar,
// then (withHelpJars) the module jars with help pages, default rules.
func listDocJars(installDir string, withHelpJars bool, ls *Listings) ([]string, error) {
	var jars []string
	modules := pycompat.Join(installDir, "modules")
	if pycompat.IsDir(modules) {
		names, err := sortedNames(modules)
		if err != nil {
			return nil, err
		}
		for _, n := range names {
			if isDocJar(n) {
				jars = append(jars, pycompat.Join(modules, n))
			}
		}
	}
	if javadoc := pycompat.Join(installDir, "javadoc", javadocJar); pycompat.IsFile(javadoc) {
		jars = append(jars, javadoc)
	}
	if withHelpJars {
		rules, err := Default()
		if err != nil {
			return nil, err
		}
		help, err := helpJars(installDir, ls, rules)
		if err != nil {
			return nil, err
		}
		jars = append(jars, help...)
	}
	return jars, nil
}

// mapped is one mapped_entries row: the entry at Pos of the jar maps to
// Rel; Collides is set when an earlier entry (First) already owns Rel, so
// this one is skipped.
type mapped struct {
	Pos        int
	Entry, Rel string
	First      string
	Collides   bool
}

// mappedEntries is mapped_entries: in jar order, every entry DestPath
// maps, third-party source archives (IsSourceArchive) left out; when two
// entries map to one path the first is extracted, each later one collides.
func mappedEntries(jarName string, names []string, rules *Rules) []mapped {
	owner := map[string]int{}
	var out []mapped
	for pos, entry := range names {
		rel, ok := destPath(jarName, entry, rules)
		if !ok || isSourceArchive(rel) {
			continue
		}
		first, seen := owner[rel]
		if !seen {
			owner[rel] = pos
			out = append(out, mapped{Pos: pos, Entry: entry, Rel: rel})
			continue
		}
		out = append(out, mapped{Pos: pos, Entry: entry, Rel: rel, First: names[first], Collides: true})
	}
	return out
}
