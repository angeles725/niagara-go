# Real-corpus golden re-record, 2026-10-23

The real nh index was rebuilt on 4.15/4.14/4.13/4.3 with niagara-go main (NH1 honesty texts,
NH2 module-jar source and lexicon label rows, NH3 module-index.json, #192, #194). 15 cases
failed; each was re-captured from the Go binary (`docs/goldens.md`, "Real-corpus goldens":
repo cwd, `COLUMNS=100`, `<TMP>`/`<REPO>`/`<T>` normalization). No case was SUSPECT: no exit
code or stderr changed and no result set lost a member that should still match.

DRIFT = the corpus or index changed. INTENDED = a merged behaviour change. The full-text index
is one FTS table over every source, so new rows (lexicon-text 75,632, source 3,451, course-text
755, package-text 62) move BM25 document frequencies and average length, which reorders ties
and picks another best page for a snippet.

| Case | Class | Reason |
|---|---|---|
| devguide-search | DRIFT | Same hit count; the shown top results reorder (Niagara RPC up, Hx in, User Activity Monitoring out) from the BM25 shift of the new rows. |
| find | DRIFT | 116 -> 123 files (source 9 -> 11, course-text 2 -> 4, new lexicon-text 3 from NH2 label rows b6b042d3); the new hits and BM25 shift reorder the top 10. |
| find-miss | DRIFT | Searched-source counts: course-text 20 -> 26, source 2,610 -> 3,459 (module-jar source), package-text absent -> 62, lexicon-text absent -> 783. |
| freshness | INTENDED + DRIFT | New text-source/index block (9f97bcfe) and module-index.json install check (8f92c9f5); source 2,610 -> 3,459 files; new baseline stamp time. |
| freshness-all | DRIFT | intended.json stdout: "retired index left: call-index.json" gone on 4.13/4.15/4.3 because the rebuild removed the file (988f1e60 names it only while on disk). |
| guide-search | DRIFT | Same results; Bacnet-BacnetNetworkLayer moves up one place from the BM25 shift. |
| guide-search-or | DRIFT | Third result ForwardingWiretap -> New BACnet IP network BBMD configuration from the BM25 shift (guides-clean unchanged at 9,053 files). |
| method | INTENDED | `AbstractCursor&lt;E&gt;` -> `AbstractCursor<E>`: bajadoc-clean keeps generic names unescaped (47a46c12). |
| method-section | DRIFT | Same 18 classes; BReliabilityAlarmSourceExt and BObixAlarmImport swap, ordered by full-text prominence, which the new rows shift. |
| package | INTENDED | DESCRIPTION block from package-text/javax.baja.alarm.txt (53477af9, package descriptions). |
| pdf | INTENDED + DRIFT | Every page count -1 (a20216b6, no extra page for the trailing form feed, e.g. docModbus 101 -> 100); snippet lines move to another best page (BM25 shift). |
| pdf-names | INTENDED | TR100 Modbus Integration Guide 41 -> 40 pages (a20216b6). |
| selftest-recall | DRIFT | source and xref recall rows 2,605 -> 3,453 classes, still 0 missing (NH2 module-jar source, 848 classes). |
| stats | INTENDED + DRIFT | intended.json stdout: source-index 2,605 -> 3,453 classes (848 from module jars, 42cfb837), xref 3,001 -> 3,813, slots 1,890 -> 1,937, text sources 94.5 -> 105.4 MB, lexicon-text 569 -> 783 files. |
| stats-json | INTENDED + DRIFT | intended.json stdout: text_dirs gains course-text, devguide-rc, devguide-examples, package-text and lexicon-text (7dd280a5), `module_jar_classes` 848, the same counts as stats, bajadoc-clean 5,630,114 -> 5,630,037 bytes (47a46c12). |
