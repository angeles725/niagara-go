"""Regenerate internal/pycsv/testdata/fixtures.json from this Python's csv.writer.

Run from the repository root: python3 -I internal/pycsv/testdata/gen.py

Each case is [delimiter, lineterminator, rows, output]: writerows(rows) of a csv.writer
with the excel dialect (QUOTE_MINIMAL, quotechar '"', doublequote) and that delimiter and
lineterminator. Values are [kind, value] pairs: none, bool, int (decimal text), float
(repr; nan, inf and -inf spelled so), str.
"""
import csv
import io
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))


def typed(v):
    if v is None:
        return ["none"]
    if isinstance(v, bool):
        return ["bool", v]
    if isinstance(v, int):
        return ["int", str(v)]
    if isinstance(v, float):
        return ["float", repr(v)]
    return ["str", v]


ROWS = [
    [["a", "b", "c"]],
    [["a,b", 'q"uote', "nl\nx", "cr\rx", "crlf\r\nx", " lead", "trail ", "", None, "a b"]],
    [[""]], [[None]], [[]], [["", ""]], [["a", ""]], [["", "a"]], [[None, None]],
    [[1, -7, 2.5, -0.0, 0.1, 1e16, 1e-7, True, False, 10 ** 20, float("nan"), float("inf"), float("-inf")]],
    [["tab\tx", "semi;x", "pipe|x", "space x", "X marks", "comma,x"]],
    [["é", "日本", "\U0001f600", "\x00", "\\"]],
    [['"'], ['""'], ['a"b"c'], ['"start'], ['end"']],
    [["\r\n"], ["\n"], ["\r"]],
    [["h1", "h2"], ["v1", None], [], [""], ["x,y", "z"]],
]
DIALECTS = [(",", "\r\n"), (",", "\n"), (";", "\r\n"), ("\t", "\n"), (" ", "\r\n"), ("|", "X"), (",", "\r")]


def write(rows, delimiter, lineterminator):
    buf = io.StringIO()
    csv.writer(buf, delimiter=delimiter, lineterminator=lineterminator).writerows(rows)
    return buf.getvalue()


def main():
    cases = [[d, lt, [[typed(v) for v in row] for row in rows], write(rows, d, lt)]
             for d, lt in DIALECTS for rows in ROWS]
    with open(os.path.join(HERE, "fixtures.json"), "w", encoding="utf-8") as fh:
        fh.write('{\n "python": %s,\n "cases": [\n' % json.dumps(sys.version.split()[0]))
        fh.write(",\n".join("  " + json.dumps(c, ensure_ascii=False) for c in cases))
        fh.write("\n ]\n}\n")
    print("fixtures.json written", file=sys.stderr)


if __name__ == "__main__":
    main()
