"""Regenerate the fixture-backed golden cases of the ported mnav commands.

    python3 -I internal/nav/testdata/gen_golden.py [--fixture | --search]

--fixture first rebuilds fixture/ (a small class index and module inventory cut
from the real module-navigator indexes, plus a synthetic module.xml), and
--search rebuilds only the synthetic grep/source/token corpus; then
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
SRC = {"NAV_CORPUS_BASE": "fixture/search-organized"}
SRC_BASE = "fixture/search"
LIC = {"NAV_CORPUS_BASE": "fixture/lic-organized"}
LIC_BASE = "fixture/lic"
CG_BASE = "fixture/cg"
STATS_BASE = "fixture/stats/base"
PLA_BASE = "fixture/pla"
RECALL = {"NAV_CORPUS_BASE": "fixture/recall/organized"}
RECALL_CLEAN = {"NAV_CORPUS_BASE": "fixture/recall/clean"}
RECALL_BASE = "fixture/recall/base"

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
    ("grep", ["grep", "ackAlarm", "-n", "3"], SRC, SRC_BASE),
    ("grep-all", ["grep", "ackAlarm|Ñandú"], SRC, SRC_BASE),
    ("grep-module", ["grep", "class", "-m", "alarm-se"], SRC, SRC_BASE),
    ("grep-type-other", ["grep", ".", "--type", "other"], SRC, SRC_BASE),
    ("grep-type-standalone", ["grep", "class", "-t", "standalone"], SRC, SRC_BASE),
    ("grep-newlines", ["grep", r";\s$|\\r|line\d$", "-m", "alarm-rt"], SRC, SRC_BASE),
    ("grep-long-line", ["grep", "veryLong", "-n", "1"], SRC, SRC_BASE),
    ("grep-miss", ["grep", "zzNothing", "-t", "rt"], SRC, SRC_BASE),
    ("grep-zero-n", ["grep", "class", "-n", "0"], SRC, SRC_BASE),
    ("grep-bad-regex", ["grep", "ack(Alarm"], SRC, SRC_BASE),
    ("grep-no-corpus", ["grep", "ackAlarm"], NO_CORPUS, SRC_BASE),
    ("source", ["source", "BAlarmService"], SRC, SRC_BASE),
    ("source-fqn-inner", ["source", "javax.baja.alarm.Inner"], SRC, SRC_BASE),
    ("source-fqn-other-package", ["source", "com.x.Entry", "--code"], SRC, SRC_BASE),
    ("source-case", ["source", "backstate", "--code"], SRC, SRC_BASE),
    ("source-miss", ["source", "BAckStat"], SRC, SRC_BASE),
    ("source-fqn-miss", ["source", "com.y.Entry"], SRC, SRC_BASE),
    ("source-code", ["source", "BAlarmService", "--code"], SRC, SRC_BASE),
    ("source-grep", ["source", "BAlarmService", "--grep", r"ack\w+\(|\s$", "--code"], SRC, SRC_BASE),
    ("source-grep-none", ["source", "BAckState", "--grep", "zz"], SRC, SRC_BASE),
    ("source-grep-bad", ["source", "BAckState", "--grep", "[z"], SRC, SRC_BASE),
    ("source-ghost", ["source", "BGhost"], SRC, SRC_BASE),
    ("source-ghost-code", ["source", "BGhost", "--code"], SRC, SRC_BASE),
    ("source-batch", ["source", "--batch", " BAckState,,BNope , Entry", "--code"], SRC, SRC_BASE),
    ("source-batch-empty", ["source", "--batch", " , "], SRC, SRC_BASE),
    ("source-usage", ["source"], SRC, SRC_BASE),
    ("source-no-corpus", ["source", "BAlarmService"], NO_CORPUS, SRC_BASE),
    ("source-no-corpus-code", ["source", "BAlarmService", "--code"], NO_CORPUS, SRC_BASE),
    ("token", ["token", "ackAlarm"], SRC, SRC_BASE),
    ("token-limit", ["token", "ackAlarm", "-n", "4"], SRC, SRC_BASE),
    ("token-module", ["token", "ackAlarm", "-m", "alarm-rt"], SRC, SRC_BASE),
    ("token-tie", ["token", "Entry"], SRC, SRC_BASE),
    ("token-module-miss", ["token", "ackAlarm", "-m", "nope-rt"], SRC, SRC_BASE),
    ("token-similar", ["token", "ackalar"], SRC, SRC_BASE),
    ("token-miss", ["token", "zzNothing"], SRC, SRC_BASE),
    ("token-context", ["token", "ackAlarm", "--context", "-n", "9"], SRC, SRC_BASE),
    ("token-context-wide", ["token", "ackAlarm", "--context", "5", "-m", "alarm-rt"], SRC, SRC_BASE),
    ("token-context-zero", ["token", "ackAlarm", "--context", "0", "-n", "2"], SRC, SRC_BASE),
    ("token-context-no-corpus", ["token", "ackAlarm", "--context", "1", "-n", "2"], NO_CORPUS, SRC_BASE),
    ("ask", ["ask", "who calls ackAlarm"], NO_CORPUS),
    ("ask-n", ["ask", "-n", "2", "como funciona el ack de alarmas"], NO_CORPUS),
    ("license-feature", ["license-feature", "LICFEAT"], LIC, LIC_BASE),
    ("license-feature-module", ["license-feature", "licFeat", "-m", "lic", "-n", "1"], LIC, LIC_BASE),
    ("license-feature-miss", ["license-feature", "feat"], LIC, LIC_BASE),
    ("license-feature-no-index", ["license-feature", "x"], LIC, "fixture/nav"),
    ("license-features", ["license-features"], LIC, LIC_BASE),
    ("license-features-n", ["license-features", "-n", "1"], LIC, LIC_BASE),
    ("license-features-module", ["license-features", "--module", "lic-rt"], LIC, LIC_BASE),
    ("license-features-module-miss", ["license-features", "-m", "nope"], LIC, LIC_BASE),
    ("license-features-unresolved", ["license-features", "--unresolved", "-n", "2"], LIC, LIC_BASE),
    ("license-features-no-corpus", ["license-features"], NO_CORPUS, LIC_BASE),
    ("resources", ["resources", "lic-rt"], LIC, LIC_BASE),
    ("resources-type", ["resources", "lic-rt.jar", "--type", "PNG", "-n", "1"], LIC, LIC_BASE),
    ("resources-type-none", ["resources", "lic-rt", "--type", "xml"], LIC, LIC_BASE),
    ("resources-n", ["resources", "lic-rt", "-n", "2"], LIC, LIC_BASE),
    ("resources-miss", ["resources", "lic"], LIC, LIC_BASE),
    ("resources-bad-jar", ["resources", "lic-wb"], LIC, LIC_BASE),
    ("resources-no-corpus", ["resources", "lic-rt"], NO_CORPUS, LIC_BASE),
    ("callers", ["callers", "BTarget", "run"], NO_CORPUS, CG_BASE),
    ("callers-n", ["callers", "BTarget", "run", "-n", "1"], NO_CORPUS, CG_BASE),
    ("callers-negative-n", ["callers", "BTarget", "run", "-n", "-1"], NO_CORPUS, CG_BASE),
    ("callers-case", ["callers", "btarget", "RUN"], NO_CORPUS, CG_BASE),
    ("callers-suffix", ["callers", "Inner", "copyInto"], NO_CORPUS, CG_BASE),
    ("callers-miss", ["callers", "BTarget", "go"], NO_CORPUS, CG_BASE),
    ("callers-class", ["callers", "BTarget"], NO_CORPUS, CG_BASE),
    ("callers-class-n", ["callers", "BTarget", "-n", "2"], NO_CORPUS, CG_BASE),
    ("callers-class-case", ["callers", "outer.inner"], NO_CORPUS, CG_BASE),
    ("callers-class-ambiguous", ["callers", "bcase"], NO_CORPUS, CG_BASE),
    ("callers-class-miss", ["callers", "BNothing"], NO_CORPUS, CG_BASE),
    ("callers-empty-graph", ["callers", "BTarget", "run"], NO_CORPUS, "fixture/cg-empty"),
    ("callees", ["callees", "BCaller", "work"], NO_CORPUS, CG_BASE),
    ("callees-n", ["callees", "BCaller", "work", "-n", "2"], NO_CORPUS, CG_BASE),
    ("callees-miss", ["callees", "BCaller", "nothing"], NO_CORPUS, CG_BASE),
    ("callees-class", ["callees", "BCaller"], NO_CORPUS, CG_BASE),
    ("callees-class-n", ["callees", "BCaller", "-n", "1"], NO_CORPUS, CG_BASE),
    ("callees-class-miss", ["callees", "caller"], NO_CORPUS, CG_BASE),
    ("callees-empty-method", ["callees", "BCaller", ""], NO_CORPUS, CG_BASE),
    ("stats", ["stats"], NO_CORPUS, STATS_BASE),
    ("stats-json", ["stats", "--json"], NO_CORPUS, STATS_BASE),
    ("stats-sparse", ["stats"], NO_CORPUS, "fixture/stats-sparse/base"),
    ("stats-sparse-json", ["stats", "--json"], NO_CORPUS, "fixture/stats-sparse/base"),
    ("palette-lexicon-agents", ["palette-lexicon-agents", "pal"], NO_CORPUS, PLA_BASE),
    ("palette-lexicon-agents-json", ["palette-lexicon-agents", "pal", "--json"], NO_CORPUS, PLA_BASE),
    ("palette-lexicon-agents-empty", ["palette-lexicon-agents", "empty"], NO_CORPUS, PLA_BASE),
    ("palette-lexicon-agents-empty-json", ["palette-lexicon-agents", "empty", "--json"], NO_CORPUS, PLA_BASE),
    ("palette-lexicon-agents-missing", ["palette-lexicon-agents", "nope"], NO_CORPUS, PLA_BASE),
    ("palette-lexicon-agents-missing-json", ["palette-lexicon-agents", "nope", "--json"], NO_CORPUS, PLA_BASE),
    ("selftest", ["selftest", "--recall"], RECALL, RECALL_BASE),
    ("selftest-sample", ["selftest", "--recall", "--sample", "3"], RECALL, RECALL_BASE),
    ("selftest-sample-all", ["selftest", "--recall", "--sample", "50"], RECALL, RECALL_BASE),
    ("selftest-clean", ["selftest", "--recall"], RECALL_CLEAN, RECALL_BASE),
    ("selftest-none", ["selftest", "--sample", "2"], RECALL, RECALL_BASE),
    ("selftest-no-corpus", ["selftest", "--recall"], NO_CORPUS, RECALL_BASE),
    ("selftest-no-index", ["selftest", "--recall"], RECALL, "fixture/recall/no-base"),
    ("selftest-bad-index", ["selftest", "--recall"], RECALL, "fixture/recall/bad-base"),
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


# callers and callees: a small call graph with a qualified callee filed under
# its dotted suffixes, a caller recording one callee twice (qualified and
# simple), 16 classes with a `go` method (the "... and N more" partial
# list), two classes differing only in case, a method with 12 callees, and
# an empty JSON object as the graph. (A missing index is an error naming its
# absolute path, which a frozen case cannot hold.)
def cg_calls():
    calls = {
        "BCaller.work": ["BTarget.run", "org.x.Outer.Inner.copyInto", "Outer.Inner.copyInto",
                         "BTarget.stop", "Helper.<init>", "util.Strings.join"],
        "BCaller.<init>": ["BTarget.<init>"],
        "BOther.run": ["BTarget.run", "BTarget.run", "BCase.get", "Bcase.get"],
        "BTarget.restart": ["BTarget.stop", "BTarget.run"],
        "Zeta.go": ["BTarget.run"],
        ".odd": ["noDots", "a..b", "trail."],
        "BMany.calls": ["Many{:02d}.go".format(i) for i in range(12)],
    }
    for i in range(16):
        calls["C{:02d}.go".format(i)] = ["K{:02d}.go".format(i)]
    return calls


def build_cg():
    fx = os.path.join(HERE, "fixture")
    for name in ("cg", "cg-empty"):
        shutil.rmtree(os.path.join(fx, name), ignore_errors=True)
        os.makedirs(os.path.join(fx, name, "indexes"))
    calls = cg_calls()
    cg = {"_meta": {"timestamp": "2026-04-05T06:07:08", "total_edges": sum(len(v) for v in calls.values())},
          "calls": calls}
    with open(os.path.join(fx, "cg", "indexes", "callgraph-index.json"), "w", encoding="utf-8") as f:
        json.dump(cg, f)
    with open(os.path.join(fx, "cg-empty", "indexes", "callgraph-index.json"), "w", encoding="utf-8") as f:
        f.write("{}")
    classes = {"BTarget": [{"module": "target-rt", "outer_class": "X"}, {"module": "target-wb"}],
               "BCaller": [{"module": "caller-rt"}], "BOther": [{"module": "other-rt", "outer_class": "Y"}],
               "Inner": [], "BCase": [{"module": "case-rt"}]}
    with open(os.path.join(fx, "cg", "indexes", "class-index.json"), "w", encoding="utf-8") as f:
        json.dump({"_meta": {"timestamp": "2026-04-05T06:07:08"}, "classes": classes}, f)


# stats: every index with a _meta block (floats, a non-ASCII name, a meta
# value Python formats with grouping), a field index with NaN (decoded whole,
# not from its prefix), an exceptions index with two _meta keys (the last
# wins), a token index and a string index that is not SQLite, a file that is
# not an index, and a niagara-help sibling; stats-sparse has only a module
# inventory with a null type, a broken class index and no help sibling.
STATS_METAS = {
    "swing-index.json": {"classes_with_ui": 3, "total_dialogs": 1, "total_dimensions": 1234,
                         "total_colors": 0, "total_fonts": 5, "total_icons": 6},
    "inheritance.json": {"unique_parents": 10, "unique_interfaces_implemented": 4, "classes_with_chains": 9,
                         "max_chain_depth": 7, "avg_chain_depth": 2.25, "with_extends": 8, "with_implements": 3},
    "xref-index.json": {"classes_with_imports": 1500, "classes_imported_by_others": 2, "total_import_statements": 3,
                        "resolved_imports": 4, "wildcard_imports": 5, "modules_with_deps": 6,
                        "modules_depended_on": 7, "total_class_import_edges": 8, "total_module_dep_edges": 9},
    "method-index.json": {"total_method_definitions": 1234567, "unique_method_names": 2, "total_constructors": 3,
                          "classes_with_methods": 4, "avg_methods_per_class": 12.35, "max_methods_per_class": 99,
                          "max_methods_class": "BÉtat"},
    "annotations-index.json": {"total_niagara_types": 1, "with_properties": 2},
    "callgraph-index.json": {"total_caller_methods": 11, "total_unique_callees": 12, "total_edges": 13,
                             "avg_callees_per_caller": 0.05},
}


def build_stats():
    fx = os.path.join(HERE, "fixture")
    for name in ("stats", "stats-sparse"):
        shutil.rmtree(os.path.join(fx, name), ignore_errors=True)
    idx = os.path.join(fx, "stats", "base", "indexes")
    os.makedirs(idx)
    with open(os.path.join(idx, "notes.txt"), "w", encoding="utf-8") as f:
        f.write("not an index\n")
    for name, meta in STATS_METAS.items():
        with open(os.path.join(idx, name), "w", encoding="utf-8") as f:
            json.dump({"_meta": meta, "data": {"x": [1, 2]}}, f)
    with open(os.path.join(idx, "field-index.json"), "w", encoding="utf-8") as f:
        f.write('{"_meta": {"total_field_definitions": 5, "avg_fields_per_class": NaN, '
                '"max_fields_class": "BMax"}, "fields": {}}')
    with open(os.path.join(idx, "exceptions-index.json"), "w", encoding="utf-8") as f:
        f.write('{"_meta": {"total_catch_blocks": 1}, "x": 1, "_meta": {"total_catch_blocks": 2}}')
    with open(os.path.join(idx, "class-index.json"), "w", encoding="utf-8") as f:
        json.dump({"_meta": {"top_level_classes": 3, "inner_classes": 0, "interfaces": 1, "unique_packages": 2},
                   "classes": {"A": [{}, {}], "B": [{}]}}, f)
    inv = {"a-rt": {"module": "a", "type": "rt", "has_code": True, "java_files": 1200, "class_count": 1300,
                    "zkm": False},
           "a-wb": {"module": "a", "type": "wb", "has_code": False, "java_files": 0, "class_count": 0, "zkm": True},
           "b": {"module": "b", "type": "", "has_code": True, "java_files": 3, "class_count": 4, "zkm": False}}
    with open(os.path.join(idx, "module-inventory.json"), "w", encoding="utf-8") as f:
        json.dump({"_meta": {}, "modules": inv}, f)
    conn = sqlite3.connect(os.path.join(idx, "token-index.db"))
    conn.executescript("CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL); INSERT INTO meta VALUES "
                       "('total_postings', '15528521'), ('unique_tokens', '7'), ('files_with_tokens', '8'), "
                       "('total_lines', '9');")
    conn.close()
    with open(os.path.join(idx, "string-index.db"), "wb") as f:
        f.write(b"not a database\n")
    help_idx = os.path.join(fx, "stats", "niagara-help", "indexes")
    os.makedirs(help_idx)
    for name, data in (("class-index.json", {"A": 1, "B": 2}), ("source-index.json", ["x"]),
                       ("slots-index.json", {}), ("other.json", {})):
        with open(os.path.join(help_idx, name), "w", encoding="utf-8") as f:
            json.dump(data, f)
    sparse = os.path.join(fx, "stats-sparse", "base", "indexes")
    os.makedirs(sparse)
    with open(os.path.join(sparse, "module-inventory.json"), "w", encoding="utf-8") as f:
        json.dump({"_meta": {}, "modules": {"z": {"module": "z", "type": None, "has_code": False,
                                                  "java_files": 0, "class_count": 0, "zkm": False}}}, f)
    with open(os.path.join(sparse, "class-index.json"), "w", encoding="utf-8") as f:
        f.write('{"_meta": {"top_level_classes": 1}, "classes": ')


# palette-lexicon-agents: module pal with an extracted artifact (nested and
# attribute-less <p>, a lexicon with duplicate keys, agents with and without
# <on>), a JAR-only artifact (palette and lexicon read from the JAR, a
# malformed module.xml ignored), an artifact whose palette is not well formed
# (a late XML declaration), 55 palette entries (the "... and N more" line) and
# a directory that is not an artifact; module empty has no artifact.
PLA_PALETTE = '''<?xml version="1.0" encoding="UTF-8"?>
<bajaObjectGraph version="1.0">
<p m="b=baja" t="b:Folder">
 <p n="Net" m="x=pal" t="x:Net"/>
 <p n="Caf\u00e9 '&amp;'"/>
 <p/>
</p>
</bajaObjectGraph>
'''
PLA_LEXICON = "# comment\nkey=1\n other = 2\nkey = 3\n\nnoequals\nother=4\nkey=5\n=x\n"
PLA_MODULE_XML = '''<module><types>
 <type name="Net" class="com.x.BNet"><agent><on type="x:A"/><on type="x:B"/></agent><agent/></type>
 <type name="It's" class="com.x.BQuote"/>
 <type name="View" class="com.x.BView"><agent><on/></agent></type>
</types></module>
'''


def build_pla():
    import zipfile
    base = os.path.join(HERE, "fixture", "pla")
    shutil.rmtree(base, ignore_errors=True)
    ext = os.path.join(base, "pal", "pal-rt", "extracted")
    os.makedirs(os.path.join(ext, "META-INF"))
    for name, text in (("module.palette", PLA_PALETTE), ("pal-rt.lexicon", PLA_LEXICON),
                       ("META-INF/module.xml", PLA_MODULE_XML)):
        with open(os.path.join(ext, name), "w", encoding="utf-8") as f:
            f.write(text)
    os.makedirs(os.path.join(base, "pal", "pal-wb"))
    with zipfile.ZipFile(os.path.join(base, "pal", "pal-wb", "pal-wb.jar"), "w") as zf:
        for name, text in (("module.palette", "<a>" + "".join('<p n="e%d"/>' % i for i in range(55)) + "</a>"),
                           ("pal-wb.lexicon", "a=1\r\na=2\r\n"), ("META-INF/module.xml", "<module>")):
            zf.writestr(zipfile.ZipInfo(name, date_time=(2026, 1, 2, 3, 4, 6)), text)
    bad = os.path.join(base, "pal", "pal-ux", "extracted")
    os.makedirs(bad)
    with open(os.path.join(bad, "module.palette"), "w", encoding="utf-8") as f:
        f.write("<!-- c -->\n<?xml version=\"1.0\"?><a><p n=\"x\"/></a>")
    os.makedirs(os.path.join(base, "pal", "docs"))
    with open(os.path.join(base, "pal", "README"), "w", encoding="utf-8") as f:
        f.write("not an artifact\n")
    os.makedirs(os.path.join(base, "empty", "notes"))
    with open(os.path.join(base, "empty", "notes", "x.txt"), "w", encoding="utf-8") as f:
        f.write("no jar, no extracted/\n")


# selftest --recall: one .java file per directory (os.walk order is the
# file system's, so a directory never holds two) with annotated, modified,
# record and @interface declarations, a header split over lines with type
# arguments and comments, a declaration missing from the index, one indexed
# without its supertypes, an original docSource source checked through its
# vineflower path, and a corpus where everything is indexed (exit 0); a
# missing and a malformed class index.
RECALL_SOURCES = {
    "m/m-rt/vineflower/a/A.java": "@Deprecated @SuppressWarnings(\"x\")\npublic final class A extends B {}\n"
                                  "class A2 {}\n",
    "m/m-rt/vineflower/a/b/C.java": "public non-sealed class C<T extends X>\n  // implements nothing\n"
                                    "  extends D<Y> /* implements */ implements E {\n}\n",
    "m/m-rt/vineflower/a/b/c/R.java": "public record R(int x) implements Q {}\n@interface Note {}\n",
    "m/m-rt/vineflower/a/b/c/d/Missing.java": "enum Missing { A }\n",
    "m/m-rt/vineflower/a/b/c/d/e/Lost.java": "abstract class Lost extends Base implements I {}\n",
    "docSource/docSource-doc/extracted/x/Orig.java": "public interface Orig extends Base {}\n",
}
RECALL_INDEX = {
    "A": [{"path": "m/m-rt/vineflower/a/A.java", "extends": "B"}],
    "A2": [{"path": "m/m-rt/vineflower/a/A.java"}],
    "C": [{"path": "m/m-rt/vineflower/a/b/C.java", "extends": "D", "implements": ["E"]}],
    "R": [{"path": "m/m-rt/vineflower/a/b/c/R.java", "implements": ["Q"]}],
    "Note": [{"path": "m/m-rt/vineflower/a/b/c/R.java"}],
    "Lost": [{"path": "m/m-rt/vineflower/a/b/c/d/e/Lost.java", "extends": "Base"},
             {"path": "m/m-rt/vineflower/a/b/c/d/e/Lost.java", "implements": ["I"], "outer_class": "X"}],
    "Orig": [{"path": "docSource/docSource-doc/vineflower/x/Orig.java", "extends": "Base"}],
}


def build_recall():
    fx = os.path.join(HERE, "fixture", "recall")
    shutil.rmtree(fx, ignore_errors=True)
    for corpus, sources in (("organized", RECALL_SOURCES),
                            ("clean", {k: v for k, v in RECALL_SOURCES.items()
                                       if "Missing" not in k and "Lost" not in k})):
        for rel, text in sources.items():
            path = os.path.join(fx, corpus, rel)
            os.makedirs(os.path.dirname(path), exist_ok=True)
            with open(path, "w", encoding="utf-8") as f:
                f.write(text)
    os.makedirs(os.path.join(fx, "base", "indexes"))
    with open(os.path.join(fx, "base", "indexes", "class-index.json"), "w", encoding="utf-8") as f:
        json.dump({"_meta": {"timestamp": "2026-05-06T07:08:09"}, "classes": RECALL_INDEX}, f)
    os.makedirs(os.path.join(fx, "no-base"))
    os.makedirs(os.path.join(fx, "bad-base", "indexes"))
    with open(os.path.join(fx, "bad-base", "indexes", "class-index.json"), "w", encoding="utf-8") as f:
        f.write('{"_meta": {}, "classes": ')


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
    kept = None  # an intended.json written by hand (e.g. a D32 retired command) survives a re-capture
    if os.path.isfile(os.path.join(d, "intended.json")):
        with open(os.path.join(d, "intended.json"), encoding="utf-8") as f:
            kept = f.read()
    shutil.rmtree(d, ignore_errors=True)
    os.makedirs(d)
    if kept is not None and cid[len("nav-"):] not in INTENDED:
        with open(os.path.join(d, "intended.json"), "w", encoding="utf-8") as f:
            f.write(kept)
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
    """--fixture rebuilds every fixture; --search only the grep/source/token one,
    --lic only the license/resources one, --cg only the call graph one, --stats
    only the stats ones, --pla only the palette-lexicon-agents one, --recall
    only the selftest one.
    --only a,b captures just those case ids (without the nav- prefix)."""
    if "--fixture" in sys.argv:
        build_fixture()
        build_drivers()
    if "--fixture" in sys.argv or "--search" in sys.argv:
        build_search()
    if "--fixture" in sys.argv or "--lic" in sys.argv:
        build_lic()
    if "--fixture" in sys.argv or "--cg" in sys.argv:
        build_cg()
    if "--fixture" in sys.argv or "--stats" in sys.argv:
        build_stats()
    if "--fixture" in sys.argv or "--pla" in sys.argv:
        build_pla()
    if "--fixture" in sys.argv or "--recall" in sys.argv:
        build_recall()
    only = set(sys.argv[sys.argv.index("--only") + 1].split(",")) if "--only" in sys.argv else None
    fresh_class_index()
    for cid, args, env, *base in CASES:
        if only is None or cid in only:
            capture("nav-" + cid, args, env, *base)
    if only is not None:
        return
    for cache in glob.glob(os.path.join(HERE, "fixture", "*", "indexes", "class-rank.cache")):
        os.remove(cache)  # Python's rank cache; the Go port keeps its own
    for cid, args in PARSE_CASES:
        capture(cid, args, {}, base="/nonexistent-base")


if __name__ == "__main__":
    main()
