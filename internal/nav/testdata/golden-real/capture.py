"""Re-capture the module-navigator keep-set golden cases from the frozen Python CLI.

    MODULE_NAVIGATOR_DIR=~/niagara-research/module-navigator \
        python3 -I internal/nav/testdata/golden-real/capture.py GO_MNAV_BIN

Runs every case of module-navigator tests/golden_cli.py whose command the Go
port keeps (decision D32: its CASES and help-<command> ids), plus help,
exactly as that harness does (run_isolated: cwd the checkout, a scratch
--base-dir linking indexes/, COLUMNS=100, PYTHONHASHSEED=0, its
normalization). Each case's sha256(stdout), sha256(stderr) and exit code must
equal tests/golden/cli.json, or nothing is written. help also gets
intended.json with the output of GO_MNAV_BIN run through the same harness.
On success every case directory next to this script is replaced.
"""

import hashlib
import importlib
import json
import os
import shutil
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
NAV = os.environ.get("MODULE_NAVIGATOR_DIR") or os.path.expanduser("~/niagara-research/module-navigator")
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


def capture(out, gobin):
    """Write every kept case into out; return the ids whose hashes differ from cli.json."""
    os.environ.pop("GOLDEN_BIN", None)
    sys.path.insert(0, os.path.join(NAV, "tests"))
    h = importlib.import_module("golden_cli")
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
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
