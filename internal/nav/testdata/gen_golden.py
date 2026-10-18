"""Regenerate the fixture-backed golden cases of the ported mnav commands.

    python3 -I internal/nav/testdata/gen_golden.py [--fixture | --search]

--fixture first rebuilds every synthetic fixture under fixture/ (fixture/nav/, a small class
index, module inventory, xref and inheritance index cut once from the real module-navigator
indexes, is frozen committed input and never rebuilt), and --search rebuilds only the synthetic
grep/source/token corpus; then every case is run from this directory through the frozen
module_nav.py of the pre-go-retire worktree (PYREF; MODULE_NAV_PYTHON=1, COLUMNS=100, NO_COLOR=1)
and written to golden/<case>/: nav-<case> runs on the fixture, and the parser-only PARSE_CASES.

The cases are in gen_golden_cases.py, the source-corpus fixtures (drivers, search, lic) in
gen_golden_corpus.py and the index fixtures (nav, cg, stats, pla, recall) in gen_golden_indexes.py.
A re-capture rewrites the keys of cmd.json it owns (args, env) and keeps any other key written by
hand (e.g. "requires"), in place.
"""

import glob
import json
import os
import shutil
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
NAV = os.path.expanduser("~/niagara-research/module-navigator")  # where the cases were frozen (unpin)
sys.path.insert(0, HERE)  # python3 -I leaves the script directory off sys.path
sys.dont_write_bytecode = True  # no __pycache__ next to the generator

from gen_golden_cases import BASE, CASES, INTENDED, PARSE_CASES  # noqa: E402
from gen_golden_corpus import build_drivers, build_lic, build_search, fresh_class_index  # noqa: E402
from gen_golden_indexes import build_cg, build_nav_synthetic, build_pla, build_recall, build_stats  # noqa: E402


def py_ref(repo, env, rev):  # D47: a clean worktree of <repo> tag pre-go-retire (docs/goldens.md)
    root = os.environ.get(env) or os.path.expanduser("~/niagara-research-worktrees/pre-go-retire-" + repo)
    git = lambda *a: subprocess.run(["git", "-C", root, *a], capture_output=True, text=True).stdout.strip()  # noqa: E731
    if git("rev-parse", "HEAD") != rev or git("status", "--porcelain"):
        sys.exit(f"{env}={root}: not a clean worktree of {repo} tag pre-go-retire ({rev[:9]}); see docs/goldens.md")
    return root


PYREF = os.path.abspath(py_ref("module-navigator", "MODULE_NAVIGATOR_PY_REF", "f897268e900138e08a82bf1b389a1715c1be26e1"))
CLI = os.path.join(PYREF, "tools", "module_nav.py")  # pyref (niagara-go internal/pyref, D47)
OWNED = ("args", "env")  # the cmd.json keys a capture writes; any other key is kept


def cmd_json(path, argv, env):
    """cmd.json of a capture: the owned keys from this run, every other key of
    the existing file kept, in the existing order (new owned keys last)."""
    new = {"args": argv, "env": env} if env else {"args": argv}
    old = {}
    if os.path.isfile(path):
        with open(path, encoding="utf-8") as f:
            old = json.load(f)
    out = {}
    for k, v in old.items():
        if k not in OWNED:
            out[k] = v
        elif k in new:
            out[k] = new[k]
    for k in OWNED:
        if k in new and k not in out:
            out[k] = new[k]
    return out


def unpin(data):
    """A traceback names the tag worktree's files; write them under the live
    checkout, where the Python ran when the cases were frozen, so a re-run from
    the tag keeps the committed bytes."""
    return data.replace(os.fsencode(PYREF + os.sep), os.fsencode(NAV + os.sep))


def capture(cid, args, env, base=BASE):
    argv = ["--base-dir", base] + args
    full_env = dict(os.environ, MODULE_NAV_PYTHON="1", COLUMNS="100", NO_COLOR="1", **env)
    p = subprocess.run([sys.executable, CLI] + argv, cwd=HERE, env=full_env,
                       stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    d = os.path.join(HERE, "golden", cid)
    kept = None  # an intended.json written by hand (e.g. a D32 retired command) survives a re-capture
    if os.path.isfile(os.path.join(d, "intended.json")):
        with open(os.path.join(d, "intended.json"), encoding="utf-8") as f:
            kept = f.read()
    cmd = cmd_json(os.path.join(d, "cmd.json"), argv, env)
    shutil.rmtree(d, ignore_errors=True)
    os.makedirs(d)
    if kept is not None and cid[len("nav-"):] not in INTENDED:
        with open(os.path.join(d, "intended.json"), "w", encoding="utf-8") as f:
            f.write(kept)
    with open(os.path.join(d, "cmd.json"), "w", encoding="utf-8") as f:
        json.dump(cmd, f)
        f.write("\n")
    for name, data in (("stdout", p.stdout), ("stderr", p.stderr)):
        if data or name == "stdout":
            with open(os.path.join(d, name), "wb") as f:
                f.write(unpin(data))
    with open(os.path.join(d, "exit"), "w") as f:
        f.write("%d\n" % p.returncode)
    if cid[len("nav-"):] in INTENDED:
        with open(os.path.join(d, "intended.json"), "w", encoding="utf-8") as f:
            json.dump(INTENDED[cid[len("nav-"):]], f, indent=1)
            f.write("\n")


def main():
    """--fixture rebuilds every synthetic fixture; --search only the grep/source/token one,
    --lic only the license/resources one, --cg only the call graph one, --stats
    only the stats ones, --pla only the palette-lexicon-agents one, --recall
    only the selftest one.
    --only a,b captures just those case ids (without the nav- prefix)."""
    if "--fixture" in sys.argv:
        build_nav_synthetic()
        build_drivers()
    if "--fixture" in sys.argv or "--search" in sys.argv:
        build_search()
    if "--fixture" in sys.argv or "--lic" in sys.argv:
        build_lic()
    if "--fixture" in sys.argv or "--cg" in sys.argv:
        build_cg()
    if "--fixture" in sys.argv or "--stats" in sys.argv:
        build_stats()
    if "--fixture" in sys.argv or "--pla" in sys.argv:
        build_pla()
    if "--fixture" in sys.argv or "--recall" in sys.argv:
        build_recall()
    only = set(sys.argv[sys.argv.index("--only") + 1].split(",")) if "--only" in sys.argv else None
    fresh_class_index()
    for cid, args, env, *base in CASES:
        if only is None or cid in only:
            capture("nav-" + cid, args, env, *base)
    if only is not None:
        return
    for cache in glob.glob(os.path.join(HERE, "fixture", "*", "indexes", "class-rank.cache")):
        os.remove(cache)  # Python's rank cache; the Go port keeps its own
    for cid, args in PARSE_CASES:
        capture(cid, args, {}, base="/nonexistent-base")


if __name__ == "__main__":
    main()
