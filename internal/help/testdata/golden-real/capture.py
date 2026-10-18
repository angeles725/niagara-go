"""Re-capture the niagara-help keep-set golden cases from the frozen Python CLI.

    NIAGARA_HELP_DIR=~/niagara-research/niagara-help \
        python3 -I internal/help/testdata/golden-real/capture.py GO_NH_BIN

Runs every case of niagara-help tests/golden_cli.py whose command the Go port
keeps (decision D32), plus help and no-command, exactly as that harness does
(cwd the main checkout, COLUMNS=100, PYTHONHASHSEED=0, its normalization) with
NIAGARA_HELP_PYTHON=1 so tools/niagara_help.py runs Python, never the Go shim.
Two roles (D47, niagara-go internal/pyref): the code (the harness, its
tests/golden/cli.json and tools/niagara_help.py) is the pre-go-retire worktree,
$NIAGARA_HELP_PY_REF; the data stays NIAGARA_HELP_DIR, the live checkout (its
fulltext.sqlite and versions/ trees are not in the tag), see harness().
Each case's sha256(stdout), sha256(stderr) and exit code must equal
tests/golden/cli.json, or nothing is written. The INTENDED cases (help and
no-command: D32; stats and stats-json: call-index retired, I3-5) also get
intended.json with the output of GO_NH_BIN run through the same harness.
On success every case directory next to this script is replaced.
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
REPO = os.environ.get("NIAGARA_HELP_DIR") or os.path.expanduser("~/niagara-research/niagara-help")


def py_ref(repo, env, rev):  # D47: a clean worktree of <repo> tag pre-go-retire (docs/goldens.md)
    root = os.environ.get(env) or os.path.expanduser("~/niagara-research-worktrees/pre-go-retire-" + repo)
    git = lambda *a: subprocess.run(["git", "-C", root, *a], capture_output=True, text=True).stdout.strip()  # noqa: E731
    if git("rev-parse", "HEAD") != rev or git("status", "--porcelain"):
        sys.exit(f"{env}={root}: not a clean worktree of {repo} tag pre-go-retire ({rev[:9]}); see docs/goldens.md")
    return root


PYREF = py_ref("niagara-help", "NIAGARA_HELP_PY_REF", "8410d74511673e38b87622f7214fa5210ba25a63")
os.environ["NIAGARA_HELP_PYTHON"] = "1"  # a tag shim runs its Python, never Go
KEEP = ["class", "search", "package", "method", "slots", "guide", "guide-search", "devguide",
        "devguide-search", "pdf", "find", "source", "source-grep", "stats", "freshness",
        "selftest", "versions"]
D32 = ("D32: the Go port lists only the kept commands; retired commands print one "
       "'retired' line (docs/retired-commands.md)")
I3_5 = ("call-index retired (I3-5): the Go port neither builds nor reads call-index.json, "
        "so stats counts 9 indexes and has no CALL INDEX block or call-index entry, "
        "even when a stale file is on disk")
INTENDED = {"help": D32, "no-command": D32, "stats": I3_5, "stats-json": I3_5}


def sha(text):
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def command_word(argv):
    words = [a for a in argv if not a.startswith("-")]
    return words[0] if words else None


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
    """The tag's tests/golden_cli.py on the live data: its BASE_DIR (the cwd and the <REPO>
    normalization) becomes REPO, and the tag's Python CLI gets NIAGARA_HELP_DIR=REPO, which
    run_case removes from the environment (a GOLDEN_BIN port runs unchanged)."""
    sys.path.insert(0, os.path.join(PYREF, "tests"))
    h = importlib.import_module("golden_cli")
    h.BASE_DIR, cli = REPO, h.cli_command
    h.cli_command = lambda: cli() if os.environ.get("GOLDEN_BIN") else ["env", "NIAGARA_HELP_DIR=" + REPO] + cli()
    return h


def capture(out, gobin):
    """Write every kept case into out; return the ids whose hashes differ from cli.json."""
    os.environ.pop("GOLDEN_BIN", None)
    h = harness()
    golden = h.load_golden()
    cases = [c for c in h.CASES
             if c[0] in INTENDED or (command_word(c[1]) in KEEP and c[2] is None)]
    tmp = tempfile.mkdtemp(prefix="nh-golden-")
    bad = []
    try:
        for c in cases:
            cid, argv, _ = c
            _, rc, so, se, written = h.run_case(c, tmp)
            rec = {"rc": rc, "stdout": sha(so), "stderr": sha(se)}
            rec.update({"file:" + n: sha(b) for n, b in written.items()})
            if rec != golden.get(cid):
                bad.append(cid)
            args = [a.replace(h.TMP, "{tmp}") for a in argv]
            if cid != "bad-version":  # the harness's one case without --niagara-version
                args = ["--niagara-version", h.VERSION] + args
            intended = None
            if cid in INTENDED:
                os.environ["GOLDEN_BIN"] = gobin
                try:
                    _, grc, gso, gse, _ = h.run_case(c, tmp)
                finally:
                    os.environ.pop("GOLDEN_BIN", None)
                intended = {"reason": INTENDED[cid], "stdout": gso, "stderr": gse, "exit": grc}
            write_case(out, cid, args, rc, so, se, intended)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    print("niagara-help: {} cases, {} differ from cli.json {}".format(len(cases), len(bad), bad))
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
    out = tempfile.mkdtemp(prefix="nh-golden-real-")
    try:
        if capture(out, os.path.abspath(argv[0])):
            return 1
        replace_cases(out)
    finally:
        shutil.rmtree(out, ignore_errors=True)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
