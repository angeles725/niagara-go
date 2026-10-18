"""Regenerate terms.json: the niagara-help tokenizer and query parser on tricky inputs.

Run: python3 -I internal/help/fts/testdata/gen_terms.py   (NIAGARA_HELP_PY_REF overrides the pre-go-retire worktree)

Each record holds the outputs of the real fulltext_terms functions (terms, word_terms of every
word, parse_query plain and relaxed, stem of every word) for one input text, plus
search_lang.STOP_WORDS.
"""
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))


def py_ref(repo, env, rev):  # D47: a clean worktree of <repo> tag pre-go-retire (docs/goldens.md)
    root = os.environ.get(env) or os.path.expanduser("~/niagara-research-worktrees/pre-go-retire-" + repo)
    git = lambda *a: subprocess.run(["git", "-C", root, *a], capture_output=True, text=True).stdout.strip()  # noqa: E731
    if git("rev-parse", "HEAD") != rev or git("status", "--porcelain"):
        sys.exit(f"{env}={root}: not a clean worktree of {repo} tag pre-go-retire ({rev[:9]}); see docs/goldens.md")
    return root


PYREF = py_ref("niagara-help", "NIAGARA_HELP_PY_REF", "8410d74511673e38b87622f7214fa5210ba25a63")
os.environ["NIAGARA_HELP_PYTHON"] = "1"  # a tag shim runs its Python, never Go
sys.path[:0] = [os.path.join(PYREF, "tools")]

from niagara_help_lib import fulltext_terms as ft  # noqa: E402

INPUTS = [
    "alarmAck", "HTMLParser", "BBacnetSetpointReference", "getXMLHttpRequest2Value",
    "ABc", "aB", "AB", "A", "x1y2", "IPv6Address", "MS/TP", "snake_case_name", "__init__",
    "configuración del módulo", "72 °F and 21 ℃", "x² + y³ = z⁴", "²", "1", "a", "ß ﬁle ㎒",
    "events configuring registration registries studies classes boss glasses stations",
    "ies ings ation bases cases", "como funciona el ack de alarmas", "the and of", "a b c",
    "Alarmas ALARMA alarmás", "bbmd wiretap", "discovery timeout", "web servlet",
    "setpoint reference", "modbus configuration", "zzqqxx wwvvyy", "!!! ???", "",
    "LeadLagCycles leadLag", "Ünïcödé ÉCOLE", "naïve café résumé", "Σίσυφος ΟΔΥΣΣΕΥΣ",
    "İstanbul DİYARBAKIR", "日本語テキスト 中文", "١٢٣ ٤٥ Ⅻ ½", "temperatura presión bomba",
    "contraseña clave usuarios", "guardan montar", "red green", "BIAlarmSource doRouteAlarm",
    "BComponent.getSlot(\"out\")", "e", "ee", "eee", "eeee", "free tree", "notes stores",
    "Ack ack ACK", "s ss sss ssss", "valves valve's", "x́y", "ＡｌａｒｍＡｃｋ",
    "a b c", "tab\tsep\nnew", "ﬃ ﬄ", "dé́jà", "abcdefgh ing", "data_points",
]


def record(text):
    words = ft._WORD.findall(ft.fold(text))
    return {
        "text": text,
        "fold": ft.fold(text),
        "terms": ft.terms(text),
        "word_terms": {w: list(ft.word_terms(w)) for w in words},
        "stem": {w.lower(): ft.stem(w.lower()) for w in words},
        "parse": [list(g) for g in ft.parse_query(text)],
        "parse_relax": [list(g) for g in ft.parse_query(text, relax=True)],
    }


def main():
    out = [record(t) for t in INPUTS]
    with open(os.path.join(HERE, "terms.json"), "w", encoding="utf-8") as f:
        f.write('{"stop_words": ' + json.dumps(sorted(ft.search_lang.STOP_WORDS)) +
                ',\n"cases": [\n' +
                ",\n".join(json.dumps(r, ensure_ascii=True) for r in out) + "\n]}\n")


if __name__ == "__main__":
    main()
