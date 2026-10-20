package helpbuild

import (
	"strings"

	"github.com/angeles725/niagara-go/internal/pypath"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// The content-type classification of doc-jar entries the coverage gate
// uses (doc_types.classify, classify_unmapped, content_type,
// example_skip_reason and REASONS):
//
//	classify(rel) -> ("index", kind) | ("exclude", reason) | ("unknown", "")
//
// Index kinds and where they become searchable:
//
//	javadoc-class  bajadoc/<pkg>/<Name>.html     -> class-index.json
//	bajadoc-xml    devguide|guides/**.bajadoc    -> class-index.json
//	jdk-xml        jdk/**.bajadoc                -> jdk-class-index.json
//	devguide-page  devguide/**.html|.txt         -> devguide-clean/
//	devguide-toc   devguide/toc.xml              -> devguide-toc.json
//	guide-page     guides/**.html|.htm|.txt|.md  -> guides-clean/
//	guide-toc      guides/<Folder>/toc.xml       -> guides-index.json titles
//	java           source|devguide-rc/**.java    -> source-index.json
//	example        devguide-examples/**          -> the full-text index

// Verdicts of Classify.
const (
	verdictIndex   = "index"
	verdictExclude = "exclude"
	verdictUnknown = "unknown"
)

var (
	exampleExt     = []string{".js", ".hbs", ".px", ".html", ".java"}
	imageExt       = []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".bmp", ".ico"}
	styleExt       = []string{".css", ".eot", ".ttf", ".woff", ".woff2", ".otf"}
	searchIndexExt = []string{".dat", ".index", ".hs"}
	guidePageExt   = []string{".html", ".htm", ".txt", ".md"}
)

var javadocNav = map[string]bool{"allclasses-frame.html": true, "allclasses-noframe.html": true,
	"constant-values.html": true, "deprecated-list.html": true, "help-doc.html": true,
	"index-all.html": true, "index.html": true, "overview-frame.html": true,
	"overview-summary.html": true, "overview-tree.html": true, "serialized-form.html": true,
	"package-summary.html": true, "package-frame.html": true, "package-tree.html": true,
	"package-use.html": true, "package-list": true}

// guideJunk is _GUIDE_JUNK: guide-folder files some OEM jars ship, by
// extension.
var guideJunk = map[string]string{
	".lexicon": "translation lexicon (UI strings, not documentation)",
	".palette": "Workbench palette (component templates, not prose)",
	".db":      "Windows thumbnail cache (Thumbs.db)",
	".ini":     "Windows folder settings (Desktop.ini)",
	".lnk":     "Windows shortcut",
	".fpj":     "image-editor project file next to screenshots",
	".pptx":    "presentation kept with the screenshots it produced (Images/)",
	".log":     "debug log shipped in the jar by mistake (not documentation)",
}

// reasons is doc_types.REASONS: the exclusion reasons by key.
var reasons = map[string]string{
	"image":              "image (no text to index)",
	"style":              "stylesheet / web font",
	"script":             "page script (viewer UI code; rendered pages are indexed)",
	"search-index":       "JavaHelp/bajadoc search index or helpset (binary; replaced by the niagara-help indexes)",
	"licence":            "licence text",
	"javadoc-nav":        "javadoc navigation/listing page (its classes are indexed one by one)",
	"bajadoc-listing":    "bajadoc module/package listing (its classes are indexed one by one)",
	"source-nav":         "source browser navigation page (every .java is indexed)",
	"meta-inf":           "jar metadata (META-INF/)",
	"class-file":         "compiled example class (its .java source is indexed)",
	"third-party-bundle": "third-party library bundled for the JavaScript playground (Babel), not Niagara documentation",
	"minified":           "minified build bundle (its readable sources are indexed)",
	"module-content":     "module code and resources outside the jar's doc/ help folder (not documentation)",
	"source-archive":     "third-party source archive (licence source offer, not documentation; not extracted)",
}

// lowerExt is doc_types._ext: os.path.splitext(name)[1].lower().
func lowerExt(name string) string {
	_, ext := pypath.PosixSplitext(name)
	return textfold.Lower(ext)
}

func isOneOf(s string, set []string) bool {
	for _, x := range set {
		if s == x {
			return true
		}
	}
	return false
}

// exampleSkipReason is example_skip_reason: why a devguide-examples/ file
// (path relative to that folder) is not indexed, "" when it is.
func exampleSkipReason(rel string) string {
	if top, _, _ := strings.Cut(rel, "/"); top == "ext" {
		return reasons["third-party-bundle"]
	}
	if strings.HasSuffix(textfold.Lower(rel), ".min.js") {
		return reasons["minified"]
	}
	return ""
}

// classify is doc_types.classify: the verdict of a corpus-relative path
// and its index kind (VerdictIndex) or exclusion reason (VerdictExclude).
func classify(rel string) (verdict, detail string) {
	parts := strings.Split(rel, "/")
	top, name := parts[0], parts[len(parts)-1]
	low, ext := textfold.Lower(name), lowerExt(name)
	if top == "devguide-examples" {
		return classifyExample(strings.Join(parts[1:], "/"), ext)
	}
	switch {
	case isOneOf(ext, imageExt):
		return verdictExclude, reasons["image"]
	case isOneOf(ext, styleExt):
		return verdictExclude, reasons["style"]
	case ext == ".js":
		return verdictExclude, reasons["script"]
	case isOneOf(ext, searchIndexExt):
		return verdictExclude, reasons["search-index"]
	case ext == ".txt" && isLicence(name):
		return verdictExclude, reasons["licence"]
	case ext == ".class":
		return verdictExclude, reasons["class-file"]
	}
	switch {
	case top == "bajadoc":
		if javadocNav[low] || isOneOf("class-use", parts) || isOneOf("index-files", parts) || len(parts) == 2 {
			return verdictExclude, reasons["javadoc-nav"]
		}
		if ext == ".html" {
			return verdictIndex, "javadoc-class"
		}
	case ext == ".bajadoc":
		if low == "module-index.bajadoc" || low == "package-index.bajadoc" {
			return verdictExclude, reasons["bajadoc-listing"]
		}
		if top == "jdk" {
			return verdictIndex, "jdk-xml"
		}
		if top == "devguide" || top == "guides" {
			return verdictIndex, "bajadoc-xml"
		}
	case top == "source" || top == "devguide-rc":
		if ext == ".java" {
			return verdictIndex, "java"
		}
		if top == "source" && len(parts) >= 2 && parts[1] == "doc" && (ext == ".html" || low == "toc.xml") {
			return verdictExclude, reasons["source-nav"]
		}
	case top == "devguide":
		if ext == ".html" || ext == ".txt" {
			return verdictIndex, "devguide-page"
		}
		if low == "toc.xml" && len(parts) == 2 {
			return verdictIndex, "devguide-toc"
		}
	case top == "guides":
		return classifyGuide(rel, parts, low, ext)
	}
	return verdictUnknown, ""
}

func classifyGuide(rel string, parts []string, low, ext string) (string, string) {
	switch {
	case isSourceArchive(rel):
		return verdictExclude, reasons["source-archive"]
	case isOneOf(ext, guidePageExt):
		return verdictIndex, "guide-page"
	case low == "toc.xml" && len(parts) == 3:
		return verdictIndex, "guide-toc"
	}
	if why, ok := guideJunk[ext]; ok {
		return verdictExclude, why
	}
	return verdictUnknown, ""
}

// classifyExample is _classify_example: a docDeveloper-ux.jar example
// source (rel is the path under devguide-examples/).
func classifyExample(rel, ext string) (string, string) {
	if skip := exampleSkipReason(rel); skip != "" {
		return verdictExclude, skip
	}
	switch {
	case isOneOf(ext, exampleExt):
		return verdictIndex, "example"
	case isOneOf(ext, styleExt):
		return verdictExclude, reasons["style"]
	}
	if why, ok := guideJunk[ext]; ok {
		return verdictExclude, why
	}
	return verdictUnknown, ""
}

// classifyUnmapped is classify_unmapped: the verdict of a jar entry the
// extractor maps to no corpus path.
func classifyUnmapped(entry string) (verdict, detail string) {
	switch {
	case strings.HasPrefix(entry, "META-INF/"):
		return verdictExclude, reasons["meta-inf"]
	case strings.HasSuffix(entry, ".class"):
		return verdictExclude, reasons["class-file"]
	}
	if why, ok := guideJunk[lowerExt(entry)]; ok {
		return verdictExclude, why
	}
	return verdictUnknown, ""
}

// contentType is content_type: the report label, the corpus top folder and
// the lower-cased extension (or the name when it has none): "guides .html".
func contentType(rel string) string {
	parts := strings.Split(rel, "/")
	name := parts[len(parts)-1]
	ext := lowerExt(name)
	if ext == "" {
		ext = name
	}
	return parts[0] + " " + ext
}
