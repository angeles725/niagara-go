"""Index fixtures of gen_golden.py: rank-skips, the synthetic organized/ module.xml, cg, stats,
pla and recall.

fixture/nav/ (class-index, module-inventory, xref-index and inheritance.json) is frozen,
committed input: it was cut once from the live module-navigator indexes (14 alarm-related
classes, 13 modules) and is never rebuilt, so a --fixture run does not depend on the live
indexes and leaves its bytes, and the goldens that print its _meta, unchanged."""

import json
import os
import shutil
import sqlite3

HERE = os.path.dirname(os.path.abspath(__file__))

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


# palette-lexicon-agents (read by gen_palette_lexicon.py's pla-* cases, which
# come from niagara-research tools/palette-lexicon-agents.py, D39 e): module pal
# with an extracted artifact (nested and attribute-less <p>, a lexicon with
# duplicate keys, agents with and without <on>), a JAR-only artifact (palette and lexicon read from the JAR, a
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


def build_nav_synthetic():
    """The synthetic parts next to the frozen fixture/nav/: rank-skips/ and the
    organized/ module.xml."""
    fx = os.path.join(HERE, "fixture")
    for name in ("rank-skips", "organized"):
        shutil.rmtree(os.path.join(fx, name), ignore_errors=True)
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
