"""Convert the frozen check-coverage golden cases of niagara-research to the Go layout (decision D7).

    NIAGARA_RESEARCH=/path/to/niagara-research python3 -I internal/checkcoverage/testdata/convert_golden.py

Source of truth: tools/tests/fixtures/golden-cli/check-coverage.json and check-coverage-empty.json
(run by tools/tests/test_golden_check_coverage.py). Those files were captured in text mode, so every
case is re-run against the Python tool to record the exact bytes, and the run is checked against the
frozen JSON first: a case whose text-mode output differs stops the conversion.

tools/check-coverage.py now execs this port (D35), so the Python is exported from niagara-research at
PY_REV, the parent of the switch commit, and staged like the Python suite does: the script and
lib/blockfiles.py copied into tools/ of a temporary repository (it resolves the repo from its own
location), beside a copy of the fixture corpus plus the skipped directories the suite creates.

Writes golden/<case>/ (cwd testdata/corpus, the repository root in TestGolden) and
empty/golden/<case>/ (no block files: the root is the case's {tmp}, passed in GOLDEN_REPO_ROOT and
read by TestGoldenNoBlocks; the Python golden's <ROOT> is that {tmp}). A case whose argv already has
a directory keeps its name. testdata/corpus must equal the staged Python tree (minus tools/*.py).
Rerunning it must leave `git status` clean.
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
SKIPPED_DIRS = ("organized", "_backups", "tools", "node_modules", "niagara-help", "module-navigator")
ENV = {"COLUMNS": "80"}
EMPTY_ENV = {"COLUMNS": "80", "GOLDEN_REPO_ROOT": "{tmp}"}


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
    return shutil.copy2(os.path.join(tools, "check-coverage.py"), os.path.join(root, "tools"))


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
    with tempfile.TemporaryDirectory() as tmp:
        tools = export_tools(os.path.join(tmp, "py"))
        root = os.path.join(tmp, "repo")
        shutil.copytree(os.path.join(FIXTURES, "check-coverage", "corpus"), root)
        for d in SKIPPED_DIRS:
            os.makedirs(os.path.join(root, d), exist_ok=True)
            with open(os.path.join(root, d, "niagara-mental-model-bloque97.md"), "w", encoding="utf-8") as fh:
                fh.write("# Skipped directory\n\nskippedDirOnly\n")
        script = stage(tools, root)
        n = convert(script, root, "check-coverage.json", os.path.join(HERE, "golden"), ENV, cwd_fixture="corpus")
        # testdata/corpus keeps only the skipped directories git can hold (organized/, tools/).
        for d in SKIPPED_DIRS:
            if not os.path.isdir(os.path.join(HERE, "corpus", d)):
                shutil.rmtree(os.path.join(root, d))
        shutil.rmtree(os.path.join(root, "tools", "lib"))
        os.remove(script)
        if not same_tree(root, os.path.join(HERE, "corpus")):
            sys.exit("testdata/corpus differs from the staged Python tree")
        empty = os.path.join(tmp, "empty")
        os.makedirs(empty)
        script = stage(tools, empty)
        n += convert(script, empty, "check-coverage-empty.json", os.path.join(HERE, "empty", "golden"), EMPTY_ENV,
                     subst="{tmp}")
    print(f"{n} cases written")


if __name__ == "__main__":
    main()
