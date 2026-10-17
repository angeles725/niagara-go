package fts

import (
	"fmt"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts/ftsfixture"
)

func TestFormatHitWithoutSnippet(t *testing.T) {
	h := &Hit{Source: "guides-clean", Path: "Gone/page.txt", Title: "Gone page"}
	got := FormatHit(nil, h, ParseQuery("page", false), "Gone")
	want := "  [Gone] Gone page\n    path: guides-clean/Gone/page.txt\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestFormatHitPrintsPythonSnippet renders the first hit of every fixture
// search and checks its snippet line against the one Python recorded.
func TestFormatHitPrintsPythonSnippet(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	checkout := ftsfixture.Checkout(t, testdata, false)
	for _, c := range loadSearchFixture(t).Searches {
		if len(c.Hits) == 0 {
			continue
		}
		base := dataRoot(checkout, c.Version)
		r, err := Search(base, c.Version, c.Query, c.Sources, c.Mode)
		if err != nil {
			t.Fatal(err)
		}
		h := r.Hits[0]
		want := fmt.Sprintf("  [tag] %s\n", h.Title)
		if snip := c.Hits[0][6].([]any); snip[0] != nil {
			want += fmt.Sprintf("    L%d: %s\n", int(snip[0].(float64)), snip[1])
		}
		want += "    path: " + c.Hits[0][0].(string) + "\n"
		if got := FormatHit(SourceDirs(base), h, r.Groups, "tag"); got != want {
			t.Errorf("%s %q:\n got %q\nwant %q", c.Version, c.Query, got, want)
		}
	}
}
