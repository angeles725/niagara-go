package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestListingKind: the review summary against the mini station's one NumericWritable (Point, no
// fallback stored: 0.0) and against variants: a missing, an extra and a twice-listed name, another
// priority, another value, no file, and a nested slot read without nested values.
func TestListingKind(t *testing.T) {
	dir := writeStation(t)
	write := func(text string) {
		if err := os.WriteFile(filepath.Join(dir, "shared", "Review.txt"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st, err := Open(dir, "", true)
	if err != nil {
		t.Fatal(err)
	}
	ck := Check{Kind: kindListing, Glob: "shared/Review.txt", Type: numericWritable,
		Match: `^(?P<name>\S+): (?P<value>-?[0-9][0-9.,]*) \{[^}]*\} @ (?P<priority>\S+)$`, Slot: "fallback.value", Default: "0.0", Want: "def"}
	for _, c := range []struct {
		text, verdict, evidence string
	}{
		{"Point: 0,00 {ok} @ def\r\n", Pass, "shared/Review.txt lists 1 of 1 control:NumericWritable, each at def with its fallback.value"},
		{"header\nPoint: 0.0 {ok} @ def\n", Pass, "shared/Review.txt lists 1 of 1"},
		{"Other: 1,00 {ok} @ def\n", Fail, "shared/Review.txt lists 0 of 1 control:NumericWritable, each at def with its fallback.value; missing: Point; not a control:NumericWritable: Other"},
		{"Point: 0,00 {ok} @ def\nPoint: 0,00 {ok} @ def\n", Fail, "shared/Review.txt lists 1 of 1 control:NumericWritable, each at def with its fallback.value; listed twice: Point"},
		{"Point: 0,00 {ok} @ 10\n", Fail, "shared/Review.txt lists 1 of 1 control:NumericWritable, each at def with its fallback.value; shared/Review.txt:1 Point is at 10, want def"},
		{"Point: 5,00 {ok} @ def\n", Fail, "shared/Review.txt lists 1 of 1 control:NumericWritable, each at def with its fallback.value; shared/Review.txt:1 Point = 5,00, want Point.fallback.value = 0.0 (default)"},
	} {
		write(c.text)
		if r := st.Run(&ck); r.Verdict != c.verdict || !strings.HasPrefix(r.Evidence, c.evidence) {
			t.Errorf("%q:\n got %s %q\nwant %s %q", c.text, r.Verdict, r.Evidence, c.verdict, c.evidence)
		}
	}
	names := ck
	names.Slot, names.Want = "", ""
	write("Point: 7 {ok} @ 3\n")
	if r := st.Run(&names); r.Verdict != Pass {
		t.Errorf("names only: %s %q", r.Verdict, r.Evidence)
	}
	missing := ck
	missing.Glob = "shared/None.txt"
	if r := st.Run(&missing); r.Verdict != Fail || r.Evidence != "found 0 files shared/None.txt, want one" {
		t.Errorf("no file: %s %q", r.Verdict, r.Evidence)
	}
	flat, err := Open(dir, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if r := flat.Run(&ck); r.Verdict != Skip || !NeedsNested([]*Check{&ck}) {
		t.Errorf("nested without nested values: %s %q", r.Verdict, r.Evidence)
	}
	bad := ck
	bad.Match = `^(\S+):`
	if r := st.Run(&bad); r.Verdict != Skip {
		t.Errorf("no name group: %s %q", r.Verdict, r.Evidence)
	}
}

// twoPoints is a station whose two NumericWritables share the short name Point, in two folders.
const twoPoints = `<?xml version="1.0" encoding="UTF-8"?>
<bajaObjectGraph version="4.0">
<p h="1" m="b=baja" t="b:Station">
 <p n="stationName" v="Two"/>
 <p n="A" h="2" t="b:Folder"><p n="Point" h="3" m="c=control" t="c:NumericWritable"/></p>
 <p n="B" h="4" t="b:Folder"><p n="Point" h="5" t="c:NumericWritable"/></p>
</p>
</bajaObjectGraph>
`

// A short name two components share cannot say which one a line lists: SKIP naming both paths,
// never PASS (one line would cover both) nor FAIL (one would look missing).
func TestListingAmbiguous(t *testing.T) {
	dir := writeStation(t)
	if err := os.WriteFile(filepath.Join(dir, "config.bog"), []byte(twoPoints), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"Review.txt": "Point: 0,00 {ok} @ def\n", "Other.txt": "x\n"} {
		if err := os.WriteFile(filepath.Join(dir, "shared", name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st, err := Open(dir, "", true)
	if err != nil {
		t.Fatal(err)
	}
	ck := Check{Kind: kindListing, Glob: "shared/Review.txt", Type: numericWritable, Match: `^(?P<name>\S+):`}
	if r := st.Run(&ck); r.Verdict != Skip || r.Evidence != "ambiguous: A/Point, B/Point share the name Point" {
		t.Errorf("same-named points: %s %q", r.Verdict, r.Evidence)
	}
	ck.Under = "A" // one folder: no ambiguity
	if r := st.Run(&ck); r.Verdict != Pass {
		t.Errorf("one folder: %s %q", r.Verdict, r.Evidence)
	}
	many := ck
	many.Glob = "shared/*.txt"
	if r := st.Run(&many); r.Verdict != Fail || r.Evidence != "found 2 files shared/*.txt, want one" {
		t.Errorf("two files: %s %q", r.Verdict, r.Evidence)
	}
	defer func(n int) { maxFileText = n }(maxFileText)
	maxFileText = 4
	if r := st.Run(&ck); r.Verdict != Skip || !strings.HasPrefix(r.Evidence, "cannot read shared/Review.txt: larger than 4 bytes") {
		t.Errorf("unreadable file: %s %q", r.Verdict, r.Evidence)
	}
}
