package help

// The approximate search the recall oracle samples (fuzzy_search.cmd_fuzzy).
// The `fuzzy` command itself is retired in the Go port; selftest --recall
// still checks that a class name finds its class through it, so its scoring
// and output are kept here byte for byte.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// runeIndex is str.find: the code-point index of sub in s, or -1.
func runeIndex(s, sub string) int {
	i := strings.Index(s, sub)
	if i < 0 {
		return -1
	}
	return utf8.RuneCountInString(s[:i])
}

// fuzzyScore is _score_match: per query word 1 for a substring, its typo
// similarity when the whole name is the word with a typo, or 0.5 when half
// of the word is a substring; positions sums where the words matched.
func fuzzyScore(name string, words []string) (score float64, positions int) {
	for _, w := range words {
		if pos := runeIndex(name, w); pos >= 0 {
			score++
			positions += pos
			continue
		}
		if sim := typoSimilarity(w, name); sim >= 0.8 {
			score += sim
			continue
		}
		r := []rune(w)
		half := len(r) / 2
		if half < 2 {
			continue
		}
		for start := 0; start <= len(r)-half; start++ {
			sub := string(r[start : start+half])
			if strings.Contains(name, sub) {
				score += 0.5
				positions += runeIndex(name, sub)
				break
			}
		}
	}
	return score, positions
}

// fuzzyHit is one scored result: score, positions, name, then the fields
// the section prints.
type fuzzyHit struct {
	score     float64
	positions int
	name      string
	line      string // the text after "  <score>  "
}

// fuzzyRanked is _ranked: best score first; ties: earlier match, then the
// shorter name, then the name.
func fuzzyRanked(hits []fuzzyHit) {
	sort.SliceStable(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.score != b.score {
			return a.score > b.score
		}
		if a.positions != b.positions {
			return a.positions < b.positions
		}
		if la, lb := runeLen(a.name), runeLen(b.name); la != lb {
			return la < lb
		}
		return a.name < b.name
	})
}

// fuzzy is cmd_fuzzy: class, method, slot and guide names scored against
// the query words, each section showing its top topN.
func (e *Env) fuzzy(query string, topN int) error {
	var words []string
	for _, w := range textfold.Fields(query) {
		if runeLen(w) >= 2 {
			words = append(words, textfold.Lower(w))
		}
	}
	if len(words) == 0 {
		return usageError(fmt.Sprintf("fuzzy needs a word of 2+ characters (got '%s')", query))
	}
	names := []string{"class-index.json", "method-index.json", "slots-index.json", "guides-index.json"}
	idx := make([]*index.Object, len(names))
	for i, n := range names {
		o, err := e.loadIndex(n, false)
		if err != nil {
			return err
		}
		idx[i] = o
	}
	classes, methods, slots, guides := idx[0], idx[1], idx[2], idx[3]

	var classHits []fuzzyHit
	for _, cls := range classes.Keys() {
		data := classes.Object(cls)
		pkg := getStr(data, "package", "")
		nameLow := textfold.Lower(cls)
		score, pos := fuzzyScore(nameLow+" "+textfold.Lower(pkg), words)
		if score > 0 {
			if nameScore, _ := fuzzyScore(nameLow, words); nameScore > 0 {
				score += nameScore * 0.5
			}
			classHits = append(classHits, fuzzyHit{score, pos, cls,
				fmt.Sprintf("%s (%s, %s)", cls, pkg, getStr(data, "type", ""))})
		}
	}
	fuzzyRanked(classHits)

	var methodHits []fuzzyHit
	for _, m := range methods.Keys() {
		score, pos := fuzzyScore(textfold.Lower(m), words)
		if score > 0 {
			entries := memberEntries(methods, m)
			sample := "?"
			if len(entries) > 0 {
				sample = getStr(entries[0], "class", "?")
			}
			methodHits = append(methodHits, fuzzyHit{score, pos, m,
				fmt.Sprintf("%s (%d classes, e.g. %s)", m, len(entries), sample)})
		}
	}
	fuzzyRanked(methodHits)

	var slotHits []fuzzyHit
	for _, cls := range slots.Keys() {
		data := slots.Object(cls)
		for _, section := range []string{"properties", "actions", "topics"} {
			for _, slot := range objList(data, section) {
				name := getStr(slot, "name", "")
				score, pos := fuzzyScore(textfold.Lower(name), words)
				if score > 0 {
					stype := get(slot, "type", get(slot, "parameterType", get(slot, "eventType", "")))
					slotHits = append(slotHits, fuzzyHit{score, pos, name,
						fmt.Sprintf("[%s] %s.%s: %s", section[:len(section)-1], cls, name, pyStr(stype))})
				}
			}
		}
	}
	fuzzyRanked(slotHits)

	var guideHits []fuzzyHit
	for _, folder := range guides.Keys() {
		data := guides.Object(folder)
		if score, pos := fuzzyScore(textfold.Lower(folder), words); score > 0 {
			count := get(data, "total_topics", get(data, "html_count", nil)) // nil: the falsy default 0
			extra := ""
			if index.Truthy(count) {
				extra = fmt.Sprintf(" (%s topics, %s)", pyStr(count), orStr(data, "source_jar", "", ""))
			}
			guideHits = append(guideHits, fuzzyHit{score, pos, folder, folder + extra})
		}
		for _, topic := range strList(data, "topics") {
			if score, pos := fuzzyScore(textfold.Lower(topic), words); score > 0 {
				name := folder + "/" + topic
				guideHits = append(guideHits, fuzzyHit{score, pos, name, name})
			}
		}
	}
	fuzzyRanked(guideHits)

	total := len(classHits) + len(methodHits) + len(slotHits) + len(guideHits)
	if total == 0 {
		return e.reportMiss("fuzzy", query, "classes")
	}
	fmt.Fprintf(e.Out, "FUZZY SEARCH: '%s' (%d total matches)\n%s\n", query, total, strings.Repeat("=", 60))
	for _, sec := range []struct {
		title string
		hits  []fuzzyHit
	}{{"CLASSES", classHits}, {"METHODS", methodHits}, {"SLOTS", slotHits}, {"GUIDES", guideHits}} {
		if len(sec.hits) == 0 {
			continue
		}
		fmt.Fprintf(e.Out, "\n%s (%d matches, showing top %d):\n\n", sec.title, len(sec.hits), min(topN, len(sec.hits)))
		for _, h := range pyHead(sec.hits, topN) {
			fmt.Fprintf(e.Out, "  %s  %s\n", strconv.FormatFloat(h.score, 'f', 1, 64), h.line)
		}
		if len(sec.hits) > topN {
			fmt.Fprintf(e.Out, "  ... +%d more\n", len(sec.hits)-topN)
		}
	}
	return nil
}
