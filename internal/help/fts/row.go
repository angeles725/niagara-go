package fts

import "fmt"

// FormatHit is the row the ranked full-text commands (find, guide-search,
// devguide-search) print for one hit, so every caller renders the exact CLI
// text: "  [<tag>] <title>", the best snippet line "    L<n>: <text>" when the
// hit has one, and "    path: <source>/<path>", each ending in a newline. The
// blank line between rows belongs to the caller.
func FormatHit(dirs []SourceDir, h *Hit, groups []Group, tag string) string {
	row := fmt.Sprintf("  [%s] %s\n", tag, h.Title)
	if n, text := Snippet(dirs, h, groups); n != 0 {
		row += fmt.Sprintf("    L%d: %s\n", n, text)
	}
	return row + fmt.Sprintf("    path: %s\n", h.Key())
}
