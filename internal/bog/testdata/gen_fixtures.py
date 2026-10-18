"""Regenerate the internal/bog parity fixtures from the Python originals (the specification).

    python3 internal/bog/testdata/gen_fixtures.py
    python3 internal/bog/testdata/gen_fixtures.py --dump <station.bog> <out.json>

The Python is niagara-research tag pre-go-retire (NIAGARA_RESEARCH_PY_REF names its worktree, D47).

Writes, next to this script:
  read/<case>       synthetic inputs for bog.ReadBytes/ReadText (ZIPs built with fixed timestamps)
  read/expected.json  per case: the BogReadError message of tools/lib/bogio.py (the input path
                      replaced by {path}) or the sha256 + length of the text read_bog_text returns
  model/<case>.json   the tools/bognav/model.py graph of every model/<case>.xml (and model/edge.bog),
                      dumped in insertion order, plus link_row() of every link and resolve() of refs
Every input is synthetic: never put a station bog here (they can hold user password hashes).
"""
import hashlib
import io
import json
import os
import subprocess
import sys
import warnings
import zipfile

HERE = os.path.dirname(os.path.abspath(__file__))


def py_ref(repo, env, rev):  # D47: a clean worktree of <repo> tag pre-go-retire (docs/goldens.md)
    root = os.environ.get(env) or os.path.expanduser("~/niagara-research-worktrees/pre-go-retire-" + repo)
    git = lambda *a: subprocess.run(["git", "-C", root, *a], capture_output=True, text=True).stdout.strip()  # noqa: E731
    if git("rev-parse", "HEAD") != rev or git("status", "--porcelain", "--untracked-files=no"):
        sys.exit(f"{env}={root}: not a clean worktree of {repo} tag pre-go-retire ({rev[:9]}); see docs/goldens.md")
    return root


ROOT = py_ref("niagara-research", "NIAGARA_RESEARCH_PY_REF", "c846ce1e5eb92ee6d938efbb3585473a545f7e50")
sys.path[:0] = [os.path.join(ROOT, "tools", "lib"), os.path.join(ROOT, "tools")]
import bogio  # noqa: E402
from bognav import model  # noqa: E402

# The XML of tools/tests/test_bog_nav.py (ParseTest, ExitTest, ReaderLimitsTest).
XML = """<?xml version='1.0' encoding='UTF-8'?>
<bajaObjectGraph version='4.0'>
 <p h='1' m='b=baja c=control conv=converters' t='b:Station'>
  <p n='Services' h='2' t='b:ServiceContainer'>
   <p n="JobService" h="c" t="b:JobService"/>
   <p n='object$20tags' h='3' t='b:Folder'/>
  </p>
  <p n='A' h='4' t='c:NumericWritable'/>
  <p n='B' h='5' t='c:BooleanWritable'>
   <p n='Link20' t='b:ConversionLink'>
    <p n="sourceOrd" t="b:Ord" v="h:4"/>
    <p n="sourceSlotName" v="out"/>
    <p n="targetSlotName" v="in10"/>
    <p n="converter" t="conv:NumericToStatusBoolean"/>
   </p>
  </p>
 </p>
</bajaObjectGraph>"""

DATE = (2020, 1, 1, 0, 0, 0)
warnings.filterwarnings("ignore", "Duplicate name", UserWarning)  # duplicate.bog on purpose


def zipped(members, method=zipfile.ZIP_STORED):
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as z:
        for name, data in members:
            z.writestr(zipfile.ZipInfo(name, DATE), data, compress_type=method)
    return buf.getvalue()


def nest(payload, depth, method=zipfile.ZIP_STORED):
    data = zipped([("file.xml", payload)], method)
    for _ in range(depth):
        data = zipped([("config.bog", data)], method)
    return data


def patched(data, local_off, cd_off, value, width=2):
    """Overwrite a little-endian field in the first local header and the first central entry."""
    out = bytearray(data)
    cd = out.find(b"PK\x01\x02")
    if local_off is not None:
        out[local_off:local_off + width] = value.to_bytes(width, "little")
    out[cd + cd_off:cd + cd_off + width] = value.to_bytes(width, "little")
    return bytes(out)


def damaged(data):
    info = zipfile.ZipFile(io.BytesIO(data)).infolist()[0]
    start = 30 + len(info.filename) + len(info.extra)
    out = bytearray(data)
    out[start:start + 8] = b"\xff" * 8
    return bytes(out)


BAD_UTF8 = (b"\xff", b"\xe2\x82", b"\xed\xa0\x80", b"\xf0\x9f\x98", b"\xc0\xaf", b"\xf4\x90\x80\x80",
            b"\xf8\x88\x80\x80\x80", b"\xe0\x80\xaf", b"\xc3", b"\x80\x80", b"\xf0\x80\x80\x80",
            "é€😀".encode())
UTF8 = XML.replace("JobService\"", "Job" + "|".join(["%s"] * len(BAD_UTF8)) + "\"", 1).encode() % BAD_UTF8

READ_CASES = {
    "ok.bog": nest(XML, 0),
    "deflated.bog": nest(XML, 0, zipfile.ZIP_DEFLATED),
    "bzip2.bog": nest(XML, 0, zipfile.ZIP_BZIP2),
    "nest1.dist": nest(XML, 1),
    "nest3.dist": nest(XML, 3, zipfile.ZIP_DEFLATED),
    "nest4.dist": nest(XML, 4),
    "deep.dist": nest(XML, 50),
    "backup.dist": zipped([("stations/S/config.bog", nest(XML, 0))]),
    "suffix.bog": zipped([("readme.txt", "x"), ("a/file.xml", XML), ("b/file.xml", "not this")]),
    "exact-wins.bog": zipped([("a/file.xml", "<not it/>"), ("file.xml", XML)]),
    "duplicate.bog": zipped([("file.xml", "first, not a bog"), ("file.xml", XML)]),
    "plain.bog": XML.encode(),
    "crlf.xml": XML.replace("\n", "\r\n").encode(),
    "utf8.bog": UTF8,
    "empty.bog": b"",
    "blank.bog": b" \n\t\r\x0b\x0c",
    "nbsp.bog": " ".encode(),
    "text.bog": b"hello, not a bog\n",
    "late-root.bog": b" " * 65536 + XML.encode(),
    "garbage.bog": b"PK\x03\x04garbage",
    "pk.bog": b"PK",
    "truncated.bog": nest(XML, 0)[:120],
    "no-member.bog": zipped([("readme.txt", "x")]),
    "empty-member.bog": zipped([("file.xml", "")]),
    "not-bog-member.bog": zipped([("file.xml", "<x/>")]),
    "nested-garbage.dist": zipped([("config.bog", b"not a zip")]),
    "nested-empty.dist": zipped([("config.bog", zipped([("readme.txt", "x")]))]),
    "method99.bog": patched(nest(XML, 0), 8, 10, 99),
    "encrypted.bog": patched(nest(XML, 0), 6, 8, 1),
    "damaged.bog": damaged(nest(XML * 50, 0)),
    "damaged-deflate.bog": damaged(nest(XML * 50, 0, zipfile.ZIP_DEFLATED)),
    "oversized.bog": patched(nest(XML, 0), None, 24, 0xFFFFFFF0, 4),
    "oversized-nested.dist": zipped([("config.bog", patched(nest(XML, 0), None, 24, 0xFFFFFFF0, 4))]),
}


def gen_read():
    d = os.path.join(HERE, "read")
    os.makedirs(d, exist_ok=True)
    expected = {}
    for name, data in sorted(READ_CASES.items()):
        p = os.path.join(d, name)
        with open(p, "wb") as fh:
            fh.write(data)
        try:
            text = bogio.read_bog_text(p)
        except bogio.BogReadError as exc:
            expected[name] = {"error": str(exc).replace(p, "{path}")}
            continue
        raw = text.encode("utf-8")
        entry = {"sha256": hashlib.sha256(raw).hexdigest(), "len": len(raw)}
        if name == "utf8.bog":
            entry["text"] = text
        expected[name] = entry
    with open(os.path.join(d, "expected.json"), "w") as fh:
        json.dump(expected, fh, indent=1, sort_keys=True, ensure_ascii=False)
        fh.write("\n")


# Edge cases of the regex line parser, every one pinned by the Python output (model/edge.json).
SPAN = "".join("<p n='Span%d' h='s%d'%st='b:Folder'/>\n" % (i, i, sep) for i, sep in enumerate(
    ["\r", "\x0b", "\x0c", "\x1c", "\x1d", "\x1e", "\x1f", "\u0085", " ", " ", " ", " "]))
EDGE = ("""<?xml version='1.0' encoding='UTF-8'?>
<bajaObjectGraph version='4.0'>
<p h='1' m='b=baja c=control' t='b:Station'>
 <p n='Dup' h='5' t='b:Folder'><p n='Inner' h='5' t='c:NumericWritable'/><p n='Kid' h='6' t='b:Folder'/></p>
 <p n='Multi' h='7'
   t='b:Folder'/>
""" + SPAN + """ <p n='CRLF' h='8' t='b:Folder'/>\r
 <pé n='Uni' h='20' t='b:Folder'/><P n='Upper' h='20b'/><p_x n='Under' h='20c'/>
 <p xn='bad' én='z' n='good' h='21' t='b:Folder'/>
 <p n="dq" h='22' t="b:Folder" v='sq' v="dq"/>
 <p n='unclosed h='23' t='b:Folder'/>
 <p n='A$20b$u00e9$zz$$4' h='24' t='b:Folder'/>
 <p n='Bad%sX' h='60' t='b:Folder'/>
 <p n='W1' h='25' t='control:BooleanWritable'/><p n='W2' h='26' t='NumericWritableX'/><p n='W3' h='27' t='IntegerWritable'/>
 <p n='W4' h='28' t='a:b:FloatWritable'/>
 <p n='Pt' h='30' t='c:NumericPoint' m='c=controlX\x1cz=zed noeq q=a=b'>
  <p n='value' t='b:Float' v='1.5'/>
  <p n='value' v='untyped'/>
  <p n='status' v='{ok}'/>
  <p n='facets' f='r'/>
  <p n='noattrs'/>
  <p n='fallback' t='b:StatusNumeric'><p n='value' v='2'/></p>
  <p n='fallback'/>
  <p n='setpoint' t='b:StatusNumeric'><p n='value' v='3'/><p n='status' v='{fault}'/><p n='other' v='9'/></p>
  <p n='setpoint' t='b:StatusNumeric2' f='s'>
  </p>
  <p n='plain' v='1'/>
  <p n='plain' v='2' f='x'/>
  <p n='wsAnnotation' t='b:WsAnnotation' v='1,1,8'/>
  <p n='out' t='b:StatusNumeric'><p n='value' v='4'/><p n='value'/></p>
  <a n='doIt' f='c'/><a n='doIt' f='d'/><a/>
  <!-- <p n='InComment' h='31' t='b:Folder'/> -->
 </p>
 <p n='Links' h='40' t='b:Folder'>
  <p n='L1' t='b:Link'><p n='sourceOrd' v='h:'/><p n='sourceSlotName' v='a$20b'/></p>
  <p n='L2' t='b:Link'><p n='sourceOrd' v='station:|slot:/X'/><p n='targetSlotName' v='in'/><p n='nested'><p n='deep' v='1'/></p></p>
  <p n='L3' t='b:ConversionLink' h='41'><p n='sourceOrd' v='h:24'/><p n='HandleKid' h='42' t='b:Folder'><p n='x' v='1'/></p></p>
  <p n='L4' t='b:FooLink'><p n='sourceOrd' v='h:30'/></p>
  <p n='L5' t='b:Link'/>
  <p n='L6' t='x:Link' h='43'><p n='sourceOrd' v=''/><p n='sourceSlotName' v=''/></p>
 </p>
 <p n='' h='' t='b:Folder'/>
 <p n='Empty' h='50' t='nopfx'/>
 <p n='Late' h='51' t='c:Folder' m='c=late'/>
 <p n='Hdup' h='51' t='b:Folder'/>
 <p n='Act' h='52' t='b:Folder'><a n='x'>y</a><p n='after' v='1'/></p>
 <p n='Last' h='53' t='b:Folder'/>
</p>
</bajaObjectGraph>
""").encode() % (b"\xff\xe2\x82",)


def dump(bog, ref_comps=None):
    """ref_comps: resolve() refs come from the first N components only (resolve is O(n))."""
    def opt(v):
        return v

    comps = []
    for h, c in bog.handle_map.items():
        slots = []
        for sn, s in c.slots.items():
            entry = {"type": s["type"], "value": s["value"], "flags": s["flags"]}
            if "child" in s:
                entry["child"] = [[k, v] for k, v in s["child"].items()]
            slots.append([sn, entry])
        comps.append({
            "key": h, "name": c.name, "handle": c.handle, "type": c.type_, "pfx": c.pfx,
            "module": c.module, "path": c.path, "parent_h": opt(c.parent_h), "children": c.children,
            "slots": slots, "actions": [[k, v] for k, v in c.actions.items()],
            "has_fallback": c.has_fallback, "is_writable": c.is_writable,
            "value_type": c.value_type, "value_val": c.value_val, "present": sorted(c.present),
            "simple_type": c.simple_type(),
        })
    refs = ["", "/", "nope", "h:nope", "h:", "h:1"]
    for c in list(bog.handle_map.values())[:ref_comps]:
        refs += [c.path, "/" + c.path + "/", c.name, "h:" + c.handle, c.path.replace(" ", "$20")]
    seen, resolved = set(), []
    for r in refs:
        if r in seen:
            continue
        seen.add(r)
        hit = bog.resolve(r)
        resolved.append([r, None if hit is None else hit.handle, None if hit is None else hit.path])
    return {
        "comps": comps,
        "handle_count": [[k, v] for k, v in bog.handle_count.items()],
        "prefix_map": [[k, v] for k, v in bog.prefix_map.items()],
        "roots": bog.roots,
        "links": bog.link_list,
        "link_rows": [bog.link_row(lk) for lk in bog.link_list],
        "resolve": resolved,
    }


def gen_model():
    d = os.path.join(HERE, "model")
    with open(os.path.join(d, "basic.xml"), "w") as fh:
        fh.write(XML)
    with open(os.path.join(d, "basic-escaped.xml"), "w") as fh:
        fh.write(XML.replace('<p n="targetSlotName" v="in10"/>', '<p n="targetSlotName" v="in$2010"/>'))
    with open(os.path.join(d, "edge.bog"), "wb") as fh:
        fh.write(EDGE)
    for name in sorted(os.listdir(d)):
        base, ext = os.path.splitext(name)
        if ext not in (".xml", ".bog"):
            continue
        p = os.path.join(d, name)
        bog = model.Bog(name, bogio.read_bog_text(p).splitlines())
        with open(os.path.join(d, base + ".json"), "w") as fh:
            json.dump(dump(bog), fh, indent=1, ensure_ascii=False)
            fh.write("\n")


REAL_REF_COMPS = 200  # same as realRefComps in model_test.go


def dump_file(bog_path, out):
    """--dump: the model.py graph of one real bog, for the env-gated TestParityRealBogs (kept
    outside the repository: a station bog and its dump can hold user password hashes)."""
    bog = model.Bog(bog_path, bogio.read_bog_text(bog_path).splitlines())
    with open(out, "w") as fh:
        json.dump(dump(bog, REAL_REF_COMPS), fh, ensure_ascii=False)


if __name__ == "__main__":
    if sys.argv[1:2] == ["--dump"]:
        dump_file(sys.argv[2], sys.argv[3])
    else:
        gen_read()
        gen_model()
