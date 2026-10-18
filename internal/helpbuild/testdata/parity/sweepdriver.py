"""Python twin of internal/helpbuild/parity_sweep_test.go (slice S11a).

Run with `python3 -I -B sweepdriver.py TOOLS MODE ARGS...` (see driver.py).

  fixture DIR    builds the synthetic install DIR/install (the
                 test_install_sweep fixture plus the corners below), the
                 course tree DIR/course-src, the repository DIR/repo
                 (versions 1.0 extracted and rebuilt with rebuild-all, 2.0
                 registered on the same install but never extracted, 3.0
                 without an install) and the rules file DIR/rules.json
                 (the test's exclusions, the shipped extract, globs and
                 course rules); then builds the full-text index of 1.0
                 (open_index), so neither side has to. What the builders
                 print goes to stderr; stdout is ["ok"].
  sweep REPO BASE VID RULES
                 install_sweep.cmd_sweep(REPO, BASE, VID or None) with
                 install_rules.default() loading RULES ("-": the shipped
                 file): ["out", line] per printed line, then ["exit",
                 status] or ["raise", exception class name], then ["took",
                 seconds] (not compared). The jar listings are cached under
                 REPO/.cache/, so each side gets a repository of its own
                 (a copy of the registry) and the caches are compared.
"""

import io
import json
import os
import sys
import time
import zipfile

sys.path.insert(0, sys.argv[1])

from niagara_help_lib import (doc_jars, fulltext_index,  # noqa: E402
                              install_rules, install_sweep, maintenance)
from niagara_help_lib.index_cache import clear_index_cache  # noqa: E402

PAGE = "<html><head><title>{0}</title></head><body><p>{1}</p></body></html>"

RULES = {
    "doc_like": {"extensions": [".html", ".htm", ".md", ".txt", ".pdf"],
                 "names": ["toc.xml"]},
    "skip_dirs": [{"dir": "jre", "reason": "bundled Java runtime"}],
    "extract": [],
    "exclude": [
        {"id": "node-modules", "entry": "**/node_modules/**",
         "reason": "bundled npm package"},
        {"id": "third-party-java", "entry": "org/**",
         "reason": "third-party library resource"},
        {"id": "docs-menu", "jar": "(docs)", "entry": "menu/**",
         "reason": "PDF library menu frame"},
    ],
}

HELP_JAR = {
    "doc/About-Help.html": PAGE.format("About Help", "helpzork page"),
    "com/acme/BHelp.class": "\xca\xfe",
}

BAJADOC = ('<?xml version="1.0" encoding="UTF-8"?>\n<bajadoc>\n'
           '<class qualifiedName="{0}.{1}" name="{1}" packageName="{0}">'
           '<description>d</description></class>\n</bajadoc>\n')

ALLCLASSES = ('<html><body><ul>\n<li><a href="javax/baja/sys/BObject.html" '
              'title="class in javax.baja.sys">BObject</a></li>\n'
              '</ul></body></html>\n')

COURSE_FILES = {
    "text/Review Assignment 1.txt": "Review Assignment 1\n\fPage two\n",
    "text/8 ORDs.txt": "ORDs\n\fNMessage link layer\n",
    "course/Review Assignments/Review Assignment 1.pdf": "%PDF-1.4",
    "course/Slides/8 ORDs.pdf": "%PDF-1.4",
    "course/Slides/9 Orphan.pdf": "%PDF-1.4",
    "course/Labs/Lab1 First/src/BFirst.java": "public class BFirst {}\n",
    "course/Labs/Lab5 Code Signing/rootca.pem": "-----BEGIN CERT",
    "course/Labs/Lab1 First/module.palette": "<palette/>",
    "niagara-4.15.6.20/dev/demo/readme.txt": "Demo readme\n",
    "niagara-4.15.6.20/dev/demo/src/BDemo.java": "public class BDemo {}\n",
    "solutions/cert-exam/BAnswer.java": "public class BAnswer {}\n",
    "MANIFEST.sha256": "abc  x\n",
}


def _jar(path, entries):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with zipfile.ZipFile(path, "w") as z:
        for name, data in entries.items():
            z.writestr(name, data)


def _write(path, data):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "wb") as f:
        f.write(data)


def make_install(root):
    mods = os.path.join(root, "modules")
    _jar(os.path.join(mods, "docGuide-doc.jar"), {
        "doc/index.html": PAGE.format("Guide", "guide home"),
        "doc/Intro.html": PAGE.format("Intro", "intro text"),
        "doc/LICENSE.txt": "licence text\n",
        "doc/toc.xml": '<toc><tocitem text="Intro" target="Intro.html"/>'
                       '</toc>\n',
        "doc/notes.text": "a .text file\n",
        "doc/odd.weird": "a file in a doc folder\n",
    })
    _jar(os.path.join(mods, "helpMod-rt.jar"), HELP_JAR)
    _jar(os.path.join(mods, "thing-ux.jar"), {
        "rc/notes.md": "# Notes\n\nquibbleNotes of the thing module\n",
        "rc/node_modules/pkg/README.md": "npm readme\n",
        "rc/thing.js": "var x;",
    })
    _jar(os.path.join(mods, "lib-rt.jar"), {
        "org/apache/words.txt": "a\nb\n",
        "com/acme/BLib.class": "\xca\xfe",
        "com/acme/readme.txt": "a module readme\n",
    })
    _jar(os.path.join(mods, "docDeveloper-doc.jar"), {
        "doc/index.html": PAGE.format("Dev", "developer guide"),
        "doc/toc.xml": '<toc><tocitem text="Dev" target="index.html"/>'
                       '</toc>\n',
        "doc/module-index.bajadoc": "<bajadoc/>\n",
        "doc/javax/baja/dev/BDev.bajadoc": BAJADOC.format("javax.baja.dev",
                                                          "BDev"),
        "doc/broken.bajadoc": "<bajadoc><class",
        "doc/Readme.TXT": "upper-case extension\n",
        "rc/Example.java": "public class Example {}\n",
        "rc/readme.txt": "rc readme\n",
    })
    _jar(os.path.join(mods, "docDeveloper-ux.jar"), {
        "examples/demo.js": "function demo() { return 1; }\n",
        "examples/demo.min.js": "function a(){}\n",
        "examples/page.html": PAGE.format("Example", "example page"),
        "examples/style.css": "p {}\n",
        "ext/babel/readme.md": "babel\n",
        "examples/Thumbs.db": "x",
        "examples/data.bin": "x",
    })
    _jar(os.path.join(mods, "docSource-doc.jar"), {
        "doc/toc.xml": "<toc/>\n",
        "doc/index.html": PAGE.format("Src", "source browser"),
        "javax/baja/sys/readme.txt": "source readme\n",
    })
    _jar(os.path.join(mods, "docJdk-doc.jar"), {
        "doc/java/lang/Object.bajadoc": BAJADOC.format("java.lang",
                                                       "Object"),
        "doc/package-index.bajadoc": "<bajadoc/>\n",
    })
    _jar(os.path.join(root, "javadoc", "niagaraJavadoc.jar"), {
        "doc/index.html": PAGE.format("Javadoc", "frames"),
        "doc/allclasses-noframe.html": ALLCLASSES,
        "doc/javax/baja/sys/BObject.html": PAGE.format("BObject", "class"),
        "doc/javax/baja/sys/BMissing.html": PAGE.format("BMissing", "none"),
        "doc/javax/baja/sys/class-use/BObject.html": "<p>use",
        "doc/javax/baja/sys/notes.txt": "javadoc text file\n",
    })
    _jar(os.path.join(mods, "mmm-ux.jar"), {
        "doc/rc/x.md": "# first\n", "rc/x.md": "# second\n"})
    _jar(os.path.join(mods, "nnn-ux.jar"), {
        "doc/rc/y.md": "# same\n", "rc/y.md": "# same\n"})
    _jar(os.path.join(mods, "ffmpeg-wb.jar"), {
        "doc/index.html": PAGE.format("ffmpeg", "licence offer"),
        "doc/ffmpeg-6.0.tar.xz": "xz"})
    _jar(os.path.join(mods, "Upper.JAR"), {"doc/a.html": "<p>a"})
    _write(os.path.join(mods, "broken-rt.jar"), b"PK\x03\x04 not a zip")
    _write(os.path.join(mods, "empty-rt.jar"), b"")
    _jar(os.path.join(mods, "ünï-rt.jar"),
         {"doc/été.html": PAGE.format("Ete", "été")})
    _jar(os.path.join(root, "loose.jar"), {"doc/a.html": "<p>root"})
    _jar(os.path.join(root, "a-very-long-top-level-folder-name-indeed",
                      "x.jar"), {"doc/a.html": "<p>long"})
    # sw/: an identical copy, a changed copy and a copy with many pages
    # the installed jar does not have (more than FIRST unclassified).
    _jar(os.path.join(root, "sw", "1.0", "helpMod-rt.jar"), HELP_JAR)
    _jar(os.path.join(root, "sw", "2.0", "helpMod-rt.jar"), {
        "doc/About-Help.html": PAGE.format("About Help", "newer text"),
    })
    _jar(os.path.join(root, "sw", "3.0", "helpMod-rt.jar"), {
        "doc/p{:02d}.html".format(i): "<p>{}".format(i) for i in range(30)})
    _jar(os.path.join(root, "sw", "locked", "x.jar"), {"doc/a.html": "<p>a"})
    _jar(os.path.join(root, "jre", "lib", "rt.jar"), {"about.html": "x"})
    _write(os.path.join(root, "docs", "menu", "tree.html"),
           PAGE.format("Docs", "menu").encode())
    _write(os.path.join(root, "docs", "Manual.pdf"), b"%PDF-1.4 not really")
    _write(os.path.join(root, "docs", "sub", "Manual2.PDF"), b"%PDF-1.4")
    _write(os.path.join(root, "docs", "Missing.pdf"), b"%PDF-1.4")
    _write(os.path.join(root, "docs", "notes.txt"), b"loose notes\n")
    _write(os.path.join(root, "docs", "image.png"), b"png")
    _write(os.path.join(root, "docs", "locked", "Manual3.pdf"), b"%PDF-1.4")
    return root


def fixture(tmp):
    install = make_install(os.path.join(tmp, "install"))
    src = os.path.join(tmp, "course-src")
    for rel, text in COURSE_FILES.items():
        _write(os.path.join(src, rel), text.encode())
    repo = os.path.join(tmp, "repo")
    for vid in ("1.0", "2.0", "3.0"):
        os.makedirs(os.path.join(repo, "versions", vid, "indexes"))
    with open(os.path.join(repo, "versions", "registry.json"), "w") as f:
        json.dump({"default": "1.0", "versions": {
            "1.0": {"label": "fixture", "install_dir": install,
                    "data_root": "versions/1.0", "course_dir": src},
            "2.0": {"label": "same install, nothing extracted",
                    "install_dir": install, "data_root": "versions/2.0"},
            "3.0": {"label": "no install", "install_dir": "",
                    "data_root": "versions/3.0"}}}, f)
    with open(install_rules.RULES_FILE, encoding="utf-8") as f:
        shipped = json.load(f)
    with open(os.path.join(tmp, "rules.json"), "w") as f:
        json.dump(dict(RULES, extract=shipped["extract"],
                       globs=shipped.get("globs", {}),
                       course=shipped.get("course", {})), f)
    data = os.path.join(repo, "versions", "1.0")
    doc_jars.extract(install, data)
    # rebuild-all runs every builder as a subprocess writing to fd 1.
    sys.stdout.flush()
    saved = os.dup(1)
    os.dup2(2, 1)
    try:
        maintenance.cmd_rebuild_all(data)
        sys.stdout.flush()
    finally:
        os.dup2(saved, 1)
        os.close(saved)
    _write(os.path.join(data, "docs-text", "Manual.txt"), b"manual text\n")
    with open(os.path.join(data, "indexes", "pdf-index.json"), "w") as f:
        json.dump({"Manual": {"source_pdf": "Manual.pdf"},
                   "Manual2": {"source_pdf": "sub/manual2.pdf"},
                   "Manual3": {"source_pdf": "locked/Manual3.pdf"},
                   "NoSource": {}}, f)
    clear_index_cache()
    fulltext_index.open_index(data).close()
    print(json.dumps(["ok"]))


def sweep(repo, base, vid, rules):
    if rules != "-":
        loaded = install_rules.load(rules)
        install_rules.default = lambda: loaded
    out = io.StringIO()
    start = time.perf_counter()
    try:
        status = install_sweep.cmd_sweep(repo, base, vid or None, out)
        last = ["exit", status]
    except Exception as e:  # noqa: BLE001 - the class is the result
        last = ["raise", type(e).__name__]
    took = time.perf_counter() - start
    for line in out.getvalue().splitlines():
        print(json.dumps(["out", line]))
    print(json.dumps(last))
    print(json.dumps(["took", round(took, 3)]))


if __name__ == "__main__":
    mode = sys.argv[2]
    if mode == "fixture":
        fixture(sys.argv[3])
    elif mode == "sweep":
        sweep(*sys.argv[3:7])
    else:
        raise SystemExit("unknown mode " + mode)
