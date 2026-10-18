"""Convert the frozen gen-catalog golden cases of niagara-research to the Go layout (decision D7).

    NIAGARA_RESEARCH=/path/to/niagara-research python3 -I internal/gencatalog/testdata/convert_golden.py

Source of truth: tools/tests/fixtures/golden-cli/gen-catalog/{absent,written,fresh,stale}.json (run
by tools/tests/test_golden_gen_catalog.py, which walks the CATALOG.md states in that order). Those
files were captured in text mode, so every case is re-run against the Python tool to record the
exact bytes, and the run is checked against the frozen JSON first: a case whose text-mode output
differs stops the conversion.

tools/gen-catalog.py now execs this port (D35), so the Python is exported from niagara-research at
PY_REV, the parent of the switch commit, and staged like the Python suite does: the script and
lib/blockfiles.py copied into tools/ of a temporary copy of the fixture corpus (it resolves the
repo from its own location).

Writes golden/<case>/ (absent: cwd testdata/corpus, no CATALOG.md) and states/<state>/golden/<case>/
for written, fresh and stale (run by TestGoldenCatalogStates on a temporary copy of the corpus whose
CATALOG.md is absent, expected/CATALOG.md, or that file with block 9's row renamed to gone.md).
The catalog the Python writes must equal expected/CATALOG.md. A case whose argv already has a
directory keeps its name. Rerunning it must leave `git status` clean.
"""
import filecmp
import io
import json
import os
import re
import shlex
import shutil
import subprocess
import sys
import tarfile
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
RESEARCH = os.environ.get("NIAGARA_RESEARCH") or sys.exit("set NIAGARA_RESEARCH to the niagara-research checkout")
PY_REV = "2f6c09a772a6a844b1e0e485ffbae1eedbf3f278"  # parent of e21fe8825 (the D35 switch)
FIXTURES = os.path.join(RESEARCH, "tools", "tests", "fixtures", "golden-cli")
ENV = {"COLUMNS": "80"}
STALE = ("| 9 | [niagara-mental-model-bloque9.md]", "| 8 | [gone.md]")


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


def stage(tools, root):
    """Copy the frozen script and its lib into root/tools/; return the script copy."""
    os.makedirs(os.path.join(root, "tools", "lib"), exist_ok=True)
    shutil.copy2(os.path.join(tools, "lib", "blockfiles.py"), os.path.join(root, "tools", "lib"))
    return shutil.copy2(os.path.join(tools, "gen-catalog.py"), os.path.join(root, "tools"))


def convert(script, root, golden_json, golden, spec_env, cwd_fixture=None, subst=None):
    with open(os.path.join(FIXTURES, golden_json), encoding="utf-8") as fh:
        frozen = json.load(fh)
    os.makedirs(golden, exist_ok=True)
    names = existing(golden)
    taken = {n.lower() for n in names.values()}
    env = dict(os.environ, PYTHONHASHSEED="0", COLUMNS="80", PYTHON_COLORS="0")
    for cmd in sorted(frozen):
        argv = shlex.split(cmd)
        res = subprocess.run([sys.executable, script, *argv], cwd=root, capture_output=True, env=env, timeout=300)
        out, err = res.stdout, res.stderr
        if subst:
            out, err = out.replace(root.encode(), subst.encode()), err.replace(root.encode(), subst.encode())
        got = {"rc": res.returncode, "stdout": text_mode(out), "stderr": text_mode(err)}
        if subst:
            got = {k: v.replace(subst, "<ROOT>") if k != "rc" else v for k, v in got.items()}
        if got != frozen[cmd]:
            sys.exit(f"{cmd!r}: the Python tool at {PY_REV[:9]} no longer matches {golden_json}")
        name = names.get(tuple(argv))
        if name is None:
            base = name = slug(cmd)
            n = 1
            while name.lower() in taken:
                n += 1
                name = f"{base}-{n}"
            taken.add(name.lower())
        spec = {"args": argv, "env": spec_env}
        if cwd_fixture:
            spec["cwd_fixture"] = cwd_fixture
        write_case(os.path.join(golden, name), spec, res.returncode, out, err)
    return len(frozen)


def main():
    corpus = os.path.join(FIXTURES, "gen-catalog", "corpus")
    if not same_tree(corpus, os.path.join(HERE, "corpus")):
        sys.exit("testdata/corpus differs from the Python fixture corpus")
    with tempfile.TemporaryDirectory() as tmp:
        tools = export_tools(os.path.join(tmp, "py"))
        root = os.path.join(tmp, "repo")
        shutil.copytree(corpus, root)
        script = stage(tools, root)
        catalog = os.path.join(root, "CATALOG.md")
        n = convert(script, root, "gen-catalog/absent.json", os.path.join(HERE, "golden"), ENV, cwd_fixture="corpus")
        states = os.path.join(HERE, "states")
        n += convert(script, root, "gen-catalog/written.json", os.path.join(states, "written", "golden"), ENV,
                     cwd_fixture="corpus")
        with open(catalog, encoding="utf-8", newline="") as fh:
            written = fh.read()
        with open(os.path.join(HERE, "expected", "CATALOG.md"), encoding="utf-8", newline="") as fh:
            if fh.read() != written:
                sys.exit("the catalog the Python writes differs from expected/CATALOG.md")
        n += convert(script, root, "gen-catalog/fresh.json", os.path.join(states, "fresh", "golden"), ENV,
                     cwd_fixture="corpus")
        with open(catalog, "w", encoding="utf-8", newline="") as fh:
            fh.write(written.replace(*STALE))
        n += convert(script, root, "gen-catalog/stale.json", os.path.join(states, "stale", "golden"), ENV,
                     cwd_fixture="corpus")
    print(f"{n} cases written")


if __name__ == "__main__":
    main()
