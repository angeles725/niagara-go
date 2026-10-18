package audits

import (
	"archive/zip"
	"fmt"
	"io"
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

// palette-lexicon-agents (niagara-research tools/palette-lexicon-agents.py,
// decision D39 e): for every artifact of <base-dir>/<module>, the <p> entries
// of module.palette, the keys of every **/*.lexicon with their duplicate bare
// keys (B759 hazard), and the <agent> registrations of
// extracted/META-INF/module.xml. Files come from extracted/; module.palette and
// the lexicons also from the artifact JAR. --all is the census of every module
// under <base-dir>. Without the global --base-dir the base is the organized/
// corpus (the script's default), not the module-navigator checkout.

// paletteEntry is one <p> element: its n, t and m attributes when present.
type paletteEntry struct{ keys, values []string }

// agent is one <agent> of a <type>.
type agent struct {
	typeName, typeClass string
	onTypes             []string
}

// artifactData is _collect_artifact_data.
type artifactData struct {
	artifact    string
	palette     []paletteEntry
	lexiconKeys int
	dups        *index.Object // duplicate bare key ("<file>:" unless <artifact>.lexicon) -> count
	agents      []agent
}

// readExtracted is _read_extracted_file: the text of a regular file (UTF-8
// with replacement, universal newlines), ok=false when absent or unreadable.
func readExtracted(path string) (string, bool) {
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() {
		return "", false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return pycompat.UniversalNewlines(pycompat.DecodeReplace(b)), true
}

// readFromJar is _read_from_jar: the member's bytes decoded as UTF-8 with
// replacement, ok=false when the JAR or the member cannot be read.
func readFromJar(jar, member string) (string, bool) {
	if st, err := os.Stat(jar); err != nil || !st.Mode().IsRegular() {
		return "", false
	}
	zr, err := zip.OpenReader(jar)
	if err != nil {
		return "", false
	}
	defer zr.Close()
	var hit *zip.File
	for _, f := range zr.File {
		if core.ZipName(f) == member {
			hit = f // zipfile keeps the last entry of a repeated name
		}
	}
	if hit == nil {
		return "", false
	}
	rc, err := hit.Open()
	if err != nil {
		return "", false
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		return "", false
	}
	return pycompat.DecodeReplace(b), true
}

// readArtifactFile is the extracted/ copy of name, else the JAR member.
func readArtifactFile(dir, artifact, name string) (string, bool) {
	if s, ok := readExtracted(filepath.Join(dir, "extracted", name)); ok {
		return s, true
	}
	return readFromJar(filepath.Join(dir, artifact+".jar"), name)
}

// parsePalette is parse_palette: every <p> with an n, t or m attribute.
func parsePalette(text string) []paletteEntry {
	if text == "" {
		return nil
	}
	root, err := parseET(text)
	if err != nil {
		return nil
	}
	var out []paletteEntry
	root.iter("p", func(e *etElement) {
		var p paletteEntry
		for _, k := range []string{"n", "t", "m"} {
			if v, ok := e.get(k); ok {
				p.keys = append(p.keys, k)
				p.values = append(p.values, v)
			}
		}
		if len(p.keys) > 0 {
			out = append(out, p)
		}
	})
	return out
}

// lexiconKeys are the bare keys of a .properties text: the text before the
// first '=' of every non-blank, non-comment line holding one, stripped.
func lexiconKeys(text string) []string {
	var keys []string
	for _, line := range pycompat.SplitLines(text, false) {
		s := pycompat.Strip(line)
		if s == "" || strings.HasPrefix(s, "#") || !strings.Contains(s, "=") {
			continue
		}
		if k := pycompat.Strip(s[:strings.IndexByte(s, '=')]); k != "" {
			keys = append(keys, k)
		}
	}
	return keys
}

// parseAgents is parse_agents: the <agent> children of every <type>.
func parseAgents(text string) []agent {
	if text == "" {
		return nil
	}
	root, err := parseET(text)
	if err != nil {
		return nil
	}
	var out []agent
	root.iter("type", func(t *etElement) {
		name, _ := t.get("name")
		class, _ := t.get("class")
		for _, a := range t.findAll("agent") {
			ag := agent{typeName: name, typeClass: class, onTypes: []string{}}
			for _, on := range a.findAll("on") {
				v, _ := on.get("type")
				ag.onTypes = append(ag.onTypes, v)
			}
			out = append(out, ag)
		}
	})
	return out
}

// collectArtifact is collect_artifact_data.
func collectArtifact(dir, artifact string) artifactData {
	d := artifactData{artifact: artifact, dups: index.NewObject()}
	if text, ok := readArtifactFile(dir, artifact, "module.palette"); ok {
		d.palette = parsePalette(text)
	}
	for _, lex := range artifactLexicons(filepath.Join(dir, "extracted"), filepath.Join(dir, artifact+".jar")) {
		prefix := lex.rel + ":"
		if lex.rel == artifact+".lexicon" {
			prefix = ""
		}
		keys := lexiconKeys(lex.text)
		d.lexiconKeys += len(keys)
		counts := map[string]int64{}
		var order []string
		for _, k := range keys {
			if counts[k] == 0 {
				order = append(order, k)
			}
			counts[k]++
		}
		for _, k := range order {
			if counts[k] > 1 {
				d.dups.Set(prefix+k, counts[k])
			}
		}
	}
	if text, ok := readExtracted(filepath.Join(dir, "extracted", "META-INF", "module.xml")); ok {
		d.agents = parseAgents(text)
	}
	return d
}

// findArtifacts is _find_artifacts: the sorted subdirectories holding their
// own JAR or an extracted/ directory.
func findArtifacts(moduleDir string) []string {
	f, err := os.Open(moduleDir)
	if err != nil {
		return nil
	}
	names, _ := f.Readdirnames(-1)
	f.Close()
	sort.Strings(names)
	var out []string
	for _, n := range names {
		dir := filepath.Join(moduleDir, n)
		if !isDir(dir) {
			continue
		}
		if st, err := os.Stat(filepath.Join(dir, n+".jar")); (err == nil && st.Mode().IsRegular()) ||
			isDir(filepath.Join(dir, "extracted")) {
			out = append(out, n)
		}
	}
	return out
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// collectModule is collect_module: the data of every artifact of
// <base>/<module>, none when it is not a directory.
func collectModule(base, module string) []artifactData {
	moduleDir := pycompat.Join(base, module)
	var all []artifactData
	for _, a := range findArtifacts(moduleDir) {
		all = append(all, collectArtifact(pycompat.Join(moduleDir, a), a))
	}
	return all
}

func runPaletteLexiconAgents(env *registry.Env, ns *argparse.Namespace) int {
	module, asJSON := ns.String("module"), ns.Bool("pla_json")
	w := env.Out
	base := env.BaseDir
	if ns.String("base_dir") == "" {
		// The script's default is the corpus, not the checkout (D39 e); pinned under nav.RunAt.
		base = env.OrganizedDir()
	}
	if !isDir(base) {
		fmt.Fprintf(env.Err, "ERROR: base-dir not found: %s\n", base)
		return core.StatusError
	}
	if ns.Bool("pla_all") {
		return paletteCensus(env, base, asJSON)
	}
	if module == "" {
		return ns.Get("pla_parser").(*argparse.Parser).Error(env.Err, "give a module name or --all")
	}
	all := collectModule(base, module)
	if len(all) == 0 {
		msg := fmt.Sprintf("No artifacts for module '%s'; searched %s (%d module dirs)",
			module, base, countDirs(base))
		if asJSON {
			o := index.NewObject()
			o.Set("module", module)
			o.Set("error", msg)
			fmt.Fprintln(w, core.Dumps(o, 2))
		} else {
			fmt.Fprintln(env.Err, "  ERROR: "+msg)
		}
		return core.StatusMiss
	}
	if asJSON {
		fmt.Fprintln(w, core.Dumps(paletteJSON(module, all), 2))
	} else {
		printPaletteReport(w, module, all)
	}
	return env.ExitCode()
}

// paletteJSON is the --json document.
func paletteJSON(module string, all []artifactData) *index.Object {
	list := make([]any, len(all))
	for i, d := range all {
		entries := make([]any, len(d.palette))
		for j, p := range d.palette {
			o := index.NewObject()
			for k, key := range p.keys {
				o.Set(key, p.values[k])
			}
			entries[j] = o
		}
		agents := make([]any, len(d.agents))
		for j, a := range d.agents {
			o := index.NewObject()
			o.Set("type_name", a.typeName)
			o.Set("type_class", a.typeClass)
			o.Set("on_types", a.onTypes)
			agents[j] = o
		}
		o := index.NewObject()
		o.Set("artifact", d.artifact)
		o.Set("palette_count", int64(len(d.palette)))
		o.Set("palette_entries", entries)
		o.Set("lexicon_keys", int64(d.lexiconKeys))
		o.Set("duplicate_bare_keys", d.dups)
		o.Set("agents", agents)
		list[i] = o
	}
	out := index.NewObject()
	out.Set("module", module)
	out.Set("artifacts", list)
	return out
}

// printPaletteReport is _module_report.
func printPaletteReport(w io.Writer, module string, all []artifactData) {
	sep := strings.Repeat("=", 65)
	fmt.Fprintf(w, "\n  %s\n  PALETTE / LEXICON / AGENTS: %s\n  %s\n", sep, module, sep)
	var palette, lexicon, agents, dups int
	for _, d := range all {
		palette += len(d.palette)
		lexicon += d.lexiconKeys
		agents += len(d.agents)
		dups += d.dups.Len()
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Artifacts scanned:   %d\n", len(all))
	fmt.Fprintf(w, "  Palette entries:     %d\n", palette)
	fmt.Fprintf(w, "  Lexicon keys:        %d\n", lexicon)
	fmt.Fprintf(w, "  Duplicate bare keys: %d\n", dups)
	fmt.Fprintf(w, "  Agent registrations: %d\n", agents)
	fmt.Fprintln(w)
	for _, d := range all {
		fmt.Fprintf(w, "  %s\n  Artifact: %s\n\n", strings.Repeat("-", 60), d.artifact)
		fmt.Fprintf(w, "  [PALETTE]  %d entries\n", len(d.palette))
		for _, p := range d.palette[:min(50, len(d.palette))] {
			parts := make([]string, len(p.keys))
			for k, key := range p.keys {
				parts[k] = key + "=" + p.values[k]
			}
			fmt.Fprintf(w, "    <p %s>\n", strings.Join(parts, "  "))
		}
		if len(d.palette) > 50 {
			fmt.Fprintf(w, "    ... and %d more\n", len(d.palette)-50)
		}
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  [LEXICON]  %d keys total\n", d.lexiconKeys)
		if d.dups.Len() > 0 {
			fmt.Fprintln(w, "  *** DUPLICATE BARE KEYS DETECTED (B759 hazard) ***")
			for _, k := range sortedKeys(d.dups) {
				fmt.Fprintf(w, "    DUP key=%s  occurrences=%d\n", pycompat.StrRepr(k), dupCount(d.dups, k))
			}
		} else {
			fmt.Fprintln(w, "    No duplicate bare keys.")
		}
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  [AGENTS]   %d registration(s)\n", len(d.agents))
		for _, a := range d.agents {
			fmt.Fprintf(w, "    type_name=%s  class=%s  on=%s\n", pycompat.StrRepr(a.typeName),
				pycompat.StrRepr(a.typeClass), pycompat.Repr(a.onTypes))
		}
		if len(d.agents) == 0 {
			fmt.Fprintln(w, "    (none)")
		}
		fmt.Fprintln(w)
	}
}
