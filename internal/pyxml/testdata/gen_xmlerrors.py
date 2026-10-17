"""Regenerate xmlerrors.json: what expat says about well-formed and malformed XML (the specification
of internal/pyxml, the parser shared by bogcycles, bogwrite and stationmodules).

    python3 -I internal/pyxml/testdata/gen_xmlerrors.py

Each case records two results, "" when the document is accepted, and for an accepted etree
document its tree as [tag, [[name, value], ...], [children...]]:
  expat  pyexpat ParserCreate().Parse(text, True) on a str (bog-cycles.py require_wellformed)
  etree  xml.etree.ElementTree.fromstring(bytes) (bog-write.py validate_xml, station-modules.py)
Inputs: hand-written cases, every truncation of SAMPLE, and every single-character insertion and
deletion of SHORT, and RANDOM documents with two random edits of SAMPLE (fixed seed). "in" is the text (UTF-8 encoded for etree); "hex" cases are raw bytes, etree only.
"""
import json
import random
import os
import xml.etree.ElementTree as ET
import xml.parsers.expat

HERE = os.path.dirname(os.path.abspath(__file__))

SAMPLE = ("<?xml version='1.0' encoding='UTF-8'?>\r\n<!-- c -->\n<bajaObjectGraph version='4.0'>"
          "<p n='A' h=\"1\" t='b:Folder' v='x&amp;y&#10;&#x41;\tz'><![CDATA[<a>]]><?pi d?>"
          "t&lt;é<q/></p>\n</bajaObjectGraph>\n<!-- e -->")
SHORT = "<?xml version='1.0'?><b a='1' c=\"&lt;\"><!-- x --><d/>t&#65;<![CDATA[y]]><?p q?></b>"
RANDOM = 800
INSERT = ["<", ">", "&", "'", '"', "/", "!", "?", "=", "-", "]", " ", "\n", "x", "1", ":", ";", "#", "\x01"]

HAND = [
    "", "   ", "<", "<b", "<b>", "<b>abc", "<b>&am", "<b>&amp", "<b a='x", "<b><!--", "<b><?pi",
    "<b><![CDATA[abcdef", "<b><![CDATA[", "<b><![CDA", "<b></b>\n<c/>", "<b/>  <!-- c --> <?p?>", "<b/>x",
    "﻿<b/>", "﻿<b></c>", "﻿<?xml version='1.0'?><b></c>", "<b>\r\nx\r\n<c></d></b>",
    "<b>\rx\r<c></d></b>", "<b>é<c></d></b>", "<b>\U0001d11e<c></d></b>", "<!DOCTYPE b><b/>",
    "<!DOCTYPE b [<!ENTITY e 'x'>]><b>&e;</b>", "<!DOCTYPE b SYSTEM 'x.dtd'><b>&e;</b>",
    "<?xml version='1.0'?>", "<?xml?><b/>", "<?xml version='1.0' encoding='latin1'?><b/>",
    "<?xml version='1.0' standalone='yes'?><b/>", "<?xml version='1.0' standalone='maybe'?><b/>",
    "<?xml encoding='UTF-8'?><b/>", "<?xml version='1.0'encoding='UTF-8'?><b/>", "<?xml version=1.0?><b/>",
    "<?xml version='1.0' foo='x'?><b/>", "<?xml version='2.0'?><b/>", "<?xml  version = '1.0' ?><b/>",
    "<b>&#x110000;</b>", "<b>&#xD800;</b>", "<b>&#65;&#x41;</b>", "<b a='&lt;&#10;'/>", "<b a='1'b='2'/>",
    "<b a = '1' />", "<b\ta='1'/>", "<b/>]]>", "<b>--</b><!-- a -- b -->", "<b><!-- a --></b>",
    "<b><!-- a ---></b>", "<?XML version='1.0'?><b/>", "<b><?xml x?></b>", "<b><?xmlx x?></b>",
    "<b><?Xml x?></b>", "<b><c></b>", "</b>", "<b></b></b>", "x", "#", " x", "x<b/>", "1<b/>", "-<b/>",
    "'a'<b/>", "\"a\"<b/>", "[<b/>", "]<b/>", "(<b/>", ")<b/>", "|<b/>", ",<b/>", "%<b/>", "%x;<b/>",
    "*<b/>", "?<b/>", "+<b/>", "=<b/>", "&<b/>", ">'<b/>", "/<b/>", "\x01<b/>", "#x<b/>", "#PCDATA<b/>",
    "x>", "x)", "x[", "x]", "x*", "x?", "x+", "x,", "x|", "x%", "x=", "x/", "x;", "x'", "x&", "x#",
    "<b/>1", "<b/>-", "<b/>'a'", "<b/>[", "<b/>#", "<b/>\x01", "<b/>&", "<b/><!DOCTYPE b>", "<b/><![CDATA[x]]>",
    "<b/><c", "<b/></c>", "<b/><!-- x", "<b/><?p", "<b/>x<", "<b/>x>", "<!-- x --><!-- y -->",
    "<b>\x00</b>", "<b c=\"'\"/>", "<b:c xmlns:b='u'/>", "<b xmlns='u'><c/></b>", "<b a:c='1'/>",
    "<a:b/>", "<b>\x7f\x80</b>", "<é/>", "<b-c.d/>", "<.b/>", "<b>&#;</b>", "<b>&#x;</b>", "<b>&#12a;</b>",
    "<b>& </b>", "<b>&1;</b>", "<b>&foo;</b>", "<b a='&foo;'/>", "<b a='x&foo;' c='1' c='2'/>",
    "<b a='1' a='2' c='&foo;'/>", "<b a='&#0;'/>", "<b a='&#;'/>", "<b a='&'/>", "<b a='a<b'/>",
    "<b xmlns:a='u' a:x='1' xmlns:c='u' c:x='2'/>", "<b a='1' a='1'/>", "<b a='1' a='1' <", "<b a='1' a='1'",
    "<b><c a='1' a='1'/></d></b>", "<b>]]</b>", "<b>]]", "<b>]", "<b>￾</b>", "<b>￿</b>",
    "<b a='￾'/>", "<b·/>", "<·b/>", "<b>&#x1F600;</b>", "<b>&#99999999999999999999;</b>",
    "<b/ >", "<b / >", "<b></b >", "<b></ b>", "<b></b\n>", "<b ></b>", "<b a='1'></b><!---->",
    "<b><![CDATA[]]></b>", "<b><![CDATA[x]]]></b>", "<b><![cdata[x]]></b>", "<b><!x></b>", "<b><!-x--></b>",
    "<b><?></b>", "<b><? x?></b>", "<b><?x?></b>", "<b><?xé?></b>", "<b><?x y\x01?></b>",
    "<b>\r</b>", "<b>\r", "<b>\n\n\n", "a\r\nb", "<b>&#xd;&#x9;&#xA;</b>", "<b>&#x00041;</b>",
    "<b xmlns:x=''/>", "<b a='x\ty\nz\r\nw\rv' c=\"&#9;&#10;&#13;&amp;&quot;\"/>",
    "<b xmlns='u' xmlns:p='v' p:a='1' a='2'><c xmlns=''><p:d/></c><e/></b>",
    "<!DOCTYPE b [<!ENTITY e 'xy'>]><b a='1&e;2'><c/></b>", "<p n='$20' t=\"b:Folder\"><p n='a' v='&lt;&gt;'/></p>", "<x:b xmlns:x='u'></x:b>", "<x:b xmlns:x='u'></b>", "<b xml:lang='en'/>",
    "<b xmlns:xml='http://www.w3.org/XML/1998/namespace'/>", "<b xmlns:a='u'><a:c/></b><a:d/>",
]

HEX = [
    "3c3f786d6c2076657273696f6e3d27312e302720656e636f64696e673d276c6174696e31273f3e3c623ee93c2f623e",
    "3c623ee93c2f623e", "3c623e61c33c2f623e", "3c623eff3c2f623e", "efbbbf3c622f3e",
    "3c3f786d6c2076657273696f6e3d27312e302720656e636f64696e673d2775732d6173636969273f3e3c623ee93c2f623e",
    "3c3f786d6c2076657273696f6e3d27312e302720656e636f64696e673d2749534f2d383835392d31273f3e3c623ee93c2f623e",
    "3c623ec3a93c2f623e", "3c623ee282ac3c2f623e", "3c623ee2823c2f623e", "3c623ef09f9880e93c2f623e",
]


def expat(text):
    p = xml.parsers.expat.ParserCreate()
    try:
        p.Parse(text, True)
    except xml.parsers.expat.ExpatError as exc:
        return str(exc)
    return ""


def tree(el):
    return [el.tag, [list(kv) for kv in el.attrib.items()], [tree(c) for c in el]]


def etree(data):
    try:
        ET.fromstring(data)
    except ET.ParseError as exc:
        return str(exc)
    except LookupError:
        return "!LookupError"  # an unknown declared encoding: Python crashes, Go says "unknown encoding"
    return ""


def main():
    texts = list(HAND)
    texts += [SAMPLE[:i] for i in range(len(SAMPLE) + 1)]
    texts += [SHORT[:i] + c + SHORT[i:] for i in range(len(SHORT) + 1) for c in INSERT]
    texts += [SHORT[:i] + SHORT[i + 1:] for i in range(len(SHORT))]
    rnd = random.Random(1)
    for _ in range(RANDOM):
        t = SAMPLE
        for _ in range(2):
            i = rnd.randrange(len(t) + 1)
            if rnd.random() < 0.3:
                t = t[:i] + t[i + 1:]
            else:
                t = t[:i] + rnd.choice(INSERT + ["<!--", "]]>", "&#", "<?", "</", "<x>", "xmlns:a='u'", "a:"]) + t[i:]
        texts.append(t)
    cases, seen = [], set()
    for t in texts:
        if t not in seen:
            seen.add(t)
            case = {"in": t, "expat": expat(t), "etree": etree(t.encode("utf-8"))}
            if case["etree"] == "":
                case["tree"] = tree(ET.fromstring(t.encode("utf-8")))
            cases.append(case)
    for h in HEX:
        cases.append({"hex": h, "etree": etree(bytes.fromhex(h))})
    with open(os.path.join(HERE, "xmlerrors.json"), "w", encoding="utf-8") as fh:
        fh.write("[\n" + ",\n".join(json.dumps(c, ensure_ascii=True) for c in cases) + "\n]\n")


if __name__ == "__main__":
    main()
