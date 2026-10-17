#!/usr/bin/env python3
"""Fake niagara-help/tools/niagara_help.py: `[--niagara-version V] find <query> -n N [--and|--or]`.

A query word picks the behaviour: docsslow sleeps past --timeout, docsdown exits 2; a word starting
with "zz" is a miss (exit 1); "relaxed" prints the any-term widening line (a license query reads it as
a zero). Otherwise a count line, a by-source line, then N '[source] title' groups (the last without a
path line, so its title is the key) and a '... and K more files' line.
"""
import sys
import time

args = sys.argv[1:]
version = None
if args[:1] == ["--niagara-version"]:
    version, args = args[1], args[2:]
if args[:1] != ["find"] or len(args) < 2:
    sys.exit("niagara_help: fake supports only `find`")
query, words = args[1], args[1].lower().split()
n = int(args[args.index("-n") + 1]) if "-n" in args else 10
mode = "and" if "--and" in args else "or" if "--or" in args else "default"
if "docsslow" in words:
    time.sleep(10)
if "docsdown" in words:
    print("niagara_help: no search index for this version", file=sys.stderr)
    sys.exit(2)
if any(w.startswith("zz") for w in words):
    print(f"No match for {query!r} in this scope.")
    print("  (searched 31 files of docs 4.14.0.162)")
    sys.exit(1)
total = 3 + len(words)
shown = min(n, total)
print(f"{total} files matching {query!r} (docs {version or 'default'}, {mode} mode):")
if "relaxed" in words:
    print("  (no file has every term; showing files with any term)")
print("by source: guides-clean 3, devguide-clean 2")
for i in range(shown):
    word = words[i % len(words)]
    print(f"  [guides-clean] {word.title()} guide, page {i + 1}")
    print(f"    L{10 + i}: how the {word} setting works")
    if i < shown - 1:
        print(f"    path: guides-clean/{word.title()}/page{i + 1}.html")
if total > shown:
    print(f"  ... and {total - shown} more files")
