package pxrender

import (
	"strings"
	"testing"
)

// A Label keeps the colours and font its Px gives it.
func TestLabelKeepsItsColoursAndFont(t *testing.T) {
	page := renderPx(t, alarmsLike, Options{OnRatio: 0.7})
	for _, want := range []string{"color:#f8f2f2;", "background:#1a70d0;", "font-family:'Times New Roman',Arial,sans-serif;",
		"font-size:22px;", "font-weight:bold;"} {
		if !strings.Contains(page, want) {
			t.Errorf("label lacks %q", want)
		}
	}
}
