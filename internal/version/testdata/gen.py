"""Regenerate internal/version/testdata/fixtures.json from tools/lib/version.py.

Run: python3 -I internal/version/testdata/gen.py   (NIAGARA_RESEARCH_PY_REF overrides the worktree path)
The git description is stubbed, so the fixture records only the line format and TOOLS_VERSION.
"""
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

import version  # noqa: E402


def line(tool, describe):
    version.describe = lambda repo=None: describe
    return version.version_line(tool)


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
    out = {"tools_version": version.TOOLS_VERSION,
           "lines": [[tool, d or "", line(tool, d)] for tool in ("bog-nav", "corpus-nav", "module-find")
                     for d in (None, "d62aeb56a-dirty", "v3.0.0-4-g1234abcde")]}
    with open(os.path.join(HERE, "fixtures.json"), "w", encoding="utf-8", newline="\n") as fh:
        fh.write("\n".join(render(out)) + "\n")


if __name__ == "__main__":
    main()
