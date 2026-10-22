package corpusnav

import (
	"path/filepath"
	"testing"
)

// find takes several words like nsearch does (cnav find alarm histories): they are joined with
// spaces, so the answer is the one-argument query's, byte for byte.
func TestFindAcceptsSeveralWords(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("testdata", "corpus"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CORPUS_NAV_ROOT", root)
	t.Setenv("CORPUS_NAV_FIND_INDEX", "off")
	t.Setenv("GIT_DIR", "/nonexistent")
	for _, mode := range []string{"--and", "--or"} {
		wantCode, wantOut, wantErr := runMain("find", "alarm histories", mode)
		code, out, errs := runMain("find", "alarm", "histories", mode)
		if code != wantCode || out != wantOut || errs != wantErr || wantCode != 0 {
			t.Errorf("%s: split words exit %d stderr %q; one query exit %d", mode, code, errs, wantCode)
		}
	}
}
