"""Generate fixtures.json: declarative parser specs plus the exact output of Python argparse.

Run from the repository root with `python3 -I internal/argparse/testdata/gen_fixtures.py`.
The Go test builds the identical parser from each spec and must reproduce stdout, stderr,
the exit status and the parsed namespace byte for byte.

Spec format (shared with spec_test.go):
  parser  = {"kw": {ArgumentParser kwargs}, "ops": [op, ...]}
  op      = {"op": "arg", "args": [names], "kw": {add_argument kwargs}}
          | {"op": "mutex", "required": bool, "ops": [arg ops]}
          | {"op": "group", "title": str|null, "description": str|null, "ops": [ops]}
          | {"op": "defaults", "kw": {dest: value}}
          | {"op": "set", "attr": "epilog"|"description"|"usage", "value": str}
          | {"op": "subparsers", "kw": {add_subparsers kwargs}, "required": bool|absent,
             "parsers": [{"name": str, "kw": {add_parser kwargs}, "ops": [ops]}]}
  Values: "$SUPPRESS" is argparse.SUPPRESS; "type" names a converter in TYPES; "action"
  "tools_version" is the custom --version action of niagara-research tools/lib/version.py;
  "parents" lists names from the top-level "parents" table (built fresh for every case);
  "formatter_class" names an argparse formatter; "_error_exit" makes error() exit with
  that status (searchdoor's _Parser). A case is run with COLUMNS set to its "columns".
"""
import argparse
import io
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
SUPPRESS = "$SUPPRESS"


def positive_int(text):
    """tools/lib/cliflags.py positive_int."""
    try:
        n = int(text)
    except ValueError:
        raise argparse.ArgumentTypeError(f"invalid int value: {text!r}") from None
    if n < 1:
        raise argparse.ArgumentTypeError(f"must be at least 1, not {n}")
    return n


def nav_positive_int(text):
    """module-navigator module_nav_lib/cliargs.py positive_int."""
    try:
        value = int(text)
    except ValueError:
        value = 0
    if value < 1:
        raise argparse.ArgumentTypeError("must be a positive integer, got {!r}".format(text))
    return value


def checked(text):
    """A converter that fails with a plain ValueError: 'invalid checked value: ...'."""
    if text.startswith("bad"):
        raise ValueError(text)
    return text.upper()


TYPES = {"int": int, "float": float, "str": str, "positive_int": positive_int,
         "nav_positive_int": nav_positive_int, "checked": checked}
FORMATTERS = {name: getattr(argparse, name) for name in (
    "HelpFormatter", "RawDescriptionHelpFormatter", "RawTextHelpFormatter",
    "ArgumentDefaultsHelpFormatter")}


class ToolsVersion(argparse.Action):
    """tools/lib/version.py VersionAction with a fixed version string."""

    def __init__(self, option_strings, dest=argparse.SUPPRESS, default=argparse.SUPPRESS,
                 help=None, tool=None):
        super().__init__(option_strings, dest=dest, default=default, nargs=0,
                         help=help or "print the tools version and exit")
        self.tool = tool

    def __call__(self, parser, namespace, values, option_string=None):
        prog = parser.prog[:-3] if parser.prog.endswith(".py") else parser.prog
        print(f"{self.tool or prog} 0.42.0")
        parser.exit(0)


def value(v):
    if v == SUPPRESS:
        return argparse.SUPPRESS
    if isinstance(v, list):
        return [value(x) for x in v]
    return v


def build(spec, parents):
    kw = dict(spec.get("kw", {}))
    error_exit = kw.pop("_error_exit", None)
    kw = convert_parser_kw(kw, parents)
    cls = argparse.ArgumentParser
    if error_exit is not None:
        class _Parser(argparse.ArgumentParser):
            def error(self, message):
                self.print_usage(sys.stderr)
                print(f"{self.prog}: error: {message}", file=sys.stderr)
                sys.exit(error_exit)
        cls = _Parser
    p = cls(**kw)
    apply_ops(p, spec.get("ops", []), parents)
    return p


def convert_parser_kw(kw, parents):
    if "formatter_class" in kw:
        kw["formatter_class"] = FORMATTERS[kw["formatter_class"]]
    if "parents" in kw:
        kw["parents"] = [parents[n] for n in kw["parents"]]
    return {k: value(v) for k, v in kw.items()}


def add_arg(container, op):
    kw = {k: value(v) for k, v in op.get("kw", {}).items()}
    if "type" in kw:
        kw["type"] = TYPES[kw["type"]]
    if kw.get("action") == "tools_version":
        kw["action"] = ToolsVersion
    if isinstance(kw.get("metavar"), list):
        kw["metavar"] = tuple(kw["metavar"])
    container.add_argument(*op.get("args", []), **kw)


def apply_ops(container, ops, parents):
    for op in ops:
        kind = op["op"]
        if kind == "arg":
            add_arg(container, op)
        elif kind == "mutex":
            apply_ops(container.add_mutually_exclusive_group(required=op.get("required", False)),
                      op["ops"], parents)
        elif kind == "group":
            g = container.add_argument_group(op.get("title"), op.get("description"))
            apply_ops(g, op["ops"], parents)
        elif kind == "defaults":
            container.set_defaults(**{k: value(v) for k, v in op["kw"].items()})
        elif kind == "set":
            setattr(container, op["attr"], op["value"])
        elif kind == "subparsers":
            sub = container.add_subparsers(**{k: value(v) for k, v in op.get("kw", {}).items()})
            if "required" in op:
                sub.required = op["required"]
            for child in op["parsers"]:
                kw = convert_parser_kw(dict(child.get("kw", {})), parents)
                sp = sub.add_parser(child["name"], **kw)
                apply_ops(sp, child.get("ops", []), parents)
        else:
            raise ValueError(f"unknown op {kind}")


def run_case(spec_name, specs, parent_specs, argv, columns):
    os.environ["COLUMNS"] = str(columns)
    parents = {}
    for name, pspec in parent_specs.items():
        parents[name] = build(pspec, parents)
    parser = build(specs[spec_name], parents)
    out, err = io.StringIO(), io.StringIO()
    saved = sys.stdout, sys.stderr
    sys.stdout, sys.stderr = out, err
    status, ns = None, None
    try:
        result = parser.parse_args(argv)
        ns = repr(dict(sorted(vars(result).items())))
    except SystemExit as exc:
        status = 0 if exc.code is None else exc.code
    finally:
        sys.stdout, sys.stderr = saved
    return {"parser": spec_name, "columns": columns, "argv": argv, "exit": status,
            "stdout": out.getvalue(), "stderr": err.getvalue(), "ns": ns}


def main():
    sys.dont_write_bytecode = True
    sys.path.insert(0, HERE)
    import matrix  # noqa: E402  (the parser and case matrix, next to this file)
    os.environ["NO_COLOR"] = "1"
    os.environ.pop("FORCE_COLOR", None)
    cases = [run_case(name, matrix.PARSERS, matrix.PARENTS, argv, cols)
             for name, argv, cols in matrix.cases()]
    doc = {"python": sys.version.split()[0], "parents": matrix.PARENTS,
           "parsers": matrix.PARSERS, "cases": cases}
    with open(os.path.join(HERE, "fixtures.json"), "w", encoding="utf-8") as f:
        json.dump(doc, f, indent=1, ensure_ascii=False, sort_keys=True)
        f.write("\n")
    print(f"{len(cases)} cases, {len(matrix.PARSERS)} parsers", file=sys.stderr)


if __name__ == "__main__":
    main()
