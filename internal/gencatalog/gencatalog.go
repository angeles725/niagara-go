// Package gencatalog is tools/gen-catalog.py: it writes CATALOG.md, the table of every
// main-corpus block with its title, or checks (--check) or prints (--stdout) it.
//
// The repository is the parent of the directory holding the executable, symlinks
// resolved (Python: Path(__file__).resolve().parent.parent), so the binary installed
// as <repo>/tools/gen-catalog finds <repo> from any working directory.
//
// Exit: 0 written/printed/fresh, 1 stale under --check or a failed write, 2 a usage
// error or a block or CATALOG.md that could not be read (intended difference: Python
// dies with a traceback and exit 1, which --check callers would take for "stale").
package gencatalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/blockfiles"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyre"
)

const catalogName = "CATALOG.md"

// snapshots are listed in their own section (gen-catalog.py SNAPSHOTS).
var snapshots = []string{"niagara-mental-model.2026-04-19.md"}

var (
	// _H1_PREFIX_RE and _H1_NO_TITLE_RE of gen-catalog.py.
	h1PrefixRe = pyre.MustCompile(`^#\s+`+
		`(?:Niagara\s+N4\s+[—\-]+\s+)?(?:Mental\s+Model\s+[·\-—]+\s+)?`+
		`Bloque\s+(?:#?\d+|TI)\s*[—:\-]+\s*`, pyre.IGNORECASE)
	h1NoTitleRe = pyre.MustCompile(
		`^(?:Niagara\s+N4\s+[—\-]+\s+)?(?:Mental\s+Model\s+[·\-—]+\s+)?Bloque\s+(?:#?\d+|TI)\s*$`, pyre.IGNORECASE)
	catalogRowRe = pyre.MustCompile(`^\| [^|]+ \| \[([^\]]+)\]`, pyre.MULTILINE)
)

// repoRoot locates the repository; tests replace it.
var repoRoot = func() (string, error) {
	exe, err := cli.InvokedPath()
	if err != nil {
		return "", err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return "", err
	}
	return filepath.Dir(filepath.Dir(exe)), nil
}

type entry struct{ label, filename, title string }

// Main runs gen-catalog with argv (without the program name).
func Main(args []string, s cli.Stdio) int {
	p := argparse.New(argparse.Config{Prog: "gen-catalog.py", Description: "Generate CATALOG.md from the block H1s."})
	mode := p.AddMutuallyExclusiveGroup(false)
	mode.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "exit 1 if CATALOG.md is stale; write nothing"}, "--check")
	mode.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "print the catalog instead of writing it"}, "--stdout")
	ns, code, done := p.Parse(args, s)
	if done {
		return code
	}
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintf(s.Err, "gen-catalog: cannot locate the repository: %v\n", err)
		return 1
	}
	entries, err := collect(root)
	if err != nil {
		fmt.Fprintf(s.Err, "gen-catalog: %v\n", err)
		return 2 // could not read a block: neither fresh nor stale (Python: traceback, exit 1)
	}
	content := render(entries)
	catalog := filepath.Join(root, catalogName)
	switch {
	case ns.Bool("stdout"):
		fmt.Fprint(s.Out, content)
		return 0
	case ns.Bool("check"):
		return check(s, catalog, content, entries)
	}
	if err := os.WriteFile(catalog, []byte(content), 0o666); err != nil {
		fmt.Fprintf(s.Err, "gen-catalog: cannot write %s: %v\n", catalog, err)
		return 1
	}
	fmt.Fprintf(s.Out, "Generated %s — %d blocks listed.\n", catalogName, len(entries))
	for _, e := range entries {
		fmt.Fprintf(s.Out, "  Bloque %s  %s\n", pycompat.RJust(e.label, 4, ' '), e.filename)
	}
	return 0
}

func check(s cli.Stdio, catalog, content string, entries []entry) int {
	current, present, err := readCatalog(catalog)
	if err != nil {
		fmt.Fprintf(s.Err, "gen-catalog: cannot read %s: %s\n", catalog, blockfiles.Strerror(err))
		return 2 // could not check: not the exit 1 of a stale catalog (Python: traceback)
	}
	if present && current == content {
		fmt.Fprintf(s.Out, "%s is up to date — %d blocks.\n", catalogName, len(entries))
		return 0
	}
	listed := map[string]bool{}
	for _, f := range catalogRowRe.FindAll(current) {
		listed[f] = true
	}
	onDisk := map[string]bool{}
	for _, e := range entries {
		onDisk[e.filename] = true
	}
	missing, orphan := 0, 0
	for f := range onDisk {
		if !listed[f] {
			missing++
		}
	}
	for f := range listed {
		if !onDisk[f] {
			orphan++
		}
	}
	absent := ""
	if !present {
		absent = " (file absent)"
	}
	fmt.Fprintf(s.Out, "%s is stale: %d block file(s) missing, %d row(s) without a file%s. "+
		"Regenerate: python3 tools/gen-catalog.py\n", catalogName, missing, orphan, absent)
	return 1
}

// readCatalog is Path.read_text when Path.is_file(): UTF-8, universal newlines.
func readCatalog(path string) (text string, present bool, err error) {
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() {
		return "", false, nil
	}
	text, err = blockfiles.ReadText(path)
	return text, err == nil, err
}

func collect(root string) ([]entry, error) {
	var out []entry
	for _, b := range blockfiles.MainBlocks(root) {
		h1, err := extractH1(b.Path)
		if err != nil {
			return nil, fmt.Errorf("cannot read %s: %s", b.Path, blockfiles.Strerror(err))
		}
		title := normalizeTitle(h1)
		if t, ok := titleOverrides[b.Label]; ok {
			title = t
		}
		if title == "" && h1 != "" {
			title = pycompat.Strip(strings.TrimLeft(h1, "# "))
		}
		if title == "" {
			title = "(no title)"
		}
		out = append(out, entry{b.Label, filepath.Base(b.Path), title})
	}
	return out, nil
}

// extractH1 is gen-catalog.extract_h1: the first stripped line starting with "# ".
func extractH1(path string) (string, error) {
	text, err := blockfiles.ReadText(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.SplitAfter(text, "\n") {
		if line = pycompat.Strip(line); strings.HasPrefix(line, "# ") {
			return line, nil
		}
	}
	return "", nil
}

// normalizeTitle is gen-catalog.normalize_title: the H1 without its block prefix, ""
// when it holds nothing but block metadata.
func normalizeTitle(h1 string) string {
	if h1 == "" {
		return ""
	}
	text := pycompat.Strip(strings.TrimLeft(h1, "# "))
	if h1NoTitleRe.MatchString(text) {
		return ""
	}
	text = "# " + text
	if m := h1PrefixRe.Search(text); m != nil {
		text = string([]rune(text)[m.End():])
	}
	return pycompat.Strip(strings.TrimLeft(text, "# "))
}

func render(entries []entry) string {
	lines := []string{
		"<!-- AUTOGENERADO por tools/gen-catalog.py — NO EDITAR A MANO. Regenerar: python3 tools/gen-catalog.py -->",
		"", "# Catálogo de bloques", "", fmt.Sprintf("Total: **%d bloques**", len(entries)), "",
		"| Bloque | Archivo | Título |", "|--------|---------|--------|",
	}
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("| %s | [%s](%s) | %s |", e.label, e.filename, e.filename, e.title))
	}
	lines = append(lines, "", "## Snapshots", "", "Archivos de snapshot histórico (excluidos del catálogo activo):", "")
	for _, s := range snapshots {
		lines = append(lines, fmt.Sprintf("- [%s](%s) — snapshot sesión httpapi (2026-04-19)", s, s))
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}
