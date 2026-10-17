"""Regenerate internal/difflib/testdata/fixtures.json from this Python's difflib.

Run from the repository root: python3 -I internal/difflib/testdata/gen.py

Each matcher case is [kind, junk, autojunk, a, b, longest, blocks, ratio, quick, real_quick]:
kind "str" compares code points, "lines" compares list items; junk names an isjunk the Go
test also defines (none, space: x in " \\t", blank: not x.strip()); ratios are IEEE bits.
Each close case is [word, possibilities, n, cutoff, result-or-null].
"""
import difflib
import json
import os
import random
import struct
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
JUNK = {"none": None, "space": lambda x: x in " \t", "blank": lambda x: not x.strip()}


def bits(f):
    return "%016x" % struct.unpack(">Q", struct.pack(">d", f))[0]


def case(kind, junk, autojunk, a, b):
    m = difflib.SequenceMatcher(JUNK[junk], a, b, autojunk)
    longest = list(m.find_longest_match(0, len(a), 0, len(b)))
    blocks = [list(x) for x in m.get_matching_blocks()]
    return [kind, junk, autojunk, a, b, longest, blocks, bits(m.ratio()), bits(m.quick_ratio()),
            bits(m.real_quick_ratio())]


def rand_str(rnd, alphabet, lo, hi):
    return "".join(rnd.choice(alphabet) for _ in range(rnd.randint(lo, hi)))


def main():
    rnd = random.Random(20261017)
    cases = []
    fixed = [("", ""), ("a", ""), ("", "b"), ("abxcd", "abcd"), ("qabxcd", "abycdf"), ("private Thread currentThread;",
             "private volatile Thread currentThread;"), (" abcd", "abcd abcd"), ("fats", "fast"), ("abc", "abcy"),
             ("añoé", "anoe"), ("😀a😀", "a😀😀")]
    for a, b in fixed:
        for junk in ("none", "space"):
            cases.append(case("str", junk, True, a, b))
    for _ in range(150):
        cases.append(case("str", rnd.choice(["none", "space"]), True, rand_str(rnd, "ab c", 0, 25),
                          rand_str(rnd, "ab c", 0, 25)))
    for _ in range(30):  # autojunk: b of 200+ elements with popular characters
        b = rand_str(rnd, "aaaaabbbc d", 200, 320)
        a = rand_str(rnd, "abcd x", 20, 80)
        cases.append(case("str", "none", rnd.choice([True, False]), a, b))
    words = ["alpha", "beta", "", "  ", "gamma", "beta", "delta", "\t"]
    for _ in range(40):
        a = [rnd.choice(words) for _ in range(rnd.randint(0, 12))]
        b = [rnd.choice(words) for _ in range(rnd.randint(0, 12))]
        cases.append(case("lines", rnd.choice(["none", "blank"]), True, a, b))
    names = ["fast", "slow", "auto", "abcx", "abcy", "zzzz", "BAlarmService", "BAlarmClass", "BAlarmRecord",
             "BWebServlet", "BServletView", "alarm", "apple", "ape", "peach", "puppy", "año"]
    close = []
    for word in ["fats", "abc", "zz", "BAlarmServce", "Servlet", "appel", "pech", "", "ano", "BALARMSERVICE"]:
        for n, cutoff in [(1, 0.6), (3, 0.6), (5, 0.0), (2, 0.75), (3, 1.0), (0, 0.6), (3, 1.5)]:
            try:
                result = difflib.get_close_matches(word, names, n, cutoff)
            except ValueError:
                result = None
            close.append([word, names, n, cutoff, result])
    with open(os.path.join(HERE, "fixtures.json"), "w", encoding="utf-8") as fh:
        fh.write('{\n "python": %s,\n "matcher": [\n' % json.dumps(sys.version.split()[0]))
        fh.write(",\n".join("  " + json.dumps(c, ensure_ascii=False) for c in cases))
        fh.write('\n ],\n "close": [\n')
        fh.write(",\n".join("  " + json.dumps(c, ensure_ascii=False) for c in close))
        fh.write("\n ]\n}\n")
    print(len(cases), "matcher cases,", len(close), "close cases", file=sys.stderr)


if __name__ == "__main__":
    main()
