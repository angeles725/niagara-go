"""Python twin of internal/helpbuild/parity_test.go.

Run with `python3 -I -B driver.py TOOLS MODE ARGS...` (-B: no __pycache__
written into the niagara-help checkout), where TOOLS is the
niagara-help tools/ directory (put on sys.path explicitly: -I leaves the
script directory and PYTHONPATH out). Every mode prints JSON lines that the
Go test rebuilds from internal/helpbuild and compares line by line.

  classify SRC_DIR JAR...  per jar ["jar", name, read_listing(jar)] (compact),
                           then per listed entry ["entry", name, is_doc_like,
                           extract id, [exclude id, reason], exclude id for
                           the "(course)" pseudo-jar, course_kind,
                           is_guide_page, is_licence,
                           is_source_archive("guides/" + name),
                           clean_text_name(name, SRC_DIR)]
  cache CACHE JAR...       Listings(CACHE).listing(jar) for each jar, then
                           save(); prints the number of jars opened
  path REPO INSTALL...     cache_path(REPO, INSTALL) per install
  glob                     stdin: [[pattern, path], ...]; prints
                           [glob_re(pattern).match(path) is not None, ...]
  map                      stdin: [[jar, [entry, ...]], ...]; per jar
                           ["jar", jar, is_doc_jar, is_module_jar, jar_stem,
                           guide_folder], per entry ["entry", entry,
                           dest_path, is_help_entry], then the
                           mapped_entries rows ["mapped", pos, entry, rel,
                           first]
  helpjars INSTALL         ["help", help_jars(INSTALL)] and
                           ["doc", list_doc_jars(INSTALL, True)]
  docjars OP INSTALL ROOT  OP extract | fill | fidelity: ["ok", result] or
                           ["raise", exception class name]
  setup REPO VID OP        version_setup.extract_and_index(REPO, VID,
                           no_index=True, ...) with OP extract | force |
                           verify | fill: ["out", line] per out() line, then
                           ["result", status, error, extracted, fidelity] or
                           ["raise", exception class name]
  emptyindexes ROOT        version_setup._empty_indexes(ROOT) lines
"""

import json
import os
import sys

sys.path.insert(0, sys.argv[1])

from niagara_help_lib import (doc_jars, doc_types, install_rules,  # noqa: E402
                              jar_listing, version_setup)


def _id(rule):
    return rule.id if rule else None


def classify(src_dir, jars):
    rules = install_rules.default()
    for jar in jars:
        name = os.path.basename(jar)
        listing = jar_listing.read_listing(jar)
        print(json.dumps(["jar", name, listing], separators=(",", ":")))
        if listing is None:
            continue
        for entry, _size, _crc in listing["entries"]:
            exclude = rules.exclude_rule(name, entry)
            print(json.dumps([
                "entry", entry, rules.is_doc_like(entry),
                _id(rules.extract_rule(name, entry)),
                [exclude.id, exclude.reason] if exclude else None,
                _id(rules.exclude_rule("(course)", entry)),
                rules.course_kind(entry),
                doc_types.is_guide_page(entry),
                doc_types.is_licence(entry),
                doc_types.is_source_archive("guides/" + entry),
                doc_types.clean_text_name(entry, src_dir)]))


def cache(path, jars):
    listings = jar_listing.Listings(path)
    for jar in jars:
        listings.listing(jar)
    listings.save()
    print(json.dumps(listings.reads))


def mapping(pairs):
    for jar, entries in pairs:
        print(json.dumps(["jar", jar, doc_jars.is_doc_jar(jar),
                          doc_jars.is_module_jar(jar), doc_jars.jar_stem(jar),
                          doc_jars.guide_folder(jar)]))
        for e in entries:
            print(json.dumps(["entry", e, doc_jars.dest_path(jar, e),
                              doc_jars.is_help_entry(jar, e)]))
        for row in doc_jars.mapped_entries(jar, entries):
            print(json.dumps(["mapped"] + list(row)))


def docjars(op, install, root):
    fn = {"extract": doc_jars.extract, "fill": doc_jars.fill_missing,
          "fidelity": doc_jars.fidelity}[op]
    try:
        result = fn(install, root)
    except Exception as e:  # noqa: BLE001 - the class is the result
        print(json.dumps(["raise", type(e).__name__]))
        return
    print(json.dumps(["ok", result]))


def setup(repo, vid, op):
    lines = []
    try:
        status, stats = version_setup.extract_and_index(
            repo, vid, force=op == "force", no_index=True,
            verify_only=op == "verify", fill_missing=op == "fill",
            out=lines.append)
    except Exception as e:  # noqa: BLE001 - the class is the result
        for line in lines:
            print(json.dumps(["out", line]))
        print(json.dumps(["raise", type(e).__name__]))
        return
    for line in lines:
        print(json.dumps(["out", line]))
    print(json.dumps(["result", status, stats.get("error"),
                      stats.get("extracted"), stats.get("fidelity")]))


def main():
    mode, args = sys.argv[2], sys.argv[3:]
    if mode == "classify":
        classify(args[0], args[1:])
    elif mode == "cache":
        cache(args[0], args[1:])
    elif mode == "path":
        for install in args[1:]:
            print(json.dumps(jar_listing.cache_path(args[0], install)))
    elif mode == "glob":
        pairs = json.load(sys.stdin)
        print(json.dumps([install_rules.glob_re(p).match(s) is not None
                          for p, s in pairs]))
    elif mode == "map":
        mapping(json.load(sys.stdin))
    elif mode == "helpjars":
        print(json.dumps(["help", doc_jars.help_jars(args[0])]))
        print(json.dumps(["doc", doc_jars.list_doc_jars(args[0], True)]))
    elif mode == "docjars":
        docjars(*args)
    elif mode == "setup":
        setup(*args)
    elif mode == "emptyindexes":
        print(json.dumps(version_setup._empty_indexes(args[0])))
    else:
        sys.exit("unknown mode " + mode)


if __name__ == "__main__":
    main()
