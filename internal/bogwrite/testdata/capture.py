"""Freeze the bog-write fixtures from the Python original (the specification).

    python3 -I internal/bogwrite/testdata/capture.py

Writes, next to this script:
  fixtures/        plans and BOG documents (all synthetic) the cases read
  golden/<case>/   CLI cases run in fixtures/ that write no file: help, usage errors, validate,
                   and generate failures. A failure Python reports with a traceback gets an
                   intended.json: Go prints only the traceback's last line.
  generated/<case>/  successful generate runs, made in a scratch directory: cmd.json, stdout,
                   stderr, exit and the written file (out.xml, or out.bog plus its file.xml)
  jsoncases.json   json.loads() of hand-written, truncated and edited plans: the typed value
                   or the JSONDecodeError text (parity suite of plan.go's decoder)
Every case runs `python3 -I tools/bog-write.py <args>` with COLUMNS=100 and NO_COLOR=1.
"""
import contextlib
import io
import json
import os
import random
import shutil
import subprocess
import sys
import tempfile
import zipfile

HERE = os.path.dirname(os.path.abspath(__file__))


def py_ref(repo, env, rev):  # D47: a clean worktree of <repo> tag pre-go-retire (docs/goldens.md)
    root = os.environ.get(env) or os.path.expanduser("~/niagara-research-worktrees/pre-go-retire-" + repo)
    git = lambda *a: subprocess.run(["git", "-C", root, *a], capture_output=True, text=True).stdout.strip()  # noqa: E731
    if git("rev-parse", "HEAD") != rev or git("status", "--porcelain", "--untracked-files=no"):
        sys.exit(f"{env}={root}: not a clean worktree of {repo} tag pre-go-retire ({rev[:9]}); see docs/goldens.md")
    return root


ROOT = py_ref("niagara-research", "NIAGARA_RESEARCH_PY_REF", "c846ce1e5eb92ee6d938efbb3585473a545f7e50")
TOOL = os.path.join(ROOT, "tools", "bog-write.py")

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
FIX = os.path.join(HERE, "fixtures")
DATE = (2020, 1, 1, 0, 0, 0)
ENV = dict(os.environ, COLUMNS="100", NO_COLOR="1")
# A Python traceback names its frames' files by path: the goldens keep the checkout they were first
# captured in, so a run from any worktree of the tag gives the same bytes.
CAPTURED_ROOT = b"/home/cristian/niagara-research"

PLAN = {  # tools/tests/test_bog_write.py PLAN
    "root": {"name": "Demo", "type": "baja:Folder"},
    "components": [
        {"name": "SelR", "type": "control:NumericWritable", "ws": [4, 10, 8],
         "slots": {"facets": {"type": "baja:Facets", "value": "min=0.0;max=4.0;precision=0"}}},
        {"name": "Mux", "type": "kitControl:BooleanSelect", "ws": [26, 8, 8]},
        {"name": "Mux?", "type": "kitControl:BooleanSelect"},
    ],
    "links": [{"from": "SelR.out", "to": "Mux.select"}, {"from": "SelR.out", "to": "Mux1.select"}],
}
RICH = {
    "root": {"name": "Plant", "type": "control:Folder"},
    "components": [
        {"name": "A", "type": "baja:Folder", "ws": [1.7, -2.2, "3", True], "slots": {
            "i": 5, "f": 1.5, "big": 1e16, "small": 1e-05, "neg": -0.0, "t": True, "n": None,
            "l": [1, "a", None, 2.5, {"k": [True]}], "q": "say \"hi\" & 'bye' <x>", "dq": 'only "double"',
            "ws": "tab\there\nline\rret", "u": "café \U0001f600", "x": {"type": "control:StatusNumeric", "value": 3},
            "nested": {"type": "baja:Struct", "children": {"a": "1", "b": {"type": "baja:Integer", "value": "2"},
                                                            "c": {"children": {}}}},
            "notype": {"value": "v"}, "empty": {}, "nullval": {"type": "baja:String", "value": None}}},
        {"name": "Foo9", "type": "kitControl:Add"}, {"name": "Foo9?", "type": "kitControl:Add"},
        {"name": "Foo009", "type": "kitControl:Add"}, {"name": "Foo009?", "type": "kitControl:Add"},
        {"name": "B?", "type": "kitControl:Add", "parent": "A", "ws": []},
        {"name": "C", "type": "baja:Folder", "parent": "A/B", "ws": [0, 0, 0, 0]},
        {"name": "Link", "type": "baja:Folder", "slots": {}},
        {"name": "Link1", "type": "baja:Folder"},
    ],
    "links": [{"from": "A.out", "to": "Foo9.inA"}, {"from": "Foo10.out", "to": "Foo9.inB"},
              {"from": "A/B/C.o\"ut", "to": "A.in&1"}, {"from": "A/B.x", "to": "Foo0010.in"},
              {"from": "A.out", "to": ".in"}],
}
DEFAULT_ROOT = {"components": [{"name": "X", "type": "baja:Folder"}]}

PLANS = {
    "plan.json": PLAN, "rich.json": RICH, "defroot.json": DEFAULT_ROOT, "empty-plan.json": {},
    "dup.json": {"components": [{"name": "A", "type": "baja:Folder"}, {"name": "A", "type": "baja:Folder"}]},
    "parent.json": {"components": [{"name": "A", "type": "baja:Folder", "parent": "Nope"}]},
    "parent-int.json": {"components": [{"name": "A", "type": "baja:Folder", "parent": 5}]},
    "illegal.json": {"components": [{"name": "Bad name", "type": "baja:Folder"}]},
    "nomod.json": {"components": [{"name": "A", "type": "Folder"}]},
    "notype.json": {"root": {"name": "R"}},
    "noname.json": {"components": [{"type": "baja:Folder"}]},
    "endpoint.json": {"components": [{"name": "A", "type": "baja:Folder"}], "links": [{"from": "A.out", "to": "B.in"}]},
    "nodot.json": {"components": [{"name": "A", "type": "baja:Folder"}], "links": [{"from": "Aout", "to": "A.in"}]},
    "password.json": {"components": [{"name": "A", "type": "baja:Folder",
                                      "slots": {"pw": {"type": "baja:Password", "value": "x"}}}]},
    "badslot.json": {"components": [{"name": "A", "type": "baja:Folder", "slots": {"bad name": 1}}]},
    "badws.json": {"components": [{"name": "A", "type": "baja:Folder", "ws": ["x", 1, 2]}]},
    "list.json": [1, 2],
}
RAW = {
    "trailing.json": b'{"root": {"name": "R", "type": "baja:Folder"},}',
    "truncated.json": b'{"components": [{"name": "A"',
    "bom.json": b'\xef\xbb\xbf{}',
    "crlf-bad.json": b'{\r\n "a": 1,\r\n\r "b": 2,\r\n}',
    "latin1.json": b'{"components": [{"name": "\xe9"}]}',
    "crlf.json": b'{\r\n "components": [\r\n  {"name": "A",\r\n   "type": "baja:Folder"}\r\n ]\r\n}\r\n',
}

H = "<?xml version='1.0'?><bajaObjectGraph version='4.0'><p n='R' h='1' m='b=baja' t='b:Folder'>"
T = "</p></bajaObjectGraph>"
LINK = ("<p n='Link' t='b:Link'><p n='sourceOrd' v='h:%s'/><p n='sourceSlotName' v='out'/>"
        "<p n='targetSlotName' v='in'/></p>")
DOCS = {
    "clean.xml": H + "<p n='A' h='2' t='b:Folder'/>" + T,
    "duphandle.xml": H + "<p n='A' h='2' t='b:Folder'/><p n='B' h='2' t='b:Folder'/>" + T,
    "dangling.xml": H + "<p n='A' h='2' t='b:Folder'>" + LINK % 99 + "</p>" + T,
    "transient.xml": H + "<p n='A' h='2' t='b:Folder'>" + (LINK % 2).replace("t='b:Link'", "f='rt' t='b:Link'") + "</p>" + T,
    "password.xml": H + "<p n='A' h='2' t='b:Folder'><p n='pw' t='b:Password' v='x'/><p n='p2' t='zPASSWORD'/></p>" + T,
    "linkkids.xml": H + "<p n='A' h='2' t='b:Folder'><p n='Link' t='b:ConversionLink'><p n='sourceOrd' v='h:2'/>"
                    "<p n='sourceSlotName' v=''/></p></p>" + T,
    "ws.xml": H + "".join("<p n='W%d' h='%d' t='b:Folder'><p n='wsAnnotation' t='b:WsAnnotation' v='%s'/></p>" % (i, i + 2, v)
                          for i, v in enumerate(["1,2,3", "-1,-2,3,4", "1,2,3|x", "1,2", "1,x,8", "1,2,3&#10;",
                                                 "1,2,3,4,5", "1,2,-3", "١,2,3", "1,2,3|a&#10;b"])) + T,
    "prefix.xml": H + "<p n='A' h='2' t='zz:Thing'/><p n='B' h='3' m='zz=zed y = q' t='zz:Thing'/><p n='C' t='y:X'/>" + T,
    "names.xml": H + "<p n='ok$20name' h='2' t='b:Folder'/><p n='bad name' h='3' t='b:Folder'/><p n='9x' h='4'/>"
                 "<p n='tail&#10;' h='5'/><p h='6'/><q n='bad q'/>" + T,
    "multi.xml": "<bajaObjectGraph version='4.0'><p n='S' h='1' m='b=baja c=control' t='b:Station'>"
                 "<p n='W' h='2' t='c:NumericWritable'/></p></bajaObjectGraph>",
    "root.xml": "<?xml version='1.0'?><bajaObjectGraph version='4.0'><x/><p n='A' h='1' t='b:Folder'/></bajaObjectGraph>"
                .replace("bajaObjectGraph version='4.0'><x/>", "bajaObjectGraph version='4.0'><x/>"),
    "nsroot.xml": "<bajaObjectGraph xmlns='urn:x' version='4.0'><p n='A' h='1' m='b=baja' t='b:Folder'/></bajaObjectGraph>",
    "unbound.xml": "<bajaObjectGraph version='4.0'><a:p n='A'/></bajaObjectGraph>",
    "trunc.xml": H + "<p n='A' h='2'>",
    "badattr.xml": H + "<p n='A' h='2 t=x></p>" + T,
    "mismatch.xml": H + "\n<p n='A' h='2'></q>" + T,
    "empty.xml": "",
    "text.xml": "plain text",
}


def zipped(members):
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as z:
        for name, data in members:
            z.writestr(zipfile.ZipInfo(name, DATE), data)
    return buf.getvalue()


GOLDEN = [
    ("help", ["--help"]), ("generate-help", ["generate", "--help"]), ("validate-help", ["validate", "--help"]),
    ("no-command", []), ("bad-command", ["write"]), ("generate-no-out", ["generate", "plan.json"]),
    ("validate-no-file", ["validate"]),
] + [("validate-" + os.path.splitext(n)[0], ["validate", n]) for n in sorted(DOCS)] + [
    ("validate-clean-bog", ["validate", "clean.bog"]), ("validate-dist", ["validate", "clean.dist"]),
    ("validate-bad-zip", ["validate", "junk.bog"]), ("validate-missing", ["validate", "nope.bog"]),
    ("validate-generated", ["validate", "plan.bog"]),
] + [("generate-" + os.path.splitext(n)[0], ["generate", n, "--out", "never-written.bog"])
     for n in sorted(set(PLANS) | set(RAW)) if n not in ("plan.json", "rich.json", "defroot.json", "empty-plan.json", "crlf.json")] + [
    ("generate-missing-plan", ["generate", "nope.json", "--out", "x.bog"]),
    ("generate-no-dir", ["generate", "plan.json", "--out", "no/such/dir.bog"]),
    ("generate-no-dir-plain", ["generate", "plan.json", "--out", "no/such/dir.xml", "--plain"]),
]
GENERATED = [
    ("plan-zip", ["generate", "plan.json", "--out", "out.bog"]),
    ("plan-plain", ["generate", "plan.json", "--out", "out.xml", "--plain"]),
    ("rich-plain", ["generate", "rich.json", "--plain", "--out", "out.xml"]),
    ("rich-zip", ["generate", "rich.json", "--out", "out.bog"]),
    ("defroot-plain", ["generate", "defroot.json", "--out", "out.xml", "--plain"]),
    ("empty-plain", ["generate", "empty-plan.json", "--out", "out.xml", "--plain"]),
    ("crlf-plain", ["generate", "crlf.json", "--out", "out.xml", "--plain"]),
]


def run(args, cwd):
    res = subprocess.run([sys.executable, "-I", "-c", FROZEN, TOOL, *args], cwd=cwd, env=ENV, capture_output=True,
                         timeout=120)
    res.stderr = res.stderr.replace(b'File "' + ROOT.encode() + b'/', b'File "' + CAPTURED_ROOT + b'/')
    return res


def store(d, args, res, intended=False):
    os.makedirs(d)
    with open(os.path.join(d, "cmd.json"), "w") as fh:
        json.dump({"args": args, "cwd_fixture": "fixtures"}, fh)
        fh.write("\n")
    for part, data in (("stdout", res.stdout), ("stderr", res.stderr), ("exit", b"%d\n" % res.returncode)):
        with open(os.path.join(d, part), "wb") as fh:
            fh.write(data)
    if intended and res.stderr.startswith(b"Traceback (most recent call last):\n"):
        last = res.stderr.decode("utf-8").splitlines()[-1] + "\n"
        with open(os.path.join(d, "intended.json"), "w") as fh:
            json.dump({"reason": "Go prints only the last line of the Python traceback", "stderr": last}, fh,
                      ensure_ascii=False)
            fh.write("\n")


def typed(v):
    if v is None or isinstance(v, bool):
        return v
    if isinstance(v, int):
        return {"int": str(v)}
    if isinstance(v, float):
        return {"float": repr(v)}
    if isinstance(v, str):
        return {"str": v.encode("utf-8", "surrogatepass").hex()}
    if isinstance(v, list):
        return [typed(x) for x in v]
    return {"dict": [[k.encode("utf-8", "surrogatepass").hex(), typed(x)] for k, x in v.items()]}


def clock_only(old, new):
    """True when two one-entry ZIPs differ at most in the entry's DOS time and date: bog-write.py stamps
    the local time now there (checkZip in bogwrite_test.go ignores it), so a rerun keeps the stored file."""
    c = new.find(b"PK\x01\x02")
    if old is None or len(old) != len(new) or not new.startswith(b"PK\x03\x04") or c < 0:
        return False
    fixed = bytearray(new)
    fixed[10:14] = old[10:14]  # local file header: time, date
    fixed[c + 12:c + 16] = old[c + 12:c + 16]  # central directory header: time, date
    return bytes(fixed) == old


def json_cases():
    sample = json.dumps(RICH)
    small = '{"a": [1, -2.5e3, true, null, "x\\u00e9\\n"], "b": {}, "c": []}'
    texts = [
        "", "  ", "{", "[", "[1,", "[1,]", "{,}", '{"a":1,}', "[1,,2]", '{"a" 1}', "[1 2]", '{"a":1 "b"', "tru",
        "truex", "nul", "nulls", "-", "01", "-01", "1.", "1e", "1e+", "-0", "0.5e-3", "1E5", "1e400", "-1e400",
        "NaN", "Infinity", "-Infinity", "-Inf", "1.5x", '"ab', '"\\', '"a\\q"', '"\\u12"', '"\\u12zz"',
        '"\\uD83D\\uDE00"', '"\\uD83Dx"', '"\\ud800"', '"\\udc00\\ud800"', '"a\\x01"'.replace("\\x01", "\x01"), '"\t"',
        '"a\\/b\\b\\f\\n\\r\\t\\"\\\\"', '{"a"  :  1  ,  "b":2}', '{"a":1,"a":2,"b":3}', "﻿{}",
        "[\n1,\n]", "\n\n  {", "123456789012345678901234567890", "-0.0", "1.0", "100", "[[[[]]]]", '{"":""}',
        "é", '"é\U0001f600"', "[1]x", "[1] ", "{\"a\":}", '"\\ud83d\\ude00', '"\\ud83d\\ude0', '"\\ud83d\\uzz00"', '"\\ud83d\\u',
        '"\\ud83d\\u0041"', '"\\ud83d\\\\"', '"x\\u00e9', '"\\u00e', "[\"a\" \"b\"]", "1 2", "{\"a\"", "{\"a\":1",
    ]
    texts += [small[:i] for i in range(len(small) + 1)]
    texts += [small[:i] + c + small[i:] for i in range(len(small) + 1) for c in ",:]}\"\\ x1-."]
    rnd = random.Random(2)
    for _ in range(120):
        i = rnd.randrange(len(sample) + 1)
        texts.append(sample[:i] + rnd.choice([",", "]", "}", '"', "\\", ":", "", "\x01"]) + sample[i + 1:])
    out, seen = [], set()
    for t in texts:
        if t in seen:
            continue
        seen.add(t)
        try:
            case = {"in": t, "value": typed(json.loads(t))}
        except json.JSONDecodeError as exc:
            case = {"in": t, "error": str(exc)}
        out.append(case)
    with open(os.path.join(HERE, "jsoncases.json"), "w", encoding="utf-8") as fh:
        fh.write("[\n" + ",\n".join(json.dumps(c, ensure_ascii=True) for c in out) + "\n]\n")


def main():
    shutil.rmtree(FIX, ignore_errors=True)
    os.makedirs(FIX)
    files = {n: json.dumps(p, indent=1).encode() for n, p in PLANS.items()}
    files.update(RAW)
    files.update({n: d.encode("utf-8") for n, d in DOCS.items()})
    files["clean.bog"] = zipped([("file.xml", DOCS["clean.xml"])])
    files["clean.dist"] = zipped([("stations/S/config.bog", files["clean.bog"])])
    files["junk.bog"] = b"PK\x03\x04junk"
    for name, data in files.items():
        with open(os.path.join(FIX, name), "wb") as fh:
            fh.write(data)
    with tempfile.TemporaryDirectory() as tmp:  # plan.bog: a Python-generated bog to validate
        shutil.copy(os.path.join(FIX, "plan.json"), tmp)
        run(["generate", "plan.json", "--out", "plan.bog"], tmp).check_returncode()
        with zipfile.ZipFile(os.path.join(tmp, "plan.bog")) as z:
            xml = z.read("file.xml")
        with open(os.path.join(FIX, "plan.bog"), "wb") as fh:
            fh.write(zipped([("file.xml", xml)]))
    stored = {}
    for case, _ in GENERATED:
        with contextlib.suppress(FileNotFoundError), open(os.path.join(HERE, "generated", case, "out.bog"), "rb") as fh:
            stored[case] = fh.read()
    for d in ("golden", "generated"):
        shutil.rmtree(os.path.join(HERE, d), ignore_errors=True)
    for case, args in GOLDEN:
        store(os.path.join(HERE, "golden", case), args, run(args, FIX), intended=True)
    for case, args in GENERATED:
        with tempfile.TemporaryDirectory() as tmp:
            shutil.copy(os.path.join(FIX, args[1]), tmp)
            res = run(args, tmp)
            d = os.path.join(HERE, "generated", case)
            store(d, args, res)
            out = os.path.join(tmp, args[args.index("--out") + 1])
            if res.returncode:
                sys.exit("generated case %s failed:\n%s" % (case, res.stderr.decode()))
            if out.endswith(".bog"):
                with open(out, "rb") as fh:
                    data = fh.read()
                with open(os.path.join(d, "out.bog"), "wb") as fh:
                    fh.write(stored[case] if clock_only(stored.get(case), data) else data)
                with zipfile.ZipFile(out) as z:
                    with open(os.path.join(d, "file.xml"), "wb") as fh:
                        fh.write(z.read("file.xml"))
            else:
                shutil.copy(out, os.path.join(d, "out.xml"))
    json_cases()


if __name__ == "__main__":
    main()
