"""Freeze golden cases for n4-type-catalog from the Python tool (decision D7 layout).

    NIAGARA_RESEARCH=/path/to/niagara-research python3 -I internal/typecatalog/testdata/freeze_golden.py

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
ROOT = os.environ.get("NIAGARA_RESEARCH") or sys.exit("set NIAGARA_RESEARCH to the niagara-research checkout")
TOOL = os.path.join(ROOT, "tools", "n4-type-catalog.py")
RUN_ENV = {"COLUMNS": "100", "NO_COLOR": "1", "PYTHONHASHSEED": "0"}

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
    res = subprocess.run([sys.executable, TOOL, *argv], cwd=HERE, capture_output=True,
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
            json.dump({"args": argv}, fh, ensure_ascii=False)
            fh.write("\n")
        for name, data in (("stdout", out), ("stderr", err)):
            with open(os.path.join(dest, name), "wb") as fh:
                fh.write(data)
        with open(os.path.join(dest, "exit"), "w", encoding="utf-8") as fh:
            fh.write(f"{rc}\n")
    print(f"{len(CASES)} cases written to {golden}")


if __name__ == "__main__":
    main()
