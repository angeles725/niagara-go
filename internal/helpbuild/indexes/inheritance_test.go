package indexes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// The fixture's bajadoc-clean/: every kind of header line, padded and
// empty lists, CRLF and lone-CR pages, a BOM, Unicode whitespace around
// names, duplicate simple names (javax.baja.* preferred, no package, an
// empty package, the same class twice), pages without a class line.
func TestBuildInheritanceFixture(t *testing.T) {
	checkS8Golden(t, "inherit", "inheritance.json")
}

func TestBuildInheritanceNoCleanDir(t *testing.T) {
	base := t.TempDir()
	res, err := BuildInheritance(base)
	want := "ERROR: bajadoc-clean/ not found at " + filepath.Join(base, "bajadoc-clean")
	if err != nil || res.Status != 1 || strings.Join(res.Log, "\n") != want {
		t.Errorf("got %v, status %d, log %q; want status 1, %q", err, res.Status, res.Log, want)
	}
	if _, err := os.Stat(filepath.Join(base, "indexes")); !os.IsNotExist(err) {
		t.Errorf("indexes/ created: %v", err)
	}
}

// pyTextLines reports a decode error when the chunk that holds it is read,
// at the position inside that chunk; the messages are CPython 3.15's for
// the same files (`for line in open(p, encoding="utf-8")`, break at the
// first empty line).
func TestPyTextLinesChunks(t *testing.T) {
	head := "CLASS: BBad\nPACKAGE: zz\n\n"
	open := "CLASS: BOpen\nPACKAGE: zz\nNOTE: "
	fill := func(prefix string, room int) string { return prefix + strings.Repeat("x", room-len(prefix)) }
	for _, c := range []struct{ name, data, err string }{
		{"in header", "CLASS: BIn\xffHeader\nPACKAGE: zz\n", "byte 0xff in position 10: invalid start byte"},
		{"after header, same chunk", head + "DESCRIPTION:\n\xff\n", "byte 0xff in position 38: invalid start byte"},
		{"after header, next chunk", fill(head, textChunk) + "\xff\n", ""},
		{"cut by the chunk end", fill(open, textChunk-1) + "\u20ac\xff\n", "byte 0xff in position 3: invalid start byte"},
		{"bad continuation after the cut", fill(open, textChunk-1) + "\xe2\x28\xac\n", "byte 0xe2 in position 0: invalid continuation byte"},
		{"blank line's CR at the chunk end", fill(open, textChunk-2) + "\r\r\xff", "byte 0xff in position 0: invalid start byte"},
		{"truncated at the end", "CLASS: B\n\xe2\x82", "bytes in position 0-1: unexpected end of data"},
		{"one byte truncated", "CLASS: B\n\xe2", "byte 0xe2 in position 0: unexpected end of data"},
		{"third chunk", "CLASS: BLong\nIMPLEMENTS: " + strings.Repeat("BIa, ", 4000) + "BIz\n\n\xff", "byte 0xff in position 3646: invalid start byte"},
		{"CRLF header, then a bad byte in the same chunk", "CLASS: A\r\nPACKAGE: p\r\n\r\n\xff", "byte 0xff in position 24: invalid start byte"},
		{"valid", "CLASS: A\r\nPACKAGE: p\r\n\r\nDESCRIPTION:\r\n", ""},
	} {
		path := filepath.Join(t.TempDir(), "x.txt")
		testkit.WriteFile(t, path, []byte(c.data))
		var lines []string
		err := pyTextLines(path, func(line string) bool {
			lines = append(lines, line)
			return line != "\n"
		})
		want := ""
		if c.err != "" {
			want = "'utf-8' codec can't decode " + c.err
		}
		if got := errText(err); got != want {
			t.Errorf("%s: %q, want %q", c.name, got, want)
		}
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Universal newlines: "\r\n" and a lone "\r" end a line as "\n", also when
// the "\r" ends a decode chunk; the last line may have no newline.
func TestPyTextLinesNewlines(t *testing.T) {
	cr := strings.Repeat("y", textChunk-1) + "\r"
	for _, c := range []struct{ data, want string }{
		{"a\r\nb\rc\n\nd", "a\n|b\n|c\n|\n|d"},
		{cr + "\nz", strings.Repeat("y", textChunk-1) + "\n|z"},
		{cr + "z", strings.Repeat("y", textChunk-1) + "\n|z"},
		{cr, strings.Repeat("y", textChunk-1) + "\n"},
		{"", ""},
	} {
		path := filepath.Join(t.TempDir(), "x.txt")
		testkit.WriteFile(t, path, []byte(c.data))
		var lines []string
		if err := pyTextLines(path, func(line string) bool { lines = append(lines, line); return true }); err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(lines, "|"); got != c.want {
			t.Errorf("%q: %q, want %q", c.data[:min(len(c.data), 12)], got, c.want)
		}
	}
}
