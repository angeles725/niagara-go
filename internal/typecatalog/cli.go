// Package typecatalog is n4-type-catalog: a static catalog of the Niagara 4 component slot
// declarations of Java source trees (Go port of niagara-research tools/n4-type-catalog.py).
//
//	n4-type-catalog build <dir>... [--out catalog.json]
//	n4-type-catalog show <catalog.json|dir> <BClass|pkg.BClass|module:Type|Type>
//	n4-type-catalog selftest
//
// Exit codes: 0 found, 1 no type (build: no declaration under the dirs; show: no match),
// 2 usage error, a missing directory or an unreadable catalog.
package typecatalog

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
)

const tool = "n4-type-catalog"

func newParser() *argparse.Parser {
	ap := argparse.New(argparse.Config{Prog: tool + ".py", Description: description,
		Formatter: argparse.RawDescriptionHelpFormatter})
	sub := ap.AddSubparsers(argparse.SubparsersConfig{Dest: "cmd", Required: true})
	b := sub.AddParser("build", argparse.SubparserConfig{})
	b.Add(argparse.Arg{Nargs: argparse.OneOrMore}, "dirs")
	b.Add(argparse.Arg{}, "--out")
	s := sub.AddParser("show", argparse.SubparserConfig{})
	s.Add(argparse.Arg{}, "catalog")
	s.Add(argparse.Arg{}, "type")
	sub.AddParser("selftest", argparse.SubparserConfig{})
	return ap
}

// Main runs n4-type-catalog with args (without the program name).
func Main(args []string, s cli.Stdio) int {
	ns, code, done := newParser().Parse(args, s)
	if done {
		return code
	}
	switch ns.String("cmd") {
	case "build":
		var dirs []string
		for _, d := range ns.Get("dirs").([]any) {
			dirs = append(dirs, d.(string))
		}
		return build(s, dirs, ns.String("out"))
	case "show":
		cat, err := Load(ns.String("catalog"))
		if err != nil {
			fmt.Fprintf(s.Err, "%s: cannot read catalog %s: %v\n", tool, ns.String("catalog"), err)
			return cli.ExitCannotSearch
		}
		return show(s.Out, s.Err, cat, ns.String("type"))
	default:
		return selftest(s.Err)
	}
}

func build(s cli.Stdio, dirs []string, out string) int {
	var missing []string
	for _, d := range dirs {
		if st, err := os.Stat(d); err != nil || !st.IsDir() {
			missing = append(missing, d)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(s.Err, "%s: not a directory: %s\n", tool, strings.Join(missing, ", "))
		return cli.ExitCannotSearch
	}
	cat, nfiles := Build(dirs)
	if cat.Len() == 0 {
		fmt.Fprintf(s.Err, "%s: 0 types with slot declarations; searched %d .java file(s) under %s\n",
			tool, nfiles, strings.Join(dirs, ", "))
		return cli.ExitNoMatch
	}
	data := cat.JSON()
	if out == "" { // Python: `if args.out:`
		io.WriteString(s.Out, data+"\n")
		return cli.ExitFound
	}
	if err := os.WriteFile(out, []byte(data), 0o666); err != nil {
		fmt.Fprintf(s.Err, "%s: cannot write %s: %v\n", tool, out, err)
		return cli.ExitCannotSearch
	}
	var props, acts, topics int
	for _, k := range cat.Keys {
		r := cat.Get(k)
		props, acts, topics = props+len(r.Properties), acts+len(r.Actions), topics+len(r.Topics)
	}
	fmt.Fprintf(s.Out, "types: %d  properties: %d  actions: %d  topics: %d\n", cat.Len(), props, acts, topics)
	return cli.ExitFound
}
