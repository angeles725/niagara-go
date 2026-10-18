"""Re-capture the module-navigator keep-set golden cases from the frozen Python CLI.

    MODULE_NAVIGATOR_DIR=~/niagara-research/module-navigator \
        python3 -I internal/nav/testdata/golden-real/capture.py GO_MNAV_BIN

Runs every case of module-navigator tests/golden_cli.py whose command the Go
port keeps (decision D32: its CASES and help-<command> ids), plus help,
exactly as that harness does (run_isolated: cwd the checkout, a scratch
--base-dir linking indexes/, COLUMNS=100, PYTHONHASHSEED=0, its
normalization). Two roles (D47, niagara-go internal/pyref): the code (the
harness, module_nav.py run with MODULE_NAV_PYTHON=1, the checkout it runs in
and its tests/golden/) is the pre-go-retire worktree, $MODULE_NAVIGATOR_PY_REF;
the data stays live: MODULE_NAVIGATOR_DIR's indexes/, its sibling niagara-help/
and its sibling organized/ ($NAV_CORPUS_BASE unless set), see harness().
Each case's sha256(stdout), sha256(stderr) and exit code must equal the tag's
tests/golden/cli.json, or nothing is written. help also gets
intended.json with the output of GO_MNAV_BIN run through the same harness.
On success every case directory next to this script is replaced, then
../gen_palette_lexicon.py --real GO_MNAV_BIN re-captures the palette-lexicon-agents
cases from niagara-research's pre-go-retire tools/palette-lexicon-agents.py (D39 e), which that
command follows instead of module_nav.py.
"""

import hashlib
import importlib
import json
import os
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
NAV = os.environ.get("MODULE_NAVIGATOR_DIR") or os.path.expanduser("~/niagara-research/module-navigator")


def py_ref(repo, env, rev):  # D47: a clean worktree of <repo> tag pre-go-retire (docs/goldens.md)
    root = os.environ.get(env) or os.path.expanduser("~/niagara-research-worktrees/pre-go-retire-" + repo)
    git = lambda *a: subprocess.run(["git", "-C", root, *a], capture_output=True, text=True).stdout.strip()  # noqa: E731
    if git("rev-parse", "HEAD") != rev or git("status", "--porcelain"):
        sys.exit(f"{env}={root}: not a clean worktree of {repo} tag pre-go-retire ({rev[:9]}); see docs/goldens.md")
    return root


PYREF = py_ref("module-navigator", "MODULE_NAVIGATOR_PY_REF", "f897268e900138e08a82bf1b389a1715c1be26e1")
os.environ["MODULE_NAV_PYTHON"] = "1"  # a tag shim runs its Python, never Go
KEEP = ["class", "search", "inventory", "module", "modules", "rank", "ask", "hierarchy",
        "implementors", "drivers", "grep", "source", "token", "license-feature",
        "license-features", "resources", "callers", "callees", "stats", "palette-lexicon-agents",
        "selftest"]
REASON = ("D32: the Go port lists only the kept commands; retired commands print one "
          "'retired' line (docs/retired-commands.md)")


def sha(text):
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def write_case(out, cid, args, rc, stdout, stderr, intended=None):
    d = os.path.join(out, cid)
    os.makedirs(d)
    with open(os.path.join(d, "cmd.json"), "w", encoding="utf-8") as f:
        json.dump({"args": args}, f, ensure_ascii=False)
        f.write("\n")
    for name, body in (("stdout", stdout), ("stderr", stderr)):
        with open(os.path.join(d, name), "w", encoding="utf-8", newline="") as f:
            f.write(body)
    with open(os.path.join(d, "exit"), "w", encoding="utf-8") as f:
        f.write("%d\n" % rc)
    if intended is not None:
        with open(os.path.join(d, "intended.json"), "w", encoding="utf-8") as f:
            json.dump(intended, f, ensure_ascii=False, indent=1, sort_keys=True)
            f.write("\n")


def harness():
    """The tag's tests/golden_cli.py reading the live data. Its BASE_DIR stays the tag (the CLI,
    the cwd, tests/golden/ and the <NAV> normalization) except while scratch_base and run_dir
    run: they link NAV/indexes and NAV's niagara-help/ sibling. The corpus is NAV's organized/
    sibling, the default of the live checkout, which the tag worktree has none of."""
    sys.path.insert(0, os.path.join(PYREF, "tests"))
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


def capture(out, gobin):
    """Write every kept case into out; return the ids whose hashes differ from cli.json."""
    os.environ.pop("GOLDEN_BIN", None)
    h = harness()
    with open(h.GOLDEN, encoding="utf-8") as f:
        golden = json.load(f)
    cases = [c for c in h.CASES if c[0] == "help" or c[1][0] in KEEP]
    cases += [c for c in h.subcommand_help_cases(h.subcommands()) if c[1][0] in KEEP]
    run = h.run_dir()
    bad = []
    try:
        for c in cases:
            cid, argv = c[0], c[1]
            assert len(c) == 2, cid  # no kept case reads stdin
            rc, so, se = h.run_isolated(argv, run)
            g = golden.get(cid) or {}
            if (rc, sha(so), sha(se)) != (g.get("rc"), g.get("sha256"), g.get("stderr_sha256")):
                bad.append(cid)
            intended = None
            if cid == "help":
                os.environ["GOLDEN_BIN"] = gobin
                try:
                    grc, gso, gse = h.run_isolated(argv, run)
                finally:
                    os.environ.pop("GOLDEN_BIN", None)
                intended = {"reason": REASON, "stdout": gso, "stderr": gse, "exit": grc}
            write_case(out, cid, ["--base-dir", "{tmp}"] + list(argv), rc, so, se, intended)
    finally:
        shutil.rmtree(run, ignore_errors=True)
    print("module-navigator: {} cases, {} differ from cli.json {}".format(len(cases), len(bad), bad))
    return bad


def replace_cases(src):
    for name in os.listdir(HERE):
        if os.path.isdir(os.path.join(HERE, name)):
            shutil.rmtree(os.path.join(HERE, name))
    for name in os.listdir(src):
        shutil.move(os.path.join(src, name), os.path.join(HERE, name))


def main(argv):
    if len(argv) != 1:
        print(__doc__, file=sys.stderr)
        return 2
    out = tempfile.mkdtemp(prefix="mnav-golden-real-")
    try:
        if capture(out, os.path.abspath(argv[0])):
            return 1
        replace_cases(out)
    finally:
        shutil.rmtree(out, ignore_errors=True)
    gen = os.path.join(os.path.dirname(HERE), "gen_palette_lexicon.py")
    return subprocess.run([sys.executable, "-I", gen, "--real", os.path.abspath(argv[0])]).returncode


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
