#!/usr/bin/env python3
"""Fake tools/corpus-nav.py for the hermetic nsearch goldens: `find <query> --limit N [--and|--or]`.

A query word picks the behaviour: blocksslow sleeps past --timeout, blocksdown exits 2, blockscrash
exits 1 without a miss line, blocksambig exits 0 silent, blocksmiss0 exits 0 with a miss line,
blocksgarbage prints rows no parser recognises; a word starting with "zz" is a miss (exit 1).
Otherwise: ranked 'B<n> · title' headers (2 with --and, 7 with --or, 4 by default), each with a hit line.
"""
import sys
import time

args = sys.argv[1:]
if args[:1] != ["find"] or len(args) < 2:
    sys.exit("corpus-nav: fake supports only `find`")
query, words = args[1], args[1].lower().split()
limit = args[args.index("--limit") + 1] if "--limit" in args else "50"
mode = "and" if "--and" in args else "or" if "--or" in args else "default"
if "blocksslow" in words:
    time.sleep(10)
if "blocksdown" in words:
    print("Traceback (most recent call last):\nRuntimeError: block index is corrupt", file=sys.stderr)
    sys.exit(2)
if "blockscrash" in words:
    print("ValueError: unexpected block header", file=sys.stderr)
    sys.exit(1)
if "blocksambig" in words:
    sys.exit(0)
if "blocksmiss0" in words:
    print(f"(no matches; searched 9 blocks for {query!r})")
    sys.exit(0)
if any(w.startswith("zz") for w in words):
    print(f"(no matches; searched 9 blocks for {query!r})")
    sys.exit(1)
if "blocksgarbage" in words:
    print("block index v0 dump")
    print("  row one without a header")
    sys.exit(0)
for i in range({"and": 2, "or": 7}.get(mode, 4)):
    word = words[i % len(words)]
    print(f"B{101 + i} · Block {101 + i} — {word} research ({mode} mode, --limit {limit})")
    print(f"    {12 + i}: > a line about {word} in block {101 + i}")
    print(f"    {40 + i}: a second line")
