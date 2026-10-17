# Text variants: folds, BM25 and glossaries

The Python tools carry four accent folds, three BM25 scorers and three glossary copies. None
of them is byte-identical to another, so the Go packages expose each one explicitly instead of
unifying them. Every row below is pinned by a parity fixture generated from the real Python
function (`python3 -I internal/<pkg>/testdata/gen.py`).

## Folds (`internal/textfold`)

| Python function | Case | Non-ASCII after NFKD | Degree sign | Go | Callers |
|---|---|---|---|---|---|
| `tools/lib/textsearch.py` `fold` | `str.lower()` first | dropped (`encode("ascii", "ignore")`) | kept as is (then dropped) | `textfold.Corpus` | corpus-nav (search, browse, findindex, semantic), searchdoor (door, course), glossary.py |
| module-navigator `class_rank.fold` | kept | dropped | dropped | `textfold.ClassRank` | module-navigator `rank` |
| niagara-help `fulltext_terms.fold` | kept | kept, combining marks dropped | `°` -> `" deg"` before NFKD (`℃` still gives `°C`) | `textfold.HelpFulltext` | niagara-help full text (index, scan, query) |
| module-navigator `corpus.strip_accents` | kept | kept, combining marks dropped | kept | `textfold.StripAccents` | module-navigator string_constants |

All four decompose with NFKD and drop canonical-combining-class != 0 characters; the ASCII
fast path of each is identical to its general path. Consequences worth knowing:
`textsearch.fold("㎒")` is `"MHz"` (lowering happens before the decomposition that creates the
capitals), `fold("ß")` is `""` in the ASCII folds and `"ß"` in the others, `ﬁ` becomes `fi`.

Go's standard library has no NFKD and ships an older Unicode than the Python the tools run on
(3.15, Unicode 17.0.0), so `internal/textfold/unicode.txt` is generated from that Python: the
decomposition without combining marks, the one-character `str.lower()`, `str.isspace()` and the
cased / case-ignorable sets that decide final sigma. The fixture proves the table with SHA-256
digests over every code point, not only samples. `golang.org/x/text` is not used: it is at
Unicode 15 and would still need the Python lower-case and whitespace rules.

Also in `textfold`: `Lower` (= `str.lower()`, final sigma included), `Fields` (= `str.split()`,
which also splits on `\x1c`-`\x1f`), `ParseQuery`/`Score` (the textsearch substring layer) and the
textsearch ranking terms `Terms`, `QueryTerms`, `Stem`, `Normalize`, `Tokens`, `IsStopword`.

## BM25 (`internal/bm25`)

| | textsearch (`BM25`, `bm25_rank`) | module-navigator `class_rank` | niagara-help `fulltext_scan` (= FTS5 `bm25()`) |
|---|---|---|---|
| Go | `bm25.Textsearch`, `Rank`, `Index` | `bm25.ClassRank` with `Rank` | `bm25.FTS5` with `Weight`/`Norm`/`Term` |
| k1, b | 1.5, 0.5 | 1.5, 0.3 | 1.2, 0.75 |
| idf | ln(1 + (N - n + 0.5) / (n + 0.5)) | same | ln((N - n + 0.5) / (n + 0.5)), not positive -> 1e-6 |
| term | w * tf * (k1 + 1) / (tf + k1 * (1 - b + b * dl / avgdl)) | same | same |
| tf | body count + 8 x title count | 8 when the term is one of the class name's terms, plus body-field counts | 3.0 x title count + body count (`fulltext.TITLE_WEIGHT`) |
| dl | sum of the weighted tf | 8 x camel parts of the name + parts of the body fields | title + body token count, unweighted |
| avgdl | mean dl | mean dl | all tokens / all rows |
| sum of terms | Python `sum()`: Neumaier compensated | sequential `+=` | sequential `+=`; an alternative in two groups counts twice |
| kept | score > 0 (`require_all` optional) | every scored class | every row matching the groups (or / and) |
| order | score desc, insertion order | caller: named classes first, score x prior desc, name, module | caller: score x coverage² x source factor, source, path |
| logs | `math.log` | `math.log`, prior 1 + 0.3 x `math.log1p(importers)` | `math.log` |

CPython's `math.log` and `math.log1p` are glibc's `__log_fma` and `__log1p_fma`. Go's `math.Log`
differs from them by one ulp on 1.4 % of idf inputs and `math.Log1p` on 66 of the integers up to
2,000,000, so `bm25.Log` and `bm25.Log1p` port glibc 2.39's machine code (table in `logtab.go`,
read from `libm.so.6` by `testdata/gen_logtab.py`). The fixtures pin 2,987,803 results.

Tokenization is part of each tool, not of `bm25`: class_rank's camel-case `identifier_terms`
and niagara-help's `word_terms`/`stem`/`parse_query` (prefix terms, phrases) are ported with
those tools; only the textsearch analyzer lives in `textfold`.

## Glossaries (`internal/glossary`)

| Python | Data | Lookup | Effect on the query | Go |
|---|---|---|---|---|
| `tools/lib/glossary.py` | `glossary_es_en.json` (178 keys) | `textsearch.fold` the word, then the word, minus `s`, minus `es` | adds the English terms: `expand`, `weighted_terms` at idf factor 0.5, `note` | `glossary.Default()`, `Lookup`, `Translations`, `Expand`, `WeightedTerms`, `Note` |
| module-navigator `class_rank.GLOSSARY` | the same 178 entries inline | the already stemmed query term against keys stemmed the same way | replaces the term with the raw English terms | `glossary.ClassRankByStem()` |
| niagara-help `search_lang.GLOSSARY` | the same minus `guardan`, `montar` (`EXCLUDED`) | the lower-cased `fulltext_terms.fold` word, then minus `s`, minus `es` | replaces the word with its English alternatives in one group | `glossary.Help()`, `LookupKey` |

The Go package embeds one copy of the JSON; a test fails when it drifts from
`tools/lib/glossary_es_en.json`.
