package course

import (
	"os"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/course/key"
)

// The numbered decks sort by number (10 after 8), the others by name with case folded (Bonus before BQL).
func TestListDecksOrder(t *testing.T) {
	decks, msg := listDecks("testdata/root")
	if msg != "" {
		t.Fatal(msg)
	}
	names := make([]string, len(decks))
	for i, d := range decks {
		names[i] = d.name
	}
	want := "1 Niagara 4 Overview|8 ORDs|10 Drivers|Bonus Slides|BQL Reference|Lab Instructions|Review Assignment 1"
	if got := strings.Join(names, "|"); got != want {
		t.Errorf("decks = %s; want %s", got, want)
	}
	if decks[1].file != "sources/n4-dev-course/text/8 ORDs.txt" {
		t.Errorf("file = %q", decks[1].file)
	}
}

// On the real checkout, when NIAGARA_RESEARCH_DIR points at it: every course text file is a deck
// and reads, so find --slides searches all of them (17 files, decks 0-10 among them).
func TestDecksReal(t *testing.T) {
	root := os.Getenv(key.RootEnv)
	if root == "" || !key.HasMarker(root) {
		t.Skipf("%s is not a niagara-research checkout: the real decks are not checked", key.RootEnv)
	}
	decks, msg := listDecks(root)
	if msg != "" {
		t.Fatal(msg)
	}
	if len(decks) < 17 || decks[0].name != "0 Course Logistics" || decks[10].name != "10 Drivers" {
		t.Errorf("decks = %v; want 17 or more, 0 Course Logistics first, 10 Drivers eleventh", decks)
	}
	total := 0
	for _, d := range decks {
		lines, msg := readText(root, d.file)
		if msg != "" || len(lines) == 0 {
			t.Errorf("%s: %d lines %s", d.name, len(lines), msg)
		}
		total += len(lines)
	}
	t.Logf("%d decks, %d lines", len(decks), total)
}
