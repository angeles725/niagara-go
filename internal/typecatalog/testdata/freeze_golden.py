"""Freeze golden cases for n4-type-catalog from the Python tool (decision D7 layout).

    python3 -I internal/typecatalog/testdata/freeze_golden.py

niagara-research has no golden suite for tools/n4-type-catalog.py, so the cases are recorded here
from the Python tool itself (the specification): stdout, stderr and exit code as exact bytes, with
cwd = this directory. Inputs are the synthetic Java trees next to this script (javasrc/ in the
organized/<module>/<module>-<part>/ layout, loose/ and plain/ without it) and catalog.json, which
this script writes from `build loose javasrc`. odd.json is hand-written (values of other
JSON types than build writes; its BNoFlags/BNoExt entries crash the Python tool and are unit tests). `selftest` is not frozen: it prints unittest timings.
A `show <dir>` case never has two hits: their order is os.walk's, i.e. the directory order of the
file system the checkout lives on (catalog.json cases cover several hits in file order).
"""
import json
import os
import shlex
import shutil
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))


def py_ref(repo, env, rev):  # D47: a clean worktree of <repo> tag pre-go-retire (docs/goldens.md)
    root = os.environ.get(env) or os.path.expanduser("~/niagara-research-worktrees/pre-go-retire-" + repo)
    git = lambda *a: subprocess.run(["git", "-C", root, *a], capture_output=True, text=True).stdout.strip()  # noqa: E731
    if git("rev-parse", "HEAD") != rev or git("status", "--porcelain", "--untracked-files=no"):
        sys.exit(f"{env}={root}: not a clean worktree of {repo} tag pre-go-retire ({rev[:9]}); see docs/goldens.md")
    return root


ROOT = py_ref("niagara-research", "NIAGARA_RESEARCH_PY_REF", "c846ce1e5eb92ee6d938efbb3585473a545f7e50")
TOOL = os.path.join(ROOT, "tools", "n4-type-catalog.py")

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
if not sys.flags.safe_path:
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
RUN_ENV = {"COLUMNS": "100", "NO_COLOR": "1", "PYTHONHASHSEED": "0"}
# Cases that print a Java source path, joined with "/" as Linux CPython prints it (Windows CPython prints
# backslashes): requires.goos binds them to Linux (docs/goldens.md).
LINUX_PATHS = {
    "build javasrc", "build loose", "build loose plain javasrc", "show javasrc control:NumericWritable",
    "show javasrc CONTROL:NumericWritable", "show javasrc acme-extras:NumericWritable",
    "show javasrc javax.baja.control.BNumericWritable", "show javasrc control.BNumericWritable",
    "show javasrc kitControl:InterstartDelayControl", "show loose BBytes", "show loose BCrlf", "show loose b:Multi",
}

CASES = [
    "", "-h", "build -h", "show -h", "selftest -h", "bogus", "build", "show", "show catalog.json",
    "build nosuchdir", "build loose nosuchdir other", "build plain", "build javasrc", "build loose",
    "build loose plain javasrc",
    "show javasrc control:NumericWritable",
    "show javasrc CONTROL:NumericWritable", "show javasrc acme:NumericWritable",
    "show javasrc acme-extras:NumericWritable", "show javasrc nosuch:NumericWritable",
    "show javasrc javax.baja.control.BNumericWritable", "show javasrc control.BNumericWritable",
    "show javasrc kitControl:InterstartDelayControl", "show javasrc Nope", "show javasrc x:Nope",
    "show loose BBytes", "show loose BCrlf", "show loose b:Multi", "show loose a:Multi",
    "show catalog.json BMulti", "show catalog.json NumericWritable", "show catalog.json control:NumericWritable",
    "show catalog.json Nope", "build loose --out",
    # a catalog with non-string flags, lists, null, dicts and a big int: show prints str() of each
    "show odd.json BX", "show odd.json control:BX",
]


def run(argv):
    res = subprocess.run([sys.executable, "-c", FROZEN, TOOL, *argv], cwd=HERE, capture_output=True,
                         env=dict(os.environ, **RUN_ENV), timeout=300)
    return res.returncode, res.stdout, res.stderr


def slug(cmd):
    out = "".join(ch if ch.isalnum() else "-" for ch in cmd.lower())
    while "--" in out:
        out = out.replace("--", "-")
    return out.strip("-") or "no-args"


def main():
    rc, out, err = run(["build", "loose", "javasrc"])
    if rc != 0:
        sys.exit(f"build loose javasrc failed: {err!r}")
    with open(os.path.join(HERE, "catalog.json"), "wb") as fh:
        fh.write(out)
    golden = os.path.join(HERE, "golden")
    shutil.rmtree(golden, ignore_errors=True)
    for i, cmd in enumerate(CASES, 1):
        argv = shlex.split(cmd)
        rc, out, err = run(argv)
        dest = os.path.join(golden, f"{i:03d}-{slug(cmd)}"[:60])
        os.makedirs(dest)
        with open(os.path.join(dest, "cmd.json"), "w", encoding="utf-8") as fh:
            spec = {"args": argv}
            if cmd in LINUX_PATHS:
                spec["requires"] = {"goos": ["linux"]}
            json.dump(spec, fh, ensure_ascii=False)
            fh.write("\n")
        for name, data in (("stdout", out), ("stderr", err)):
            with open(os.path.join(dest, name), "wb") as fh:
                fh.write(data)
        with open(os.path.join(dest, "exit"), "w", encoding="utf-8") as fh:
            fh.write(f"{rc}\n")
    print(f"{len(CASES)} cases written to {golden}")


if __name__ == "__main__":
    main()
