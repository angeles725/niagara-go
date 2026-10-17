"""Regenerate internal/glossary/testdata/fixtures.json from the three Python glossaries.

Run: python3 -I internal/glossary/testdata/gen.py   (NIAGARA_RESEARCH overrides the checkout path)

  canonical   tools/lib/glossary.py: lookup, translations, expand, weighted_terms, note
  classrank   module-navigator class_rank._GLOSSARY_BY_STEM (keys stemmed like its query terms)
  help        niagara-help search_lang.GLOSSARY (canonical minus EXCLUDED) via fulltext_terms._glossary
  parse       glossary.glossary() on malformed data files (the str / list / bad-entry rules)
"""
import contextlib
import io
import json
import os
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
NR = os.environ.get("NIAGARA_RESEARCH", "/home/cristian/niagara-research")
sys.path[:0] = [os.path.join(NR, "tools", "lib"), os.path.join(NR, "module-navigator", "tools"),
                os.path.join(NR, "niagara-help", "tools")]

import glossary  # noqa: E402
from module_nav_lib import class_rank  # noqa: E402
from niagara_help_lib import fulltext_terms, search_lang  # noqa: E402

QUERIES = [
    "", "alarm", "alarma", "Alarmas de temperatura alta", "¿Cómo configuro las alarmas del cuarto frío?",
    "configuración del servidor", "configuraciones", "conexiones y sesiones", "histeresis del termostato",
    "chasis", "chasises", "registro de usuarios", "como montar un servlet", "como se guardan las trends",
    "copia de respaldo", "clave del usuario", "BACnet red", "ALARMA ALARMA alarma", "estado y estados",
    "descongelamiento deshielo evaporadora", "valvulas y válvulas", "contraseña", "Contraseñas",
    "punto puntos", "aviso de falla", "presión de succión", "mínimo y máximo", "reconocer alarmas",
    "the alarm config", "config configuracion", "notificacion notification", "x y z", "ñandú",
]
WORDS = ["alarma", "alarmas", "alarmases", "configuracion", "configuraciones", "conexiones", "chasis",
         "chasises", "histeresis", "s", "es", "", "ses", "montar", "guardan", "puntos", "valvulas",
         "red", "estados", "Alarma", "ALARMAS", "cuartos", "ñandu", "straße", "sesiones"]

PARSE = {
    "bare_string.json": '{"glossary": {"alarma": "alarm", "aire": ["air"]}}',
    "bad_entries.json": '{"glossary": {"a": ["x", 1], "b": 5, "c": ["y"], "d": [], "e": null, "f": {"g": 1}}}',
    "duplicate_key.json": '{"glossary": {"alarma": ["one"], "aire": ["air"], "alarma": ["two"]}}',
    "no_glossary_key.json": '{"about": "x"}',
    "glossary_is_list.json": '{"glossary": ["alarma"]}',
    "top_is_list.json": '["glossary"]',
    "not_json.json": '{"glossary": {',
    "empty.json": '',
    "nested_dup_top.json": '{"glossary": {"a": ["x"]}, "glossary": {"b": ["y"]}}',
}


def parse_cases():
    out = {}
    with tempfile.TemporaryDirectory() as tmp:
        for name, text in sorted(PARSE.items()):
            path = os.path.join(tmp, name)
            with open(path, "w", encoding="utf-8") as fh:
                fh.write(text)
            glossary.DATA = path
            glossary.glossary.cache_clear()
            err = io.StringIO()
            with contextlib.redirect_stderr(err):
                g = glossary.glossary()
            out[name] = {"input": text, "entries": [[k, list(v)] for k, v in g.items()],
                         "warned": bool(err.getvalue())}
    return out


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
    canonical = {
        "lookup": [[w, list(glossary.lookup(w) or ())] for w in WORDS],
        "translations": [[q, [[w, list(t)] for w, t in glossary.translations(q)]] for q in QUERIES],
        "expand": [[q, glossary.expand(q)] for q in QUERIES],
        "weighted_terms": [[q, [[t, w.hex()] for t, w in glossary.weighted_terms(q).items()]] for q in QUERIES],
        "note": [[q, glossary.note(q)] for q in QUERIES],
        "keys": list(glossary.glossary().keys()),
    }
    by_stem = class_rank._GLOSSARY_BY_STEM
    out = {
        "canonical": canonical,
        "classrank": {"by_stem": [[k, list(v)] for k, v in by_stem.items()]},
        "help": {"keys": list(search_lang.GLOSSARY), "excluded": list(search_lang.EXCLUDED),
                 "lookup": [[w, list(fulltext_terms._glossary(w))] for w in WORDS]},
        "parse": parse_cases(),
    }
    with open(os.path.join(HERE, "fixtures.json"), "w", encoding="utf-8", newline="\n") as fh:
        fh.write("\n".join(render(out)) + "\n")


if __name__ == "__main__":
    main()
