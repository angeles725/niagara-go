"""Regenerate the palette-lexicon-agents cases from the niagara-research tool (D39 e).

    python3 -I internal/nav/testdata/gen_palette_lexicon.py [--real GO_MNAV_BIN]

`mnav palette-lexicon-agents` follows niagara-research tools/palette-lexicon-agents.py
(the superset twin, docs/tools-compatibility-contract.md section 6), so that script is
the reference here, not module-navigator's palette_lexicon_agents.py.

Without --real it rebuilds the synthetic organized/ trees under
internal/nav/audits/testdata/palette/ and writes one golden/<case>/ per CASES entry
from the script's output (cwd that directory, --base-dir moved after the command as
the script takes it). Every case's args are the mnav command line. The pla-* cases
read gen_golden.py's fixture/pla (malformed XML, 55 entries, agent edge cases).

--real also writes the real-corpus cases next to internal/nav/testdata/golden-real/
capture.py (which runs this after its own capture), run as module-navigator
tests/golden_cli.py runs a case (cwd the checkout, "{tmp}" a scratch base linking
indexes/, COLUMNS=100, its normalization): the REAL cases and the kept
palette-lexicon-agents case get the script's output. help-palette-lexicon-agents
keeps module_nav.py's bytes and gets an intended.json with GO_MNAV_BIN's help: the
script's own --help names another program and takes --base-dir itself, so it is no
reference. The real cases count corpus modules: re-run --real after a corpus change.

Code and data (D47, niagara-go internal/pyref): the script is niagara-research's
pre-go-retire worktree ($NIAGARA_RESEARCH_PY_REF), whose __main__ is already the
exec shim of the Go mnav, so it runs through FROZEN (the pre-switch source, as in
internal/bogcycles/testdata/capture.py); the harness is module-navigator's
pre-go-retire worktree ($MODULE_NAVIGATOR_PY_REF).
The data stays live: the real cases run in MODULE_NAVIGATOR_DIR (../organized is
the corpus) and the harness links its indexes/ (harness()).
"""

import importlib
import json
import os
import shutil
import subprocess
import sys
import warnings
import zipfile

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.expanduser("~/niagara-research")
NAV = os.environ.get("MODULE_NAVIGATOR_DIR") or os.path.join(REPO, "module-navigator")


def py_ref(repo, env, rev):  # D47: a clean worktree of <repo> tag pre-go-retire (docs/goldens.md)
    root = os.environ.get(env) or os.path.expanduser("~/niagara-research-worktrees/pre-go-retire-" + repo)
    git = lambda *a: subprocess.run(["git", "-C", root, *a], capture_output=True, text=True).stdout.strip()  # noqa: E731
    if git("rev-parse", "HEAD") != rev or git("status", "--porcelain"):
        sys.exit(f"{env}={root}: not a clean worktree of {repo} tag pre-go-retire ({rev[:9]}); see docs/goldens.md")
    return root


PYREF = py_ref("niagara-research", "NIAGARA_RESEARCH_PY_REF", "c846ce1e5eb92ee6d938efbb3585473a545f7e50")
NAV_PYREF = py_ref("module-navigator", "MODULE_NAVIGATOR_PY_REF", "f897268e900138e08a82bf1b389a1715c1be26e1")
os.environ["MODULE_NAV_PYTHON"] = "1"  # a tag shim runs its Python, never Go
TOOL = os.path.join(PYREF, "tools", "palette-lexicon-agents.py")

# At the tag, running tools/<x>.py execs the Go port (its D35 switch block); the Python under that block
# is the frozen specification. `python3 [-I] -c FROZEN <tool> <args>` runs it like `python3 [-I] <tool>
# <args>` did before the switch: the block is cut out (giving back the 06b317d7b^ code; only the module
# docstring, 3 lines shorter at the tag, differs), the code runs as __main__ under the tool's own path, and
# a traceback skips this frame. Copied from internal/bogcycles/testdata/capture.py.
FROZEN = r"""
import linecache, os, sys, types
del sys.argv[0]
tool = sys.argv[0]
with open(tool, encoding="utf-8") as fh:
    lines = fh.read().splitlines(True)
i = next(n for n, line in enumerate(lines) if line.startswith('if __name__ == "__main__":  # Go switch (D35)'))
j = i + 1
while j < len(lines) and lines[j][:1] in (" ", "\t"):
    j += 1
if j < len(lines) and not lines[j].strip():
    j += 1
elif i and not lines[i - 1].strip():
    i -= 1
del lines[i:j]
linecache.cache[tool] = (None, None, lines, tool)
if not getattr(sys.flags, "safe_path", False):  # -P / -I (Python 3.11+)
    sys.path[0] = os.path.dirname(tool)
mod = sys.modules["__main__"] = types.ModuleType("__main__")
mod.__file__ = tool
try:
    exec(compile("".join(lines), tool, "exec"), mod.__dict__)
except SystemExit:
    raise
except BaseException as exc:
    exc = exc.with_traceback(exc.__traceback__.tb_next)
    sys.excepthook(type(exc), exc, exc.__traceback__)
    sys.exit(1)
"""
PALETTE = os.path.join(HERE, "..", "audits", "testdata", "palette")
REAL = os.path.join(HERE, "golden-real")
CMD = "palette-lexicon-agents"
DATE = (2026, 1, 2, 3, 4, 6)
PLA = "../../../testdata/fixture/pla"  # gen_golden.py --pla builds it

# (case id, mnav args). Paths are relative to PALETTE.
CASES = [
    ("multi", ["--base-dir", "tree", CMD, "multi"]),
    ("multi-json", ["--base-dir", "tree", CMD, "multi", "--json"]),
    ("jar", ["--base-dir", "tree", CMD, "jar"]),
    ("jar-json", ["--base-dir", "tree", CMD, "jar", "--json"]),
    ("mixed", ["--base-dir", "tree", CMD, "mixed"]),
    ("mixed-json", ["--base-dir", "tree", CMD, "mixed", "--json"]),
    ("unknown", ["--base-dir", "tree", CMD, "nope"]),
    ("unknown-json", ["--base-dir", "tree", CMD, "nope", "--json"]),
    ("no-artifacts", ["--base-dir", "tree", CMD, "bare"]),
    ("all", ["--base-dir", "tree", CMD, "--all"]),
    ("all-json", ["--base-dir", "tree", CMD, "--all", "--json"]),
    ("all-module", ["--base-dir", "tree", CMD, "multi", "--all"]),
    ("all-empty", ["--base-dir", "tree-empty", CMD, "--all"]),
    ("all-empty-json", ["--base-dir", "tree-empty", CMD, "--all", "--json"]),
    ("base-missing", ["--base-dir", "no-such-dir", CMD, "multi"]),
    ("pla-pal", ["--base-dir", PLA, CMD, "pal"]),
    ("pla-pal-json", ["--base-dir", PLA, CMD, "pal", "--json"]),
    ("pla-empty", ["--base-dir", PLA, CMD, "empty"]),
    ("pla-empty-json", ["--base-dir", PLA, CMD, "empty", "--json"]),
    ("pla-missing", ["--base-dir", PLA, CMD, "nope"]),
    ("pla-missing-json", ["--base-dir", PLA, CMD, "nope", "--json"]),
    ("pla-all", ["--base-dir", PLA, CMD, "--all"]),
]

# Real-corpus cases: module-navigator's checkout is the cwd, so ../organized is the corpus.
REAL_CASES = [
    ("palette-lexicon-agents-lexicons", ["--base-dir", "../organized", CMD, "niagaraLexiconEs"]),
    ("palette-lexicon-agents-nested", ["--base-dir", "../organized", CMD, "easyBinding", "--json"]),
    ("palette-lexicon-agents-unknown", ["--base-dir", "../organized", CMD, "noSuchModule"]),
    ("palette-lexicon-agents-all", ["--base-dir", "../organized", CMD, "--all"]),
    (CMD, ["--base-dir", "{tmp}", CMD, "alarm", "--json"]),
]
INTENDED_REASON = ("D39 e: palette-lexicon-agents follows niagara-research tools/"
                   "palette-lexicon-agents.py (every **/*.lexicon, lexicon JAR fallback, "
                   "exit 1 naming the searched base dir; docs/tools-compatibility-contract.md "
                   "section 6)")


def write(path, text, mode="w"):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, mode, **({} if "b" in mode else {"encoding": "utf-8", "newline": ""})) as f:
        f.write(text)


def jar(path, members):
    """A JAR holding members ((name, text) pairs, a repeated name kept twice)."""
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with zipfile.ZipFile(path, "w") as zf, warnings.catch_warnings():
        warnings.simplefilter("ignore")  # the repeated name is on purpose
        for name, text in members:
            zf.writestr(zipfile.ZipInfo(name, date_time=DATE), text)


def build_trees():
    """tree: module multi (extracted lexicons at several depths, one named like the
    artifact but nested), jar (JAR-only, a repeated lexicon member), mixed (an extracted
    lexicon hides the JAR's; an extracted/ without one falls back to the JAR), bare (no
    artifact), a plain file; tree-empty: no module with an artifact."""
    shutil.rmtree(PALETTE, ignore_errors=True)
    t = os.path.join(PALETTE, "tree")
    ext = os.path.join(t, "multi", "multi-rt", "extracted")
    write(os.path.join(ext, "module.palette"), '<a><p n="Net" t="m:Net" m="m=multi"/></a>')
    write(os.path.join(ext, "META-INF", "module.xml"),
          '<module><types><type name="Net" class="com.m.BNet"><agent><on type="m:A"/></agent>'
          '</type></types></module>')
    write(os.path.join(ext, "multi-rt.lexicon"), "# own\nk=1\nk=2\nonly=3\n")
    write(os.path.join(ext, "b.lexicon"), "k=1\nk=2\nk=3\n")
    write(os.path.join(ext, "Zed.lexicon"), "z = 1\r\nz=2\r\n")
    write(os.path.join(ext, "sub", "dir", "c.lexicon"), "c=1\n\nc=2\nnoequals\n=x\n")
    write(os.path.join(ext, "sub", "multi-rt.lexicon"), "k=1\nk=1\n")
    write(os.path.join(ext, "notes.lexicon.txt"), "k=1\nk=2\n")
    write(os.path.join(ext, "café.lexicon"), "é=1\né=2\n")
    write(os.path.join(t, "multi", "multi-wb", "extracted", "multi-wb.lexicon"), "w=1\n")
    jar(os.path.join(t, "jar", "jar-rt", "jar-rt.jar"),
        [("module.palette", '<a><p n="J"/></a>'), ("jar-rt.lexicon", "a=1\na=2\nb=1\n"),
         ("other/x.lexicon", "x=1\n"), ("other/x.lexicon", "x=1\nx=2\nx=3\n"),
         ("res/", ""), ("README", "a=1\na=2\n")])
    write(os.path.join(t, "mixed", "mixed-rt", "extracted", "mixed-rt.lexicon"), "e=1\n")
    jar(os.path.join(t, "mixed", "mixed-rt", "mixed-rt.jar"), [("mixed-rt.lexicon", "j=1\nj=2\n")])
    write(os.path.join(t, "mixed", "mixed-ux", "extracted", "module.palette"), "<a/>")
    jar(os.path.join(t, "mixed", "mixed-ux", "mixed-ux.jar"),
        [("mixed-ux.lexicon", "u=1\nu=2\n"), ("deep/y.lexicon", "y=1\n")])
    write(os.path.join(t, "bare", "notes", "x.txt"), "no artifact\n")
    write(os.path.join(t, "README"), "not a module\n")
    write(os.path.join(PALETTE, "tree-empty", "bare", "notes", "x.txt"), "no artifact\n")


def tool_args(args):
    """The script's argv for an mnav command line: --base-dir moves after the command."""
    assert args[0] == "--base-dir" and args[2] == CMD, args
    return args[3:] + ["--base-dir", args[1]]


def write_case(d, args, rc, stdout, stderr, mode="w"):
    shutil.rmtree(d, ignore_errors=True)
    write(os.path.join(d, "cmd.json"), json.dumps({"args": args}, ensure_ascii=False) + "\n")
    write(os.path.join(d, "stdout"), stdout, mode)
    write(os.path.join(d, "stderr"), stderr, mode)
    write(os.path.join(d, "exit"), "%d\n" % rc)


def capture_hermetic():
    env = dict(os.environ, PYTHONHASHSEED="0", COLUMNS="100")
    for cid, args in CASES:
        p = subprocess.run([sys.executable, "-I", "-c", FROZEN, TOOL] + tool_args(args), cwd=PALETTE,
                           env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        write_case(os.path.join(PALETTE, "golden", cid), args, p.returncode, p.stdout, p.stderr, "wb")


def harness():
    """module-navigator's tests/golden_cli.py from NAV_PYREF reading the live data: its
    BASE_DIR is NAV while scratch_base and run_dir run (indexes/, niagara-help/)."""
    sys.path.insert(0, os.path.join(NAV_PYREF, "tests"))
    h = importlib.import_module("golden_cli")
    os.environ.setdefault("NAV_CORPUS_BASE", os.path.join(os.path.dirname(NAV), "organized"))

    def on_data(f):
        def run(*args):
            code, h.BASE_DIR = h.BASE_DIR, NAV
            try:
                return f(*args)
            finally:
                h.BASE_DIR = code
        return run
    h.scratch_base, h.run_dir = on_data(h.scratch_base), on_data(h.run_dir)
    return h


def capture_real(gobin):
    h = harness()
    run = h.run_dir()
    env = dict(os.environ, PYTHONHASHSEED="0", COLUMNS="100")

    def tool(args, base):
        argv = [a.replace("{tmp}", base) for a in tool_args(args)]
        p = subprocess.run([sys.executable, "-I", "-c", FROZEN, TOOL] + argv, cwd=NAV, env=env,
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
                           timeout=h.TIMEOUT)
        # <NAV> is the checkout the case ran in (the harness's BASE_DIR is the tag's)
        return (p.returncode, h.normalize(p.stdout.replace(NAV, "<NAV>"), base),
                h.normalize(p.stderr.replace(NAV, "<NAV>"), base, h._NORMALIZE_STDERR))

    try:
        for cid, args in REAL_CASES:
            base = h.scratch_base(run)
            rc, so, se = tool(args, base)
            write_case(os.path.join(REAL, cid), args, rc, so, se)
            shutil.rmtree(base, ignore_errors=True)
        os.environ["GOLDEN_BIN"] = os.path.abspath(gobin)
        try:
            rc, so, se = h.run_isolated([CMD, "--help"], run)
        finally:
            os.environ.pop("GOLDEN_BIN", None)
        intended(os.path.join(REAL, "help-" + CMD), rc, so, se)
    finally:
        shutil.rmtree(run, ignore_errors=True)


def intended(d, rc, stdout, stderr):
    body = {"reason": INTENDED_REASON, "stdout": stdout, "stderr": stderr, "exit": rc}
    write(os.path.join(d, "intended.json"),
          json.dumps(body, ensure_ascii=False, indent=1, sort_keys=True) + "\n")


def main(argv):
    if argv and (argv[0] != "--real" or len(argv) != 2):
        print(__doc__, file=sys.stderr)
        return 2
    build_trees()
    capture_hermetic()
    if argv:
        capture_real(argv[1])
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
