package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The trend CSV an Export writes: a BOM, the history id, a blank line, the header, then rows with
// the locale's decimal comma (TrainingStation/shared, 2026-10-15), here with CRLF line ends.
const trendCSV = "\ufeffhistory:Mini/NumericWritable1\r\n\r\nTimestamp,Trend Flags,Status,Value\r\n" +
	"15 oct 2026 12:43:20 CST,{start},{ok},\"82,9\"\r\n15 oct 2026 12:43:21 CST,{ },{ok},\"75,7\"\r\n"

func TestFileTextKind(t *testing.T) {
	dir := writeStation(t)
	files := map[string]string{"Mini_NumericWritable1.csv": trendCSV, "Mini_NumericWritable2.csv": trendCSV,
		"Mini_Old.csv": "Timestamp,Value,Status\n15 oct 2026 12:43:20 CST,82.9,{ok}\n", "Review.txt": "a: 1 {ok} @ def\r\nb: 2 {ok} @ def\r\n"}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, "shared", name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st, err := Open(dir, "", false)
	if err != nil {
		t.Fatal(err)
	}
	row := `^\d{1,2} \S+ \d{4} \d{2}:\d{2}:\d{2} \S+,\{[^}]*\},\{[^}]*\},"?-?\d+(?:[.,]\d+)?"?$`
	cases := []struct {
		ck       Check
		verdict  string
		evidence string
	}{
		{Check{Kind: kindFileText, Glob: "shared/{station}_NumericWritable*.csv", Line: 1, Match: `^history:{station}/NumericWritable\d*$`},
			Pass, "2 of 2 files: shared/Mini_NumericWritable1.csv:1 = history:Mini/NumericWritable1"},
		{Check{Kind: kindFileText, Glob: "shared/{station}_NumericWritable1.csv", Line: 3, Match: `^Timestamp,Trend Flags,Status,Value$`},
			Pass, "shared/Mini_NumericWritable1.csv:3 = Timestamp,Trend Flags,Status,Value"},
		{Check{Kind: kindFileText, Glob: "shared/{station}_NumericWritable*.csv", Match: row, Op: opMin, N: 2},
			Pass, "2 of 2 files: shared/Mini_NumericWritable1.csv: 2 lines match"},
		{Check{Kind: kindFileText, Glob: "shared/{station}_*.csv", Line: 1, Match: `^history:`},
			Fail, "2 of 3 files pass, want line 1 =~ ^history:: shared/Mini_Old.csv:1 = Timestamp,Value,Status"},
		{Check{Kind: kindFileText, Glob: "shared/Review.txt", Line: 4, Match: `.`},
			Fail, "0 of 1 files pass, want line 4 =~ .: shared/Review.txt has 2 lines"},
		{Check{Kind: kindFileText, Glob: "shared/Review.txt", Match: `^\S+: \S+ \{[^}]*\} @ \S+$`, Op: opMin, N: 3},
			Fail, "0 of 1 files pass, want at least 3 lines =~ ^\\S+: \\S+ \\{[^}]*\\} @ \\S+$: shared/Review.txt: 2 lines match"},
		{Check{Kind: kindFileText, Glob: "shared/nothing*.txt", Line: 1, Match: `.`},
			Fail, "no file shared/nothing*.txt"},
		{Check{Kind: kindFileText, Glob: "shared/Review.txt", Line: 1, Match: `(`}, Skip, "bad pattern ("},
	}
	for _, c := range cases {
		r := st.Run(&c.ck)
		if r.Verdict != c.verdict || !strings.HasPrefix(r.Evidence, c.evidence) {
			t.Errorf("%s line %d %s:\n got %s %q\nwant %s %q", c.ck.Glob, c.ck.Line, c.ck.Match, r.Verdict, r.Evidence, c.verdict, c.evidence)
		}
	}
}
