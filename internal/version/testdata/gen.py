"""Regenerate internal/version/testdata/fixtures.json from tools/lib/version.py.

Run: python3 -I internal/version/testdata/gen.py   (NIAGARA_RESEARCH overrides the checkout path)
The git description is stubbed, so the fixture records only the line format and TOOLS_VERSION.
"""
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
NR = os.environ.get("NIAGARA_RESEARCH", "/home/cristian/niagara-research")
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
