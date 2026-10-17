// Package bogwrite is the Go port of niagara-research tools/bog-write.py: an offline BOG-XML
// generator (generate) and structural validator (validate) for planned station logic.
//
// Intended differences: a failure Python reports with a traceback (an unreadable or invalid
// plan, a bad plan value, an unwritable --out) prints only the traceback's last line,
// "<Exception>: <message>", with the same exit status 1; type errors on unusual plan values
// follow Python's messages for the common cases only. The written .bog has Python's ZIP
// layout (one deflated file.xml, the same header fields, the local time now) but Go's
// deflate stream, so the compressed bytes differ while the XML inside is byte-identical.
// selftest runs built-in checks equivalent to tools/tests/test_bog_write.py (the real
// GreenMAX template check is skipped: no repository next to the binary) and prints
// unittest's summary.
package bogwrite

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

const description = "" +
	"bog-write: offline BOG-XML generator + validator for planned station logic (B1186, gap AM31).\n" +
	"\n" +
	"WHY: tools/bog-nav.py only READS bogs. The MCP's offline route (R-F) needs the opposite: turn a PLAN\n" +
	"(components, slot values, links, wire-sheet positions) into an importable `.bog` snippet and refuse malformed\n" +
	"documents before anyone imports them. Grammar facts are those read from real artefacts in B1186:\n" +
	"`<p n h m t f v>` nodes, ZIP container with one entry `file.xml` (or plain XML), links = persistent children\n" +
	"`Link`/`Link1` of the TARGET with children sourceOrd (`h:<source handle>`), sourceSlotName, targetSlotName,\n" +
	"module prefixes declared on first use (document order), `wsAnnotation` = \"x,y,w[,h]\".\n" +
	"\n" +
	"Usage:\n" +
	"    bog-write.py generate <plan.json> --out file.bog [--plain]\n" +
	"    bog-write.py validate <file.bog|backup.dist|file.xml>\n" +
	"    bog-write.py selftest\n" +
	"\n" +
	"Plan JSON:\n" +
	"  {\"root\": {\"name\": \"Demo\", \"type\": \"baja:Folder\"},\n" +
	"   \"components\": [{\"name\": \"SelR\", \"type\": \"control:NumericWritable\", \"parent\": \"optional/path\",\n" +
	"                   \"ws\": [x, y, w], \"slots\": {\"slot\": \"scalar\" | {\"type\": \"baja:Facets\", \"value\": \"...\"}}}],\n" +
	"   \"links\": [{\"from\": \"SelR.out\", \"to\": \"Mux.select\"}]}\n" +
	"A component name ending in `?` is made unique like BComponent.add (Foo -> Foo1). Never emits passwords.\n" +
	"Validation never proves the station will accept the document (no checkLink/checkAdd, no licence): it is a\n" +
	"structural gate, the live import stays a tracked PoC (B1186 AM32).\n"

func parser() *argparse.Parser {
	p := argparse.New(argparse.Config{Prog: "bog-write.py", Description: description,
		Formatter: argparse.RawDescriptionHelpFormatter})
	sub := p.AddSubparsers(argparse.SubparsersConfig{Dest: "cmd", Required: true})
	g := sub.AddParser("generate", argparse.SubparserConfig{})
	g.Add(argparse.Arg{}, "plan")
	g.Add(argparse.Arg{Required: true}, "--out")
	g.Add(argparse.Arg{Action: argparse.StoreTrue}, "--plain")
	v := sub.AddParser("validate", argparse.SubparserConfig{})
	v.Add(argparse.Arg{}, "file")
	sub.AddParser("selftest", argparse.SubparserConfig{})
	return p
}

// Main is bog-write.py main().
func Main(args []string, s cli.Stdio) int {
	ns, code, done := parser().Parse(args, s)
	if done {
		return code
	}
	switch ns.String("cmd") {
	case "generate":
		out := ns.String("out")
		if err := generate(ns.String("plan"), out, ns.Bool("plain")); err != nil {
			fmt.Fprintln(s.Err, err)
			return 1
		}
		fmt.Fprintf(s.Out, "wrote %s\n", out)
		return 0
	case "validate":
		fs := validatePath(ns.String("file"))
		errs := 0
		for _, f := range fs {
			fmt.Fprintf(s.Out, "%s  %s\n", f[0], f[1])
			if f[0] == "ERROR" {
				errs++
			}
		}
		fmt.Fprintf(s.Out, "%d error(s), %d warning(s)\n", errs, len(fs)-errs)
		if errs > 0 {
			return 1
		}
		return 0
	}
	return selftest(s.Err)
}

func generate(planPath, out string, plain bool) error {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return osError(err, planPath)
	}
	text, err := readPlanText(data)
	if err != nil {
		return err
	}
	plan, err := decodePlan(text)
	if err != nil {
		return err
	}
	return writeBog(plan, out, plain)
}

// writeBog is write_bog: generate, refuse a document with an ERROR finding, then write
// plain XML or a ZIP holding file.xml.
func writeBog(plan pyjson.Value, path string, plain bool) error {
	xml, err := generateXML(plan)
	if err != nil {
		return err
	}
	if err := surrogateError(xml); err != nil {
		return err
	}
	var errs []string
	for _, f := range validateXML([]byte(xml)) {
		if f[0] == "ERROR" {
			errs = append(errs, f.repr())
		}
	}
	if len(errs) > 0 {
		return raise("ValueError", "generated document failed validation: [%s]", strings.Join(errs, ", "))
	}
	if plain {
		if err := os.WriteFile(path, []byte(xml), 0o666); err != nil {
			return osError(err, path)
		}
		return nil
	}
	return writeZip(path, []byte(xml))
}

// osError renders an OS error as the Python OSError subclass and text open() raises.
//
// TODO(pycompat-format): replace with pycompat.OSErrorText once feat/pycompat-format lands
// (its errno tables also cover Windows).
func osError(err error, path string) error {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return raise("OSError", "%s", err)
	}
	typ := map[syscall.Errno]string{syscall.ENOENT: "FileNotFoundError", syscall.EISDIR: "IsADirectoryError",
		syscall.ENOTDIR: "NotADirectoryError", syscall.EACCES: "PermissionError", syscall.EPERM: "PermissionError",
		syscall.EEXIST: "FileExistsError"}[errno]
	if typ == "" {
		typ = "OSError"
	}
	msg := errno.Error()
	if r, size := utf8.DecodeRuneInString(msg); size > 0 {
		msg = string(unicode.ToUpper(r)) + msg[size:]
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		path = pe.Path
	}
	return raise(typ, "[Errno %d] %s: %s", int(errno), msg, pycompat.StrRepr(path))
}
