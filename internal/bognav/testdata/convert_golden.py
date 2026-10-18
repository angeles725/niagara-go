"""Convert the frozen bog-nav golden cases of niagara-research to the Go layout (decision D7).

    python3 -I internal/bognav/testdata/convert_golden.py

Source of truth: tools/tests/fixtures/golden-cli/bog-nav.json (63 cases, run by
tools/tests/test_golden_bog_nav.py). That file was captured in text mode, which turns the
"\\r\\n" that csv.writer prints into "\\n", so every case is re-run here against the Python
tool to record the exact bytes, and the run is checked against the frozen JSON first: a case
whose text-mode output no longer equals the JSON stops the conversion (the Python tool drifted).

EXTRA cases (help texts, --limit and usage errors) are frozen from the Python tool the same way;
they have no entry in bog-nav.json.

Writes, next to this script: station.xml, escaped.xml, proxy.xml (copied), organized-mini/
(copied: the hermetic type catalog) and golden/<NNN-slug>/{cmd.json,stdout,stderr,exit}.
Every case runs with cwd = this directory, NIAGARA_ORGANIZED=organized-mini and the parse cache
off, like the Python suite (which passes an absolute organized-mini path).
"""
import io
import json
import os
import re
import shlex
import shutil
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))


def py_ref(repo, env, rev):  # D47: a clean worktree of <repo> tag pre-go-retire (docs/goldens.md)
    root = os.environ.get(env) or os.path.expanduser("~/niagara-research-worktrees/pre-go-retire-" + repo)
    git = lambda *a: subprocess.run(["git", "-C", root, *a], capture_output=True, text=True).stdout.strip()  # noqa: E731
    if git("rev-parse", "HEAD") != rev or git("status", "--porcelain"):
        sys.exit(f"{env}={root}: not a clean worktree of {repo} tag pre-go-retire ({rev[:9]}); see docs/goldens.md")
    return root


ROOT = py_ref("niagara-research", "NIAGARA_RESEARCH_PY_REF", "c846ce1e5eb92ee6d938efbb3585473a545f7e50")
TOOL = os.path.join(ROOT, "tools", "bog-nav.py")

# At the tag, running tools/<x>.py execs the Go port (its D35 switch block); the Python under that block
# is the frozen specification. `python3 [-I] -c FROZEN <tool> <args>` runs it like `python3 [-I] <tool>
# <args>` did before the switch: the block is cut out (giving back the exact e21fe8825^ source, line
# numbers included), the code runs as __main__ under the tool's own path, and a traceback skips this frame.
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
FIXTURES = os.path.join(ROOT, "tools", "tests", "fixtures")
GOLDEN_JSON = os.path.join(FIXTURES, "golden-cli", "bog-nav.json")
COPIES = ["station.xml", "escaped.xml", "proxy.xml"]
ENV = {"NIAGARA_ORGANIZED": "organized-mini", "NIAGARA_CACHE_DIR": "off"}
RUN_ENV = {"COLUMNS": "100", "NO_COLOR": "1", "PYTHONHASHSEED": "0", **ENV}
# slot --json cases that print the defining class's source path, joined with "/" as Linux CPython prints it
# (Windows CPython prints backslashes): requires.goos binds them to Linux (docs/goldens.md).
LINUX_PATHS = {("proxy.xml", "slot", "Drivers/Net/points/ro2/proxyExt", "--json"),
               ("proxy.xml", "slot", "Room", "defrostEnabled", "--json"), ("station.xml", "slot", "h:20", "--json")}

EXTRA = [
    "-h", "station.xml slot -h", "station.xml links -h", "station.xml pointcache -h",
    "", "station.xml", "station.xml bogus",
    "station.xml tree --limit 2", "--limit 3 station.xml tree",
    "station.xml find --type c:BooleanWritable --limit 1",
    "station.xml find --type c:BooleanWritable --limit 1 --json",
    "--json station.xml find --type c:BooleanWritable -n 2 --csv",
    "station.xml types --limit 0", "station.xml types -n x",
    "station.xml writable --klass bogus", "station.xml tree --depth x",
    "station.xml links --to Logic --json", "station.xml pointcache --stale-before 2020",
    "station.xml pointcache --live --csv", "station.xml slot Station/Panel --no-defaults --json",
    "proxy.xml writable --module CRP", "proxy.xml hoa --module CRP --json",
    "station.xml relays --module CRP", "station.xml tiles --json",
    "x.bog selftest",
    # the silent-zero sentence prints as text even under --json/--csv, exit 1 (parity with Python)
    "station.xml find --type bac:BacnetBinaryPv --json", "station.xml find --type bac:BacnetBinaryPv --csv",
]


def text_mode(raw):
    """What subprocess text=True gives back: UTF-8, universal newlines."""
    return io.TextIOWrapper(io.BytesIO(raw), encoding="utf-8", newline=None).read()


def run(argv):
    res = subprocess.run([sys.executable, "-c", FROZEN, TOOL, *argv], cwd=HERE, capture_output=True,
                         env=dict(os.environ, **RUN_ENV), timeout=300)
    return res.returncode, res.stdout, res.stderr


def slug(cmd):
    s = re.sub(r"[^A-Za-z0-9]+", "-", cmd).strip("-").lower()
    return s or "no-args"


def write_case(dest, argv, rc, out, err):
    os.makedirs(dest)
    with open(os.path.join(dest, "cmd.json"), "w", encoding="utf-8") as fh:
        spec = {"args": argv, "env": ENV}
        if tuple(argv) in LINUX_PATHS:
            spec["requires"] = {"goos": ["linux"]}
        json.dump(spec, fh, indent=1, ensure_ascii=False)
        fh.write("\n")
    for name, data in (("stdout", out), ("stderr", err)):
        with open(os.path.join(dest, name), "wb") as fh:
            fh.write(data)
    with open(os.path.join(dest, "exit"), "w", encoding="utf-8") as fh:
        fh.write(f"{rc}\n")


def main():
    for name in COPIES:
        shutil.copyfile(os.path.join(FIXTURES, "golden-cli", name), os.path.join(HERE, name))
    shutil.rmtree(os.path.join(HERE, "organized-mini"), ignore_errors=True)
    shutil.copytree(os.path.join(FIXTURES, "organized-mini"), os.path.join(HERE, "organized-mini"))
    golden = os.path.join(HERE, "golden")
    shutil.rmtree(golden, ignore_errors=True)
    with open(GOLDEN_JSON, encoding="utf-8") as fh:
        frozen = json.load(fh)
    seen = set()
    cases = [(c, frozen[c]) for c in sorted(frozen)] + [(c, None) for c in EXTRA]
    if LINUX_PATHS - {tuple(shlex.split(c)) for c, _ in cases}:
        sys.exit(f"LINUX_PATHS names no case: {sorted(LINUX_PATHS - {tuple(shlex.split(c)) for c, _ in cases})}")
    for i, (cmd, want) in enumerate(cases, 1):
        argv = shlex.split(cmd)
        rc, out, err = run(argv)
        if want is not None:
            got = {"rc": rc, "stdout": text_mode(out), "stderr": text_mode(err)}
            if got != want:
                sys.exit(f"{cmd!r}: the Python tool no longer matches bog-nav.json; refusing to convert")
        name = f"{i:03d}-{slug(cmd)}"[:60]
        if name in seen:
            sys.exit(f"duplicate case name {name}")
        seen.add(name)
        write_case(os.path.join(golden, name), argv, rc, out, err)
    print(f"{len(frozen)} frozen + {len(EXTRA)} extra cases written to {golden}")


if __name__ == "__main__":
    main()
