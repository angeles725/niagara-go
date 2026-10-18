"""Regenerate scan.json: license_features._scan_file calls and getLicenseManager()
sites on sample sources.

    python3 -I internal/nav/lib/license/testdata/gen_scan.py [<module-navigator>/tools]

Every file is written under a temporary root and read back through
_Resolver._text (UTF-8 with replacement, universal newlines); the files table
of an in-memory SQLite database lists them for the cross-file lookups.
"""

import json
import os
import sqlite3
import sys
import tempfile

TOOLS = sys.argv[1] if len(sys.argv) > 1 else os.path.expanduser(
    "~/niagara-research/module-navigator/tools")
sys.path.insert(0, TOOLS)
from module_nav_lib import license_features as lf  # noqa: E402

FILES = {
    "drv/drv-rt/vineflower/a/BDrvNetwork.java": '''package a;
public class BDrvNetwork extends BDeviceNetwork {
  static final String VENDOR = "acme";
  private static final String FEATURE= "drv" ;
  public void started() {
    Sys.getLicenseManager().checkFeature("tridium", "drvTcp");
    Sys.getLicenseManager() . getFeature ( VENDOR , FEATURE );
    // Sys.getLicenseManager().checkFeature("no", "comment");
    /* Sys.getLicenseManager().checkFeature("no", "block") */
    String s = "Sys.getLicenseManager().checkFeature(\\"no\\", \\"string\\")";
    LicenseManager lm = Sys.getLicenseManager();
    lm.checkFeature(Consts.VENDOR, "lm");
    lm.getFeature(this.VENDOR, TYPE.getModule().getModuleName());
    doc.getFeature(feature, version);
    Helper.checkFeature(vendorVar, featureVar);
    FooManager.INSTANCE.getFeature("inst", "ance");
    x.checkFeature("three", "args", 3);
    y.checkFeature();
    Sys.getLicenseManager().checkFeature("a\\"b", "c\\\\d");
    getLicenseManager().getFeature(Other.NAME, "x");
  }
  public Feature getFeature(String vendor, String feature) { return null; }
  int f() { return checkFeature("kw", "this"); }
}
''',
    "drv/drv-rt/vineflower/a/Consts.java": 'class Consts { public static final String VENDOR = "consts-vendor"; }\n',
    "other/other-rt/vineflower/b/Other.java": 'class Other { String NAME = "other-in-other-module"; }\n',
    "drv/drv-wb/vineflower/b/Other.java": 'class Other { String NAME = "other-in-same-module"; }\n',
    "drv/drv-rt/vineflower/a/BDrvLicenseManager.java": '''class BDrvLicenseManager<T extends X> extends BBase {
  void go() { this.checkFeature(vendor(), name()); getFeature("v", label()); }
}
''',
    "drv/drv-rt/vineflower/a/BBase.java": '''class BBase<T>
  extends BRoot { String vendor() { return "base-vendor"; } }
''',
    "drv/drv-rt/vineflower/a/BRoot.java": 'class BRoot { public String name ( ) {\n return  "root-name" ; } }\n',
    "uni/uni-rt/vineflower/c/BÉtat.java": '''class BÉtat {
  BLicenseManager élm;
  void go() { élm.getFeature("uni", "code"); xélm.getFeature("not", "typed"); ñ$lm.getFeature("a", "b"); }
  Object ñ$lm = Sys.getLicenseManager();
}
''',
    "crlf/crlf-rt/vineflower/d/BCrlf.java": None,  # raw bytes below
    "site/site-rt/vineflower/e/BSites.java": '''package e;
class BSites {
  public LicenseManager getLicenseManager() { return null; }
  // Sys.getLicenseManager() in a comment
  void a() { Sys.getLicenseManager().getFeatures(); }
  void b(Object o) { if (Sys.getLicenseManager() instanceof BLicenseManager) { } }
  void c(Object o) { boolean q = o instanceof
      getLicenseManager(); }
  void d() { LicenseManager unused = Sys.getLicenseManager(); }
  void e() { field.checkFeature("v", "fieldFeature"); }
  private BLicenseManager field;
  BSites() { this.field = Sys.getLicenseManager(); }
  void f() { LicenseManager lm = Sys.getLicenseManager(); lm.checkFeature(vendorVar, featureVar); }
  void g() { foo(Sys.getLicenseManager(),   bar(  1  ) ); }
  void h() { LicenseManager lm2; if (x) { lm2 = Sys.getLicenseManager(); } lm2.getFeature("v", "late"); }
  void i() { lm3 = (BLicenseManager) Sys.getLicenseManager(); if (y) { lm3.checkFeature("v", "inner"); } }
  void j() { lm2.checkFeature("v", "otherScope"); }
  Runnable r = new Runnable() { public void run() { mgr = Sys.getLicenseManager(); mgr.checkFeature("a", "anon"); } };
  void k() { LicenseManager é = Sys.getLicenseManager()
      ; é.getFeature("u", "nicode"); x.getLicenseManager( ).checkFeature("t", "wo"); }
}
''',
}

RAW = {
    "crlf/crlf-rt/vineflower/d/BCrlf.java":
        b'class BCrlf {\r\n  // a comment \xff\xfe\r  Sys.getLicenseManager().checkFeature("cr", "lf");\r\n'
        b'  String s = "\xe9\xff";\r\n  mgr.getFeature(KEY, "x");\r\n  BMyLicenseManager mgr;\r\n'
        b'  static String KEY = "k\\\r";\r\n}\r\n',
}

SCANS = [
    ("drv/drv-rt/vineflower/a/BDrvNetwork.java", "drv-rt", "BDrvNetwork"),
    ("drv/drv-rt/vineflower/a/BDrvLicenseManager.java", "drv-rt", "BDrvLicenseManager"),
    ("uni/uni-rt/vineflower/c/BÉtat.java", "uni-rt", "BÉtat"),
    ("crlf/crlf-rt/vineflower/d/BCrlf.java", "crlf-rt", "BCrlf"),
    ("site/site-rt/vineflower/e/BSites.java", "site-rt", "BSites"),
]

with tempfile.TemporaryDirectory() as root:
    conn = sqlite3.connect(":memory:")
    conn.execute("CREATE TABLE files (id INTEGER PRIMARY KEY, path TEXT, module TEXT, class_name TEXT)")
    for rel, text in FILES.items():
        os.makedirs(os.path.dirname(os.path.join(root, rel)), exist_ok=True)
        with open(os.path.join(root, rel), "wb") as f:
            f.write(RAW[rel] if text is None else text.encode("utf-8"))
        conn.execute("INSERT INTO files (path, module, class_name) VALUES (?, ?, ?)",
                     (rel, rel.split("/")[1], os.path.basename(rel)[:-5]))
    resolver = lf._Resolver(root, conn)
    scans = []
    for rel, module, cls in SCANS:
        calls, sites = lf._scan_file(resolver._text(rel), rel, module, cls, resolver)
        scans.append([rel, module, cls,
                      [[c["form"], c["receiver"], c["vendor"], c["feature"], c["line"], c["args"]]
                       for c in calls],
                      [[s["line"], s["features"], s["reason"]] for s in sites]])

def records(rows):
    """A JSON list with one compact record per line."""
    if not rows:
        return "[]"
    return "[\n    " + ",\n    ".join(json.dumps(r, ensure_ascii=False) for r in rows) + "\n   ]"


head = json.dumps({"files": {k: v for k, v in FILES.items() if v is not None},
                   "raw": {k: v.hex() for k, v in RAW.items()}}, ensure_ascii=False, indent=1)
body = ",\n".join("  [{}, {}, {},\n   {},\n   {}]".format(
    json.dumps(rel, ensure_ascii=False), json.dumps(module), json.dumps(cls, ensure_ascii=False),
    records(calls), records(sites)) for rel, module, cls, calls, sites in scans)
with open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "scan.json"), "w",
          encoding="utf-8") as f:
    f.write(head[:-2] + ',\n "scans": [\n' + body + "\n ]\n}\n")
