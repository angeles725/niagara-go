package corpusnav

import "testing"

// Expected outputs: corpus-nav.json of the Python golden.

func TestGraphCommands(t *testing.T) {
	c, out, _ := testCorpus(t)
	if code := c.cmdConnections("4", false); code != 0 || out.String() != "B4 · Enrutamiento de alarmas\n\n"+
		"## Forward links (this block -> others, citations anywhere in its text)\n  B5 · Histories and trend logs\n"+
		"  B6 · Security incident review\n\n## Reverse links (other blocks -> B4, whole-corpus scan)\n"+
		"  B5 · Histories and trend logs\n" {
		t.Errorf("connections 4: %d %q", code, out.String())
	}
	out.Reset()
	if code := c.cmdPath("4", "1-3", false, false); code != 0 || out.String() != "B4 · Enrutamiento de alarmas\n"+
		"  -> B5 · Histories and trend logs\n  -> B1-3 · Bloques 1-3 consolidados\n\n"+
		"2 hop(s); '->' cites the next block, '<-' is cited by it.\n" {
		t.Errorf("path 4 1-3: %d %q", code, out.String())
	}
	out.Reset()
	if code := c.cmdGraph(true, 10, false); code != 0 || out.String() != `{"nodes": ["B1-3", "B4", "B5", "B6", "BTI"], `+
		`"forward": {"B1-3": [], "B4": ["B5", "B6"], "B5": ["B1-3", "B4", "B77"], "B6": [], "BTI": []}, `+
		`"reverse": {"B1-3": ["B5"], "B4": ["B5"], "B5": ["B4"], "B6": ["B4"], "B77": ["B5"], "BTI": []}, `+
		`"dangling": ["B77"], "unreadable": [], "stats": {"nodes": 5, "edges": 5, "dangling": 1, "unreadable": 0}}`+"\n" {
		t.Errorf("graph --json: %d %q", code, out.String())
	}
	out.Reset()
	t.Chdir("testdata/corpus")
	c, out, _ = testCorpus(t)
	c.root = "."
	if code := c.cmdLintRefs([]string{"niagara-mental-model-bloque5.md"}, false, 50); code != 1 ||
		out.String() != "B5:9: [Bloque N]     [Bloque 4]  -> [B4]\n\n1 non-canonical citation(s) in 1 file(s) "+
			"(canonical form: [B<N>] or [B<N> §x]; report-only, nothing rewritten)\n  [Bloque N]          1\n" {
		t.Errorf("lint-refs: %d %q", code, out.String())
	}
}

func TestParseLabel(t *testing.T) {
	for in, want := range map[string]string{"B720": "720", "b0720": "720", " 720 ": "720", "ti": "TI", "1-3": "1-3", "B": "B"} {
		if got := parseLabel(in); got != want {
			t.Errorf("parseLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
