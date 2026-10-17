package helpbuild

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// The doc_types predicates the builders share (niagara_help_lib/doc_types.py
// is_source_archive, is_guide_page, is_licence, clean_text_name). The rest
// of doc_types (classify and its tables) arrives with the slices that use
// it. Lower-casing is Python's str.lower (textfold.Lower), so "İ" becomes
// "i̇" and never reads as an ASCII "i".

var (
	licencePrefixes   = []string{"license", "licence", "copying", "notice"}
	licenceMarkers    = []string{"-license", "lgpl", "gpl-"}
	sourceArchiveExts = []string{".tar.xz", ".tar.gz", ".tgz", ".tar.bz2"}
	guidePageExts     = []string{".html", ".htm", ".txt", ".md"}
)

func hasAnySuffix(s string, suffixes []string) bool {
	for _, x := range suffixes {
		if strings.HasSuffix(s, x) {
			return true
		}
	}
	return false
}

// pyBasename is os.path.basename: posixpath splits on "/"; ntpath also on
// "\\" and after a drive colon.
func pyBasename(p string) string {
	if runtime.GOOS != "windows" {
		return pycompat.PosixBasename(p)
	}
	if len(p) >= 2 && p[1] == ':' {
		p = p[2:]
	}
	return p[strings.LastIndexAny(p, `/\`)+1:]
}

// IsSourceArchive: a third-party source archive under guides/ (an LGPL
// source offer such as ffmpeg-wb.jar doc/ffmpeg-6.0.tar.xz).
func IsSourceArchive(rel string) bool {
	return strings.HasPrefix(rel, "guides/") && hasAnySuffix(textfold.Lower(rel), sourceArchiveExts)
}

// IsLicence: a licence text by file name.
func IsLicence(name string) bool {
	low := textfold.Lower(pyBasename(name))
	for _, p := range licencePrefixes {
		if strings.HasPrefix(low, p) {
			return true
		}
	}
	for _, m := range licenceMarkers {
		if strings.Contains(low, m) {
			return true
		}
	}
	return false
}

// IsGuidePage: an .html / .htm page or a .txt / .md plain-text page, a
// licence .txt excluded.
func IsGuidePage(name string) bool {
	low := textfold.Lower(name)
	return hasAnySuffix(low, guidePageExts) && !(strings.HasSuffix(low, ".txt") && IsLicence(name))
}

// CleanTextName is the guides-clean / devguide-clean path of a page path
// relative to srcDir: X.html -> X.txt; X.htm and X.md keep their extension
// (X.htm.txt, X.md.txt); X.txt keeps its name unless srcDir holds an X.html
// sibling in any letter case, which owns X.txt: then it is X.txt.txt. An
// empty srcDir (Python's None) skips the sibling check.
func CleanTextName(rel, srcDir string) string {
	low := textfold.Lower(rel)
	switch {
	case strings.HasSuffix(low, ".html"):
		return rel[:len(rel)-len(".html")] + ".txt"
	case strings.HasSuffix(low, ".txt"):
		if srcDir != "" && hasHTMLSibling(srcDir, rel[:len(rel)-len(".txt")]) {
			return rel + ".txt"
		}
		return rel
	}
	return rel + ".txt"
}

// hasHTMLSibling: srcDir holds <stem>.html in any letter case. The join and
// split are posixpath's on POSIX (no normalisation), filepath's on Windows.
func hasHTMLSibling(srcDir, stem string) bool {
	var folder, base string
	if runtime.GOOS == "windows" {
		folder, base = filepath.Split(filepath.Join(srcDir, stem))
	} else {
		p := pycompat.PosixJoin(srcDir, stem)
		i := strings.LastIndex(p, "/") + 1
		folder, base = p[:i], p[i:]
	}
	if folder == "" {
		return false // os.listdir("") raises FileNotFoundError
	}
	ents, err := os.ReadDir(folder)
	if err != nil {
		return false
	}
	want := textfold.Lower(base + ".html")
	for _, e := range ents {
		if textfold.Lower(e.Name()) == want {
			return true
		}
	}
	return false
}
