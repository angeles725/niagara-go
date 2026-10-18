#!/usr/bin/env python3
"""Convert the frozen nsearch/find3 CLI goldens of niagara-research into this package's D7 cases.

  python3 -I internal/searchdoor/testdata/convert_goldens.py

The goldens are read from niagara-research tag pre-go-retire (NIAGARA_RESEARCH_PY_REF names its worktree,
D47 in docs/goldens.md); the Phase 5 retire deleted them from the live checkout.

Reads tools/tests/fixtures/golden-cli/{nsearch,nsearch-old-nav,nsearch-missing,nsearch-timeouts,find3}.json
(frozen from the Python tools by tools/tests/test_golden_nsearch.py) and writes, under this directory:
  fake/, course/, empty-course/      copies of the fake backends and the course tree (the case cwd)
  golden/<case>/                     nsearch cases (cwd: this directory)
  find3/golden/<case>/               find3 cases (cwd_fixture "..")
  timeouts/golden/<case>/            nsearch --timeout cases (cwd_fixture ".."; skipped by go test -short)
Each case gets the env the Python test used. Rerunning it must leave `git status` clean.
"""
import json
import os
import re
import shlex
import shutil
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ENV = {"COLUMNS": "80", "PYTHON_COLORS": "0", "GIT_DIR": "/nonexistent", "PYTHONIOENCODING": "utf-8",
       "PYTHONHASHSEED": "0", "NSEARCH_CORPUS_NAV": "fake/corpus-nav.py",
       "NSEARCH_NIAGARA_HELP": "fake/niagara_help.py", "NSEARCH_MODULE_NAV": "fake/module_nav.py",
       "NSEARCH_COURSE_ROOT": "course"}
MISSING = dict(ENV, NSEARCH_CORPUS_NAV="missing/corpus-nav.py", NSEARCH_NIAGARA_HELP="missing/niagara_help.py",
               NSEARCH_COURSE_ROOT="empty-course")
# golden file -> (output root under testdata, case-name prefix, env, cwd_fixture)
GROUPS = {
    "nsearch.json": ("golden", "", ENV, None),
    "nsearch-old-nav.json": ("golden", "old-nav-", dict(ENV, FAKE_MODULE_NAV_OLD="1"), None),
    "nsearch-missing.json": ("golden", "missing-", MISSING, None),
    "nsearch-timeouts.json": ("timeouts/golden", "", ENV, ".."),
    "find3.json": ("find3/golden", "", ENV, ".."),
}


def slug(cmd):
    """A case name: the argv words lower-cased, `--` spelled dd, other punctuation dropped."""
    if not cmd:
        return "no-args"
    if not cmd.strip("' "):
        return "blank-spaces" if " " in cmd else "blank"
    words = ["dd" if w == "--" else w for w in shlex.split(cmd)]
    return re.sub(r"[^a-z0-9]+", "-", " ".join(words).lower()).strip("-")


def main(research):
    src = os.path.join(research, "tools", "tests", "fixtures", "golden-cli")
    for tree in ("fake", "course", "empty-course"):
        dst = os.path.join(HERE, tree)
        shutil.rmtree(dst, ignore_errors=True)
        shutil.copytree(os.path.join(src, "nsearch", tree), dst,
                        ignore=shutil.ignore_patterns("__pycache__"))
    for out in {g[0] for g in GROUPS.values()}:
        shutil.rmtree(os.path.join(HERE, out), ignore_errors=True)
    for name, (out, prefix, env, cwd) in GROUPS.items():
        with open(os.path.join(src, name), encoding="utf-8") as fh:
            cases = json.load(fh)
        used = set()
        for cmd in sorted(cases):
            case = prefix + slug(cmd)
            if case in used:
                sys.exit(f"convert_goldens: two commands map to case {case}")
            used.add(case)
            d = os.path.join(HERE, out, case)
            os.makedirs(d)
            spec = {"args": shlex.split(cmd), "env": env}
            if cwd:
                spec["cwd_fixture"] = cwd
            with open(os.path.join(d, "cmd.json"), "w", encoding="utf-8") as fh:
                json.dump(spec, fh, ensure_ascii=False, sort_keys=True)
                fh.write("\n")
            want = cases[cmd]
            for part in ("stdout", "stderr"):
                if want[part] or part == "stdout":
                    with open(os.path.join(d, part), "w", encoding="utf-8", newline="") as fh:
                        fh.write(want[part])
            with open(os.path.join(d, "exit"), "w", encoding="utf-8") as fh:
                fh.write(f"{want['rc']}\n")


def py_ref(repo, env, rev):  # D47: a clean worktree of <repo> tag pre-go-retire (docs/goldens.md)
    root = os.environ.get(env) or os.path.expanduser("~/niagara-research-worktrees/pre-go-retire-" + repo)
    git = lambda *a: subprocess.run(["git", "-C", root, *a], capture_output=True, text=True).stdout.strip()  # noqa: E731
    if git("rev-parse", "HEAD") != rev or git("status", "--porcelain", "--untracked-files=no"):
        sys.exit(f"{env}={root}: not a clean worktree of {repo} tag pre-go-retire ({rev[:9]}); see docs/goldens.md")
    return root


if __name__ == "__main__":
    if len(sys.argv) != 1:
        sys.exit(__doc__)
    main(py_ref("niagara-research", "NIAGARA_RESEARCH_PY_REF", "c846ce1e5eb92ee6d938efbb3585473a545f7e50"))
