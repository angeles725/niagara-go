package helpbuild

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/helpbuild/indexes"
	"github.com/angeles725/niagara-go/internal/helpbuild/pdftext"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// rebuild-all (maintenance.cmd_rebuild_all): every builder of the plan in
// dependency order, then the corpus manifest indexes/.manifest.json. The
// Python orchestrator starts each builder script as a child process with
// the data root as its argument; RebuildAll calls the Go builders in
// process and writes to opt.Stdout and opt.Stderr what the parent and its
// children print there, byte for byte. Each builder writes its own outputs
// atomically (see the S10a section of docs/helpbuild.md); RebuildAll takes
// no lock (nh calls it under the scope lock of the full-text index).

// builder is one row of maintenance.BUILDERS: the Python script it ports
// (printed in the plan and matched by --only), the artifacts it writes,
// and the data-root folder it needs ("" when it finds its own input: the
// PDF manuals of the registered install, the course bound in the
// registry). The skip-if folder is always None in Python and is not
// ported.
type builder struct {
	Script   string
	Produces []string
	Source   string
	run      func(root string, w *rebuildIO, opt *RebuildOptions) (status int, err error)
}

// optionalArtifacts are the artifacts a builder writes only under a
// condition, shown in the plan (OPTIONAL_ARTIFACTS).
var optionalArtifacts = map[string]string{"jdk-class-index.json": "when jdk/ exists"}

// rebuildBuilders is maintenance.BUILDERS without build_call_index.py
// (call-index.json is retired, decision I3-5), plus the Go-only text
// sources (package-text/, lexicon-text/, module-examples/), module-index.json,
// palette-index.json, tag-index.json, lexicon-index.json and
// bytecode-index.json. Tests swap a
// run function.
var rebuildBuilders = []builder{
	{"bajadoc_parser.py", []string{"bajadoc-clean/"}, "bajadoc", runBajadoc},
	{"bajadoc_xml_parser.py", []string{"bajadoc-clean/"}, "devguide", runBajadocXML},
	{"build_class_index.py", []string{"class-index.json", "jdk-class-index.json"}, "devguide", indexRun(indexes.BuildClassIndex)},
	{"inheritance_builder.py", []string{"inheritance.json"}, "bajadoc-clean", indexRun(indexes.BuildInheritance)},
	{"method_indexer.py", []string{"method-index.json"}, "bajadoc-clean", indexRun(indexes.BuildMethodIndex)},
	{"source_indexer.py", []string{"source-index.json"}, "source", indexRun(indexes.BuildSourceIndex)},
	{"slots_indexer.py", []string{"slots-index.json"}, "source", indexRun(indexes.BuildSlotsIndex)},
	{"xref_builder.py", []string{"xref-index.json"}, "source", indexRun(indexes.BuildXrefIndex)},
	{"guides_parser.py", []string{"guides-clean/"}, "guides", textRun(parseGuides)},
	{"build_guides_index.py", []string{"guides-index.json"}, "guides", textRun(buildGuidesIndex)},
	{"devguide_parser.py", []string{"devguide-clean/"}, "devguide", textRun(parseDevguide)},
	{"build_devguide_toc.py", []string{"devguide-toc.json"}, "devguide", textRun(buildDevguideTOC)},
	{"pdf_extractor.py", []string{"pdf-index.json", "docs-text/"}, "", runPDF},
	{"course_extractor.py", []string{"course-text/"}, "", runCourse},
	{packageTextScript, []string{packageTextDir + "/"}, "devguide", textRun(buildPackageText)},
	{lexiconScript, []string{lexiconTextDir + "/"}, "", textRun(buildLexiconText)},
	{moduleExamplesScript, []string{moduleExamplesDir + "/"}, "", textRun(buildModuleExamples)},
	{moduleIndexScript, []string{moduleIndexName}, "", textRun(buildModuleIndex)},
	{paletteIndexScript, []string{paletteIndexName}, "", textRun(buildPaletteIndex)},
	{tagIndexScript, []string{tagIndexName}, "", textRun(buildTagIndex)},
	{lexiconIndexScript, []string{lexiconIndexName}, "", textRun(buildLexiconIndex)},
	{bytecodeIndexScript, []string{bytecodeIndexName}, "", textRun(buildBytecodeIndex)},
}

// RebuildOptions are rebuild-all's switches. Only keeps the builders whose
// script name starts with it ("" keeps all); DryRun prints the plan and
// runs nothing; StampOnly writes the manifest and nothing else. Stdout and
// Stderr receive what Python prints on each stream (nil discards). Now
// stamps the manifest (nil = time.Now). Pdftotext is PDFOptions.Pdftotext.
type RebuildOptions struct {
	Only              string
	DryRun, StampOnly bool
	Stdout, Stderr    io.Writer
	Now               func() time.Time
	Pdftotext         string
}

// RebuildResult counts the plan. Stamped is the manifest's stamped_utc
// when it was written ("" for a dry run or a failed build).
type RebuildResult struct {
	Ran, Skipped, Failed int
	Stamped              string
}

// rebuildIO writes the two streams and keeps the first write error.
type rebuildIO struct {
	stdout, stderr io.Writer
	err            error
}

func (w *rebuildIO) write(dst io.Writer, s string) {
	if w.err == nil {
		_, w.err = io.WriteString(dst, s)
	}
}

// out prints one stdout line.
func (w *rebuildIO) out(format string, a ...any) { w.write(w.stdout, fmt.Sprintf(format, a...)+"\n") }

// lines prints each of lines and a newline on dst.
func (w *rebuildIO) lines(dst io.Writer, lines []string) {
	for _, l := range lines {
		w.write(dst, l+"\n")
	}
}

// RebuildAll is cmd_rebuild_all on the data root root. It returns an error
// where Python raises (a stream that cannot be written, a manifest that
// cannot be saved): the CLI exits 1 then and 0 otherwise, failed builders
// included (they are counted, and the manifest is not re-stamped).
func RebuildAll(root string, opt RebuildOptions) (*RebuildResult, error) {
	w := &rebuildIO{stdout: opt.Stdout, stderr: opt.Stderr}
	if w.stdout == nil {
		w.stdout = io.Discard
	}
	if w.stderr == nil {
		w.stderr = io.Discard
	}
	now := opt.Now
	if now == nil {
		now = time.Now
	}
	res := &RebuildResult{}
	if opt.StampOnly {
		stamped, err := stampManifest(root, now())
		if err != nil {
			return res, err
		}
		res.Stamped = stamped
		w.out("Manifest stamped at %s (baseline = current corpus).", stamped)
		return res, w.err
	}
	header := ""
	if opt.DryRun {
		header = "(dry run)"
	}
	w.out("REBUILD-ALL %s", header)
	w.out("%s", strings.Repeat("=", 60))
	for _, b := range rebuildBuilders {
		if w.err != nil {
			return res, w.err
		}
		if opt.Only != "" && !strings.HasPrefix(b.Script, opt.Only) {
			continue
		}
		label := b.label()
		if b.Source != "" && !pycompat.IsDir(pycompat.Join(root, b.Source)) {
			w.out("  SKIP  %s  (source '%s/' absent)", label, b.Source)
			res.Skipped++
			continue
		}
		if opt.DryRun {
			w.out("  PLAN  %s", label)
			continue
		}
		w.out("  RUN   %s", label)
		if w.err != nil {
			return res, w.err
		}
		if rc := b.call(root, w, &opt); rc == 0 {
			res.Ran++
		} else {
			w.out("  FAIL  %s (exit %d)", b.Script, rc)
			res.Failed++
		}
	}
	if opt.DryRun {
		w.out("")
		w.out("Dry run — nothing executed. Drop --dry-run to build.")
		return res, w.err
	}
	w.out("")
	w.out("Ran %d, skipped %d, failed %d.", res.Ran, res.Skipped, res.Failed)
	if w.err != nil {
		return res, w.err
	}
	if res.Failed > 0 {
		w.out("Builders failed — manifest NOT updated (still reflects last good build).")
		return res, w.err
	}
	stamped, err := stampManifest(root, now())
	if err != nil {
		return res, err
	}
	res.Stamped = stamped
	w.out("Manifest re-stamped. Run 'freshness' to confirm FRESH.")
	return res, w.err
}

// label is "{:26s} -> {}" of the script and its artifacts, the optional
// ones with their condition.
func (b builder) label() string {
	parts := make([]string, len(b.Produces))
	for i, a := range b.Produces {
		parts[i] = a
		if cond, ok := optionalArtifacts[a]; ok {
			parts[i] = fmt.Sprintf("%s (%s)", a, cond)
		}
	}
	return fmt.Sprintf("%-26s -> %s", b.Script, strings.Join(parts, ", "))
}

// call runs the builder as the child process would: its exit status, or
// 1 with an ERROR line on stderr where the script raises (an error or a
// panic of the Go builder; Python prints a traceback there).
func (b builder) call(root string, w *rebuildIO, opt *RebuildOptions) (status int) {
	defer func() {
		if p := recover(); p != nil {
			w.write(w.stderr, fmt.Sprintf("ERROR: %s raised: panic: %v\n", b.Script, p))
			status = 1
		}
	}()
	status, err := b.run(root, w, opt)
	if err != nil {
		w.write(w.stderr, fmt.Sprintf("ERROR: %s raised: %s\n", b.Script, strings.ReplaceAll(err.Error(), "\n", " ")))
		return 1
	}
	return status
}

func runBajadoc(root string, w *rebuildIO, _ *RebuildOptions) (int, error) {
	res, err := indexes.ParseBajadoc(root)
	if res == nil {
		return 1, err
	}
	w.lines(w.stdout, res.Log)
	return res.Status, err
}

func runBajadocXML(root string, w *rebuildIO, _ *RebuildOptions) (int, error) {
	res, err := indexes.ParseBajadocXML(root)
	if res != nil {
		w.lines(w.stdout, res.Log)
	}
	return 0, err
}

// indexRun adapts the S8/S9 index builders: stdout lines, stderr lines,
// exit status.
func indexRun(build func(string) (*indexes.IndexResult, error)) func(string, *rebuildIO, *RebuildOptions) (int, error) {
	return func(root string, w *rebuildIO, _ *RebuildOptions) (int, error) {
		res, err := build(root)
		if res == nil {
			return 1, err
		}
		w.lines(w.stdout, res.Log)
		w.lines(w.stderr, res.Stderr)
		return res.Status, err
	}
}

// textRun adapts the S5/S6 builders: stdout lines, then the warnings on
// stderr; their scripts always exit 0.
func textRun(build func(string) (*textResult, error)) func(string, *rebuildIO, *RebuildOptions) (int, error) {
	return func(root string, w *rebuildIO, _ *RebuildOptions) (int, error) {
		res, err := build(root)
		if res != nil {
			w.lines(w.stdout, res.Log)
			w.lines(w.stderr, res.Warnings)
		}
		return 0, err
	}
}

// runPDF streams pdf_extractor's output as it is produced (its progress
// entries start with "\r" and end without a newline).
func runPDF(root string, w *rebuildIO, opt *RebuildOptions) (int, error) {
	res, err := pdftext.ExtractPDFs(root, pdftext.PDFOptions{Pdftotext: opt.Pdftotext, Emit: func(e string) {
		if !strings.HasPrefix(e, "\r") {
			e += "\n"
		}
		w.write(w.stdout, e)
	}})
	if res == nil {
		return 1, err
	}
	return res.Status, err
}

func runCourse(root string, w *rebuildIO, _ *RebuildOptions) (int, error) {
	rules, err := Default()
	if err != nil {
		return 1, err
	}
	res, err := courseExtractor(root, rules)
	if res == nil {
		return 1, err
	}
	w.lines(w.stdout, res.Stdout)
	w.lines(w.stderr, res.Stderr)
	return res.Status, err
}

// RebuildToLog returns the Rebuild hook of SetupOptions: _rebuild's
// rebuild-all on the data root with its stdout in root/build.log (stderr
// to stderr, nil discards), returning the build.log lines that start with
// "  SKIP", "  FAIL" or "Ran " (ExtractAndIndex adds the indexes/ folder
// and the pdf-index.json stub before it and the EMPTY lines after).
// build.log is a plain truncating write, as Python's open(path, "w"): it
// is a log no reader depends on.
func RebuildToLog(stderr io.Writer) func(root string) ([]string, error) {
	return func(root string) ([]string, error) {
		path := pycompat.Join(root, "build.log")
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
		if err != nil {
			return nil, err
		}
		_, err = RebuildAll(root, RebuildOptions{Stdout: f, Stderr: stderr})
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return buildLogTail(data), nil
	}
}

// buildLogTail is _rebuild's read-back of build.log: decoded with
// errors="replace", split as universal newlines, the lines that start with
// "  SKIP", "  FAIL" or "Ran ", rstripped.
func buildLogTail(data []byte) []string {
	text := strings.ReplaceAll(pycompat.DecodeReplace(data), "\r\n", "\n")
	var tail []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r", "\n"), "\n") {
		if strings.HasPrefix(line, "  SKIP") || strings.HasPrefix(line, "  FAIL") || strings.HasPrefix(line, "Ran ") {
			tail = append(tail, strings.TrimRightFunc(line, textfold.IsSpace))
		}
	}
	return tail
}
