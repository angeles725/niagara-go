"""Convert the frozen module-find golden cases of niagara-research to the Go layout (decision D7).

    NIAGARA_RESEARCH=/path/to/niagara-research python3 -I internal/modulefind/testdata/convert_golden.py

Source of truth: tools/tests/fixtures/golden-cli/module-find.json and module-find-unreadable.json
(run by tools/tests/test_golden_module_find.py). Those files were captured in text mode, which
turns the "\\r\\n" that csv.writer prints into "\\n", so every case is re-run against the Python
tool to record the exact bytes, and the run is checked against the frozen JSON first: a case
whose text-mode output differs stops the conversion.

tools/module-find.py now execs this port (D35), so the Python is exported from niagara-research at
PY_REV, the parent of the switch commit (tools/ only, into a temporary directory). It runs like the
Python suite: cwd = the fixture directory, the roots srcA, srcB, nojava relative to it.

Writes golden/<case>/ (cwd testdata) and unreadable/golden/<case>/ (run by TestGoldenUnreadable in a
temporary directory holding src/, a copy of srcA with com/acme/util/Helper.java at mode 000, like
the Python suite). A case whose argv already has a directory keeps its name; cases whose argv is not
in the JSON (Go-only) are left untouched. srcA, srcB and nojava must equal the Python fixtures.
Must not run as root (root reads a mode-000 file). Rerunning it must leave `git status` clean.
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
ENV = {"COLUMNS": "80", "GIT_DIR": "/nonexistent"}
LOCKED = os.path.join("src", "com", "acme", "util", "Helper.java")


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


def convert(script, root, golden_json, golden, spec_env, cwd_fixture=None, subst=None):
    with open(os.path.join(FIXTURES, golden_json), encoding="utf-8") as fh:
        frozen = json.load(fh)
    os.makedirs(golden, exist_ok=True)
    names = existing(golden)
    taken = {n.lower() for n in names.values()}
    env = dict(os.environ, PYTHONHASHSEED="0", COLUMNS="80", PYTHON_COLORS="0", GIT_DIR="/nonexistent")
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
    if os.geteuid() == 0:
        sys.exit("run as a normal user: root reads the mode-000 file of the unreadable cases")
    fixture = os.path.join(FIXTURES, "module-find")
    for tree in ("srcA", "srcB", "nojava"):
        if not same_tree(os.path.join(fixture, tree), os.path.join(HERE, tree)):
            sys.exit(f"testdata/{tree} differs from the Python fixture")
    with tempfile.TemporaryDirectory() as tmp:
        script = os.path.join(export_tools(os.path.join(tmp, "py")), "module-find.py")
        n = convert(script, fixture, "module-find.json", os.path.join(HERE, "golden"), ENV)
        work = os.path.join(tmp, "work")
        shutil.copytree(os.path.join(fixture, "srcA"), os.path.join(work, "src"))
        os.chmod(os.path.join(work, LOCKED), 0)
        try:
            n += convert(script, work, "module-find-unreadable.json", os.path.join(HERE, "unreadable", "golden"), ENV)
        finally:
            os.chmod(os.path.join(work, LOCKED), 0o644)
    print(f"{n} cases written")


if __name__ == "__main__":
    main()
