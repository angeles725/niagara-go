"""Freeze what the Python niagara-help does when the full-text index must be rebuilt.

Run: python3 -I internal/help/testdata/rebuild_gen.py   (NIAGARA_HELP_PY_REF overrides the pre-go-retire worktree)

fulltext_index.open_index rebuilds indexes/fulltext.sqlite in process whenever it is missing or
its stored fingerprint differs from the current text sources: it prints "building the full-text
index for <root> ..." and build's "full-text index: N documents -> <path>" on stderr, then the
command answers. When indexes/ is not writable it notes that and builds in the user cache
directory; when that is not writable either it keeps the index in memory for the process.

Each case runs tools/niagara_help.py on a fresh copy of fulltext/ (every file's mtime FIXED_NS,
as fulltext_gen.py) prepared as the case says, and records in fulltext-rebuild/:

  <case>.txt   stdout, with <ROOT> for the checkout
  cases.json   arguments, setup, exit status, stderr (<CACHEDB> for the data root's cache
               database, <CACHEDIR> for its folder, <CACHE> for XDG_CACHE_HOME, <ROOT> for the
               checkout) and where an
               up-to-date index exists afterwards ("indexes", "cache" or "none")
"""
import json
import os
import shutil
import sqlite3
import stat
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))


def py_ref(repo, env, rev):  # D47: a clean worktree of <repo> tag pre-go-retire (docs/goldens.md)
    root = os.environ.get(env) or os.path.expanduser("~/niagara-research-worktrees/pre-go-retire-" + repo)
    git = lambda *a: subprocess.run(["git", "-C", root, *a], capture_output=True, text=True).stdout.strip()  # noqa: E731
    if git("rev-parse", "HEAD") != rev or git("status", "--porcelain"):
        sys.exit(f"{env}={root}: not a clean worktree of {repo} tag pre-go-retire ({rev[:9]}); see docs/goldens.md")
    return root


PYREF = py_ref("niagara-help", "NIAGARA_HELP_PY_REF", "8410d74511673e38b87622f7214fa5210ba25a63")
os.environ["NIAGARA_HELP_PYTHON"] = "1"  # a tag shim runs its Python, never Go
TOOLS = os.path.join(PYREF, "tools")
sys.path[:0] = [TOOLS]

from niagara_help_lib import fulltext_index, text_sources  # noqa: E402

FIXED_NS = 1_700_000_000_123_456_789
VERSIONS = {"1.0": ".", "2.0": os.path.join("versions", "2.0")}

# (case, version, setup, arguments). setup:
#   missing        no fulltext.sqlite anywhere
#   stale          indexes/fulltext.sqlite built, then its stored fingerprint changed
#   ro-indexes     missing, and the data root's indexes/ is read-only
#   ro-everywhere  missing, indexes/ read-only and XDG_CACHE_HOME a read-only folder
CASES = [
    ("missing-find", "1.0", "missing", ["find", "alarm ack"]),
    ("missing-guide-search", "1.0", "missing", ["guide-search", "wiretap"]),
    ("missing-devguide-search", "1.0", "missing", ["devguide-search", "servlet"]),
    ("missing-pdf", "1.0", "missing", ["pdf", "discovery timeout"]),
    ("missing-pdf-shared", "2.0", "missing", ["pdf", "discovery"]),
    ("missing-method", "1.0", "missing", ["method", "ackAlarm"]),
    ("missing-find-miss", "1.0", "missing", ["find", "zzqq wwvv"]),
    ("missing-find-bad", "1.0", "missing", ["find", "!!! ???"]),
    ("missing-devguide-absent", "2.0", "missing", ["devguide-search", "servlet"]),
    ("missing-pdf-names", "1.0", "missing", ["pdf", "modbus", "--no-fulltext"]),
    ("stale-find", "1.0", "stale", ["find", "alarm ack"]),
    ("stale-guide-search", "1.0", "stale", ["guide-search", "wiretap"]),
    ("ro-indexes-find", "1.0", "ro-indexes", ["find", "alarm ack"]),
    ("ro-everywhere-find", "1.0", "ro-everywhere", ["find", "alarm ack"]),
]


def prepare(tmp):
    """Copy fulltext/ to tmp/checkout with every file's mtime FIXED_NS."""
    checkout = os.path.join(tmp, "checkout")
    shutil.copytree(os.path.join(HERE, "fulltext"), checkout)
    for root, _dirs, files in os.walk(checkout):
        for name in files:
            os.utime(os.path.join(root, name), ns=(FIXED_NS, FIXED_NS))
    return checkout


def where(base, cache_home):
    """Where an up-to-date index of base exists after the run."""
    current = fulltext_index._fingerprint(text_sources.source_dirs(base))
    os.environ["XDG_CACHE_HOME"] = cache_home
    for name, path in (("indexes", fulltext_index.db_path(base)),
                       ("cache", fulltext_index.cache_path(base))):
        if os.path.isfile(path) and fulltext_index._stored_fingerprint(path) == current:
            return name
    return "none"


def run_case(tmp, vid, setup, args):
    checkout = prepare(tmp)
    base = os.path.normpath(os.path.join(checkout, VERSIONS[vid]))
    cache_home = os.path.join(tmp, "cache")
    indexes = os.path.join(base, "indexes")
    if setup == "stale":
        path = fulltext_index.build(base, quiet=True)
        con = sqlite3.connect(path)
        con.execute("UPDATE meta SET value = 'stale' WHERE key = 'fingerprint'")
        con.commit()
        con.close()
    locked = []
    if setup in ("ro-indexes", "ro-everywhere"):
        locked.append(indexes)
    if setup == "ro-everywhere":
        os.makedirs(cache_home)
        locked.append(cache_home)
    for d in locked:
        os.chmod(d, stat.S_IRUSR | stat.S_IXUSR)
    env = {"PATH": os.environ.get("PATH", ""), "COLUMNS": "100", "NO_COLOR": "1",
           "PYTHONHASHSEED": "0", "NIAGARA_HELP_DIR": checkout, "XDG_CACHE_HOME": cache_home,
           "NIAGARA_HELP_PYTHON": "1", "NIAGARA_GO_BIN_DIR": os.environ.get("NIAGARA_GO_BIN_DIR", "")}
    argv = ["--niagara-version", vid] + args
    try:
        p = subprocess.run([sys.executable, "-I", os.path.join(TOOLS, "niagara_help.py")] + argv,
                           cwd=checkout, env=env, capture_output=True, text=True)
        os.environ["XDG_CACHE_HOME"] = cache_home
        cache_db = fulltext_index.cache_path(base)
        built = where(base, cache_home)
    finally:
        for d in locked:
            os.chmod(d, stat.S_IRWXU)

    def norm(s):
        return s.replace(cache_db, "<CACHEDB>").replace(
            os.path.dirname(cache_db), "<CACHEDIR>").replace(cache_home, "<CACHE>").replace(
            checkout, "<ROOT>")
    return argv, p, norm, built


def main():
    out = os.path.join(HERE, "fulltext-rebuild")
    os.makedirs(out, exist_ok=True)
    cases = []
    for name, vid, setup, args in CASES:
        with tempfile.TemporaryDirectory() as tmp:
            argv, p, norm, built = run_case(tmp, vid, setup, args)
        with open(os.path.join(out, name + ".txt"), "w", encoding="utf-8") as f:
            f.write(norm(p.stdout))
        cases.append({"name": name, "args": argv, "setup": setup, "exit": p.returncode,
                      "stderr": norm(p.stderr), "built": built})
    with open(os.path.join(out, "cases.json"), "w", encoding="utf-8") as f:
        f.write("[\n" + ",\n".join(json.dumps(c) for c in cases) + "\n]\n")


if __name__ == "__main__":
    main()
