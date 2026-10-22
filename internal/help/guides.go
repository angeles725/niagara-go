package help

// Table-of-contents lookups (guide_search.py cmd_guide, cmd_devguide): guide
// folders by name, jar and page, and Developer Guide entries by title,
// description and file name. The ranked full-text forms come with S6.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// objList is o.get(k, []) keeping its object items (Python would raise on
// any other item; such items are skipped here).
func objList(o *index.Object, k string) []*index.Object {
	arr, _ := get(o, k, nil).([]any)
	var out []*index.Object
	for _, v := range arr {
		if x, ok := v.(*index.Object); ok {
			out = append(out, x)
		}
	}
	return out
}

// topicCount is data.get("total_topics", data.get("html_count", def)).
func topicCount(o *index.Object, def any) string {
	return pyStr(get(o, "total_topics", get(o, "html_count", def)))
}

type guideHit struct {
	score   int
	folder  string
	data    *index.Object
	matched []string
}

// Guide is cmd_guide: folders whose name (10), jar (5) or pages and their
// toc.xml titles (1 each) contain the query, best first.
func (e *Env) Guide(query string) error {
	// guide_search reads the file directly: a missing index is an error.
	idx, err := e.loadIndex("guides-index.json", true)
	if err != nil {
		return err
	}
	q := textfold.Lower(query)
	var hits []guideHit
	for _, folder := range idx.Keys() {
		data := idx.Object(folder)
		h := guideHit{folder: folder, data: data}
		if strings.Contains(textfold.Lower(folder), q) {
			h.score += 10
		}
		if strings.Contains(textfold.Lower(truthyStr(data, "source_jar")), q) {
			h.score += 5
		}
		titles := getObj(data, "titles")
		for _, topic := range strList(data, "topics") {
			title := getStr(titles, topic, "")
			if strings.Contains(textfold.Lower(topic), q) || strings.Contains(textfold.Lower(title), q) {
				if title != "" {
					topic = title + " (" + topic + ")"
				}
				h.matched = append(h.matched, topic)
				h.score++
			}
		}
		if h.score > 0 {
			hits = append(hits, h)
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if len(hits) == 0 {
		return e.reportMiss("guide", query, "guides")
	}
	fmt.Fprintf(e.Out, "%d guide folders matching '%s':\n\n", len(hits), fts.Echo(query))
	for _, h := range hits[:min(len(hits), 15)] {
		fmt.Fprintf(e.Out, "  %s (%s topics, %s)\n", h.folder, topicCount(h.data, 0),
			orStr(h.data, "source_jar", "", "?"))
		if len(h.matched) > 0 {
			fmt.Fprintf(e.Out, "    matching topics: %s\n", strings.Join(h.matched[:min(len(h.matched), 5)], ", "))
			if len(h.matched) > 5 {
				fmt.Fprintf(e.Out, "    ... and %d more\n", len(h.matched)-5)
			}
		}
		fmt.Fprintf(e.Out, "    path: guides/%s/\n\n", h.folder)
	}
	return nil
}

type devguideHit struct {
	score   int
	section string
	item    *index.Object
}

// Devguide is cmd_devguide: TOC entries scored by the query in the title
// (10), description (5) and file name (3), plus each word of three or more
// characters in the title (2) and description (1), best first.
func (e *Env) Devguide(query string) error {
	toc, err := e.loadIndex("devguide-toc.json", true)
	if err != nil {
		return err
	}
	q := textfold.Lower(query)
	var hits []devguideHit
	for _, section := range objList(toc, "sections") {
		secTitle := getStr(section, "title", "")
		for _, item := range objList(section, "items") {
			title := textfold.Lower(getStr(item, "title", ""))
			desc := textfold.Lower(getStr(item, "description", ""))
			file := textfold.Lower(getStr(item, "file", ""))
			score := 0
			if strings.Contains(title, q) {
				score += 10
			}
			if strings.Contains(desc, q) {
				score += 5
			}
			if strings.Contains(file, q) {
				score += 3
			}
			for _, w := range textfold.Fields(q) {
				if runeLen(w) < 3 {
					continue
				}
				if strings.Contains(title, w) {
					score += 2
				}
				if strings.Contains(desc, w) {
					score++
				}
			}
			if score > 0 {
				hits = append(hits, devguideHit{score, secTitle, item})
			}
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if len(hits) == 0 {
		return e.reportMiss("devguide", query, "devguide")
	}
	fmt.Fprintf(e.Out, "%d devguide entries matching '%s':\n\n", len(hits), fts.Echo(query))
	for _, h := range hits[:min(len(hits), 10)] {
		fmt.Fprintf(e.Out, "  [%s] %s\n", h.section, getStr(h.item, "title", "?"))
		desc := pycompat.Strip(strings.ReplaceAll(getStr(h.item, "description", ""), "\n", " "))
		if desc != "" {
			fmt.Fprintf(e.Out, "    %s\n", pycompat.Cut(desc, 120))
		}
		fmt.Fprintf(e.Out, "    file: devguide/%s\n\n", getStr(h.item, "file", "?"))
	}
	return nil
}
