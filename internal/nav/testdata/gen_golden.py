"""Regenerate the fixture-backed golden cases of the ported mnav commands.

    python3 -I internal/nav/testdata/gen_golden.py [--fixture]

--fixture first rebuilds fixture/ (a small class index and module inventory cut
from the real module-navigator indexes, plus a synthetic module.xml); then
every case below is run through Python module_nav.py from this directory
(COLUMNS=100, NO_COLOR=1) and written to golden/<case>/: the nav-<case>
command runs on the fixture, and the parser-only PARSE_CASES.
"""

import glob
import json
import os
import shutil
import sqlite3
import subprocess
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
NAV = os.path.expanduser("~/niagara-research/module-navigator")
CLI = os.path.join(NAV, "tools", "module_nav.py")
BASE = "fixture/nav"
NO_CORPUS = {"NAV_CORPUS_BASE": "fixture/no-corpus"}
DRV = {"NAV_CORPUS_BASE": "fixture/drivers-organized"}
DRV_BASE = "fixture/drivers"
CORPUS = {"NAV_CORPUS_BASE": "fixture/organized"}

CLASS_NAMES = ["BAlarmService", "BAlarmClass", "BAlarmRecord", "AlarmDbConnection", "BAckState",
               "AlarmSupport", "BAlarmArchive", "BAlarmablePointMarkerTag", "BAlarm", "Entry",
               "BAlarmConsole", "BComponent", "BAlarmSourceInfo", "BAlarmAcknowledger"]
MODULES = ["alarm-rt", "alarm-ux", "alarm-wb", "alarm-se", "alarmOrion-rt", "apachePoi-rt",
           "docSource-doc", "baja", "backup-rt", "opcUaCore-rt", "obix-se", "niagaraTest-se", "lonNico-rt"]

# Python fails on these with a traceback; the port prints what the case's
# intended.json holds instead.
INTENDED = {
    "module-null-bytecode": {
        "reason": "Python raises TypeError comparing a null bytecode with 45 after printing half "
                  "the detail block; the port prints the whole block with 'vNone (Java ?)'",
        "stdout": "\n  Module:      lonNico-rt\n  Parent:      lonNico\n  Type:        rt\n"
                  "  JAR:         lonNico-rt.jar\n  Has code:    no\n  Class count: 0\n"
                  "  Java files:  0\n  ZKM:         YES\n  Bytecode:    vNone (Java ?)\n"
                  "  Vineflower:  no\n  CFR:         yes\n\n",
        "stderr": "",
        "exit": 0,
    },
}

MODULE_XML = """<?xml version="1.0" encoding="UTF-8"?>
<module name="alarm-rt" vendor="Tridium" preferredSymbol="alarm" moduleName="alarm">
  <dependencies>
    <dependency name="baja" vendor="Tridium" vendorVersion="4.14.0"/>
    <dependency name="aVeryLongDependencyModuleNameThatIsCut-rt" vendor="TridiumVendorLong" vendorVersion="4.14.0"/>
    <dependency name="control-rt" vendor="Tridium"/>
  </dependencies>
  <permissions>
    <niagara-permission-groups type="station"/>
    <java-permissions type="workbench">
      <java-permission class="java.lang.RuntimePermission" name="loadLibrary.alarmDialog"/>
      <java-permission class="com.tridium.nre.security.NiagaraSocketPermission" name="*:1-100000" action="connect,resolve"/>
    </java-permissions>
    <java-permissions type="station">
      <java-permission class="java.io.FilePermission" name="${protected.station.home}/-" action="read"/>
    </java-permissions>
  </permissions>
</module>
"""

CASES = [
    ("class", ["class", "BAlarmService"], NO_CORPUS),
    ("class-detail", ["class", "BAlarmConsole"], NO_CORPUS),
    ("class-fqn", ["class", "javax.baja.alarm.BAlarmService"], NO_CORPUS),
    ("class-case", ["class", "balarmservice"], NO_CORPUS),
    ("class-miss", ["class", "javax.baja.BNoSuchClassXyz"], NO_CORPUS),
    ("search", ["search", "BAlarm*", "-n", "4"], NO_CORPUS),
    ("search-miss", ["search", "*Nothing*"], NO_CORPUS),
    ("package", ["package", "javax.baja.alarm", "-n", "3"], NO_CORPUS),
    ("package-sub", ["package", "javax.baja"], NO_CORPUS),
    ("package-all", ["package", "--all"], NO_CORPUS),
    ("package-miss", ["package", "org.nothing"], NO_CORPUS),
    ("package-usage", ["package"], NO_CORPUS),
    ("search-negative-n", ["search", "BAlarm*", "-n", "-2"], NO_CORPUS),
    ("package-all-negative-n", ["package", "--all", "-n", "-1"], NO_CORPUS),
    ("package-zero-n", ["package", "javax.baja.alarm", "-n", "0"], NO_CORPUS),
    ("inventory", ["inventory"], CORPUS),
    ("module", ["module", "alarm-rt"], CORPUS),
    ("module-multiple", ["module", "alarm"], CORPUS),
    ("module-miss", ["module", "alarmz"], CORPUS),
    ("module-deps", ["module", "alarm-rt", "--dependencies"], CORPUS),
    ("module-perms", ["module", "alarm-rt", "--permissions", "--dependencies"], CORPUS),
    ("module-xml-missing", ["module", "backup-rt", "--permissions"], CORPUS),
    ("modules-type", ["modules", "--type", "rt"], CORPUS),
    ("modules-zkm-top", ["modules", "--zkm", "--top", "2"], CORPUS),
    ("modules-bytecode", ["modules", "-b", "52", "--has-code"], CORPUS),
    ("module-null-bytecode", ["module", "lonNico-rt"], CORPUS),
    ("rank", ["rank", "como", "funciona el ack de alarmas", "-n", "3"], NO_CORPUS),
    ("rank-exact", ["rank", "que es un AlarmService y un BAckState", "-n", "4"], NO_CORPUS),
    ("rank-all", ["rank", "alarm"], NO_CORPUS),
    ("rank-accents", ["rank", "configuración de la consola", "--limit", "2"], NO_CORPUS),
    ("rank-miss", ["rank", "zzzqqq", "xxyyzz"], NO_CORPUS),
    ("rank-no-terms", ["rank", "the a of"], NO_CORPUS),
    ("rank-skips", ["rank", "alarm service"], NO_CORPUS, "fixture/rank-skips"),
    ("hierarchy", ["hierarchy", "BAlarmRecipient"], NO_CORPUS),
    ("hierarchy-depth1", ["hierarchy", "BComponent", "--depth", "1"], NO_CORPUS),
    ("hierarchy-depth0", ["hierarchy", "BAlarmRecipient", "--depth", "0"], NO_CORPUS),
    ("hierarchy-negative", ["hierarchy", "BAlarmRecipient", "--depth", "-1"], NO_CORPUS),
    ("hierarchy-leaf", ["hierarchy", "BAlarmService"], NO_CORPUS),
    ("hierarchy-miss", ["hierarchy", "BAlarmServce"], NO_CORPUS),
    ("hierarchy-chain", ["hierarchy", "BAlarmService", "--chain"], NO_CORPUS),
    ("hierarchy-chain-root", ["hierarchy", "BAlarmAcknowledger", "--chain"], NO_CORPUS),
    ("hierarchy-chain-miss", ["hierarchy", "BNoSuchThing", "--chain"], NO_CORPUS),
    ("implementors", ["implementors", "BIAlarmSource", "-n", "2"], NO_CORPUS),
    ("implementors-case", ["implementors", "biservice"], NO_CORPUS),
    ("implementors-zero", ["implementors", "BIService", "-n", "0"], NO_CORPUS),
    ("implementors-class", ["implementors", "BComponent"], NO_CORPUS),
    ("implementors-miss", ["implementors", "BIAlarmSorce"], NO_CORPUS),
    ("drivers", ["drivers"], DRV, DRV_BASE),
    ("drivers-module", ["drivers", "acme-rt"], DRV, DRV_BASE),
    ("drivers-dir", ["drivers", "acmeTcp"], DRV, DRV_BASE),
    ("drivers-json", ["drivers", "solo", "--json"], DRV, DRV_BASE),
    ("drivers-json-nn", ["drivers", "nn-rt", "--json"], DRV, DRV_BASE),
    ("drivers-miss", ["drivers", "alarm-rt"], DRV, DRV_BASE),
    ("drivers-miss-unknown", ["drivers", "acme-r"], DRV, DRV_BASE),
    ("drivers-miss-json", ["drivers", "alarm", "--json"], DRV, DRV_BASE),
    ("drivers-no-corpus", ["drivers"], NO_CORPUS, DRV_BASE),
    ("ask", ["ask", "who calls ackAlarm"], NO_CORPUS),
    ("ask-n", ["ask", "-n", "2", "como funciona el ack de alarmas"], NO_CORPUS),
]


# Parser-only cases (usage errors and help) against a base directory that does
# not exist, like the S0 cases frozen with them.
PARSE_CASES = [
    ("err-ask-positive-int", ["ask", "q", "-n", "-3"]),
    ("err-selftest-sample", ["selftest", "--sample", "x"]),
    ("err-token-context-int", ["token", "foo", "--context", "abc"]),
    ("err-token-context-float", ["token", "foo", "--context", "1.5"]),
    ("err-session-bad-sub", ["session", "bogus"]),
    ("err-session-save-missing", ["session", "save"]),
    ("err-session-export-bad-as", ["session", "export", "x", "--as", "yaml"]),
    ("help-repl", ["repl", "--help"]),
    ("help-alarm-types", ["alarm-types", "--help"]),
]


# rank-skips: the cases class_rank skips or tolerates (a mirror copy, an inner
# class, a null module and path, empty interfaces) and an xref index without
# class_imported_by (every prior is 1).
SKIPS_INDEX = {
    "_meta": {"timestamp": "2026-01-02T03:04:05"},
    "classes": {
        "BAlarmService": [
            {"module": "docSource-doc", "package": "javax.baja.alarm", "path": "doc/BAlarmService.java"},
            {"module": "alarm-rt", "package": "javax.baja.alarm", "extends": "BAbstractService",
             "implements": ["BIAlarmClassFolder", None, ""], "path": "alarm/alarm-rt/BAlarmService.java"}],
        "Inner": [{"module": "alarm-rt", "outer_class": "BAlarmService", "path": "x"}],
        "ServiceAlarm": [{"module": None, "package": "", "path": None}],
        "BServiceAlarm": [{"module": "b-rt", "package": "b.alarm"}, {"module": "a-rt", "package": "b.alarm"}],
    },
}


def cut_xref(names):
    """class_imported_by of `names` from the real xref index, each list cut to
    25 importers plus one repeat (class_rank counts distinct importers)."""
    with open(os.path.join(NAV, "indexes", "xref-index.json"), encoding="utf-8") as f:
        xref = json.load(f)
    out = {}
    for n in names:
        importers = xref["class_imported_by"].get(n)
        if importers:
            out[n] = importers[:25] + importers[:1]
    return {"_meta": {"timestamp": xref["_meta"]["timestamp"]}, "class_imported_by": out}


def cut_inheritance(names):
    """inheritance.json restricted to `names` plus the descendants of
    BAlarmRecipient and the chains of `names`: children and implementors
    lists keep only kept classes (a small tree with real shapes)."""
    with open(os.path.join(NAV, "indexes", "inheritance.json"), encoding="utf-8") as f:
        inh = json.load(f)
    p2c, c2c, i2i = inh["parent_to_children"], inh["class_to_chain"], inh["interface_to_implementors"]
    keep, todo = set(names), ["BAlarmRecipient"]
    while todo:
        n = todo.pop()
        if n not in keep or n == "BAlarmRecipient":
            keep.add(n)
            todo += [c for c in p2c.get(n, []) if c not in keep]
    for n in list(keep):
        keep.update(c2c.get(n, []))
    cut = lambda d: {k: [x for x in v if x in keep] for k, v in d.items()
                     if k in keep and any(x in keep for x in v)}
    i2i_keep = {k: v for k, v in cut(i2i).items() if k in ("BIAlarmSource", "BIService", "BIAlarmClassFolder")}
    for k in ("BIAlarmSource", "BIService", "BIAlarmClassFolder"):
        i2i_keep.setdefault(k, [x for x in i2i.get(k, []) if x in keep] or i2i.get(k, [])[:12])
    return {"_meta": {"timestamp": inh["_meta"]["timestamp"]}, "parent_to_children": cut(p2c),
            "interface_to_implementors": i2i_keep,
            "class_to_chain": {k: v for k, v in c2c.items() if k in keep}}


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


def fresh_class_index():
    """Date the drivers class index an hour ahead so that it is never older than
    the corpus (no staleness warning); golden_test.go does the same after a
    checkout, which gives every file the same mtime."""
    path = os.path.join(HERE, "fixture", "drivers", "indexes", "class-index.json")
    future = time.time() + 3600
    os.utime(path, (future, future))


def build_fixture():
    fx = os.path.join(HERE, "fixture")
    shutil.rmtree(fx, ignore_errors=True)
    idx = os.path.join(fx, "nav", "indexes")
    os.makedirs(idx)
    with open(os.path.join(NAV, "indexes", "class-index.json"), encoding="utf-8") as f:
        ci = json.load(f)
    meta = {k: ci["_meta"][k] for k in ("description", "modules", "timestamp")}
    classes = {n: ci["classes"][n] for n in CLASS_NAMES}
    with open(os.path.join(idx, "class-index.json"), "w", encoding="utf-8") as f:
        json.dump({"_meta": meta, "classes": classes}, f)
    with open(os.path.join(NAV, "indexes", "module-inventory.json"), encoding="utf-8") as f:
        inv = json.load(f)
    mods = {n: inv["modules"][n] for n in MODULES}
    with open(os.path.join(idx, "module-inventory.json"), "w", encoding="utf-8") as f:
        json.dump({"_meta": inv["_meta"], "modules": mods}, f)
    with open(os.path.join(idx, "xref-index.json"), "w", encoding="utf-8") as f:
        json.dump(cut_xref(CLASS_NAMES), f)
    with open(os.path.join(idx, "inheritance.json"), "w", encoding="utf-8") as f:
        json.dump(cut_inheritance(CLASS_NAMES), f)
    skips = os.path.join(fx, "rank-skips", "indexes")
    os.makedirs(skips)
    with open(os.path.join(skips, "class-index.json"), "w", encoding="utf-8") as f:
        json.dump(SKIPS_INDEX, f)
    with open(os.path.join(skips, "xref-index.json"), "w", encoding="utf-8") as f:
        json.dump({"_meta": {}}, f)
    xml = os.path.join(fx, "organized", "alarm", "alarm-rt", "extracted", "META-INF")
    os.makedirs(xml)
    with open(os.path.join(xml, "module.xml"), "w", encoding="utf-8") as f:
        f.write(MODULE_XML)


def capture(cid, args, env, base=BASE):
    argv = ["--base-dir", base] + args
    full_env = dict(os.environ, COLUMNS="100", NO_COLOR="1", **env)
    p = subprocess.run([sys.executable, CLI] + argv, cwd=HERE, env=full_env,
                       stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    d = os.path.join(HERE, "golden", cid)
    shutil.rmtree(d, ignore_errors=True)
    os.makedirs(d)
    with open(os.path.join(d, "cmd.json"), "w", encoding="utf-8") as f:
        json.dump({"args": argv, "env": env} if env else {"args": argv}, f)
        f.write("\n")
    for name, data in (("stdout", p.stdout), ("stderr", p.stderr)):
        if data or name == "stdout":
            with open(os.path.join(d, name), "wb") as f:
                f.write(data)
    with open(os.path.join(d, "exit"), "w") as f:
        f.write("%d\n" % p.returncode)
    if cid[len("nav-"):] in INTENDED:
        with open(os.path.join(d, "intended.json"), "w", encoding="utf-8") as f:
            json.dump(INTENDED[cid[len("nav-"):]], f, indent=1)
            f.write("\n")


def main():
    if "--fixture" in sys.argv:
        build_fixture()
        build_drivers()
    fresh_class_index()
    for cid, args, env, *base in CASES:
        capture("nav-" + cid, args, env, *base)
    for cache in glob.glob(os.path.join(HERE, "fixture", "*", "indexes", "class-rank.cache")):
        os.remove(cache)  # Python's rank cache; the Go port keeps its own
    for cid, args in PARSE_CASES:
        capture(cid, args, {}, base="/nonexistent-base")


if __name__ == "__main__":
    main()
