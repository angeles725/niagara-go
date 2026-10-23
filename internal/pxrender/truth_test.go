package pxrender

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Truth tests: the page never claims a full reproduction of a Px it only partly rendered, and a
// Label keeps the colours and font the Px gives it.

func renderPx(t *testing.T, px string, o Options) string {
	t.Helper()
	dir := t.TempDir()
	o.Px = filepath.Join(dir, "screen.px")
	if err := os.WriteFile(o.Px, []byte(px), 0o644); err != nil {
		t.Fatal(err)
	}
	o.Shared = dir
	page, _, err := render(o)
	if err != nil {
		t.Fatal(err)
	}
	return page
}

const alarmsLike = `<?xml version="1.0" encoding="UTF-8"?>
<px version="1.0"><content><ScrollPane><CanvasPane name="content" viewSize="1200.0,800.0">
<Label layout="340.0,40.0,220.0,70.0" text="Alarmas" font="bold 22.0pt Times New Roman" foreground="#f8f2f2" background="#1a70d0"/>
<HistoryChart layout="10.0,140.0,360.0,220.0"><WbViewBinding ord="station:|slot:/A"/></HistoryChart>
<HistoryChart layout="380.0,360.0,370.0,220.0"><WbViewBinding ord="station:|slot:/B"/></HistoryChart>
</CanvasPane></ScrollPane></content></px>`

func TestPageListsEveryWidgetItDidNotRender(t *testing.T) {
	page := renderPx(t, alarmsLike, Options{OnRatio: 0.7})
	for _, want := range []string{
		`class="omitted"`,
		"No reproducido (2)",
		"HistoryChart en x=10, y=140 (360×220)",
		"HistoryChart en x=380, y=360 (370×220)",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q", want)
		}
	}
	if strings.Contains(page, "en las mismas posiciones") {
		t.Error("page claims a full reproduction of a Px with omitted widgets")
	}
}

func TestPageWithNothingOmittedHasNoNotice(t *testing.T) {
	px := strings.Replace(alarmsLike, `<HistoryChart layout="10.0,140.0,360.0,220.0"><WbViewBinding ord="station:|slot:/A"/></HistoryChart>`, "", 1)
	px = strings.Replace(px, `<HistoryChart layout="380.0,360.0,370.0,220.0"><WbViewBinding ord="station:|slot:/B"/></HistoryChart>`, "", 1)
	page := renderPx(t, px, Options{OnRatio: 0.7})
	if strings.Contains(page, `class="omitted"`) || !strings.Contains(page, "en las mismas posiciones") {
		t.Error("a fully rendered Px must keep the full-reproduction note and no omitted notice")
	}
}
