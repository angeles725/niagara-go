package fulltext

// What one indexed file contributes (fulltext_index.py _split_header,
// doc_title, index_title, _documents, iter_documents): one document per file,
// or one per non-blank page for the paged sources.

import (
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// paged are fulltext_index.PAGED_SOURCES: indexed page by page, pages
// separated by form feeds (PDF text and the developer course text).
var paged = map[string]bool{"docs-text": true, "course-text": true}

// document is one row of docs plus the text its fts row tokenizes.
type document struct {
	source, path, title string
	titleText, body     string
	line, page          int
}

// headerLine is fulltext_index._HEADER (`^([A-Z][A-Z ]*):\s?(.*)$`) on a
// line split at "\n": key is the upper-case run before the colon, value
// everything after it (the caller strips it, which also drops `\s?`).
func headerLine(line string) (key, value string, ok bool) {
	if line == "" || !isUpper(line[0]) {
		return "", "", false
	}
	j := 1
	for j < len(line) && (isUpper(line[j]) || line[j] == ' ') {
		j++
	}
	if j == len(line) || line[j] != ':' {
		return "", "", false
	}
	return line[:j], line[j+1:], true
}

func isUpper(b byte) bool { return 'A' <= b && b <= 'Z' }

// splitHeader is fulltext_index._split_header: the "KEY: value" lines that
// open a clean-text file (first value of a repeated key wins) and the index
// of the first body line, past one blank separator line; (nil, 0) without
// a header.
func splitHeader(lines []string) (map[string]string, int) {
	header := map[string]string{}
	i := 0
	for ; i < len(lines); i++ {
		key, value, ok := headerLine(lines[i])
		if !ok {
			break
		}
		if _, seen := header[key]; !seen {
			header[key] = pycompat.Strip(value)
		}
	}
	if len(header) == 0 {
		return nil, 0
	}
	if i < len(lines) && pycompat.Strip(lines[i]) == "" {
		i++
	}
	return header, i
}

// stemName is os.path.splitext(os.path.basename(rel))[0].
func stemName(rel string) string {
	stem, _ := pycompat.PosixSplitext(pycompat.PosixBasename(rel))
	return stem
}

// docTitle is fulltext_index.doc_title: the human title of one file.
func docTitle(source, rel string, header map[string]string) string {
	if t := header["TITLE"]; t != "" {
		return t
	}
	if class := header["CLASS"]; class != "" {
		if pkg := header["PACKAGE"]; pkg != "" {
			return class + " (" + pkg + ")"
		}
		return class
	}
	switch source {
	case "docs-text":
		return strings.ReplaceAll(stemName(rel), "_", " ")
	case "course-text":
		root, _ := pycompat.PosixSplitext(rel)
		return "Course: " + root
	}
	return stemName(rel)
}

// indexTitle is fulltext_index.index_title: the text of the boosted title
// column (the class name, the file stem for source code, the title for
// paged sources, else the title plus the file stem).
func indexTitle(source, rel string, header map[string]string, title string) string {
	if class := header["CLASS"]; class != "" {
		return class
	}
	switch {
	case source == "source" || source == "devguide-rc":
		return stemName(rel)
	case paged[source]:
		return title
	}
	return title + " " + stemName(rel)
}

// documents is fulltext_index._documents over a decoded file text.
func documents(source, rel, text string) []document {
	lines := strings.Split(text, "\n")
	header, bodyStart := splitHeader(lines)
	title := docTitle(source, rel, header)
	titleText := indexTitle(source, rel, header, title)
	if paged[source] && strings.Contains(text, "\f") {
		var out []document
		line := 1
		for i, page := range strings.Split(text, "\f") {
			if pycompat.Strip(page) != "" {
				out = append(out, document{source, rel, title, titleText, page, line, i + 1})
			}
			line += strings.Count(page, "\n")
		}
		return out
	}
	body := strings.Join(lines[bodyStart:], "\n")
	return []document{{source, rel, title, titleText, body, bodyStart + 1, 0}}
}
