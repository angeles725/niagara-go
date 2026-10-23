package corpusnav

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestExtractSectionNumberedHeadings: the corpus numbers its Connections headings
// ("## 4.7 — Connections", "## §12.3 — Connections", "## 9.x — Conexiones") and often
// extends the title ("Connections & open gaps"); show and stats read every such heading,
// not only the bare "## Connections" (half the real blocks were reported without one).
func TestExtractSectionNumberedHeadings(t *testing.T) {
	cases := []struct {
		name, heading string
		found         bool
	}{
		{"bare", "## Connections", true},
		{"bare Spanish", "## Conexiones", true},
		{"lower case", "## connections", true},
		{"numbered with em dash", "## 4.7 — Connections", true},
		{"section sign", "## §12.3 — Connections", true},
		{"numbered Spanish", "## 76.2 — Conexiones", true},
		{"letter sub-number", "## 9.x — Connections", true},
		{"trailing dot number", "## 5. Connections", true},
		{"no dash", "## §3.1 Connections", true},
		{"en dash", "## 3.1 – Connections", true},
		{"extended title", "## 4.7 — Connections & open gaps", true},
		{"extended Spanish", "## 8.8 Conexiones con otros bloques", true},
		{"mentioned later", "## 4.7 — Tesis viva y Connections", false},
		{"longer word", "## 4.7 — Connectionsless", false},
		{"level three", "### 4.7 — Connections", false},
		{"other section", "## 4.7 — Gaps", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "b.md")
			text := "# B4\n\n## 4.1 — Intro\nintro\n\n" + tc.heading + "\n- B5 link\n\n## 4.9 — Next\nnext\n"
			if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
			got := extractSection(path, "Connections", "Conexiones")
			var want []string
			if tc.found {
				want = []string{"- B5 link", ""}
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("extractSection(%q) = %q, want %q", tc.heading, got, want)
			}
		})
	}
}
