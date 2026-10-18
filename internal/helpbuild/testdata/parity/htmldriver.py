"""Python twin of internal/helpbuild/parity_html_test.go (slices S5 and S6).

Run with `python3 -I -B htmldriver.py TOOLS MODE ARGS...` (see driver.py).
Every mode prints JSON lines (json.dumps defaults) that the Go test rebuilds.

  pages            stdin: [page, ...]; per page [events, title, text]:
                   the HTMLParser(convert_charrefs=True) callbacks the
                   converter overrides (["s", tag, attrs], ["se", tag,
                   attrs], ["e", tag], ["d", data]) and html_text.convert
  fuzz SEED COUNT  COUNT seeded random snippets, per snippet
                   [page, events, title, text]
  entities         sorted(html.entities.html5)
  unescape         stdin: [s, ...]; [html.unescape(s), ...]
  toc PATH...      per path ["toc", parse_toc(path) as [[page, title,
                   trail], ...], count_items(path), stderr lines]
  run BUILDER BASE BUILDER guides | devguide | guidesindex | devguidetoc:
                   ["out", line] / ["err", line] per printed line, then
                   ["result", value] or ["raise", exception class name]
"""

import contextlib
import html
import html.entities
import html.parser
import io
import json
import random
import sys

sys.path.insert(0, sys.argv[1])

from niagara_help_lib import guide_toc, html_text  # noqa: E402


class _Recorder(html.parser.HTMLParser):

    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.events = []

    def handle_starttag(self, tag, attrs):
        self.events.append(["s", tag, [list(a) for a in attrs]])

    def handle_startendtag(self, tag, attrs):
        self.events.append(["se", tag, [list(a) for a in attrs]])

    def handle_endtag(self, tag):
        self.events.append(["e", tag])

    def handle_data(self, data):
        self.events.append(["d", data])


def events(page):
    rec = _Recorder()
    rec.feed(page)
    rec.close()
    return rec.events


def pages(items):
    for page in items:
        title, text = html_text.convert(page)
        print(json.dumps([events(page), title, text]))


# Fragments the fuzzer strings together: every construct the tokenizer and
# the converter branch on, their malformed variants and the characters
# whose class differs between ASCII and Python's Unicode rules.
_FRAGMENTS = [
    "<p>", "</p>", "<P class=x>", "<div>", "</div>", "<br>", "<br/>", "<hr>",
    "<b>", "</b>", "<i>", "</I>", "<span>", "</span>", "<a href='x.html'>",
    "</a>", "<h1>", "</h1>", "<h2 id=a>", "</h2>", "<li>", "</li>", "<ul>",
    "</ul>", "<table>", "</table>", "<TABLE border=1>", "<tr>", "</tr>",
    "<td>", "</td>", "<th>", "</th>", "<td colspan=\"2\">", "<pre>", "</pre>",
    "<PRE>", "<title>", "</title>", "<TITLE>", "<head>", "</head>",
    "<script>", "</script>", "</script >", "</SCRIPT>", "<style>", "</style>",
    "<noscript>", "</noscript>", "<textarea>", "</textarea>", "<xmp>",
    "</xmp>", "<plaintext>", "<iframe>", "</iframe>",
    "<img src=\"a.png\" alt=\"A pic\">", "<img alt='only alt'/>",
    "<img src=b.gif>", "<img alt=\"&amp;x&ampy&lt=&#65;&#x42\" src=' s '>",
    "<img alt>", "<img alt=>", "<img alt=\"a\" alt=\"b\">", "<IMG SRC=c.png>",
    "<a b='unclosed>", "<a b=\"c\"d>", "<a =x>", "<a/>", "<a / >", "<a//>",
    "<a x='1'/>", "<a x= 'y>", "<a x = \"q\" y>", "<a\x0b>", "<a\tb\nc>",
    "</ a>", "</>", "</1>", "</a x='>'>", "</td x>", "<!-- c -->", "<!--",
    "-->", "<!-->", "<!--->", "--!>", "<!-- a -- b --!>", "<!doctype html>",
    "<!DOCTYPE x", "<![CDATA[x<y]]>", "<![CDATA[", "<!x>", "<?pi x?>", "<?",
    "</", "<", ">", "<1>", "< p>", "<é>", "<K>", "</Kr>",
    "&amp;", "&#8212;", "&#x41", "&#65", "&notit;", "&", "&#", "&#x", "&nbsp",
    "&rsquo;", "&lt;b&gt;", "&#0;", "&#1;", "&#128;", "&#xD800;", "&bogus;",
    "\"", "'", "=", "/", " ", "  ", "\t", "\n", "\r\n", "\r", "\n\n\n",
    "\x00", "\x0b", "\x0c", "\x1c", "\x85", "\xa0", " ", "　",
    "word", "Word", "x_y", "42", "été", "日本", "K",
    "İ", "ı", "ſ", "²", "٠", "|", "a|b",
    "Index | Prev | Next", "Prev|Next", "Index |Prev | Next", "```",
    "``` ", "` ``", "- ", "#",
]
_CHARS = "<>/&#;='\"!-?abAB01 \t\n\r\x00éK"


def snippet(rng):
    parts = []
    for _ in range(rng.randint(1, 40)):
        if rng.random() < 0.15:
            parts.append("".join(rng.choice(_CHARS)
                                 for _ in range(rng.randint(1, 6))))
        else:
            parts.append(rng.choice(_FRAGMENTS))
    page = "".join(parts)
    if rng.random() < 0.2 and page:  # cut anywhere: unterminated constructs
        page = page[:rng.randint(0, len(page))]
    return page


def fuzz(seed, count):
    rng = random.Random(seed)
    for _ in range(count):
        page = snippet(rng)
        title, text = html_text.convert(page)
        print(json.dumps([page, events(page), title, text]))


def toc(paths):
    for path in paths:
        err = io.StringIO()
        with contextlib.redirect_stderr(err):
            pages_ = guide_toc.parse_toc(path)
        rows = [[k, v[0], v[1]] for k, v in pages_.items()]
        print(json.dumps(["toc", rows, guide_toc.count_items(path),
                          err.getvalue().splitlines()]))


def run(builder, base):
    import build_devguide_toc
    import build_guides_index
    import devguide_parser
    import guides_parser
    out, err = io.StringIO(), io.StringIO()
    try:
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            if builder == "guides":
                result = guides_parser.parse_guides(base)
            elif builder == "devguide":
                result = devguide_parser.parse_devguide(base)
            elif builder == "guidesindex":
                sys.argv = ["build_guides_index.py", base]
                result = build_guides_index.main()
            elif builder == "devguidetoc":
                sys.argv = ["build_devguide_toc.py", base]
                result = build_devguide_toc.main()
            else:
                raise SystemExit("unknown builder " + builder)
        last = ["result", result]
    except Exception as e:  # noqa: BLE001 - the class is the result
        last = ["raise", type(e).__name__]
    for line in out.getvalue().splitlines():
        print(json.dumps(["out", line]))
    for line in err.getvalue().splitlines():
        print(json.dumps(["err", line]))
    print(json.dumps(last))


def main():
    mode, args = sys.argv[2], sys.argv[3:]
    if mode == "pages":
        pages(json.load(sys.stdin))
    elif mode == "fuzz":
        fuzz(int(args[0]), int(args[1]))
    elif mode == "entities":
        print(json.dumps(sorted(html.entities.html5)))
    elif mode == "unescape":
        print(json.dumps([html.unescape(s) for s in json.load(sys.stdin)]))
    elif mode == "toc":
        toc(args)
    elif mode == "run":
        run(args[0], args[1])
    else:
        raise SystemExit("unknown mode " + mode)


if __name__ == "__main__":
    main()
