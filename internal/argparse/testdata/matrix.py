"""The parser and case matrix of gen_fixtures.py: one spec per feature family seen in the
niagara-research consumers (docs/argparse.md), each exercised by help, success and error cases."""

S = "$SUPPRESS"


def arg(*args, **kw):
    return {"op": "arg", "args": list(args), "kw": kw}


def mutex(*ops, required=False):
    return {"op": "mutex", "required": required, "ops": list(ops)}


def sub(name, ops=(), **kw):
    return {"name": name, "kw": kw, "ops": list(ops)}


def subparsers(parsers, required=None, **kw):
    op = {"op": "subparsers", "kw": kw, "parsers": list(parsers)}
    if required is not None:
        op["required"] = required
    return op


def defaults(**kw):
    return {"op": "defaults", "kw": kw}


# tools/lib/cliflags.py output_parents(): the main copy and the SUPPRESS-default copy.
def _output_flags(default, limit_default):
    return {"kw": {"add_help": False}, "ops": [
        arg("--json", action="store_true", default=default, help="machine JSON output"),
        arg("--csv", action="store_true", default=default, help="CSV output (header + rows)"),
        arg("--limit", "-n", type="positive_int", default=limit_default, metavar="N",
            help="print at most N rows (default: all); the cut is reported"),
    ]}


PARENTS = {"top": _output_flags(False, None), "common": _output_flags(S, S)}

PARSERS = {
    "basic": {"kw": {"prog": "basic.py", "description":
                     "Read a station bog and print what it holds. The auto-detected root is used "
                     "when --root is omitted; values are re-read on every run (no cache)."},
              "ops": [
        arg("src", help="input file (config.bog or a bare .xml)"),
        arg("dst", nargs="?", default="out.txt", help="output file (default: %(default)s)"),
        arg("-n", "--limit", type="int", default=20, help="max rows (default %(default)s)"),
        arg("--ratio", type="float", default=0.5, help="sampling ratio"),
        arg("-v", "--verbose", action="store_true", help="say more"),
        arg("-q", "--quiet", action="store_false", dest="loud", help="say less"),
        arg("--mode", choices=["fast", "slow", "auto"], default="auto",
            help="scan mode: %(choices)s"),
        arg("--tag", action="append", default=[], metavar="TAG", help="tag (repeatable)"),
        arg("--level", type="int", choices=[1, 2, 3], help="detail level"),
        arg("-c", "--count", action="count", help="count flag"),
        arg("--const", action="store_const", const="K", help="store a constant"),
        arg("--hidden", help=S),
        arg("--names", nargs="+", help="one or more names"),
        arg("--pair", nargs=2, metavar=["KEY", "VALUE"], help="a key and a value"),
        arg("--opt", nargs="*", help="zero or more"),
        arg("--maybe", nargs="?", const="C", default="D", help="optional value"),
        arg("-x", action="store_true", help="flag x"),
        arg("-y", action="store_true", help="flag y"),
        arg("-z", type="checked", help="checked value"),
        arg("--extend", action="extend", nargs="+", help="extend a list"),
        arg("--ac", action="append_const", const=7, dest="consts", help="append a constant"),
        arg("--version", action="version", version="%(prog)s 1.2.3"),
    ]},
    "required": {"kw": {"prog": "req"}, "ops": [
        arg("files", nargs="+", metavar="FILE", help="input files"),
        arg("--out", "-o", required=True, help="output directory"),
        arg("--fmt", required=True, choices=["md", "json"]),
    ]},
    "mutex": {"kw": {"prog": "mx", "description": "Mutually exclusive groups."}, "ops": [
        arg("query", nargs="+", help="free-text query"),
        mutex(arg("--and", dest="mode", action="store_const", const="and", help="every term"),
              arg("--or", dest="mode", action="store_const", const="or", help="any term")),
        mutex(arg("--json", action="store_true", help="JSON"),
              arg("--csv", action="store_true", help="CSV"),
              arg("--tsv", action="store_true", help=S), required=True),
    ]},
    "mutex_pos": {"kw": {"prog": "mxp"}, "ops": [
        arg("--first", action="store_true"),
        mutex(arg("--name", help="by name"), arg("target", nargs="?", help="by position")),
        arg("--last", type="int"),
    ]},
    "groups": {"kw": {"prog": "grp", "epilog": "See also: grp --help."}, "ops": [
        arg("input"),
        {"op": "group", "title": "output", "description": "How results are printed.", "ops": [
            arg("--json", action="store_true", help="JSON output"),
            mutex(arg("--wide", action="store_true"), arg("--narrow", action="store_true")),
        ]},
        {"op": "group", "title": "filters", "description": None, "ops": [
            arg("--min", type="int", default=0, metavar="N", help="minimum"),
            arg("pattern", nargs="*", help="patterns"),
        ]},
    ]},
    "rawdesc": {"kw": {"prog": "raw.py", "formatter_class": "RawDescriptionHelpFormatter",
                       "description": "Raw description.\n\n  indented line kept\n"
                                      "    deeper   spaced   line\n\nlast paragraph",
                       "epilog": "Examples:\n  %(prog)s one\n  %(prog)s two --flag\n"}, "ops": [
        arg("thing", help="the help text is still wrapped and   normalised   by the formatter"),
        arg("--flag", action="store_true", help="a flag"),
    ]},
    "rawtext": {"kw": {"prog": "rawtext", "formatter_class": "RawTextHelpFormatter",
                       "description": "Line one\n  line two"}, "ops": [
        arg("--a", help="first line\n  second line indented\nthird"),
        arg("--b", default="x", help="value is %(default)s"),
    ]},
    "argdefaults": {"kw": {"prog": "argdef", "formatter_class": "ArgumentDefaultsHelpFormatter"},
                    "ops": [
        arg("pos", help="a positional"),
        arg("rest", nargs="*", help="the rest"),
        arg("--num", type="int", default=3, help="a number"),
        arg("--name", help="a name"),
        arg("--flag", action="store_true", help="a flag"),
        arg("--req", required=True, help="required"),
        arg("--nohelp", default="v"),
        arg("--sup", default=S, help="suppressed default"),
        arg("--explicit", default=1, help="explicit %(default)s"),
    ]},
    "searchdoor": {"kw": {"prog": "nsearch", "allow_abbrev": False, "_error_exit": 3,
                          "description": "One query across our blocks, official docs and code."},
                   "ops": [
        arg("query", nargs="+", help="free-text query (`license <name>` for licensing)"),
        arg("--version", action="tools_version", tool="nsearch"),
        arg("--limit", "-n", type="int", default=5, help="ranked results per source (default 5)"),
        arg("--timeout", type="int", default=60,
            help="timeout budget per source in seconds, shared by all its backend calls (default 60)"),
        arg("--doc-version", metavar="VERSION",
            help="niagara-help documentation version for the docs source (default: niagara-help's own)"),
        mutex(arg("--and", dest="mode", action="store_const", const="and", help="every term (blocks, docs)"),
              arg("--or", dest="mode", action="store_const", const="or", help="any term (blocks, docs)")),
    ]},
    "usage": {"kw": {"prog": "custom", "usage": "%(prog)s [-h] [--base-dir DIR] <command> [<args>]",
                     "add_help": True}, "ops": [arg("--base-dir", "-d", help="base")]},
    "noargs": {"kw": {"prog": "bare", "add_help": False}, "ops": []},
    "longprog": {"kw": {"prog": "a-very-long-program-name-that-takes-most-of-the-line-by-itself.py"},
                 "ops": [
        arg("--alpha", help="a"), arg("--beta-option-with-long-name", metavar="SOME_LONG_VALUE"),
        arg("--gamma", action="store_true"), arg("first"), arg("second", nargs="+"),
    ]},
    "wrap": {"kw": {"prog": "wrap", "description": "x" * 120}, "ops": [
        arg("--opt-%d" % i, metavar="VALUE%d" % i, help="option number %d" % i) for i in range(8)
    ] + [
        arg("--a-really-long-option-name-to-force-a-break", help="help text that is long enough "
            "to wrap over several lines, with hyphenated-words, em--dashes and "
            "supercalifragilisticexpialidocious-unbreakable-ish tokens"),
        arg("positional_with_long_name", nargs="?", help="pos"),
    ]},
    "negnum": {"kw": {"prog": "neg"}, "ops": [
        arg("value", type="int", nargs="?"), arg("--x", type="float"), arg("-f", action="store_true")]},
    "negopt": {"kw": {"prog": "negopt"}, "ops": [
        arg("value", nargs="?"), arg("-1", dest="one", action="store_true")]},
    "abbrev": {"kw": {"prog": "abbr"}, "ops": [
        arg("--foo", type="int"), arg("--foobar", action="store_true"), arg("--fob"),
        arg("-n", type="int"), arg("-v", action="store_true"), arg("-w", action="store_true"),
        arg("--zeta"), arg("items", nargs="*"),
    ]},
    "unicode": {"kw": {"prog": "licensador", "description":
                       "Genera y verifica licencias de módulos Niagara: año, señal, cañón."}, "ops": [
        arg("--feature", action="append", default=[], help="NAME o NAME:QTY (repetible)"),
        arg("--dueño", help="nombre del dueño de la licencia, con acentos áéíóú y eñes"),
    ]},
    "qnx": {"kw": {"prog": "qnx6read.py", "formatter_class": "RawDescriptionHelpFormatter",
                   "description": "Read a QNX6 image.\n  part   partition name"}, "ops": [
        arg("part", nargs="?", default="P2", choices=["P1", "P2", "P3"], help="partition (default P2)"),
        arg("action", nargs="*", metavar="extract SRC DST", help="extract one file instead of listing"),
    ]},
    "bognav": {"kw": {"prog": "bog-nav.py", "formatter_class": "RawDescriptionHelpFormatter",
                      "description": "bog-nav -- navigate a Niagara station bog.\n\nCommands below.",
                      "parents": ["top"]}, "ops": [
        arg("--version", action="tools_version", tool="bog-nav"),
        arg("bog", nargs="?", help="config.bog (ZIP) or a bare file.xml (omit for selftest)"),
        subparsers([
            sub("tree", [arg("--depth", type="int", default=3, help="max depth"),
                         arg("--root", default="/", help="start path"), defaults(run="tree")],
                parents=["common"], help="component tree"),
            sub("find", [arg("type", help="type name, e.g. control:NumericWritable"),
                         defaults(run="find")], parents=["common"],
                help="flat list of components of a type"),
            sub("grep", [arg("regex"), arg("--in", dest="scope", choices=["path", "type", "slot"],
                                           default="path")],
                parents=["common"], help="regex over component paths, types, slot names"),
            sub("selftest", [defaults(run="selftest")], parents=["common"],
                help="run in-memory parser/query assertions (no file needed)"),
        ], dest="cmd", required=True),
    ]},
    "modnav": {"kw": {"prog": "module-nav", "formatter_class": "RawDescriptionHelpFormatter",
                      "usage": "module-nav [-h] [--base-dir DIR] <command> [<args>]",
                      "description": "Module Navigator - CLI for Niagara N4 decompiled modules"},
               "ops": [
        arg("--base-dir", "-d", help="Base directory (auto-detected if omitted)"),
        subparsers([
            sub("class", [arg("name", help="Class name (simple or FQN)"),
                          arg("--members", action="store_true", help="also list members"),
                          defaults(run="class")], help="Show a class"),
            sub("find", [arg("query", nargs="+", help="Free text"),
                         arg("-n", "--limit", type="nav_positive_int", default=20,
                             help="Max classes to show (default 20)"),
                         arg("--depth", choices=["quick", "full"], default="quick",
                             help="audit depth")],
                help="Rank classes for a question",
                description="Rank classes by BM25 over names, members and strings.",
                epilog="examples: find alarm routing | find --depth full servlet"),
            sub("ask", [arg("question", nargs="+")], help="Alias of find for questions"),
        ], dest="command", metavar="<command>", help=S),
        {"op": "set", "attr": "epilog", "value":
         "commands (module-nav <command> --help for its options):\n\n  lookup:\n"
         "    class  Show a class\n    find   Rank classes for a question\n    ask    Alias"},
    ]},
    "nhelp": {"kw": {"prog": "niagara_help", "description": "Niagara N4 Documentation Navigator",
                     "formatter_class": "RawDescriptionHelpFormatter",
                     "epilog": "Examples:\n  %(prog)s class BWebServlet\n"
                               "  %(prog)s guide-search \"discovery timeout\"\n"}, "ops": [
        subparsers([
            sub("class", [arg("name", help="Class name")], help="Show class documentation"),
            sub("guide-search", [
                arg("query", help="Search text (e.g. 'discovery timeout')"),
                arg("-n", "--max-results", type="int", default=10),
                mutex(arg("--and", dest="gs_mode_and", action="store_true", default=False,
                          help="ALL terms must appear in each file (default)"),
                      arg("--or", dest="gs_mode_or", action="store_true", default=False,
                          help="ANY term in file"))], help="Full-text search of the guides"),
            sub("versions", [subparsers([
                sub("add", [arg("install", help="Path of the Niagara install (the folder with modules/)"),
                            arg("--label", help="Human name (default: the install folder name)"),
                            arg("--default", action="store_true", help="Make it the default version")],
                    help="Register a Niagara install, extract and index all of its documentation, "
                         "run both gates and print a summary"),
                sub("list", [], help="List registered versions"),
            ], required=True, dest="action", metavar="{add,list}")], help="Manage Niagara versions"),
        ], dest="command", help="Available commands"),
        arg("--niagara-version", dest="_niagara_version_help", metavar="VERSION",
            help="Niagara version to query (default from versions/registry.json; "
                 "env NIAGARA_HELP_VERSION). Accepted anywhere on the command line."),
    ]},
    "corpus": {"kw": {"prog": "corpus-nav.py", "formatter_class": "RawDescriptionHelpFormatter",
                      "description": "Navigate the niagara-research knowledge corpus (blocks, docs, retros)."},
               "ops": [
        arg("--version", action="tools_version", tool="corpus-nav"),
        subparsers([
            sub("show", [arg("n", type="int", metavar="N")], help="print one block"),
            sub("path", [arg("a", metavar="A"), arg("b", metavar="B")], help="citation path A -> B"),
            sub("lint", [arg("files", nargs="*", metavar="FILE", help="files to lint (default: all)")],
                help="lint blocks"),
            sub("semantic", [arg("query", nargs="+"), arg("--top", type="positive_int", default=10)],
                aliases=["rank"], help="semantic ranking (alias: rank)"),
            sub("quiet", []),
        ], dest="command", metavar="<command>"),
    ]},
    "hyphens": {"kw": {"prog": "hy", "description":
                       "a x-ray b-c-d e-mail re-read co-op self-contained up-to-date 3-d x-y-z "
                       "ab-c a-bc mid-sentence-hyphen --flag-like -- dash-- --lead well-known."}, "ops": [
        arg("--pick", choices=["abcx", "abcy", "zzzz"], help="x-ray a-b ab-cd abc-d e-mail-x"),
    ]},
    "subnodest": {"kw": {"prog": "nodest"}, "ops": [
        subparsers([sub("alpha", help="first"), sub("beta", help="second")], required=True),
    ]},
    "subtitle": {"kw": {"prog": "titled"}, "ops": [
        arg("--verbose", action="store_true"),
        subparsers([sub("run", [arg("target")], help="run it"), sub("stop", help="stop it")],
                   title="commands", description="Available commands.", dest="cmd"),
    ]},
}

HELP_COLUMNS = (100, 80)
EXTRA_COLUMNS = (200, 60, 40, 30, 20)
NARROW = range(14, 40)


def cases():
    """(parser, argv, columns) triples."""
    out = []
    for name in PARSERS:
        for cols in HELP_COLUMNS:
            out.append((name, ["-h"] if name != "noargs" else [], cols))
    for name in ("basic", "wrap", "longprog", "mutex", "groups", "bognav", "nhelp", "unicode",
                 "hyphens"):
        for cols in EXTRA_COLUMNS:
            out.append((name, ["--help"], cols))
    out += [("hyphens", ["-h"], cols) for cols in NARROW]
    c = 100
    out += [(n, a, c) for n, a in [
        ("basic", ["in.bog"]),
        ("basic", ["in.bog", "x.txt", "-n", "5", "--ratio", "1e3", "-v", "-q", "--mode", "fast",
                   "--tag", "a", "--tag=b", "--level", "2", "-ccc", "--const", "--names", "a", "b",
                   "--pair", "k", "v", "--opt", "--maybe", "-xy", "-z", "low", "--extend", "p", "q",
                   "--extend", "r", "--ac", "--ac"]),
        ("basic", ["in.bog", "--maybe", "val", "--opt", "1", "2", "-n=7", "-n8", "--ratio=-2.5"]),
        ("basic", ["in.bog", "--ratio", "inf", "-n", " 1_000 "]),
        ("basic", ["in.bog", "--ratio", "nan", "--hidden", "h"]),
        ("basic", ["--version"]), ("basic", ["in.bog", "--version", "-n", "x"]),
        ("basic", []), ("basic", ["in.bog", "-n", "x"]), ("basic", ["in.bog", "-n", "1.5"]),
        ("basic", ["in.bog", "--ratio", "abc"]), ("basic", ["in.bog", "--ratio", "1_e3"]),
        ("basic", ["in.bog", "--mode", "fats"]), ("basic", ["in.bog", "--mode", "zzz"]),
        ("basic", ["in.bog", "--level", "4"]), ("basic", ["in.bog", "--level", "x"]),
        ("basic", ["in.bog", "-z", "bad"]), ("basic", ["in.bog", "-z", "bad'q\"\n\\"]),
        ("basic", ["in.bog", "out", "extra1", "extra2"]), ("basic", ["in.bog", "--nope"]),
        ("basic", ["in.bog", "--names"]), ("basic", ["in.bog", "--pair", "k"]),
        ("basic", ["in.bog", "-n"]), ("basic", ["in.bog", "-vx=1"]), ("basic", ["in.bog", "-vq1"]),
        ("basic", ["in.bog", "--verbose=yes"]), ("basic", ["in.bog", "--li", "4", "--ver"]),
        ("basic", ["in.bog", "--na", "a", "--r", "2"]), ("basic", ["--", "-in.bog", "-x"]),
        ("basic", ["in.bog", "--", "--"]), ("basic", ["-x", "--", "-n", "5"]),
        ("basic", ["in.bog", "--tag", "-x"]), ("basic", ["in.bog", "--tag", "-5"]),
        ("basic", ["-", "-"]), ("basic", ["in.bog", "-h", "--nope"]),
        ("basic", ["in.bog", "--nope", "-h"]), ("basic", ["in.bog", "-n", "x", "-h"]),
        ("required", []), ("required", ["a"]), ("required", ["a", "-o", "d"]),
        ("required", ["-o", "d", "--fmt", "md", "a", "b"]), ("required", ["--fmt", "xml", "a"]),
        ("mutex", ["q"]), ("mutex", ["q", "--json"]), ("mutex", ["q", "--json", "--csv"]),
        ("mutex", ["q", "--and", "--or", "--json"]), ("mutex", ["q", "--and", "--and", "--csv"]),
        ("mutex", ["--json"]), ("mutex", ["q", "--tsv"]), ("mutex", ["q", "--js"]),
        ("mutex_pos", []), ("mutex_pos", ["t"]), ("mutex_pos", ["--name", "n", "t"]),
        ("groups", ["in", "--wide", "--narrow"]), ("groups", ["in", "p1", "p2", "--min", "3"]),
        ("rawtext", ["--b", "y"]), ("argdefaults", ["p", "--req", "r"]),
        ("searchdoor", ["alarm", "routing"]), ("searchdoor", ["q", "--lim", "3"]),
        ("searchdoor", ["q", "-n", "x"]), ("searchdoor", ["--version"]),
        ("searchdoor", ["q", "--and", "--or"]), ("searchdoor", ["q", "-n3", "--doc-version=4.15"]),
        ("usage", ["--bogus"]), ("noargs", ["x"]), ("longprog", ["only"]),
        ("wrap", ["--opt-1"]), ("negnum", ["-5"]), ("negnum", ["-5", "--x", "-1.5", "-f"]),
        ("negnum", ["--x", "-.5"]), ("negnum", ["-f", "-7"]), ("negnum", ["- 5"]),
        ("negopt", ["-5"]), ("negopt", ["-1"]), ("negopt", ["--", "-5"]),
        ("abbrev", ["--fo", "1"]), ("abbrev", ["--foo", "1"]), ("abbrev", ["--foob"]),
        ("abbrev", ["--z", "q"]), ("abbrev", ["--z=q", "i1", "i2"]), ("abbrev", ["--fo=3"]),
        ("abbrev", ["-n5", "-vw"]), ("abbrev", ["-vn5"]), ("abbrev", ["-wvn", "5"]),
        ("abbrev", ["-n", "-5"]), ("abbrev", ["-vx"]), ("abbrev", ["i", "--", "-v", "--foo"]),
        ("abbrev", ["--foo", "1", "--foo", "2"]), ("abbrev", ["-n="]), ("abbrev", ["--zeta="]),
        ("abbrev", ["-x"]), ("abbrev", ["--x y"]),
        ("unicode", ["--feature", "bacnet:2", "--dueño", "Ñandú"]), ("unicode", ["--due", "x"]),
        ("qnx", []), ("qnx", ["P1", "extract", "a", "b"]), ("qnx", ["P9"]),
        ("bognav", []), ("bognav", ["x.bog"]), ("bognav", ["x.bog", "tree"]),
        ("bognav", ["x.bog", "tree", "--depth", "2", "--json"]),
        ("bognav", ["--json", "x.bog", "find", "control:NumericWritable", "-n", "3"]),
        ("bognav", ["x.bog", "find", "T", "--csv", "--limit", "0"]),
        ("bognav", ["x.bog", "tre"]), ("bognav", ["x.bog", "zzz"]), ("bognav", ["x.bog", "find"]),
        ("bognav", ["x.bog", "tree", "-h"]), ("bognav", ["x.bog", "find", "--help"]),
        ("bognav", ["x.bog", "grep", "-h"]), ("bognav", ["x.bog", "tree", "--bogus"]),
        ("bognav", ["x.bog", "grep", "re", "--in", "slots"]), ("bognav", ["--version"]),
        ("bognav", ["x.bog", "tree", "--depth", "x"]), ("bognav", ["x.bog", "selftest", "extra"]),
        ("modnav", []), ("modnav", ["class", "BComponent"]), ("modnav", ["find", "-h"]),
        ("modnav", ["find", "alarm", "routing", "-n", "0"]), ("modnav", ["find", "a", "-n", "x"]),
        ("modnav", ["find", "a", "--depth", "ful"]), ("modnav", ["bogus"]), ("modnav", ["class"]),
        ("modnav", ["-d", "/x", "ask", "why", "not"]), ("modnav", ["ask", "-h"]),
        ("nhelp", []), ("nhelp", ["class", "-h"]), ("nhelp", ["guide-search", "-h"]),
        ("nhelp", ["guide-search", "q", "--and", "--or"]), ("nhelp", ["guide-search", "q", "--or"]),
        ("nhelp", ["versions"]), ("nhelp", ["versions", "-h"]), ("nhelp", ["versions", "add", "-h"]),
        ("nhelp", ["versions", "add", "/opt/n4", "--default"]), ("nhelp", ["versions", "bogus"]),
        ("nhelp", ["--niagara-version", "4.15", "class", "X"]), ("nhelp", ["clas", "X"]),
        ("corpus", []), ("corpus", ["show", "12"]), ("corpus", ["show", "x"]),
        ("corpus", ["rank", "a", "b"]), ("corpus", ["semantic", "-h"]), ("corpus", ["lint"]),
        ("corpus", ["path", "1"]), ("corpus", ["quiet"]), ("corpus", ["semantik", "q"]),
        ("corpus", ["--version"]), ("corpus", ["rank", "q", "--top", "0"]),
        ("hyphens", ["--pick", "abcz"]), ("hyphens", ["--pick", "abc"]), ("hyphens", ["--pick", "zz"]),
        ("subnodest", []), ("subnodest", ["gamma"]), ("subnodest", ["alpha", "-h"]),
        ("subtitle", ["run"]), ("subtitle", ["run", "t", "--verbose"]),
        ("subtitle", ["--verbose", "stop"]),
    ]]
    return out
