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
// least one; 1 none (what was searched on stderr); 2 the text directory or a file cannot be read,
// with nothing on stdout (the hits are printed only once every deck was read).
func cmdFindSlides(s cli.Stdio, root, q string, re *regexp.Regexp) int {
	var hits []string
	lines, files, msg := scanDecks(root, re, func(d deck, n int, line string) {
		hits = append(hits, fmt.Sprintf("%s:%d: %s\n", d.name, n, truncate(strings.TrimSpace(line), 120)))
	})
	if msg != "" {
		fmt.Fprintf(s.Err, "course: %s\n", msg)
		return cli.ExitCannotSearch
	}
	for _, h := range hits {
		fmt.Fprint(s.Out, h)
	}
	if len(hits) == 0 {
		fmt.Fprintf(s.Err, "course: no text line matches %s (searched %s in %s in %s)\n", q,
			key.Plural(lines, "line"), key.Plural(files, "text file"), textDir)
		return cli.ExitNoMatch
	}
	return cli.ExitFound
}

// deckRefRe is a deck line reference: "8 ORDs:343", "8 ORDs.txt:110-163" or the text path a key
// cites; an item ID or a group never holds a colon.
var deckRefRe = regexp.MustCompile(`^(.+):(\d+)(?:-(\d+))?$`)

// deckAliases are the abbreviations the answer keys cite a text file by (`LI:7`).
var deckAliases = map[string]string{"li": "lab instructions"}

// resolveDeck finds the deck a name stands for: its basename, without any directory or ".txt",
// case folded; an alias; else the one deck the name is a prefix of, a prefix that ends a word
// winning over one that cuts a word ("1" is "1 Niagara 4 Overview", not "10 Drivers"). It
// returns the deck, or the candidates when the name is ambiguous (none: unknown).
func resolveDeck(decks []deck, name string) (*deck, []deck) {
	q := strings.ToLower(name[strings.LastIndex(name, "/")+1:])
	q = strings.TrimSuffix(q, ".txt")
	if q == "" { // a directory or a bare ".txt": no name, so no deck (an empty prefix fits them all)
		return nil, nil
	}
	if a, ok := deckAliases[q]; ok {
		q = a
	}
	var prefix, word []deck
	for i, d := range decks {
		n := strings.ToLower(d.name)
		if n == q {
			return &decks[i], nil
		}
		if strings.HasPrefix(n, q) {
			prefix = append(prefix, d)
			if rest := n[len(q):]; rest[0] == ' ' {
				word = append(word, d)
			}
		}
	}
	if len(word) > 0 {
		prefix = word
	}
	if len(prefix) == 1 {
		return &prefix[0], nil
	}
	return nil, prefix
}

// cmdDeckText prints the numbered lines a..b of the deck name stands for. Exit: 0 printed; 1 no
// such deck (the decks searched on stderr); 2 an ambiguous name, a range outside the file, or the
// text directory or the file cannot be read.
func cmdDeckText(s cli.Stdio, root, name string, r key.Range) int {
	decks, msg := listDecks(root)
	if msg != "" {
		fmt.Fprintf(s.Err, "course: %s\n", msg)
		return cli.ExitCannotSearch
	}
	d, candidates := resolveDeck(decks, name)
	if d == nil && len(candidates) > 0 {
		names := make([]string, len(candidates))
		for i, c := range candidates {
			names[i] = c.name
		}
		fmt.Fprintf(s.Err, "course: text file %s is ambiguous: %s\n", name, strings.Join(names, ", "))
		return cli.ExitCannotSearch
	}
	if d == nil {
		names := make([]string, len(decks))
		for i, c := range decks {
			names[i] = c.name
		}
		fmt.Fprintf(s.Err, "course: no text file %s (searched %s in %s: %s)\n", name,
			key.Plural(len(decks), "text file"), textDir, strings.Join(names, ", "))
		return cli.ExitNoMatch
	}
	return printText(s, root, d.file, d.name, []key.Range{r})
}
