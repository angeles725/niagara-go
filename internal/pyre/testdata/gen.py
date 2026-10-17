"""Regenerate errors.json and matches.json from the real Python re module:
python3 -I internal/pyre/testdata/gen.py

errors.json: every pattern that does not compile, with its re.error text. matches.json: the subjects and, per compiling pattern, the search result
(spans of the match and of each group, None when a group did not take part) on every subject,
plus findall for the patterns whose findall is a list of strings. One case per line."""
import json
import os
import re

I, M, S, X, A = re.IGNORECASE, re.MULTILINE, re.DOTALL, re.VERBOSE, re.ASCII
SUBJECTS = ["", "abc", "ABC xyz", "a\nb\n", "foo.bar(baz)", "set|temp", "Temperature setpoint",
            "B722 [Block 18] bloque106", "aaa", "año ÑANDÚ ½ x²", "tab\there", "x\x1cy", "word_1 two-3",
            "[CERT-doc/INFER] alarm", "slot:/Programacion/ColdRoom_1", "abab", "12 3456", "\u2028 sep"]
PATTERNS = [
    # errors
    "[", "(", ")", "a)", "[a", "(?", "(?P", "(?Px)", "(?P<", "(?P<>x)", "(?P<1a>x)", "(?P<n>a)(?P<n>b)",
    "(?P=x)", "(?P<n>(?P=n))", "(?<n>a)", "(?<x)", "(?#abc", "\\", "a\\", "\\q", "\\8", "[\\q]", "[z-a]",
    "[\\d-z]", "a**", "a{2}{3}", "*a", "^*", "\\b+", "a{3,1}", "(?i)a(?m)b", "a(?i)", "(?z)", "(?i-i:a)",
    "(?-a:x)", "(?au)", "(?L)", "(?i", "(?-", "(?-:", "(?i-:a)", "\\x1", "\\u12", "\\U0011FFFF", "\\N{DASH}",
    "\\N", "(?(1)a|b|c)", "(?(0)a)", "(?(x)a)", "(?(2)a)(b)", "(?(3)a)", "(?<=a+)b", "(?<=ab|c)d",
    "(?<!(a)\\1)", "(a)(?<=\\1)", "\\400", "[\\400]", "abc\n(", "x{99999999999}", "(?P<a-b>x)",
    # valid
    "abc", "a.c", "^a", "c$", "b$", "\\Aa", "c\\Z", "c\\z", "a|b|c", "(a)|(b)", "(?:ab)+", "a{,2}", "a{2,}",
    "a{2}", "a{1,2}?", "a*+", "(?>a+)b", "x{", "x{}", "x{a}", "x{1,a}", "\\d+", "\\D+", "\\s", "\\S+", "\\w+",
    "\\W+", "[\\w]+", "[^\\W]+", "[\\s]", "[a-c]+", "[^a-c]+", "[]a]", "[a-]", "[-a]", "[\\]]", "[\\^]",
    "\\bbloque\\d+\\b", "\\Bb", "(?i)abc", "(?i:a)bc", "(?-i:a)", "(?m)^b", "(?s)a.b", "(?x) a b # c\n c",
    "(?x)[ ]a", "(?a)\\w+", "(?a:\\d+)", "(?a)\\bx", "(?P<n>a)(?P=n)", "(a)\\1", "(?P<word>\\w+)",
    "(?=a)a", "(?!a).", "(?<=a)b", "(?<!a)b", "(a)?(?(1)b|c)", "(a)?(?(1)b)", "\\x41", "\\u0041",
    "\\U00000041", "\\0", "\\012", "\\101", "[\\101]", "\\.", "\\(", "\\|", "set|temp", "panel|label",
    "parent\\.set", "B[0-9]", "alarm(s)?", "^B", "DEF", "\\[\\s*(CERT|INFER)", "(?i)ñandú", "²", "\\a\\t",
    "[\\b]", "(?#comment)a", "a(?#x)*", "(?:)", "()", "(|a)", "x|", "\\$", "[$]", "[.]", "(?i)[a-z]+",
    "[^\\W\\d]+", "[\\W\\d]+", "(?a)[\\W]+", "(?a)[^\\D]+", "[\\S]+", "[x\\W\\S]", "(?s:.)", "a(?=b)|ab", "(?:a|b)*c", "(?x)a\\ b", "\\u2028", "[\\u2028]", "(?m)$", "\\Bb\\B",
]


def run():
    cases = []
    for flags in (0, I):
        for p in PATTERNS:
            case = {"pattern": p, "flags": flags}
            try:
                rx = re.compile(p, flags)
            except (re.error, OverflowError) as e:
                case["error"] = str(e)
                cases.append(case)
                continue
            out = []
            for s in SUBJECTS:
                m = rx.search(s)
                if m is None:
                    out.append(None)
                else:
                    out.append([list(m.span())] + [list(m.span(g)) if m.group(g) is not None else None
                                                   for g in range(1, rx.groups + 1)])
            case["search"] = out
            # findall after an empty match differs by design (see the package doc): not pinned.
            empty = any(m.start() == m.end() for s in SUBJECTS for m in rx.finditer(s))
            if rx.groups <= 1 and not empty:
                case["findall"] = [rx.findall(s) for s in SUBJECTS]
            cases.append(case)
    return {"subjects": SUBJECTS, "cases": cases}


def dump(path, head, items):
    """JSON with one item of *items* per line, so a fixture diff shows the changed cases."""
    with open(path, "w", encoding="utf-8") as fh:
        fh.write("{" + "".join(f"{json.dumps(k)}: {json.dumps(v, ensure_ascii=False)}, " for k, v in head.items()))
        fh.write('"cases": [\n' + ",\n".join(json.dumps(i, ensure_ascii=False) for i in items) + "\n]}\n")


if __name__ == "__main__":
    here = os.path.dirname(os.path.abspath(__file__))
    data = run()
    dump(os.path.join(here, "errors.json"), {},
         [c for c in data["cases"] if "error" in c])
    dump(os.path.join(here, "matches.json"), {"subjects": data["subjects"]},
         [c for c in data["cases"] if "error" not in c])
