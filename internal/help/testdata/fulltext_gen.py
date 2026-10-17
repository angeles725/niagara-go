"""Regenerate the full-text fixtures from the Python niagara-help.

Run: python3 -I internal/help/testdata/fulltext_gen.py   (NIAGARA_RESEARCH overrides the checkout)

fulltext/ is a two-version niagara-help checkout (1.0 at ".", 2.0 under versions/2.0 without
manuals, so it borrows 1.0's docs-text). The script copies it to a temporary folder, gives every
file the mtime FIXED_NS (the fingerprint hashes mtimes; the Go tests set the same one), and:

  fulltext-db/<version>.sqlite  the index fulltext_index.build writes for that data root, with
                                the temporary folder in the stored fingerprint replaced by <ROOT>
  fulltext-search.json          fulltext.search results (score as float.hex) with each hit's
                                fulltext.snippet, and fulltext.clip of hand-picked lines
"""
import json
import os
import shutil
import sqlite3
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
NR = os.environ.get("NIAGARA_RESEARCH", "/home/cristian/niagara-research")
TOOLS = os.path.join(NR, "niagara-help", "tools")
sys.path[:0] = [TOOLS]

from niagara_help_lib import fulltext, fulltext_index  # noqa: E402

FIXED_NS = 1_700_000_000_123_456_789
ROOT = "<ROOT>"
VERSIONS = {"1.0": ".", "2.0": os.path.join("versions", "2.0")}


def prepare(tmp):
    """Copy fulltext/ to tmp/checkout with every file's mtime FIXED_NS."""
    checkout = os.path.join(tmp, "checkout")
    shutil.copytree(os.path.join(HERE, "fulltext"), checkout)
    for root, _dirs, files in os.walk(checkout):
        for name in files:
            os.utime(os.path.join(root, name), ns=(FIXED_NS, FIXED_NS))
    return checkout


def build_dbs(checkout):
    out = os.path.join(HERE, "fulltext-db")
    os.makedirs(out, exist_ok=True)
    for vid, rel in VERSIONS.items():
        base = os.path.normpath(os.path.join(checkout, rel))
        path = fulltext_index.build(base, quiet=True)
        dest = os.path.join(out, vid + ".sqlite")
        shutil.copyfile(path, dest)
        con = sqlite3.connect(dest)
        con.execute("UPDATE meta SET value = replace(value, ?, ?) WHERE key = 'fingerprint'",
                    (checkout, ROOT))
        con.commit()
        con.execute("VACUUM")
        con.close()


# (version, query, sources, mode) searched through fulltext.search.
SEARCHES = [
    ("1.0", "alarm ack", None, "and"),
    ("1.0", "alarm", None, "and"),
    ("1.0", "discovery timeout", None, "and"),
    ("1.0", "discovery timeout", None, "or"),
    ("1.0", "LeadLag", None, "and"),
    ("1.0", "servlet", ["devguide-clean", "devguide-examples"], "and"),
    ("1.0", "servlet registration", ["devguide-clean"], "and"),
    ("1.0", "como funciona el ack de alarmas", None, "and"),
    ("1.0", "wiretap", ["guides-clean"], "and"),
    ("1.0", "discovery timeout", ["docs-text"], "and"),
    ("1.0", "zzqq", None, "and"),
    ("1.0", "discovery", ["course-text"], "and"),
    ("2.0", "discovery", None, "and"),
    ("2.0", "discovery wiretap", None, "or"),
]

# (line, query) shortened through fulltext.clip (width 40).
CLIPS = [
    ("short line about wiretap", "wiretap"),
    ("x" * 30 + " the wiretap evaluates messages after a network port handled them", "wiretap"),
    ("Configuración ºººº de la clase de alarma: the alarm class has a priority and more text here", "alarm"),
    ("°°°°°° °F settings for the alarm class and the many other words of this line", "alarm"),
    ("no match anywhere in this rather long line of plain words, really long", "zzqq"),
    ("ﬁﬁﬁﬁﬁﬁﬁﬁﬁﬁ an alarm in a ligature line that is longer than forty", "alarm"),
    ("word" * 20, "word"),
]


def search_records(checkout):
    out = {"searches": [], "clips": []}
    for vid, query, sources, mode in SEARCHES:
        base = os.path.normpath(os.path.join(checkout, VERSIONS[vid]))
        r = fulltext.search(base, query, sources=sources, mode=mode)
        out["searches"].append({
            "version": vid, "query": query, "sources": sources, "mode": mode,
            "groups": [list(g) for g in r.groups], "mode_used": r.mode_used,
            "fallback": r.fallback, "and_hits": r.and_hits,
            "hits": [[h.key, float(h.score).hex(), h.title, h.line, h.page, h.coverage,
                      list(fulltext.snippet(base, h, r.groups))] for h in r.hits]})
    for line, query in CLIPS:
        out["clips"].append([line, query, fulltext.clip(line, fulltext.parse_query(query), 40)])
    with open(os.path.join(HERE, "fulltext-search.json"), "w", encoding="utf-8") as f:
        f.write('{"searches": [\n' + ",\n".join(json.dumps(r) for r in out["searches"]) +
                '],\n"clips": [\n' + ",\n".join(json.dumps(c) for c in out["clips"]) + "]}\n")


def main():
    with tempfile.TemporaryDirectory() as tmp:
        checkout = prepare(tmp)
        build_dbs(checkout)
        search_records(checkout)


if __name__ == "__main__":
    main()
