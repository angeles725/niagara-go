"""Convert the frozen bog-nav golden cases of niagara-research to the Go layout (decision D7).

    NIAGARA_RESEARCH=/path/to/niagara-research python3 -I internal/bognav/testdata/convert_golden.py

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
ROOT = os.environ.get("NIAGARA_RESEARCH") or sys.exit("set NIAGARA_RESEARCH to the niagara-research checkout")
TOOL = os.path.join(ROOT, "tools", "bog-nav.py")
FIXTURES = os.path.join(ROOT, "tools", "tests", "fixtures")
GOLDEN_JSON = os.path.join(FIXTURES, "golden-cli", "bog-nav.json")
COPIES = ["station.xml", "escaped.xml", "proxy.xml"]
ENV = {"NIAGARA_ORGANIZED": "organized-mini", "NIAGARA_CACHE_DIR": "off"}
RUN_ENV = {"COLUMNS": "100", "NO_COLOR": "1", "PYTHONHASHSEED": "0", **ENV}

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
    res = subprocess.run([sys.executable, TOOL, *argv], cwd=HERE, capture_output=True,
                         env=dict(os.environ, **RUN_ENV), timeout=300)
    return res.returncode, res.stdout, res.stderr


def slug(cmd):
    s = re.sub(r"[^A-Za-z0-9]+", "-", cmd).strip("-").lower()
    return s or "no-args"


def write_case(dest, argv, rc, out, err):
    os.makedirs(dest)
    with open(os.path.join(dest, "cmd.json"), "w", encoding="utf-8") as fh:
        json.dump({"args": argv, "env": ENV}, fh, indent=1, ensure_ascii=False)
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
