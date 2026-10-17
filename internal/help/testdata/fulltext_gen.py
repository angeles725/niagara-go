"""Regenerate the full-text fixtures from the Python niagara-help.

Run: python3 -I internal/help/testdata/fulltext_gen.py   (NIAGARA_RESEARCH overrides the checkout)

fulltext/ is a two-version niagara-help checkout (1.0 at ".", 2.0 under versions/2.0 without
manuals, so it borrows 1.0's docs-text). The script copies it to a temporary folder, gives every
file the mtime FIXED_NS (the fingerprint hashes mtimes; the Go tests set the same one), and:

  fulltext-db/<version>.sqlite  the index fulltext_index.build writes for that data root, with
                                the temporary folder in the stored fingerprint replaced by <ROOT>
"""
import os
import shutil
import sqlite3
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
NR = os.environ.get("NIAGARA_RESEARCH", "/home/cristian/niagara-research")
TOOLS = os.path.join(NR, "niagara-help", "tools")
sys.path[:0] = [TOOLS]

from niagara_help_lib import fulltext_index  # noqa: E402

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


def main():
    with tempfile.TemporaryDirectory() as tmp:
        checkout = prepare(tmp)
        build_dbs(checkout)


if __name__ == "__main__":
    main()
