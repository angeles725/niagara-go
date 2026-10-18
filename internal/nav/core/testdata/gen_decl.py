"""Regenerate decl.json: corpus.py's declaration regexes applied to sample lines.

    python3 -I internal/nav/core/testdata/gen_decl.py [<module-navigator>/tools]
"""

import json
import os
import sys

TOOLS = sys.argv[1] if len(sys.argv) > 1 else os.path.expanduser(
    "~/niagara-research/module-navigator/tools")
sys.path.insert(0, TOOLS)
from module_nav_lib import corpus  # noqa: E402

LINES = [
    "public class Foo {", "  public static final class Inner$1 extends X {", "class A",
    "@Deprecated public abstract class B", "@Ann(x = \")\") class C", "@Ann(x)class D",
    "@interface Marker {", "public @interface Marker {", "@interfaceX class E",
    "@interface.Foo class F", "  record Point(int x, int y) {", "record Pair<A, B>(A a) {}",
    "record = 5;", "record Foo$Bar(int x)", "record  Foo  (", "enum Color {", "enumeration x",
    "interface I<T>", "sealed interface S permits A", "non-sealed class N", "final finally class X",
    "publicclass Z", "　class Wide", "class été", "\tprivate static class $X",
    "protected strictfp class P", "@a.b.C @D(\"q\") final class Q", "@A(1) @B", "class", "class ",
    "// class Commented", "new Object() { class Local {} }", "subclass Foo", "x.class Foo",
    "@Override", "@interface", "@interfaces", "@ interface", "package com.x.y;", "package com.x.y ;",
    "package  a_b.c2 ;  // c", "package com.x", "packagecom.x;", "package é.x;", "",
    "foo record Bar<T>(x)", "class Nbsp", "a enum B", "_class X", "$class Y",
]


def main():
    out = []
    for line in LINES:
        m = corpus.DECL_RE.match(line)
        p = corpus.PACKAGE_RE.match(line)
        out.append({
            "line": line,
            "decl": list(m.groups()) if m else None,
            "end": m.end() if m else None,
            "type": corpus.type_decl_name(line),
            "annotation": bool(corpus.ANNOTATION_LINE_RE.match(line)),
            "package": p.group(1) if p else None,
        })
    here = os.path.dirname(os.path.abspath(__file__))
    with open(os.path.join(here, "decl.json"), "w", encoding="utf-8") as f:
        f.write("[\n" + ",\n".join(json.dumps(o, ensure_ascii=False) for o in out) + "\n]\n")


if __name__ == "__main__":
    main()
