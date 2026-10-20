package course

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/course/key"
)

// The requirement paragraph of an exam problem ends at a blank line (spaces allowed) or at the next
// bold-lead line, so a design written right under it never joins the prompt.
func TestExamQuestionParagraphEnd(t *testing.T) {
	for _, c := range []struct{ name, body, prompt, answer string }{
		{"blank line", "**Requirement.** Clamp it.\nto a band.\n\n**Design.** A.", "P\nClamp it.\nto a band.", "**Design.** A."},
		{"blank line of spaces", "**Requirement.** Clamp it.\n  \t\n**Design.** A.", "P\nClamp it.", "**Design.** A."},
		{"next bold lead", "**Requirement.** Clamp it.\n**Design.** A.\nB.", "P\nClamp it.", "**Design.** A.\nB."},
		{"no requirement", "**Design.** A.", "P", "**Design.** A."},
	} {
		prompt, answer := examQuestion(&key.Item{Step: "P", Answer: c.body})
		if prompt != c.prompt || answer != c.answer {
			t.Errorf("%s: prompt %q, answer %q; want %q, %q", c.name, prompt, answer, c.prompt, c.answer)
		}
	}
}

// A key with CRLF line ends splits the same way (the loader drops the CRs).
func TestExamQuestionCRLF(t *testing.T) {
	root := examRoot(t)
	exam := filepath.Join(root, filepath.FromSlash(key.ExamFile))
	if err := os.WriteFile(exam, []byte(strings.ReplaceAll(readFile(t, exam), "\n", "\r\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	_, out, _ := run("--root", root, "quiz", "--ids", "P2", "--answers")
	if !strings.Contains(out, "    Problem 2 — BExport\n    Export histories.\n") || !strings.Contains(out, "    **Design.** A service writes a CSV.\n") {
		t.Errorf("CRLF key:\n%q", out)
	}
}

// A record line of exactly maxRecordLine bytes is read; one byte more is skipped, with or without
// a final newline; a final oversize line without a newline is reported once.
func TestHistoryLineBoundary(t *testing.T) {
	pad := func(n int) string {
		r := rec("2026-10-11T00:00:00Z", "D1.1", "decision", "RA1", "wrong")
		return r + strings.Repeat(" ", n-len(r))
	}
	for _, c := range []struct {
		name, data string
		answers    int
		warned     string
	}{
		{"at the bound", pad(maxRecordLine) + "\n", 1, ""},
		{"at the bound, no newline", pad(maxRecordLine), 1, ""},
		{"one over", pad(maxRecordLine+1) + "\n", 0, ":1: not a record"},
		{"one over, no newline", pad(maxRecordLine + 1), 0, ":1: not a record"},
		{"huge last line, no newline", rec("2026-10-11T00:00:00Z", "T1.1", "trap", "RA1", "right") + "\n" + strings.Repeat("x", 2<<20), 1,
			":2: not a record"},
	} {
		path := filepath.Join(t.TempDir(), "h.jsonl")
		if err := os.WriteFile(path, []byte(c.data), 0o644); err != nil {
			t.Fatal(err)
		}
		var errb strings.Builder
		recs, err := loadHistory(cli.Stdio{Err: &errb}, path)
		if err != nil || len(recs) != c.answers || strings.Count(errb.String(), "not a record") != min(1, len(c.warned)) ||
			!strings.Contains(errb.String(), c.warned) {
			t.Errorf("%s: %d records, err %v, stderr %q", c.name, len(recs), err, errb.String())
		}
	}
}

// progress --offline --json: the total object is the sum of the groups, and a JSON document that
// cannot be written is exit 2 (no summary: the run did not complete).
func TestProgressOfflineJSONTotalAndWriteFailure(t *testing.T) {
	clearStationEnv(t)
	_, out, _ := run("--root", "testdata/root", "progress", "--offline", "--json")
	total := out[strings.Index(out, `"total"`):]
	for _, w := range []string{`"items": 24`, `"done": 4`, `"needs_run": 6`, `"todo": 14`} {
		if !strings.Contains(total, w) {
			t.Errorf("total lacks %s:\n%s", w, total)
		}
	}
	var errb strings.Builder
	code := Main([]string{"--root", "testdata/root", "progress", "--offline", "--json"}, cli.Stdio{Out: failWriter{}, Err: &errb})
	if code != 2 || errb.String() != "course: --json: write refused\n" {
		t.Errorf("exit %d, stderr %q", code, errb.String())
	}
}
