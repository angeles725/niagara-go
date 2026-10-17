"""Freeze the bog-cycles golden cases from the Python original (the specification).

    NIAGARA_RESEARCH=/path/to/niagara-research python3 -I internal/bogcycles/testdata/capture.py

fixtures/cycles.xml, station.xml and escaped.xml are copies of niagara-research
tools/tests/fixtures/golden-cli (synthetic); the other inputs are written here. Every case runs
`python3 -I tools/bog-cycles.py <args>` in fixtures/ with COLUMNS=100, NO_COLOR=1 and
NIAGARA_CACHE_DIR=off, and is stored as golden/<case>/{cmd.json,stdout,stderr,exit}.
The first seven cases are the niagara-research golden suite (test_golden_bog_cycles.py).
"""
import io
import json
import os
import shutil
import subprocess
import sys
import zipfile

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.environ.get("NIAGARA_RESEARCH") or sys.exit("set NIAGARA_RESEARCH to the niagara-research checkout")
TOOL = os.path.join(ROOT, "tools", "bog-cycles.py")
FIX = os.path.join(HERE, "fixtures")
DATE = (2020, 1, 1, 0, 0, 0)


def doc(body):
    return ("<?xml version='1.0'?><bajaObjectGraph version='4.0'><p n='R' h='1' m='b=baja' t='b:Folder'>"
            + body + "</p></bajaObjectGraph>")


def link(name, src_h, sslot, tslot):
    return ("<p n='%s' t='b:Link'><p n='sourceOrd' v='h:%s'/><p n='sourceSlotName' v='%s'/>"
            "<p n='targetSlotName' v='%s'/></p>" % (name, src_h, sslot, tslot))


def big():
    """60 two-node loops, one 10-node ring with 14 edges, 22 dangling links."""
    body, h = [], 2
    for i in range(60):
        body.append("<p n='L%02da' h='%d' t='b:Folder'>%s</p><p n='L%02db' h='%d' t='b:Folder'>%s</p>"
                    % (i, h, link("Link", h + 1, "out", "in"), i, h + 1, link("Link", h, "out", "in")))
        h += 2
    ring = list(range(h, h + 10))
    for k, rh in enumerate(ring):
        extra = link("Link1", ring[k - 1], "x", "y") if k < 4 else ""
        body.append("<p n='Ring%d' h='%d' t='b:Folder'>%s%s</p>" % (k, rh, link("Link", ring[k - 1], "out", "in"), extra))
    h += 10
    body.append("<p n='D' h='%d' t='b:Folder'>%s</p>" % (h, "".join(link("Link%d" % j, 900 + j, "o", "i%d" % j) for j in range(22))))
    return doc("".join(body))


INPUTS = {
    "trunc.xml": "<?xml version='1.0'?><bajaObjectGraph version='4.0'><p n='A' h='1'>",
    "bad.xml": "<?xml version='1.0'?><bajaObjectGraph version='4.0'><p n='A' h='1 t=x></p></bajaObjectGraph>",
    "mismatch.xml": "<?xml version='1.0'?>\n<bajaObjectGraph version='4.0'>\n <p n='A' h='1'></q>\n</bajaObjectGraph>",
    "dash.xml": doc("<p n='A' h='2' t='b:Folder'><p n='Link' t='b:Link'><p n='sourceOrd' v='h:2'/></p>"
                    "<p n='Link1' t='b:Link'><p n='sourceOrd' v='h:77'/></p></p>"),
    "named.xml": doc("<p n='Room$201' h='2' t='b:Folder'>" + link("Link", 2, "out$20a", "in") + "</p>"
                     "<p n='A' h='3' t='b:Folder'>" + link("Link", 4, "out", "in") + link("Link1", 4, "x", "y") + "</p>"
                     "<p n='B' h='4' t='b:Folder'>" + link("Link", 3, "out", "in") + "</p>"
                     "<p n='C' h='5' t='b:Folder'><p n='Link' t='b:Link'><p n='sourceOrd' v='station:|slot:/A'/></p></p>"),
    "big.xml": big(),
    "empty.bog": "",
    "text.bog": "not xml",
    "junk.bog": b"PK\x03\x04junk",
}

CASES = [
    ("cycles", ["cycles.xml"]), ("cycles-json", ["cycles.xml", "--json"]), ("station", ["station.xml"]),
    ("station-json", ["station.xml", "--json"]), ("escaped", ["escaped.xml"]), ("missing", ["missing.bog"]),
    ("help", ["--help"]),
    ("zip", ["cycles.bog"]), ("dist", ["backup.dist", "--json"]), ("truncated", ["trunc.xml"]),
    ("invalid-token", ["bad.xml"]), ("mismatched-tag", ["mismatch.xml"]), ("missing-slots", ["dash.xml"]),
    ("missing-slots-json", ["dash.xml", "--json"]), ("named-root", ["named.xml"]),
    ("named-root-json", ["named.xml", "--json"]), ("many", ["big.xml"]), ("empty", ["empty.bog"]),
    ("not-bog", ["text.bog"]), ("bad-zip", ["junk.bog"]), ("no-args", []), ("unknown-flag", ["x.xml", "--bogus"]),
]


def zipped(name, data):
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as z:
        z.writestr(zipfile.ZipInfo(name, DATE), data)
    return buf.getvalue()


def main():
    for name, data in INPUTS.items():
        with open(os.path.join(FIX, name), "wb") as fh:
            fh.write(data.encode("utf-8") if isinstance(data, str) else data)
    with open(os.path.join(FIX, "cycles.xml"), "rb") as fh:
        bog = zipped("file.xml", fh.read())
    with open(os.path.join(FIX, "cycles.bog"), "wb") as fh:
        fh.write(bog)
    with open(os.path.join(FIX, "backup.dist"), "wb") as fh:
        fh.write(zipped("stations/S/config.bog", bog))
    golden = os.path.join(HERE, "golden")
    shutil.rmtree(golden, ignore_errors=True)
    env = dict(os.environ, COLUMNS="100", NO_COLOR="1", NIAGARA_CACHE_DIR="off")
    for case, args in CASES:
        res = subprocess.run([sys.executable, "-I", TOOL, *args], cwd=FIX, env=env, capture_output=True, timeout=120)
        d = os.path.join(golden, case)
        os.makedirs(d)
        with open(os.path.join(d, "cmd.json"), "w") as fh:
            json.dump({"args": args, "cwd_fixture": "fixtures"}, fh)
            fh.write("\n")
        for part, data in (("stdout", res.stdout), ("stderr", res.stderr), ("exit", b"%d\n" % res.returncode)):
            with open(os.path.join(d, part), "wb") as fh:
                fh.write(data)


if __name__ == "__main__":
    main()
