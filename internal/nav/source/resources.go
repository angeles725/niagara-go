package source

import (
	"archive/zip"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/registry"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// resources (extras.py cmd_resources): the non-class entries of a submodule
// JAR, organized/*/<sub>/<sub>.jar.

func runResources(env *registry.Env, ns *argparse.Namespace) int {
	name, typeFilter, limit := ns.String("name"), ns.String("res_type"), ns.Int("n")
	organized, tried := core.ResolveOrganized()
	if organized == "" {
		env.Error("corpus not found: "+tried, "")
		return env.ExitCode()
	}
	sub := strings.TrimSuffix(name, ".jar")
	jars := map[string]string{}
	var names []string
	for _, s := range core.IterSubmodules(organized, nil) {
		if _, seen := jars[s.Name]; !seen {
			names = append(names, s.Name)
		}
		jars[s.Name] = filepath.Join(s.Path, s.Name+".jar")
	}
	jar, ok := jars[sub]
	if st, err := os.Stat(jar); !ok || err != nil || !st.Mode().IsRegular() {
		env.Report(fmt.Sprintf("submodule '%s'", sub),
			fmt.Sprintf("%s submodules under %s", core.Commas(int64(len(names))), organized),
			sub, names, "modules --has-code")
		return env.ExitCode()
	}
	w := env.Out
	rule := "  " + strings.Repeat("=", 65)
	fmt.Fprintf(w, "\n%s\n  RESOURCES: %s.jar\n%s\n\n", rule, sub, rule)
	entries, err := jarNames(jar)
	if err != nil {
		env.Error(fmt.Sprintf("cannot read %s: %v", jar, err), "")
		return env.ExitCode()
	}
	var classes, resources []string
	for _, e := range entries {
		switch {
		case strings.HasSuffix(e, "/"):
		case strings.HasSuffix(e, ".class"):
			classes = append(classes, e)
		default:
			resources = append(resources, e)
		}
	}
	byExt := map[string][]string{}
	var exts []string
	for _, r := range resources {
		ext := "(none)"
		if strings.Contains(r, ".") {
			_, e := pycompat.PosixSplitext(r)
			ext = pycompat.Lower(e)
		}
		if _, seen := byExt[ext]; !seen {
			exts = append(exts, ext)
		}
		byExt[ext] = append(byExt[ext], r)
	}
	if typeFilter != "" {
		tf := typeFilter
		if !strings.HasPrefix(tf, ".") {
			tf = "." + tf
		}
		filtered := byExt[pycompat.Lower(tf)]
		fmt.Fprintf(w, "  Filter: %s\n", tf)
		fmt.Fprintf(w, "  Matches: %s\n\n", core.Commas(int64(len(filtered))))
		listMore(env, filtered, limit)
		fmt.Fprintln(w)
		return env.ExitCode()
	}
	fmt.Fprintf(w, "  Total entries:   %s\n", core.RJust(core.Commas(int64(len(entries))), 6))
	fmt.Fprintf(w, "  Class files:     %s\n", core.RJust(core.Commas(int64(len(classes))), 6))
	fmt.Fprintf(w, "  Resources:       %s\n\n", core.RJust(core.Commas(int64(len(resources))), 6))
	fmt.Fprintln(w, "  BY EXTENSION:")
	sort.SliceStable(exts, func(a, b int) bool { return len(byExt[exts[a]]) > len(byExt[exts[b]]) })
	for _, ext := range exts {
		fmt.Fprintf(w, "    %s %s files\n", core.LJust(ext, 15), core.RJust(core.Commas(int64(len(byExt[ext]))), 5))
	}
	fmt.Fprintln(w)
	shown := min(limit, len(resources))
	fmt.Fprintf(w, "  ALL RESOURCES (%d, showing %d):\n", len(resources), shown)
	fmt.Fprintln(w, "  "+strings.Repeat("-", 65))
	listMore(env, resources, limit)
	fmt.Fprintln(w)
	return env.ExitCode()
}

// listMore prints items[:limit] one per line, then "... and N more" when
// some were left out.
func listMore(env *registry.Env, items []string, limit int) {
	end := min(limit, len(items))
	if limit < 0 {
		end = max(0, len(items)+limit)
	}
	for _, r := range items[:end] {
		fmt.Fprintf(env.Out, "    %s\n", r)
	}
	if len(items) > limit {
		fmt.Fprintf(env.Out, "    ... and %d more\n", len(items)-limit)
	}
}

// jarNames is zipfile.ZipFile(path).namelist(): the central directory names
// in order, UTF-8 when the entry says so, else cp437, cut at a NUL. Errors
// read as Python's: "File is not a zip file" for a malformed archive, the
// OSError text for a file that cannot be opened.
func jarNames(path string) ([]string, error) {
	zr, err := zip.OpenReader(path)
	var pe *fs.PathError
	switch {
	case errors.Is(err, zip.ErrFormat):
		return nil, errors.New("File is not a zip file")
	case errors.As(err, &pe):
		return nil, errors.New(index.OSError(pe.Err, path))
	case err != nil:
		return nil, err
	}
	defer zr.Close()
	names := make([]string, len(zr.File))
	for i, f := range zr.File {
		n := f.Name
		if f.Flags&0x800 == 0 {
			n = decodeCP437(n)
		}
		if k := strings.IndexByte(n, 0); k >= 0 {
			n = n[:k]
		}
		names[i] = n
	}
	return names, nil
}

// cp437High maps bytes 0x80-0xFF of code page 437 to their characters.
const cp437High = "ÇüéâäàåçêëèïîìÄÅÉæÆôöòûùÿÖÜ¢£¥₧ƒáíóúñÑªº¿⌐¬½¼¡«»░▒▓│┤╡╢╖╕╣║╗╝╜╛┐└┴┬├─┼╞╟╚╔╩╦╠═╬╧╨╤╥╙╘╒╓╫╪┘┌█▄▌▐▀αßΓπΣσµτΦΘΩδ∞φε∩≡±≥≤⌠⌡÷≈°∙·√ⁿ²■ "

var cp437Runes = []rune(cp437High)

// decodeCP437 is bytes.decode("cp437"): ASCII as is, the high half by table.
func decodeCP437(s string) string {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if c := s[i]; c < 0x80 {
			b.WriteByte(c)
		} else {
			b.WriteRune(cp437Runes[c-0x80])
		}
	}
	return b.String()
}
