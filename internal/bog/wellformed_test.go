package bog

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const wellFormedDoc = `<?xml version='1.0' encoding='UTF-8'?>
<!-- a comment -->
<bajaObjectGraph version='4.0'>
 <p h='1' m='b=baja' t='b:Station'>
  <p n='A' h='2' t='b:Folder' v="a &gt; b"/>
  <p n="B" h="3" t="b:Folder"><![CDATA[x < y]]></p>
  <?pi data?>
 </p>
</bajaObjectGraph>
`

func TestCheckWellFormed(t *testing.T) {
	cases := []struct {
		name, text, want string // want is "" for a well-formed text, else the start of the message
	}{
		{"well-formed", wellFormedDoc, ""},
		{"truncated mid-element", wellFormedDoc[:strings.Index(wellFormedDoc, "<p n='A'")], "no element found"},
		{"unterminated start tag", "<bajaObjectGraph version='4.0'>\n<p", "unclosed token"},
		{"unterminated root tag", "<bajaObjectGraph version='4.0'", "unclosed token"},
		{"unclosed quote", "<bajaObjectGraph v='4.0>\n</bajaObjectGraph>", "unclosed token"},
		{"mismatched end tag", "<bajaObjectGraph><p></a></bajaObjectGraph>", "mismatched tag"},
		{"junk after root", "<bajaObjectGraph/><p/>", "junk after document element"},
		{"text after root", "<bajaObjectGraph/>x", "junk after document element"},
		{"unclosed comment", "<bajaObjectGraph><!-- x", "unclosed token"},
		{"lt in attribute", "<bajaObjectGraph v='<'/>", "not well-formed (invalid token)"},
		{"no root", "<?xml version='1.0'?>\n", "no element found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckWellFormed(tc.text)
			switch {
			case tc.want == "" && err != nil:
				t.Fatalf("well-formed text rejected: %v", err)
			case tc.want != "" && err == nil:
				t.Fatalf("malformed text accepted, want %q", tc.want)
			case tc.want != "" && !strings.HasPrefix(err.Error(), tc.want+": line "):
				t.Fatalf("error %q, want prefix %q", err, tc.want+": line ")
			}
		})
	}
}

func TestCheckWellFormedPosition(t *testing.T) {
	err := CheckWellFormed("<bajaObjectGraph>\n <p></a>\n</bajaObjectGraph>")
	if err == nil || err.Error() != "mismatched tag: line 2, column 6" {
		t.Fatalf("got %v", err)
	}
}

// Load fails closed: a truncated bog is a *ReadError, never a smaller graph.
func TestLoadRejectsTruncatedBog(t *testing.T) {
	p := filepath.Join(t.TempDir(), "t.xml")
	if err := os.WriteFile(p, []byte(wellFormedDoc[:len(wellFormedDoc)/2]), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Load(p)
	var re *ReadError
	if b != nil || !errors.As(err, &re) || !strings.HasPrefix(err.Error(), p+": not well-formed XML (") {
		t.Fatalf("Load = %v, %v", b, err)
	}
	if _, err := ReadCheckedText(p); err == nil || err.Error() != re.Error() {
		t.Fatalf("ReadCheckedText = %v, want %v", err, re)
	}
}
