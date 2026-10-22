package qnx6read

import (
	"bytes"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// --help documents every exit code the command returns, including exit 1 for an extract source
// that is not in the image (ExitNoMatch in extract).
func TestHelpDocumentsExtractNotFoundExit(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Main([]string{"--help"}, cli.Stdio{Out: &out, Err: &errb}); code != 0 {
		t.Fatalf("--help exit %d", code)
	}
	help := strings.Join(strings.Fields(out.String()), " ")
	if !strings.Contains(help, "1 listing incomplete (depth limit hit) or extract <src> not found in the image") {
		t.Errorf("--help does not document exit 1 for a missing extract source:\n%s", out.String())
	}
}
