package corpusnav

import "testing"

// Expected outputs: corpus-nav.json of the Python golden.

func TestBrowseCommands(t *testing.T) {
	c, out, _ := testCorpus(t)
	if code := c.cmdStats(); code != 0 || out.String() != "Corpus totals\n  blocks : 5\n  sub-corpus blocks: 3\n"+
		"  docs   : 2\n  retros : 1\n  focuses: 1\n  blocks with a Connections section: 2\n\n"+
		"Evidence markers (occurrences across blocks)\n  CERT                    4\n  INFER                   3\n"+
		"  CERT-doc                1\n  CERT-hw                 1\n  CERT-live               1\n" {
		t.Errorf("stats: %d %q", code, out.String())
	}
	out.Reset()
	if code := c.cmdByFocus("beta"); code != 0 || out.String() != "B5 · Histories and trend logs\n\nTotal: 1 block(s) in focus 'beta'.\n" {
		t.Errorf("by-focus beta: %d %q", code, out.String())
	}
	out.Reset()
	if code := c.cmdSemantic("alarm escalation", true, false, true, 3); code != 0 || out.String() !=
		"[backend: tfidf (forced by --tfidf)]\nB4 · Enrutamiento de alarmas   (score 0.392)\n"+
			"  » Escalación de alarmas: the escalation levels [CERT via B5 §2].\nB6 · Security incident review   (score 0.121)\n"+
			"  » The incident touched alarm recipients and user accounts [CERT+INFER] [CERT*].\n" {
		t.Errorf("semantic: %d %q", code, out.String())
	}
}

func TestMarkerComponents(t *testing.T) {
	got := markerComponents("[CERT-doc/INFER] [CERT via B5 §3] [certainly] [CERTAIN] [INFER, grounded] [CERT+INFER]")
	want := "CERT-doc,INFER,CERT,INFER,CERT,INFER"
	s := ""
	for i, x := range got {
		if i > 0 {
			s += ","
		}
		s += x
	}
	if s != want {
		t.Errorf("markerComponents = %s, want %s", s, want)
	}
}
