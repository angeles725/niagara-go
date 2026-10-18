"""Regenerate internal/bm25/testdata/fixtures.json from the real Python functions.

Run: python3 -I internal/bm25/testdata/gen.py   (the *_PY_REF variables override the worktree paths)

  log, log1p   math.log / math.log1p over input families both sides rebuild with integer
               arithmetic (sha256 of the little-endian results), plus readable samples
  sum          builtins.sum over float lists (Neumaier since Python 3.12)
  textsearch   textsearch.BM25(...).rank over a small corpus, with and without the glossary
  classrank    module-navigator class_rank.ClassRanker.rank_all scores and their BM25 inputs
  fts5         niagara-help fulltext_scan.ScanIndex.rows scores and their inputs
Floats travel as float.hex() strings so nothing is lost.
"""
import hashlib
import json
import math
import os
import random
import struct
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))


def py_ref(repo, env, rev):  # D47: a clean worktree of <repo> tag pre-go-retire (docs/goldens.md)
    root = os.environ.get(env) or os.path.expanduser("~/niagara-research-worktrees/pre-go-retire-" + repo)
    git = lambda *a: subprocess.run(["git", "-C", root, *a], capture_output=True, text=True).stdout.strip()  # noqa: E731
    if git("rev-parse", "HEAD") != rev or git("status", "--porcelain"):
        sys.exit(f"{env}={root}: not a clean worktree of {repo} tag pre-go-retire ({rev[:9]}); see docs/goldens.md")
    return root


NR = py_ref("niagara-research", "NIAGARA_RESEARCH_PY_REF", "c846ce1e5eb92ee6d938efbb3585473a545f7e50")
MNAV = py_ref("module-navigator", "MODULE_NAVIGATOR_PY_REF", "f897268e900138e08a82bf1b389a1715c1be26e1")
NHELP = py_ref("niagara-help", "NIAGARA_HELP_PY_REF", "8410d74511673e38b87622f7214fa5210ba25a63")
sys.path[:0] = [os.path.join(NR, "tools", "lib"), os.path.join(MNAV, "tools"), os.path.join(NHELP, "tools")]

import glossary  # noqa: E402
import textsearch  # noqa: E402
from module_nav_lib import class_rank  # noqa: E402
from niagara_help_lib import fulltext_scan  # noqa: E402

M64 = (1 << 64) - 1


def lcg(n, seed):
    """n 64-bit values of x = x * 6364136223846793005 + 1442695040888963407 (mod 2**64)."""
    x = seed
    for _ in range(n):
        x = (x * 6364136223846793005 + 1442695040888963407) & M64
        yield x


def from_bits(b):
    return struct.unpack("<d", struct.pack("<Q", b))[0]


def families():
    """name -> list of inputs; Go's familyInputs rebuilds the same lists."""
    idf = []
    for n in range(1, 1201):
        for df in range(0, n + 1):
            r = (n - df + 0.5) / (df + 0.5)
            idf += [r, 1 + r]
    # positive finite doubles from random bit patterns (exponent field 0..0x7fe), subnormals included
    bits = [from_bits(b & 0x7FEFFFFFFFFFFFFF) for b in lcg(300000, 1)]
    bits = [x for x in bits if x > 0]
    near = [1 + k * 2.0 ** -32 for k in range(-(1 << 28), 1 << 28, 997)]
    return {"idf": idf, "bits": bits, "near_one": near}


def l1p_families():
    ints = [float(n) for n in range(0, 300001)]
    # x in (-1, 2**60): from bit patterns of positive doubles below 2**60 and of negatives above -1
    pos = [from_bits(b % 0x43B0000000000000) for b in lcg(200000, 2)]
    neg = [-from_bits(b % 0x3FF0000000000000) for b in lcg(200000, 3)]
    small = [k * 2.0 ** -40 for k in range(-20000, 20000, 7)]
    return {"ints": ints, "pos": pos, "neg": neg, "small": small}


def digest(xs):
    h = hashlib.sha256()
    for x in xs:
        h.update(struct.pack("<d", x))
    return h.hexdigest()


def hx(x):
    return float(x).hex()


DOCS = [
    ("d00", "Alarm service", "The alarm service routes alarms to recipients and consoles."),
    ("d01", "Alarm console", "Acknowledge alarms in the console; alarm ack and alarm class."),
    ("d02", "History service", "Histories record point values; the history service collects them."),
    ("d03", "BACnet network", "Configure the BACnet network, devices and points discovery."),
    ("d04", "Modbus network", "Configure Modbus devices, registers and polling."),
    ("d05", "Schedules", "Weekly schedules, holidays and calendar events control points."),
    ("d06", "Configuración de alarmas", "Cómo configurar las alarmas de temperatura alta en la estación."),
    ("d07", "Estación", "La estación guarda la configuración y los históricos."),
    ("d08", "Alarm service", "The alarm service routes alarms to recipients and consoles."),
    ("d09", "Point", "A control point has a value, a status and an override."),
    ("d10", "", "Temperature setpoint and high temperature alarm limits for a cold room."),
    ("d11", "Defrost", "Defrost cycles of the evaporator; coil temperature terminates defrost."),
    ("d12", "Compressor staging", "Suction pressure setpoint and differential stage the compressors."),
    ("d13", "Empty", ""),
    ("d14", "Servlet", "Register a web servlet in the module; BWebServlet handles GET and POST."),
    ("d15", "Users and roles", "User passwords, roles and permissions; authentication of sessions."),
    ("d16", "Point", "A control point has a value, a status and an override."),
    ("d17", "Network tuning", "Tuning policies: poll frequency, retry and timeout of the network."),
    ("d18", "Graphics", "Px graphics views bind to points; widgets and palettes."),
    ("d19", "Alarm alarm alarm", "alarm alarm alarm alarm alarm alarm alarm alarm alarm alarm"),
]
QUERIES = [
    "alarm", "alarm service", "alarmas de temperatura alta", "configure network", "point value status",
    "how do I acknowledge an alarm", "defrost temperature", "registro de usuario", "servlet", "the of and",
    "nothing matches zzzz", "", "estación configuración", "poll retry timeout", "compressor setpoint",
]


def textsearch_cases():
    ix = textsearch.BM25(DOCS)
    out = []
    for q in QUERIES:
        for require_all in (False, True):
            for use_glossary in (False, True):
                factors = glossary.weighted_terms(q) if use_glossary and not require_all else None
                if use_glossary and require_all:
                    continue
                ranked = ix.rank(q, require_all=require_all, factors=factors)
                out.append({"query": q, "require_all": require_all,
                            "factors": [[t, hx(w)] for t, w in (factors or {}).items()],
                            "ranked": [[hx(s), k] for s, k in ranked]})
    return out


CLASSES = [
    ("BAlarmService", "javax.baja.alarm", "alarm", "BAbstractService", ["BIAlarmHandler"]),
    ("BAlarmConsole", "com.tridium.alarm.ui", "alarm", "BWbView", []),
    ("BAlarmClass", "javax.baja.alarm", "alarm", "BComponent", ["BIAlarmClass"]),
    ("BHistoryService", "javax.baja.history", "history", "BAbstractService", []),
    ("BBacnetNetwork", "javax.baja.bacnet", "bacnet", "BDeviceNetwork", ["BIBacnetNetwork"]),
    ("BModbusNetwork", "com.tridium.modbus", "modbus", "BDeviceNetwork", []),
    ("BWebServlet", "javax.baja.web", "web", "BComponent", ["BIWebServlet"]),
    ("BWebService", "javax.baja.web", "web", "BAbstractService", []),
    ("BNumericPoint", "javax.baja.control", "control", "BControlPoint", ["BINumeric"]),
    ("BBooleanPoint", "javax.baja.control", "control", "BControlPoint", ["BIBoolean"]),
    ("BWeeklySchedule", "javax.baja.schedule", "schedule", "BAbstractSchedule", []),
    ("BUserService", "javax.baja.user", "baja", "BAbstractService", ["BIUserService"]),
    ("BTemperatureAlarmExt", "javax.baja.alarm.ext", "alarm", "BAlarmSourceExt", []),
    ("BHTTPServerX2Y", "com.example.x", "example", "BObject", []),
]
IMPORTERS = {"BAlarmService": 175, "BWebServlet": 184, "BNumericPoint": 22550, "BComponent": 9,
             "BAlarmClass": 1, "BHistoryService": 47751, "BWebService": 0}
CR_QUERIES = ["alarm service", "que es un BWebServlet", "alarmas de temperatura", "network",
              "point", "history", "web servlet service", "configuracion de usuario", "zzz", ""]


def classrank_cases():
    classes = {name: [{"module": mod, "package": pkg, "extends": ext, "implements": impl,
                       "path": "%s/%s.java" % (mod, name)}] for name, pkg, mod, ext, impl in CLASSES}
    imported_by = {n: ["I%d" % i for i in range(k)] for n, k in IMPORTERS.items()}
    ranker = class_rank.ClassRanker(classes, imported_by, {})
    cases = []
    for q in CR_QUERIES:
        terms = class_rank.query_terms(q)
        post = {}
        for t in terms:
            hits = ((i, ranker._tf(i, t)) for i, h in enumerate(ranker.haystacks) if t in h)
            post[t] = {str(i): n for i, n in hits if n}
        results = ranker.rank_all(q)
        cases.append({"query": q, "terms": terms, "postings": post,
                      "scores": {r.name: hx(r.score) for r in results}})
    return {"names": ranker.doc_names, "lengths": ranker.lengths, "avg_len": hx(ranker.avg_len),
            "prior": {n: hx(ranker.prior[n]) for n in sorted(ranker.prior)},
            "importers": {n: IMPORTERS.get(n, 0) for n in ranker.doc_names}, "cases": cases}


def fts5_cases():
    rnd = random.Random(7)
    alts = ["alarm", "ack", "acknowledge", "configur*", "lead lag", "point", "rare"]
    ix = fulltext_scan.ScanIndex([])
    ix.alts, ix.scanned, ix.n_rows, ix.tokens = set(alts), True, 150, 0
    ix.n_hit = dict.fromkeys(alts, 0)
    for doc_id in range(1, 151):
        length = rnd.randint(3, 900)
        ix.tokens += length
        tf = {}
        for a in alts:
            if rnd.random() < (0.9 if a == "point" else 0.004 if a == "rare" else 0.2):
                tf[a] = (rnd.randint(0, 3), rnd.randint(0, 25))
                if tf[a] == (0, 0):
                    tf[a] = (0, 1)
                ix.n_hit[a] += 1
        if tf:
            ix.found[doc_id] = ("docs" if doc_id % 5 else "pdf", "p%03d" % doc_id, "T", 1, 0, length, tf)
    queries = [[("alarm",)], [("ack", "acknowledge"), ("alarm",)], [("point",)], [("configur*",), ("lead lag",)],
               [("ack", "acknowledge"), ("ack",)], [("rare",), ("point",)]]
    cases = []
    for groups in queries:
        for mode in ("or", "and"):
            rows = ix.rows(groups, mode, {"docs", "pdf"}, 3.0)
            cases.append({"groups": groups, "mode": mode, "rows": [[r[0], hx(-r[6])] for r in rows]})
    found = {str(d): [v[5], {a: list(c) for a, c in v[6].items()}] for d, v in ix.found.items()}
    return {"n_rows": ix.n_rows, "tokens": ix.tokens, "n_hit": ix.n_hit, "title_weight": 3.0,
            "found": found, "cases": cases}


def sums():
    rnd = random.Random(11)
    out = []
    for _ in range(100):
        xs = [rnd.choice([rnd.uniform(0, 1), rnd.uniform(0, 1e-9), rnd.uniform(1e6, 1e9), rnd.expovariate(1)])
              for _ in range(rnd.randint(0, 12))]
        out.append([[hx(x) for x in xs], hx(sum(xs))])
    return out


def render(v, depth=0):
    """Lines of v: containers above depth 2 one member per line, deeper ones compact."""
    if depth >= 2 or not isinstance(v, (list, dict)) or not v:
        return [json.dumps(v, ensure_ascii=False)]
    pairs = list(v.items()) if isinstance(v, dict) else [(None, e) for e in v]
    lines = ["{" if isinstance(v, dict) else "["]
    for i, (k, e) in enumerate(pairs):
        sub = render(e, depth + 1)
        if k is not None:
            sub[0] = json.dumps(k) + ": " + sub[0]
        if i < len(pairs) - 1:
            sub[-1] += ","
        lines += sub
    lines.append("}" if isinstance(v, dict) else "]")
    return lines


def main():
    fam, l1p = families(), l1p_families()
    out = {
        "python": sys.version.split()[0],
        "log": {k: {"n": len(v), "sha256": digest(map(math.log, v))} for k, v in fam.items()},
        "log_samples": [[hx(x), hx(math.log(x))] for v in fam.values() for x in v[:20]],
        "log1p": {k: {"n": len(v), "sha256": digest(map(math.log1p, v))} for k, v in l1p.items()},
        "log1p_samples": [[hx(x), hx(math.log1p(x))] for v in l1p.values() for x in v[:20]],
        "sum": sums(),
        "textsearch": textsearch_cases(),
        "classrank": classrank_cases(),
        "fts5": fts5_cases(),
    }
    with open(os.path.join(HERE, "fixtures.json"), "w", encoding="utf-8", newline="\n") as fh:
        fh.write("\n".join(render(out)) + "\n")


if __name__ == "__main__":
    main()
