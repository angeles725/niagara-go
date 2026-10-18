"""Convert the frozen corpus-nav golden cases of niagara-research to the Go layout (decision D7).

    NIAGARA_RESEARCH=/path/to/niagara-research python3 -I internal/corpusnav/testdata/convert_golden.py

Source of truth: tools/tests/fixtures/golden-cli/corpus-nav.json (run by
tools/tests/test_golden_corpus_nav.py). That file was captured in text mode, so every case is
re-run against the Python tool to record the exact bytes, and the run is checked against the
frozen JSON first: a case whose text-mode output differs stops the conversion.

tools/corpus-nav.py now execs this port (D35), so the Python is exported from niagara-research at
PY_REV, the parent of the switch commit (tools/ only, into a temporary directory). It runs like
the Python suite: cwd = the fixture corpus, CORPUS_NAV_ROOT = its absolute path, the find index
off (that suite pins the same find output with and without it).

Writes golden/<case>/{cmd.json,stdout,stderr,exit}; a case runs in testdata with
CORPUS_NAV_ROOT=corpus (IN_CORPUS cases: in testdata/corpus, CORPUS_NAV_ROOT=.). A case whose argv already has a directory keeps its name; cases whose argv
is not in the JSON (Go-only) are left untouched. testdata/corpus must equal the Python fixture.
Rerunning it must leave `git status` clean.
"""
import filecmp
import io
import json
import os
import re
import shlex
import subprocess
import sys
import tarfile
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
RESEARCH = os.environ.get("NIAGARA_RESEARCH") or sys.exit("set NIAGARA_RESEARCH to the niagara-research checkout")
PY_REV = "2f6c09a772a6a844b1e0e485ffbae1eedbf3f278"  # parent of e21fe8825 (the D35 switch)
FIXTURES = os.path.join(RESEARCH, "tools", "tests", "fixtures", "golden-cli")
GOLDEN_JSON = os.path.join(FIXTURES, "corpus-nav.json")
CASE_ENV = {"COLUMNS": "80", "GIT_DIR": "/nonexistent", "CORPUS_NAV_ROOT": "corpus", "CORPUS_NAV_FIND_INDEX": "off"}
# A file argument is resolved against the cwd, which was the corpus in the Python suite: these
# cases run there (cwd_fixture corpus, CORPUS_NAV_ROOT=.).
IN_CORPUS = {"lint-refs niagara-mental-model-bloque5.md"}


def export_tools(dest):
    """Extract tools/ of niagara-research at PY_REV into dest; return the tools directory."""
    raw = subprocess.run(["git", "-C", RESEARCH, "archive", PY_REV, "tools"], capture_output=True, check=True).stdout
    with tarfile.open(fileobj=io.BytesIO(raw)) as tar:
        tar.extractall(dest, filter="data")
    return os.path.join(dest, "tools")


def text_mode(raw):
    """What subprocess text=True gives back: UTF-8, universal newlines."""
    return io.TextIOWrapper(io.BytesIO(raw), encoding="utf-8", newline=None).read()


def same_tree(a, b):
    cmp = filecmp.dircmp(a, b)
    if cmp.left_only or cmp.right_only or cmp.diff_files or cmp.funny_files:
        return False
    _, mismatch, errors = filecmp.cmpfiles(a, b, cmp.common_files, shallow=False)
    return not mismatch and not errors and all(same_tree(os.path.join(a, d), os.path.join(b, d)) for d in cmp.common_dirs)


def slug(cmd):
    return re.sub(r"[^A-Za-z0-9]+", "-", cmd).strip("-") or "noargs"


def existing(golden):
    """Map argv (tuple) -> case directory name for the cases already under golden/."""
    names = {}
    for name in sorted(os.listdir(golden)):
        with open(os.path.join(golden, name, "cmd.json"), encoding="utf-8") as fh:
            names[tuple(json.load(fh)["args"])] = name
    return names


def write_case(dest, spec, rc, out, err):
    os.makedirs(dest, exist_ok=True)
    with open(os.path.join(dest, "cmd.json"), "w", encoding="utf-8") as fh:
        fh.write(json.dumps(spec, ensure_ascii=False) + "\n")
    with open(os.path.join(dest, "stdout"), "wb") as fh:
        fh.write(out)
    stderr = os.path.join(dest, "stderr")
    if err:
        with open(stderr, "wb") as fh:
            fh.write(err)
    elif os.path.exists(stderr):
        os.remove(stderr)
    with open(os.path.join(dest, "exit"), "w", encoding="utf-8") as fh:
        fh.write(f"{rc}\n")


def main():
    corpus = os.path.join(FIXTURES, "corpus")
    if not same_tree(corpus, os.path.join(HERE, "corpus")):
        sys.exit("testdata/corpus differs from the Python fixture corpus")
    golden = os.path.join(HERE, "golden")
    with open(GOLDEN_JSON, encoding="utf-8") as fh:
        frozen = json.load(fh)
    names = existing(golden)
    taken = {n.lower() for n in names.values()}
    env = dict(os.environ, PYTHONHASHSEED="0", COLUMNS="80", PYTHON_COLORS="0", GIT_DIR="/nonexistent",
               CORPUS_NAV_ROOT=corpus, CORPUS_NAV_VENV="/nonexistent", CORPUS_NAV_FIND_INDEX="off")
    with tempfile.TemporaryDirectory() as tmp:
        tool = os.path.join(export_tools(tmp), "corpus-nav.py")
        for cmd in sorted(frozen):
            argv = shlex.split(cmd)
            res = subprocess.run([sys.executable, tool, *argv], cwd=corpus, capture_output=True, env=env, timeout=300)
            got = {"rc": res.returncode, "stdout": text_mode(res.stdout), "stderr": text_mode(res.stderr)}
            if got != frozen[cmd]:
                sys.exit(f"{cmd!r}: the Python tool at {PY_REV[:9]} no longer matches corpus-nav.json")
            name = names.get(tuple(argv))
            if name is None:
                base = name = slug(cmd)
                n = 1
                while name.lower() in taken:
                    n += 1
                    name = f"{base}-{n}"
                taken.add(name.lower())
            spec = {"args": argv, "env": CASE_ENV}
            if cmd in IN_CORPUS:
                spec = {"args": argv, "env": dict(CASE_ENV, CORPUS_NAV_ROOT="."), "cwd_fixture": "corpus"}
            write_case(os.path.join(golden, name), spec, res.returncode, res.stdout, res.stderr)
    print(f"{len(frozen)} cases written to {golden}")


if __name__ == "__main__":
    main()
