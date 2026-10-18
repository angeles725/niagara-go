"""Regenerate internal/fnmatch/testdata/fixtures.json from this Python's fnmatch.

Run from the repository root: python3 -I internal/fnmatch/testdata/gen.py

Each case is [pattern, [names that fnmatchcase matches], number of names it does not
match]; every pattern is tried on every name of NAMES plus random ones.
"""
import fnmatch
import json
import os
import random
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
PATTERNS = ["[a-a]", "[a-c-c]", "[!a-a]x", "", "*", "**", "?", "*.py", "a*b*c", "a*b?c*", "[!a-c]x", "[]]", "[!]]", "[a-]", "[z-a]", "[!]",
            "[]", "[a", "*?[", "a**b", "[\\-z]", "[a&&b]", "[[:alpha:]]", "[^x]", "[\\]", "x.y+z", "[c-a-z]",
            "é?ñ", "B*Servlet", "*Alarm*", "do*Alarm", "[A-Z]*", "[!A-Z]*", "*[0-9]", "a[b-d-f]g", "[--z]",
            "[!--]", "[a-c-e-g]", "[%-*]", "[*]", "[?]", "\\*", "a\\b", "(x)|{y}$", "[a|b]", "[~]", "*\n*",
            "[\n]", "[é-ñ]", "*.*", ".*", "a*", "*a", "*a*a*a*", "?*?", "[!]a]", "[]-a]", "[!-]", "[-]", "[a[:alpha:]]", "[x[:digit:]y]*"]
NAMES = ["", "a", "b", "x", "ax", "dx", "]", "-", "^", "!", "\\", "[", "*", "?", ".", "&", "|", "~", ":", "z",
         "abc", "aXbYc", "abbc", "x.y+z", "xay+z", "é1ñ", "éñ", "BWebServlet", "BServlet", "BAlarmService",
         "doRouteAlarm", "doAlarm", "Alarm", "alarm", "1", "x9", "abg", "aeg", "acg", "%", "+", "(x)|{y}$",
         "a\nb", "\n", "ü", "f.py", ".py", "py", "aaa", "aa", "a.b", "AB", "aZ", "a\\b", "b]", "a]", "5]", "x5"]


def main():
    rnd = random.Random(20261017)
    pats = list(PATTERNS) + ["".join(rnd.choice("ab*?[]!-^\\.&é") for _ in range(rnd.randint(1, 8)))
                             for _ in range(150)]
    names = NAMES + ["".join(rnd.choice("ab.-]^\\é\n*") for _ in range(rnd.randint(0, 6))) for _ in range(60)]
    cases = []
    for p in pats:
        hits = [n for n in names if fnmatch.fnmatchcase(n, p)]
        miss = [n for n in names if not fnmatch.fnmatchcase(n, p)]
        cases.append([p, hits, len(miss)])
    with open(os.path.join(HERE, "fixtures.json"), "w", encoding="utf-8") as fh:
        fh.write('{\n "python": %s,\n "names": %s,\n "cases": [\n'
                 % (json.dumps(sys.version.split()[0]), json.dumps(names, ensure_ascii=False)))
        fh.write(",\n".join("  " + json.dumps(c, ensure_ascii=False) for c in cases))
        fh.write("\n ]\n}\n")
    print(len(cases), "patterns x", len(names), "names", file=sys.stderr)


if __name__ == "__main__":
    main()
