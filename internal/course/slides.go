package course

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/course/key"
)

// textDir holds every course text file: the slide decks, the BQL reference, the lab and review
// instructions (relative to the root, with "/").
const textDir = "sources/n4-dev-course/text"

// deck is one course text file: name is its basename without ".txt", file its root-relative path.
type deck struct{ name, file string }

// deckNumber is the number a deck name starts with ("8 ORDs" is 8), or -1.
func deckNumber(name string) int {
	head, _, _ := strings.Cut(name, " ")
	n, err := strconv.Atoi(head)
	if err != nil {
		return -1
	}
	return n
}

// listDecks returns the .txt files of textDir in course order: the numbered decks by number,
// then the others by name (case folded); or why it cannot.
func listDecks(root string) ([]deck, string) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(textDir)))
	if os.IsNotExist(err) {
		return nil, "missing text directory " + textDir
	}
	if err != nil {
		return nil, fmt.Sprintf("cannot read %s: %v", textDir, err)
	}
	var decks []deck
	for _, e := range entries {
		if n := e.Name(); !e.IsDir() && strings.HasSuffix(strings.ToLower(n), ".txt") {
			decks = append(decks, deck{name: n[:len(n)-len(".txt")], file: textDir + "/" + n})
		}
	}
	if len(decks) == 0 {
		return nil, "no .txt file in " + textDir
	}
	sort.SliceStable(decks, func(i, j int) bool {
		a, b := deckNumber(decks[i].name), deckNumber(decks[j].name)
		if (a < 0) != (b < 0) {
			return a >= 0
		}
		if a != b {
			return a < b
		}
		return strings.ToLower(decks[i].name) < strings.ToLower(decks[j].name)
	})
	return decks, ""
}

// scanDecks calls hit for every line of every deck that re matches; it returns the lines and
// files searched, or why a file cannot be read.
func scanDecks(root string, re *regexp.Regexp, hit func(d deck, n int, line string)) (lines, files int, msg string) {
	decks, msg := listDecks(root)
	if msg != "" {
		return 0, 0, msg
	}
	for _, d := range decks {
		text, msg := readText(root, d.file)
		if msg != "" {
			return 0, 0, msg
		}
		for i, l := range text {
			if re.MatchString(l) {
				hit(d, i+1, l)
			}
		}
		lines += len(text)
	}
	return lines, len(decks), ""
}

// cmdFindSlides prints <deck>:<line>: <text> for every course text line re matches. Exit: 0 at
// least one; 1 none (what was searched on stderr); 2 the text directory or a file cannot be read.
func cmdFindSlides(s cli.Stdio, root, q string, re *regexp.Regexp) int {
	found := 0
	lines, files, msg := scanDecks(root, re, func(d deck, n int, line string) {
		found++
		fmt.Fprintf(s.Out, "%s:%d: %s\n", d.name, n, truncate(strings.TrimSpace(line), 120))
	})
	if msg != "" {
		fmt.Fprintf(s.Err, "course: %s\n", msg)
		return cli.ExitCannotSearch
	}
	if found == 0 {
		fmt.Fprintf(s.Err, "course: no text line matches %s (searched %s in %s in %s)\n", q,
			key.Plural(lines, "line"), key.Plural(files, "text file"), textDir)
		return cli.ExitNoMatch
	}
	return cli.ExitFound
}
