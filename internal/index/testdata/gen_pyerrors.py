"""Regenerate pyerrors.json: the error text Python 3 prints for each case.

    python3 -I internal/index/testdata/gen_pyerrors.py

"loads" cases are json.loads(text); "file" cases are bytes written to a file and
read with json.load(open(path, encoding="utf-8")) (universal newlines, UTF-8
decoding), the way module-navigator reads its indexes.
"""

import json
import os
import sys
import tempfile

LOADS = [
    '', ' ', '{', '{"a"', '{"a":', '{"a":1', '{"a":1,', '{"a":1,}', '[1,]', '[1', '[',
    '{1:2}', '"abc', '"a\\x"', '"a\\u12"', '"a\x01"', 'tru', 'nul', '1 2', '{} x', '[1 2]',
    '{"a" 1}', '-', '01', '1.', '1e', '-x', '﻿{}', '{"a":1 "b":2}', '[1,,2]', '{,}',
    '\n\n  {"a":}', '"é\\n"x', '\r\n[\r\n1,\r]', '"\\ud83dx', '"ab\\', '{"a":[1,{"b":}]}',
    '[Infinit]', '"\\ud83d\\uzzzzzz"', '[1]\né',
]

FILE = [
    b'\r\n[\r\n1,\r]', b'\r\r{"a" 1}', b'{"a": "\xff"}', b'[1, "\xe4A"]', b'["x\xe4\xb8',
    b'\xef\xbb\xbf{}', b'{"k": [1, 2,]}\n', b'',
]


def main():
    out = {"loads": [], "file": []}
    for text in LOADS:
        try:
            json.loads(text)
            msg = None
        except json.JSONDecodeError as e:
            msg = str(e)
        out["loads"].append([text, msg])
    with tempfile.TemporaryDirectory() as tmp:
        path = os.path.join(tmp, "index.json")
        for data in FILE:
            with open(path, "wb") as f:
                f.write(data)
            try:
                with open(path, "r", encoding="utf-8") as f:
                    json.load(f)
                msg = None
            except ValueError as e:
                msg = str(e)
            out["file"].append([data.hex(), msg])
    here = os.path.dirname(os.path.abspath(__file__))
    with open(os.path.join(here, "pyerrors.json"), "w", encoding="utf-8") as f:
        f.write('{"python": %s' % json.dumps(sys.version.split()[0]))
        for key in ("loads", "file"):  # one case per line: readable diffs
            f.write(',\n "%s": [\n  ' % key)
            f.write(",\n  ".join(json.dumps(c) for c in out[key]))
            f.write("]")
        f.write("}\n")


if __name__ == "__main__":
    main()
