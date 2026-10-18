package licensador

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
)

func newParser() *argparse.Parser {
	p := argparse.New(argparse.Config{Prog: "licensador",
		Description: "REF-License — licenciamiento firmado propio (referencia)"})
	sub := p.AddSubparsers(argparse.SubparsersConfig{Dest: "cmd", Required: true})
	req := argparse.Arg{Required: true}

	g := sub.AddParser("genkeys", argparse.SubparserConfig{Help: "genera raiz privada + publica"})
	g.Add(req, "--out-dir")
	g.Add(argparse.Arg{Type: argparse.Int, Default: 3072}, "--bits")

	i := sub.AddParser("issue", argparse.SubparserConfig{Help: "emite una licencia .refl"})
	i.Add(req, "--key")
	i.Add(req, "--product")
	i.Add(req, "--host")
	i.Add(argparse.Arg{Default: ""}, "--licensee")
	i.Add(argparse.Arg{}, "--expires")
	i.Add(req, "--out")
	i.Add(argparse.Arg{Action: argparse.Append, Default: []any{}, Help: "NAME o NAME:QTY (repetible)"}, "--feature")

	v := sub.AddParser("verify", argparse.SubparserConfig{Help: "verifica una licencia (pipeline completo)"})
	v.Add(req, "--root")
	v.Add(req, "--file")
	for _, name := range []string{"--product", "--host", "--state", "--blacklist"} {
		v.Add(argparse.Arg{}, name)
	}

	ins := sub.AddParser("inspect", argparse.SubparserConfig{Help: "muestra los claims sin verificar"})
	ins.Add(req, "--file")

	h := sub.AddParser("hostid", argparse.SubparserConfig{Help: "calcula el REF-... de esta maquina"})
	h.Add(argparse.Arg{Default: saltDefault}, "--salt")

	r := sub.AddParser("revoke", argparse.SubparserConfig{Help: "firma una blacklist de seriales"})
	r.Add(req, "--key")
	r.Add(req, "--serial")
	r.Add(req, "--out")

	s := sub.AddParser("selftest", argparse.SubparserConfig{Help: "matriz de aceptacion A1..A10"})
	s.Add(req, "--tmp")
	s.Add(argparse.Arg{Type: argparse.Int, Default: 3072}, "--bits")
	return p
}

// opt returns nil for an option left at its None default.
func opt(ns *argparse.Namespace, dest string) *string {
	if ns.Get(dest) == nil {
		return nil
	}
	v := ns.String(dest)
	return &v
}

// Main is the licensador entry point (licensador.py main()).
func Main(args []string, s cli.Stdio) int {
	ns, code, done := newParser().Parse(args, s)
	if done {
		return code
	}
	switch ns.String("cmd") {
	case "genkeys":
		return RunGenkeys(ns.String("out_dir"), ns.Int("bits"), s)
	case "issue":
		return RunIssue(IssueCmd{Key: ns.String("key"), Product: ns.String("product"), Host: ns.String("host"),
			Out: ns.String("out"), Licensee: ns.String("licensee"), Expires: opt(ns, "expires"),
			Features: ns.Strings("feature")}, s)
	case "verify":
		return RunVerify(VerifyCmd{Root: ns.String("root"), File: ns.String("file"), Product: opt(ns, "product"),
			Host: opt(ns, "host"), State: opt(ns, "state"), Blacklist: opt(ns, "blacklist")}, s)
	case "inspect":
		return RunInspect(ns.String("file"), s)
	case "hostid":
		return RunHostID(ns.String("salt"), s)
	case "revoke":
		return RunRevoke(ns.String("key"), ns.String("serial"), ns.String("out"), s)
	}
	return RunSelftest(ns.String("tmp"), ns.Int("bits"), s)
}
