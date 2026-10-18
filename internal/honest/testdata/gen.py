"""Regenerate internal/honest/testdata/fixtures.json from tools/lib/honest.py.

Run: python3 -I internal/honest/testdata/gen.py   (NIAGARA_RESEARCH_PY_REF overrides the worktree path)
"""
import contextlib
import io
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))


def py_ref(repo, env, rev):  # D47: a clean worktree of <repo> tag pre-go-retire (docs/goldens.md)
    root = os.environ.get(env) or os.path.expanduser("~/niagara-research-worktrees/pre-go-retire-" + repo)
    git = lambda *a: subprocess.run(["git", "-C", root, *a], capture_output=True, text=True).stdout.strip()  # noqa: E731
    if git("rev-parse", "HEAD") != rev or git("status", "--porcelain"):
        sys.exit(f"{env}={root}: not a clean worktree of {repo} tag pre-go-retire ({rev[:9]}); see docs/goldens.md")
    return root


NR = py_ref("niagara-research", "NIAGARA_RESEARCH_PY_REF", "c846ce1e5eb92ee6d938efbb3585473a545f7e50")
sys.path.insert(0, os.path.join(NR, "tools", "lib"))

import honest  # noqa: E402

COUNTS = [[], [["components", 12]], [["components", 12], ["links", 3]], [["java_files", 0], ["unreadable", 2]],
          [["blocks", 1234567], ["files_read", 1], ["x", -1]]]
WHERE = ["config.bog", "organized/ (3 modules)", "", "C:\\stations\\PANCCADIA\\config.bog"]


def fail(tool, message, code):
    err = io.StringIO()
    with contextlib.redirect_stderr(err):
        try:
            honest.fail(tool, message, code=code)
        except SystemExit as exc:
            return [err.getvalue(), exc.code]
    raise AssertionError("honest.fail did not exit")


def render(v, depth=0):
    """Lines of v: containers above depth 2 one member per line, deeper ones compact."""
    if depth >= 2 or not isinstance(v, (list, dict)) or not v:
        return [json.dumps(v, ensure_ascii=False)]
    pairs = list(v.items()) if isinstance(v, dict) else [(None, e) for e in v]
    lines = ["{" if isinstance(v, dict) else "["]
    for i, (k, e) in enumerate(pairs):
        sub = render(e, depth + 1)
        if k is not None:
            sub[0] = json.dumps(k) + ": " + sub[0]
        if i < len(pairs) - 1:
            sub[-1] += ","
        lines += sub
    lines.append("}" if isinstance(v, dict) else "]")
    return lines


def main():
    out = {
        "exit_input": honest.EXIT_INPUT,
        "scope": [[w, c, honest.scope(w, **dict(c))] for w in WHERE for c in COUNTS],
        "miss": [[what, w, c, honest.miss(what, w, **dict(c))] for what in ("matches", "semantic matches")
                 for w in WHERE[:2] for c in COUNTS],
        "fail": [[t, m, code] + fail(t, m, code) for t, m, code in
                 [("bog-nav", "cannot read x.bog", 2), ("module-find", "no .java files under r", 3), ("t", "", 2)]],
    }
    with open(os.path.join(HERE, "fixtures.json"), "w", encoding="utf-8", newline="\n") as fh:
        fh.write("\n".join(render(out)) + "\n")


if __name__ == "__main__":
    main()
