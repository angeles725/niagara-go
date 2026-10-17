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
"""

import json
import os
import sys

sys.path.insert(0, sys.argv[1])

from niagara_help_lib import doc_types, install_rules, jar_listing  # noqa: E402


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
    else:
        sys.exit("unknown mode " + mode)


if __name__ == "__main__":
    main()
