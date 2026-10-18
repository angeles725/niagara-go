"""Synthetic source-corpus fixtures of gen_golden.py: drivers, search (grep, source, token) and
lic (license-feature(s), resources), each an indexes/ directory plus an organized/ tree."""

import json
import os
import shutil
import sqlite3
import time

HERE = os.path.dirname(os.path.abspath(__file__))


# drivers: a synthetic corpus with one network per family, inherited
# extensions, lexicons (help.guide.base, a language variant, a latin-1 file),
# a missing doc module, a docSource-doc copy, an inner class and a licensing
# call per network.
def drv(module, sub, name, extends=None, implements=(), modifiers=("public",), outer=None, pkg="com.x"):
    e = {"package": pkg, "module": sub, "kind": "class", "modifiers": list(modifiers),
         "path": "{}/{}/vineflower/{}/{}.java".format(module, sub, pkg.replace(".", "/"), name)}
    if extends:
        e["extends"] = extends
    if implements:
        e["implements"] = list(implements)
    if outer:
        e["outer_class"] = outer
    return name, e


DRV_CLASSES = [
    drv("acme", "acme-rt", "BAcmeNetwork", "javax.baja.driver.BDdfNetwork<BAcmeDevice>"),
    drv("acme", "acme-rt", "BAcmeDevice", "BDevice"),
    drv("acme", "acme-rt", "BAcmePointDeviceExt", "BPointDeviceExt"),
    drv("acme", "acme-rt", "BAcmeProxyExt", "BProxyExt"),
    drv("acme", "acme-rt", "BAcmeOtherProxyExt", "BAcmeProxyExt"),
    drv("acme", "acme-rt", "BAcmeDiscoverJob", "BSimpleJob"),
    drv("acme", "acme-rt", "BAcmeLearnJob", "BJob"),
    drv("acme", "acme-rt", "BAcmePollJob", "BJob"),
    drv("acme", "acme-rt", "Inner", "BDeviceNetwork", outer="BAcmeNetwork"),
    drv("acme", "acme-wb", "BAcmeDeviceManager", "BDeviceManager"),
    drv("acme", "acme-wb", "BAcmeFolderManager", "BAbstractManager"),
    drv("acme", "acme-ux", "BAcmeUxManager", "BObject"),
    drv("acme", "acme-ux", "AcmeManager", "BObject"),
    drv("acmeTcp", "acmeTcp-rt", "BAcmeTcpNetwork", "BAcmeNetwork"),
    drv("acmeTcp", "acmeTcp-rt", "BAcmeTcpNetwork", "BAcmeNetwork", pkg="com.y"),
    drv("solo", "solo-rt", "BSoloNetwork", "BDeviceNetwork", modifiers=("public", "abstract")),
    drv("solo", "solo-rt", "BSoloDevice", "BDevice"),
    drv("docSource", "docSource-doc", "BSoloNetwork", "BDeviceNetwork"),
    drv("nn", "nn-rt", "BNnNetwork", "BNNetwork"),
    drv("nn", "nn-wb", "BNnWbNetwork", "BNNetwork"),
    drv("alarm", "alarm-rt", "BAlarmService", "BAbstractService"),
]

DRV_CHAINS = {
    "BDdfNetwork": ["BDeviceNetwork", "BComponent"], "BNNetwork": ["BDeviceNetwork", "BComponent"],
    "BDeviceNetwork": ["BComponent"], "BDevice": ["BComponent"], "BPointDeviceExt": ["BComponent"],
    "BProxyExt": ["BComponent"], "BSimpleJob": ["BJob", "BComponent"], "BJob": ["BComponent"],
    "BDeviceManager": ["BComponent"], "BAbstractManager": ["BComponent"],
    "BAcmeNetwork": ["BDdfNetwork", "BDeviceNetwork", "BComponent"],
    "BAcmeProxyExt": ["BProxyExt", "BComponent"],
}

DRV_INVENTORY = {
    "acme-rt": {"module": "acme", "type": "rt"}, "acme-wb": {"module": "acme", "type": "wb"},
    "acme-ux": {"module": "acme", "type": "ux"}, "acmeTcp-rt": {"module": "acmeTcp", "type": "rt"},
    "solo-rt": {"module": "solo", "type": "rt"}, "nn-rt": {"module": "nn", "type": "rt"},
    "alarm-rt": {"module": "alarm", "type": "rt"}, "docAcme-doc": {"module": "docAcme", "type": "doc"},
    "docAc-doc": {"module": "docAc", "type": "doc"}, "docNn-doc": {"module": "docNn", "type": "doc"},
    "docSource-doc": {"module": "docSource", "type": "doc"},
}

DRV_SOURCES = {
    "BAcmeNetwork": "public class BAcmeNetwork extends BDdfNetwork {\n"
                    "  public Type getDeviceType() { return BAcmeDevice.TYPE; }\n"
                    "  void check() { Sys.getLicenseManager().checkFeature(\"acme\", \"acmeDriver\"); }\n}\n",
    "BAcmeTcpNetwork": "public class BAcmeTcpNetwork extends BAcmeNetwork {\n"
                       "  void check() { Sys.getLicenseManager().getFeature(null, \"acmeTcp\"); "
                       "Sys.getLicenseManager().checkFeature(\"\", \"blankVendor\"); }\n}\n",
    "BSoloNetwork": "public abstract class BSoloNetwork {\n  public Type getDeviceType() { return BSoloDevice.TYPE; }\n"
                    "  public Type getDeviceType  ( ) {\n return\tBOther$Device.TYPE; }\n}\n",
}

DRV_LEXICONS = {
    "acme/acme-rt/extracted/acme.lexicon": b"help.guide.base=module\\://docAcme/doc\n",
    "acme/acme-rt/extracted/acme_fr.lexicon": b"help.guide.base=module://docFr/doc\n",
    "acme/acme-wb/extracted/acme.lexicon": b"help.guide.base = module://docAcmeWb/doc\nhelp.guide.base: second\\ value \r\n",
    "acme/acme-wb/extracted/acmeWb.lexicon": b"help.guide.base=module://ignored/doc\n",
    "solo/solo-rt/extracted/solo.lexicon": b"  help.guide.base\t:\tmodule://docMissing/doc\n",
    "nn/nn-rt/extracted/nn.lexicon": b"title=caf\xe9\nhelp.guide.base = module\\://docNn/doc   \n",
}


def build_drivers():
    fx = os.path.join(HERE, "fixture")
    idx = os.path.join(fx, "drivers", "indexes")
    org = os.path.join(fx, "drivers-organized")
    for d in (os.path.join(fx, "drivers"), org):
        shutil.rmtree(d, ignore_errors=True)
    os.makedirs(idx)
    classes = {}
    for name, e in DRV_CLASSES:
        classes.setdefault(name, []).append(e)
    with open(os.path.join(idx, "class-index.json"), "w", encoding="utf-8") as f:
        json.dump({"_meta": {"timestamp": "2026-02-03T04:05:06"}, "classes": classes}, f)
    with open(os.path.join(idx, "inheritance.json"), "w", encoding="utf-8") as f:
        json.dump({"_meta": {}, "parent_to_children": {}, "class_to_chain": DRV_CHAINS}, f)
    with open(os.path.join(idx, "module-inventory.json"), "w", encoding="utf-8") as f:
        json.dump({"_meta": {}, "modules": DRV_INVENTORY}, f)
    conn = sqlite3.connect(os.path.join(idx, "token-index.db"))
    conn.executescript("CREATE TABLE files (id INTEGER PRIMARY KEY, path TEXT NOT NULL, module TEXT NOT NULL, "
                       "class_name TEXT NOT NULL); CREATE TABLE postings (token TEXT NOT NULL, file_id INTEGER "
                       "NOT NULL, line_no INTEGER NOT NULL); CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT "
                       "NOT NULL); INSERT INTO meta VALUES ('timestamp', '2026-02-03T04:05:06');")
    for name, e in DRV_CLASSES:
        if name in DRV_SOURCES and e["module"] != "docSource-doc":
            path = os.path.join(org, e["path"])
            os.makedirs(os.path.dirname(path), exist_ok=True)
            with open(path, "w", encoding="utf-8") as f:
                f.write(DRV_SOURCES[name])
            cur = conn.execute("INSERT INTO files (path, module, class_name) VALUES (?, ?, ?)",
                               (e["path"], e["module"], name))
            conn.execute("INSERT INTO postings VALUES ('checkFeature', ?, 2)", (cur.lastrowid,))
    conn.commit()
    conn.execute("VACUUM")
    conn.close()
    for rel, data in DRV_LEXICONS.items():
        os.makedirs(os.path.dirname(os.path.join(org, rel)), exist_ok=True)
        with open(os.path.join(org, rel), "wb") as f:
            f.write(data)
    os.makedirs(os.path.join(org, "docNn"))
    with open(os.path.join(org, "docNn", "README"), "w") as f:
        f.write("docNn exists in organized/ (help_doc_module_exists)\n")
    fresh_class_index()


# search: a corpus for grep, source and token. BAlarmService mixes CRLF, a
# lone CR, an invalid UTF-8 byte, trailing blanks and a long line; Entry
# exists in two packages of one submodule (a token tie on module, class and
# line); BGhost is indexed but not on disk; twelve modules hold ackAlarm
# postings of files absent from the corpus.
SRC_FILES = {
    "alarm/alarm-rt/vineflower/javax/baja/alarm/BAlarmService.java":
        b"package javax.baja.alarm;\r\n\r\npublic class BAlarmService {\r\n"
        b"  public void ackAlarm(BAlarmRecord r) { this.ackAlarm(r, null); }\r\n"
        b"  void line5() {}   \r\n  // lone cr\rline7\n  String s = \"caf\xe9\";\n"
        b"  void veryLong() { ackAlarm(" + b"x" * 140 + b"); }\n"
        b"  class Inner {}\n  void ackAlarm(BAlarmRecord r, Object o) {\n    // ackAlarm twice\n  }\n}",
    "alarm/alarm-rt/vineflower/javax/baja/alarm/BAckState.java":
        b"package javax.baja.alarm;\n\npublic final class BAckState { // \xc3\x91and\xc3\xba\n}\n",
    "alarm/alarm-rt/vineflower/javax/baja/alarm/Entry.java": b"package javax.baja.alarm;\nclass X {\n  Entry e;\n}\n",
    "alarm/alarm-rt/vineflower/com/x/Entry.java": b"package com.x;\nclass Y {\n  Entry e;\n}\n",
    "alarm/alarm-rt/vineflower/javax/baja/alarm/package-info.java": b"package javax.baja.alarm; // ackAlarm\n",
    "alarm/alarm-se/vineflower/com/x/BPrinter.java":
        b"package com.x;\npublic class BPrinter {\n  void p() { ackAlarm(); }\n  void ackAlarm() {}\n}\n",
    "alarm/alarm/vineflower/Standalone.java": b"class Standalone {}\n",
    "other/other-zz/vineflower/Odd.java": b"// odd\n",
}


def src_entry(sub, pkg, name, lines, kind="class", outer=None, path=None):
    module = sub.split("-")[0]
    return {"package": pkg, "module": sub, "kind": kind, "lines": lines, "outer_class": outer,
            "path": path or "{}/{}/vineflower/{}/{}.java".format(module, sub, pkg.replace(".", "/"), name)}


def build_search():
    import re
    fx = os.path.join(HERE, "fixture")
    idx = os.path.join(fx, "search", "indexes")
    org = os.path.join(fx, "search-organized")
    for d in (os.path.join(fx, "search"), org):
        shutil.rmtree(d, ignore_errors=True)
    os.makedirs(idx)
    for rel, data in SRC_FILES.items():
        os.makedirs(os.path.dirname(os.path.join(org, rel)), exist_ok=True)
        with open(os.path.join(org, rel), "wb") as f:
            f.write(data)
    svc = src_entry("alarm-rt", "javax.baja.alarm", "BAlarmService", 1234)
    classes = {
        "BAlarmService": [svc],
        "Inner": [dict(svc, outer_class="BAlarmService", kind="class", lines=1)],
        "BAckState": [src_entry("alarm-rt", "javax.baja.alarm", "BAckState", 4)],
        "Entry": [src_entry("alarm-rt", "javax.baja.alarm", "Entry", 4, outer="X"),
                  src_entry("alarm-rt", "com.x", "Entry", 4)],
        "BPrinter": [src_entry("alarm-se", "com.x", "BPrinter", 5)],
        "BGhost": [src_entry("alarm-rt", "javax.baja.alarm", "BGhost", 9)],
    }
    with open(os.path.join(idx, "class-index.json"), "w", encoding="utf-8") as f:
        json.dump({"_meta": {"timestamp": "2026-03-04T05:06:07", "modules": 3}, "classes": classes}, f)
    conn = sqlite3.connect(os.path.join(idx, "token-index.db"))
    conn.executescript("CREATE TABLE files (id INTEGER PRIMARY KEY, path TEXT NOT NULL, module TEXT NOT NULL, "
                       "class_name TEXT NOT NULL); CREATE TABLE postings (token TEXT NOT NULL, file_id INTEGER "
                       "NOT NULL, line_no INTEGER NOT NULL); CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT "
                       "NOT NULL); CREATE INDEX idx_token ON postings(token); "
                       "INSERT INTO meta VALUES ('timestamp', '2026-03-04T05:06:07'), "
                       "('source_root', '/nonexistent/organized');")
    for rel, data in SRC_FILES.items():
        sub = rel.split("/")[1]
        cur = conn.execute("INSERT INTO files (path, module, class_name) VALUES (?, ?, ?)",
                           (rel, sub, os.path.splitext(os.path.basename(rel))[0]))
        text = data.decode("utf-8", "replace").replace("\r\n", "\n").replace("\r", "\n")
        for n, line in enumerate(text.split("\n"), 1):
            for tok in sorted(set(re.findall(r"[A-Za-z_]\w+", line))):
                conn.execute("INSERT INTO postings VALUES (?, ?, ?)", (tok, cur.lastrowid, n))
    for i in range(12):
        sub = "m{:02d}-rt".format(11 - i)
        for j in range(i % 3 + 1):
            cur = conn.execute("INSERT INTO files (path, module, class_name) VALUES (?, ?, ?)",
                               ("m/{}/vineflower/C{}.java".format(sub, j), sub, "BAVeryLongClassNameThatIsCutAt35Chars{}".format(j)))
            conn.execute("INSERT INTO postings VALUES ('ackAlarm', ?, ?)", (cur.lastrowid, 10 + j))
    conn.execute("INSERT INTO postings VALUES ('AckAlarmRequest', 1, 99), ('xackalarmy', 1, 98)")
    conn.commit()
    conn.execute("VACUUM")
    conn.close()


def fresh_class_index():
    """Date the drivers and search class indexes an hour ahead so that they are
    never older than their corpus (no staleness warning); golden_test.go does
    the same after a checkout, which gives every file the same mtime."""
    future = time.time() + 3600
    for base in ("drivers", "search"):
        path = os.path.join(HERE, "fixture", base, "indexes", "class-index.json")
        os.utime(path, (future, future))


# license-feature(s) and resources: a token index over three candidate files
# (direct, variable and helper calls, every unresolved site reason, an
# unreadable file), small method and class indexes for the miss hints, and a
# JAR with resources by extension (a cp437 name, a directory entry, a file
# without extension); lic-wb holds a JAR that is not a zip.
LIC_SOURCES = {
    "lic/lic-rt/vineflower/com/x/BLicNetwork.java": """package com.x;
public class BLicNetwork {
  static final String VENDOR = "acme";
  void a() { Sys.getLicenseManager().checkFeature(VENDOR, "licFeat"); }
  void b() { LicenseManager lm = Sys.getLicenseManager(); lm.getFeature("Acme", "otherFeat"); }
  void c() { Sys.getLicenseManager().getFeatures(); }
  void d() { Sys.getLicenseManager().checkFeature(vendorVar, featureVar); }
  void e() { use(Sys.getLicenseManager()); }
}
""",
    "lic/lic-wb/vineflower/com/x/BLicView.java": """package com.x;
class BLicView {
  void a() { Sys.getLicenseManager().checkFeature("tridium", "licFeat"); }
  void b() { LicenseManager unused = Sys.getLicenseManager(); }
}
""",
    "other/other-rt/vineflower/com/y/Helper.java": """class Helper {
  void a() { Helper.checkFeature(null, "licFeat"); }
}
""",
}

LIC_JAR = ["META-INF/MANIFEST.MF", "META-INF/", "com/x/BLicNetwork.class", "com/x/BLicView.class",
           "module.palette", "lic-rt.lexicon", "com/x/icons/a.png", "com/x/icons/b.PNG", "README",
           "dir.d/NOTICE", "~t~.png"]  # ~t~ becomes cp437 bytes (no UTF-8 flag): "été"


def build_lic():
    import zipfile
    fx = os.path.join(HERE, "fixture")
    idx = os.path.join(fx, "lic", "indexes")
    org = os.path.join(fx, "lic-organized")
    for d in (os.path.join(fx, "lic"), org):
        shutil.rmtree(d, ignore_errors=True)
    os.makedirs(idx)
    conn = sqlite3.connect(os.path.join(idx, "token-index.db"))
    conn.executescript("CREATE TABLE files (id INTEGER PRIMARY KEY, path TEXT NOT NULL, module TEXT NOT NULL, "
                       "class_name TEXT NOT NULL); CREATE TABLE postings (token TEXT NOT NULL, file_id INTEGER "
                       "NOT NULL, line_no INTEGER NOT NULL); CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT "
                       "NOT NULL); INSERT INTO meta VALUES ('timestamp', '2026-03-04T05:06:07');")
    for rel in sorted(LIC_SOURCES) + ["lic/lic-rt/vineflower/com/x/Gone.java"]:
        cur = conn.execute("INSERT INTO files (path, module, class_name) VALUES (?, ?, ?)",
                           (rel, rel.split("/")[1], os.path.basename(rel)[:-5]))
        conn.execute("INSERT INTO postings VALUES ('getLicenseManager', ?, 3)", (cur.lastrowid,))
        conn.execute("INSERT INTO postings VALUES ('checkFeature', ?, 4)", (cur.lastrowid,))
        if rel in LIC_SOURCES:
            path = os.path.join(org, rel)
            os.makedirs(os.path.dirname(path), exist_ok=True)
            with open(path, "w", encoding="utf-8") as f:
                f.write(LIC_SOURCES[rel])
    conn.commit()
    conn.execute("VACUUM")
    conn.close()
    with open(os.path.join(idx, "method-index.json"), "w", encoding="utf-8") as f:
        json.dump({"_meta": {}, "methods": {"getFeature": [], "featureOf": [], "feat": [], "Feature": []}}, f)
    with open(os.path.join(idx, "class-index.json"), "w", encoding="utf-8") as f:
        json.dump({"_meta": {"timestamp": "2026-03-04T05:06:07"}, "classes": {"BFeatureView": []}}, f)
    jar = os.path.join(org, "lic", "lic-rt", "lic-rt.jar")
    with zipfile.ZipFile(jar, "w") as zf:
        for name in LIC_JAR:
            info = zipfile.ZipInfo(name, date_time=(2026, 1, 2, 3, 4, 6))
            zf.writestr(info, b"" if name.endswith("/") else name.encode("utf-8"))
    with open(jar, "rb") as f:
        data = f.read()
    with open(jar, "wb") as f:
        f.write(data.replace(b"~t~.png", b"\x82t\x82.png"))
    with open(os.path.join(org, "lic", "lic-wb", "lic-wb.jar"), "wb") as f:
        f.write(b"not a zip file\n")
