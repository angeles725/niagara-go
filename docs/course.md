# course: the N4 developer-course answer keys

`course` looks up the written answers of the N4 Developer Certification course by item ID,
counts them by verification status, checks a station against the labs (`course check`), and
decides the predicted answers (`course verify-predictions`), or the answers of any status
(`course verify --status`), from the stations; `course progress` shows how far the labs, review
assignments and exam are and what to do next, and `course quiz` draws study questions.
It reads three Markdown files of a niagara-research checkout and one or two course text
files. It is new (no Python original), so its golden cases run over the synthetic tree
`internal/course/testdata/root/`.

## Packages

| Package | Holds | Prints |
|---|---|---|
| `internal/course/key` | the answer-key parser and model: `Load(root)`, `Corpus`, `Item`, `Group`, the status vocabulary, `GroupKeys`, `Counts` (one row of `status`) | nothing (reusable, e.g. by nsearch) |
| `internal/course/check` | the check spec (`Spec`), the kinds and their evidence (live, offline), the station and jar reader (`Open`, `Station.Run`), the verdicts of `verify` and the suggestion lines | nothing |
| `internal/course` | the CLI: argument parsing, root discovery, the tables, JSON and summaries | everything |
| `internal/niagaraannot` | the Slot-o-Matic annotation parser the `annotation` kind reads (owner Integrante3; not course-specific) | nothing |

The fixtures of all three stay under `internal/course/testdata/`.

## Sources

| File (relative to the root) | Items |
|---|---|
| `docs/n4-dev-course-solutions/labs.md` | lab items `L<lab>.<n>`, `LB.<n>` (Bonus lab), `LA.<n>` (Appendix A) |
| `docs/n4-dev-course-solutions/review-assignments.md` | requirements, design decisions, traps, needs-a-run checks |
| `docs/n4-dev-course-solutions/certification-exam.md` | exam problems `P<n>` |
| `sources/n4-dev-course/text/Lab Instructions.txt` | read only by `course text` (lab items, labs) |
| `sources/n4-dev-course/text/Review Assignment <k>.txt` | read only by `course text` (requirements) |

All three Markdown files are required: a missing one is exit 2, never an empty answer.
The text files are needed only by `course text`.

## Root discovery

1. `--root DIR`.
2. `NIAGARA_RESEARCH_DIR`, trimmed (an empty or blank value counts as unset).
3. The first directory at or above the working directory that holds
   `docs/n4-dev-course-solutions/labs.md`.
4. The same walk from the directory of the executable (symlinks resolved).

`--root` or the variable naming a directory without `labs.md` is exit 2 (no fallback to the
walk). Nothing found is exit 2 with one line naming the three options.

## Groups

| Group | Heading | `--lab` / `text` spellings (any case) |
|---|---|---|
| `L1`..`L11` | `## Lab <n> — <title> (`LI:a-b`)` | `3`, `L3` (leading zeros ignored: `03`, `L03`) |
| `LB` | `## Bonus lab — <title> (`LI:a-b`)` | `B`, `LB` |
| `LA` | `## Appendix A — <title> (`LI:a-b`)` | `A`, `LA` |
| `RA1`, `RA2` | `## <n>. Review Assignment <k> — <title>` | `RA1`, `RA2` |
| `RAX` | `## <n>. Needs a run ...` | `RAX` |
| `EXAM` | the whole `certification-exam.md` | `EXAM` |

`--lab RA` selects RA1, RA2 and RAX together. Groups are discovered from the headings, never
from a fixed list. The `LI:a-b` range of a lab heading is the lab's text range.

## labs.md items

An item is a bullet at column 0 inside a lab section:

    - **<ID>** · <step> · <answer>

- ID grammar: `L<lab>.<n>`, `LB.<n>`, `LA.<n>` (`<lab>`, `<n>` decimal), or a range of them
  in one group, `L2.2–L2.4` (en dash or hyphen): one item that `L2.3` also finds. The prefix
  must name the enclosing group (`L2.5` under Lab 1 is an error).
- The item continues over the following lines indented by at least two spaces and over blank
  lines followed by such a line; it ends at the next line at column 0. Continuation lines lose
  two spaces of indent, so code blocks are kept as written.
- The head is the first paragraph (up to a blank line or a code fence), joined with single
  spaces. The step and the answer are split at the **last** ` · ` of the head that is outside
  backticks (one item, L3.3, has `step · question · answer`).
- A head without a separator is either a question and its answer (`*Why ...?* The registry
  ...`): the leading `*...?*` question is the step and the rest is the answer; or a step with no answer
  (`` `gradlew jar` (`LI:474`). ``), reported as `(step only)`.
- The answer is the rest of the head plus every later line of the item.
- `LI:a-b` (or `LI:a`) means lines `a..b` of `Lab Instructions.txt`. The item's ranges are the
  `LI:` references of the step; when the step has none, those of the whole head.
- A bold line at column 0 (`**Customising the Device Manager (automanager)**`) inside a lab is
  a subsection; `**Goal.**` and `**Pitfalls.**` paragraphs are not items.
- `## Answers that need a live run` lists IDs and ranges (`L9.9–L9.13`); each one marks its
  item `live run: needed`. An ID there that is not an item is an error.

## review-assignments.md items

Tables are recognised by their header row; cells are split on `|` except `\|`.

| Header | Kind | ID column | Step | Answer |
|---|---|---|---|---|
| `\| ID \| Requirement \| Source line \|` | requirement | `RA<k>.<X><n>[a-z]`, a range `RA1.V9–V14`, or `RA<k>.S` | requirement | its traceability row |
| `\| # \| Decision \| Why ... \|` | decision | `D<k>.<n>` | decision | why |
| `\| Trap \| What goes wrong \| This solution \|` | trap | first word of the cell, `T<k>.<n>` | rest of the cell | what goes wrong + this solution |
| `\| ID \| What is unconfirmed \| Cheapest check \|` | check | `X<n>` | what is unconfirmed | cheapest check |
| `\| Requirement \| Satisfied by \| Verified by \|` | traceability, joined onto requirements | a requirement ID or range | | |

- Requirements read their text file from the requirements heading
  (``(from `sources/n4-dev-course/text/Review Assignment 1.txt`)``); the `Source line` cell
  (`:2-3`, `:2, :5`) gives the ranges.
- A range ID (`RA1.V9–V14`, en dash or hyphen) is one item; `RA1.V12` finds it. A traceability
  range (`RA1.M1–M6`) applies to every requirement it covers. A traceability row whose ID is
  not a requirement is an error; a parenthesised label cell such as `(robustness)` is skipped, any
  other first cell that is not an `RA` ID is an error.
- A data row with fewer than three cells in a recognised table is an error.
- Headings and tables inside code fences are ignored.

## certification-exam.md items

`## <n>. Problem <m> — <title>` is item `P<m>`: the step is `Problem <m> — <title>`, the
answer is the section body up to the next `## ` heading. Other `## ` sections are not items.

## Status vocabulary

An item carries every marker found in its own text; markers overlap (L1.9 is identical and
unverified). `written` means none of the others.

| Status | Marker |
|---|---|
| `predicted` | `*(predicted)*` |
| `unverified` | `*(unverified — not compiled)*`; a heading containing `unverified`; the `Needs a run` section |
| `corrected` | `**Corrected**` (any case) |
| `identical` | `**Identical**` (any case) |
| `verified` | `*(verified live <YYYY-MM-DD>)*` (confirmed from what a station stores) or `*(verified offline <YYYY-MM-DD>)*` (confirmed from a jar, the source tree or the lab sources); a file-level `**Status: verified ...**` line (certification-exam.md; `verified live.` is live evidence, `verified offline` offline, a bare `verified` states none) |
| `written` | none of the above |

Both verified markers are the status `verified`. `course status` splits it in two more columns,
`live` and `offline` (`verified_live`, `verified_offline` in JSON): the items carrying a marker of
that evidence (an item with both counts in both; a verified status of unstated evidence, in
neither). `show --json` lists an item's evidence in `verified`; the text of `show` keeps the plain
status line, the one nsearch renders too (`TestShowLinesMatchCourseShow`).

A file-level note such as `Status of every code listing: unverified` describes code listings,
not items, and is not applied to the items.

## Errors (exit 2)

Duplicate IDs (within or across files), malformed IDs in a lab section (`- **L1.x** · ...`),
an ID under the wrong group, unknown IDs in the live-run list or a traceability table, a
reversed or cross-group live-run range (`L2.4–L2.2`), a table row with fewer than three cells
and a traceability row that names no requirement are parse errors. Every error is printed as `course: <file>:<line>: <reason>` and the command
exits 2: an answer key that cannot be read without ambiguity is not searched.

## Commands

- `course show ID [ID ...] [--json]`: kind, ID, section, step, text ranges, status, live run,
  `file:line`, full answer. IDs are case-insensitive. An unknown ID prints
  `no item <ID> (searched N items in <files>)` to stderr; if any ID is unknown the exit is 1,
  even when others were printed.
- `course status [--lab G] [--status S] [--json]`: items per group and status (columns
  overlap; `live` and `offline` split `verified`), then with `--status` the matching IDs and
  steps (truncated to 80 characters).
  No match is exit 1 with the scope and counts.
- `course find REGEXP [-i]`: every item whose step or one of its answer lines matches the Go
  regexp (RE2 syntax, not Python's `re`; `-i` folds case), in answer-key order, one row each:
  group, ID, statuses and the first matching line (the step first, then the answer lines,
  trimmed, cut at 120 characters), the columns aligned. No match is exit 1 with
  `course: no item matches <q> (searched N items in labs.md, review-assignments.md,
  certification-exam.md)` on stderr, the scope `show` prints; a regexp that does not compile
  is exit 2.
- `course text ID|GROUP`: the text-file lines of the item's (or the lab's) ranges, numbered,
  with no trailing whitespace (a blank line prints only its number).
  No range is exit 1; a missing text file or a range past its end is exit 2. Requirements
  print their `Review Assignment <k>.txt` lines.

## course check

`course check [--station <dir|config.bog>] [--install DIR] [--source DIR] [--labsrc DIR] [--solutions DIR] [--lab N ...] [--json]`
verifies what a correct solution of each lab leaves in a station and in the install, and prints one
verdict per check. It reads the station, the jars and the source tree only; it needs no
niagara-research root.

- `--station`: the station directory (its `config.bog`, zipped or plain XML, and `shared/`), or
  the bog file itself (its directory is the station directory). Every selected check runs on it.
- `--install`: a Niagara install; the jar checks read `<install>/modules/<part>.jar`, the install
  rows (`Root: install`) its files (`defaults/system.properties`). Without it (and without
  `COURSE_NIAGARA_INSTALL`) every jar check and install row is SKIP.
- `--source`: a Niagara source tree, one directory per module part (`baja/javax/baja/sys/...`):
  the 4.15.6.20 sources a niagara-research checkout holds in
  `niagara-help/versions/4.15.6.20/source`. The `source-text` checks read it; without it (and
  without `COURSE_NIAGARA_SOURCE`) they are SKIP, and the summary says `source (none: source
  checks skipped)`. A tree that is missing or not a directory is a configuration error: each
  `source-text` check is SKIP with `cannot read <dir>: ...` (a missing file inside the tree is
  FAIL). An explicit `--source` wins over the variable, even an empty one (`--source ""`: no
  tree, SKIP). The summary names the tree only when a selected check reads it.
- `--labsrc`: the participant's lab module sources: the Niagara dev home that holds `training/`
  and `trafficLightDriver/` as the New Module Wizard lays them out
  (`training/training-rt/src/com/tridiumuniversity/training/...`). The `annotation` checks and
  the lab-source rows (`Root: labsrc`) read it, with the rules of `--source` (no tree or an
  unreadable one: SKIP; a missing file: FAIL; the flag wins over the variable, even empty; the
  summary says `labsrc (none: lab-source checks skipped)`).
- `--solutions`: the participant's own module sources as the niagara-research corpus keeps them,
  `sources/n4-dev-course/solutions` (`review/`, `cert-exam/`). The review rows with `Root:
  solutions` read it (annotation rows over `review/Cristian_Angeles_Review/...`), with the rules of
  `--labsrc` (the summary says `solutions (none: solutions checks skipped)`); the lab sources are
  never a fallback.
- `--lab`: repeatable; `3`, `03`, `L3`, `B`, `LB`, `RA1`, `RA2`, `EXAM` (any case). A lab without
  checks (8) is exit 1. Without `--lab` only the labs run (1-11, B: the TrainingStation); `RA1`,
  `RA2` and `EXAM` run only when named, on their own station.

Defaults (explicit flags win; a blank variable counts as unset):

| Flag | Default |
|---|---|
| `--install` | `COURSE_NIAGARA_INSTALL` |
| `--source` | `COURSE_NIAGARA_SOURCE` |
| `--labsrc` | `COURSE_LAB_SOURCES` |
| `--solutions` | `COURSE_SOLUTIONS` |
| `--station` | per check, the variable of its station role: `COURSE_TRAINING_STATION` for the labs (1-11, B), `COURSE_REVIEW_STATION` for RA1 and RA2, `COURSE_CERT_STATION` for EXAM |

Without `--station`, a run whose checks span several roles reads each role's station once, and
the summary names every station read. A role whose variable is unset is the usage error of a
missing `--station` naming the groups of that role and its variable (`the following arguments are
required: --station (EXAM needs --station or COURSE_CERT_STATION)`; `the labs` for more than three
labs), exit 2. The same defaults apply to `verify-predictions` and `verify`, which check first that
some station is given (`--station` or one of the three variables): without any, the plain usage
error comes before the root and `--date` errors.

The spec is the table `Spec` in `internal/course/check/spec.go`: one row per check with an
ID (`C<lab>.<n>`), the lab, the answer-key items it confirms (`L4.13`, ...), a kind and a
one-line description. Expected defaults are cited next to each lab's rows (lab `.java`
file:line, or `organized/` for a framework type). Component types are written
`module:Type` with the module name (`training:Tstat`, `control:NumericWritable`); the bog's
own prefixes (`t:`, `tld:`) are resolved through its `m=` declarations.

| Kind | Checks | PASS when |
|---|---|---|
| `type` | components of a type, optionally under an ancestor type (`Within`) or below a folder path (`Under`, `Review2`) | the count is at least / exactly N; with `Under`, a FAIL lists where the components are instead |
| `slot` | a slot on every component of a type (filtered by `Within` and `Under` as for `type`); its effective value is the stored value (a composite's nested `value`), else the type default. A dotted slot (`address.port`, `health.lastOkTime`) is a value below a composite slot of the component | every value equals `Want`, and every `Stored` slot is stored (`health.down = false` is also the default of a device never pinged, so C11.12 and C11.14 need `health.lastOkTime` stored); or (`Set`) every slot is stored; or (`Distinct`) the distinct values reach N |
| `link` | links `From.FromSlot -> Type.Slot`, source by handle or `slot:` ord | the count is at least / exactly N |
| `files` | files matching a glob under the row's root (below), `{station}` = the `stationName` slot (station root); `{a,b}` is one of the exact names a or b, a name ending in `/` a folder (printed with the `/`) | the count is at least / exactly N |
| `jar` | `modules/<part>.jar` | it exists |
| `jar-entry` | entries matching a glob (`**` crosses `/`; `{A,B}` is one of the exact names A or B) | the count reaches N |
| `jar-text` | the first entry matching a glob | it holds every listed string |
| `jar-signed` | `META-INF/*.SF` and `META-INF/*.RSA`, `.DSA` or `.EC` | both exist |
| `jar-cert` | the signer certificate of the jar's first signature block (`META-INF/*.RSA`, `.DSA`, `.EC`: a PKCS#7 SignedData, below); `Fact` `subject`, `issuer` (the DN, RFC 2253 order, CN first), `chain`, `timestamp`, `same` (`CertOf`: another jar part), `pem` (`Glob` under `Root`) | `subject`/`issuer`: the DN matches `Match`; `chain`: the signer's path inside the block (each next certificate is the issuer by name and signed the one before, up to a self-signed root) has at least / exactly N certificates; `timestamp`: the signer carries an RFC 3161 token (`id-aa-timeStampToken`) whose signer's subject matches `Match`; `same`: both jars' signer certificates are the same DER; `pem`: the signer is the first `CERTIFICATE` of the PEM file. No jar or no block is FAIL; a block that cannot be read is SKIP |
| `file-text` | every file matching a glob under the row's root (globs as for `files`), read as text: a UTF-8 BOM and CRLF line ends dropped; `Match` is a Go regexp (`{station}` = the station name, quoted, station root only) | `Line` > 0: that line matches; `Line` 0: the matching lines are at least / exactly N; `Whole`: the matches over the whole text (lines joined with `\n`, so `\s*` crosses them; `(?m)` for `^`/`$` per line) are at least / exactly N, the evidence gives the first one's line and text; with `Names`, the first match's group 1 is a comma-separated `NAME=value` list (a DN) whose names, sorted, equal `Names` (each once, none other). No matching file is FAIL; a file over 16 MiB is SKIP (`cannot read`), never checked on its first part |
| `audit` | the records of every history file matching a glob under the station dir (`history/*/*/AuditHistory.hdb`), decoded by `internal/hdbread`; `Hist` holds Go regexps on `operation`, `target`, `slotName`, `oldValue`, `value` (empty = any) | the matching records, summed over the files, are at least / exactly N |
| `log` | as `audit`, over `history:LogRecord` files; regexps on `logName`, `level` (`INFO`, `SEVERE`, ...), `message` | as `audit` |
| `trend` | every trend history matching the glob (`history/*/*/NumericWritable*.hdb`); a boolean value counts 0/1, an enum its ordinal; any other value type is SKIP | each file has at least / exactly N records (`Distinct`: N distinct values, NaN not counted), and every value lies in `Hist.Min..Max` (NaN never does; the evidence's min..max leaves it out) |
| `annotation` | one Java file (`Glob`) of the `--labsrc` tree, or of the `--solutions` tree with `Root: solutions`, read by `internal/niagaraannot`: only the annotations before the top-level type, never the generated region. `Fact` renders `flags` (`out=rs in3=-`: each slot's Slot Sheet letters, `Flags.encodeToString` order), `enum` (`DIRECT=0 REVERSE=1 DEFAULT=direct`), `slots` (`property timeExecuted: BAbsTime, action submit(BWorkOrder = new BWorkOrder())`), `facets`, `defaults` (`summaryFile: BOrd = BOrd.DEFAULT`; no or an empty `defaultValue` is `""`) or `extends` (the superclass of the class declaration, as written: `BAbstractService`, `BList<BObject>`; the one fact read past the annotations, from `class <Name>` to the body's brace with comments and string literals blanked, so a brace or an `extends` inside an annotation or a comment is not read); `Slot` narrows it to one slot | the fact equals `Want`. No tree, an unreadable one, a file the parser refuses or a flags expression it cannot evaluate is SKIP; a missing file or slot FAIL |
| `listing` | one file under the station dir (`Glob`) that lists the components of `Type` (`Within`, `Under` as for `type`), one line each; `Match` is a Go regexp with a `(?P<name>...)` group, and with `Slot` also `(?P<value>...)` and `(?P<priority>...)` | the matching lines name exactly the components (none missing, none extra, none twice; two selected components sharing a short name are SKIP `ambiguous: <paths>`, since a line names one by its short name only); with `Slot`, every line is at priority `Want` and its value (a decimal comma is a point) equals the component's effective `Slot` (`Default` when not stored; a dotted slot needs nested values). Not one matching file is FAIL |
| `source-text` | one file of the `--source` tree (`Glob` is its path, `/`-separated); `Match` is a Go regexp over the whole text (`(?s)` and `\s*` cross lines) | the matches are at least / exactly N; the evidence gives the first one's line. No `--source`, or a tree that cannot be read, is SKIP; a missing file FAIL |

File roots. A `files` or `file-text` row reads the station directory unless it names a root
(`Root`, `check/roots.go`): `install` (`--install`) or `labsrc` (`--labsrc`, the dev home; `..`
reaches the `certificates/` folder beside it, where the course's `gradlew createProfile
--profile-path ..\certificates\code_signing.xml` run from `niagara_dev_home` writes the signing
profile, `LI:887-898`). A root the run was not given, or one that is not a readable directory, is
SKIP. Install and lab-source rows are offline evidence (no station holds those files). A row's
own `{a,b}` groups are expanded before `{station}` is replaced, so a station name holding braces,
commas or glob characters matches only itself; `{station}` is valid only with the station root. A
row whose fields contradict each other (a `Root` on a kind that does not read it, `Whole` with
`Line`, `Names` without a `Whole` file-text row, `{station}` outside the station root) is SKIP
`bad row: ...` (`Check.validate`; `TestRowValidation` holds every spec row valid). The rows
(2026-10-18):

| Row | Item | Root | Reads |
|---|---|---|---|
| C1.9-C1.11 | L1.4 (PARTIAL: the dev home is read after the later labs, not as the wizard left it) | labsrc | the root Gradle and wrapper files; `training-rt/` and `training-wb/`: `src/`, `srcTest/`, the part's `.gradle.kts`, `module.lexicon`, `module-permissions.xml`, (rt) `module.palette` |
| C1.12 | L1.14 | labsrc | `training/training-rt/module-include.xml` holds the `<type>` of `BFirstComponent` (either attribute order: the 4.15 build writes `class` first) |
| C5.4, C5.5 | L5.2 | install | `defaults/system.properties`: one uncommented `niagara.moduleVerificationMode`, and it is `high` |
| C5.6 | L5.8, L5.9 | labsrc | `../certificates/code_signing.jceks` and `code_signing.xml` |
| C5.7, C5.8 | L5.10 (PARTIAL: that C, ST, L, O are your own values) | labsrc | the `niagara.signing.dname` entry: C, ST, L, O and CN once each (no OU, no duplicate: `Names`), and `CN=${alias}` |
| C5.9, C5.10 | L5.11 (PARTIAL: verification after the certificate expires is runtime) | labsrc | the `standardtsa` entry `http://timestamp.digicert.com` (its value on its own line, as `LI:927-929` writes it) and `profileType` `RestrictedSigningProfile` |

The profile holds the keystore passwords in plain text (L5.12): the rows match only the entries
named above, and their evidence prints only the matched text.

Signature blocks. Go has no PKCS#7, so `check/pkcs7.go` walks the SignedData with `encoding/asn1`
after `berToDER` rewrites BER indefinite lengths as definite ones (the 4.15 build's blocks open their
ContentInfo, SignedData and certificate set with indefinite lengths; a primitive's contents, and so
every DER certificate, are copied as they are). It keeps the certificates, the certificate the first
SignerInfo names (issuer and serial number, or subject key identifier) and the timestamp token's
SignedData; it verifies only the certificate chain (`CheckSignatureFrom`), never the jar signature.
A malformed block is an error, never a panic (`FuzzParseSignedData`). The rows (2026-10-18):

| Row | Item | Reads (`training-rt`, live) |
|---|---|---|
| C5.11 | L5.18 (PARTIAL: the Owner of the self-signed certificate, and the trust-store validation, are not in the jar) | issuer `CN=Tridium University Root Certification Authority,O=Tridium University,L=Richmond,ST=Virginia,C=US` |
| C5.12 | L5.18 | the chain `CN=(CodeSigning),O=VeytrixControls,...` -> the root (self-signed): 2 of 2 certificates |
| C5.13 | L5.11 (PARTIAL, with C5.9 and C5.10) | timestamped by `CN=DigiCert SHA256 RSA4096 Timestamp Responder 2026 1,O=DigiCert\, Inc.,C=US` |
| C5.14 | L5.17, L5.25 (PARTIAL: the Module Info view) | the signer (serial `45e7056ff7ac1091bcfa953b`) is the first certificate of `../certificates/code_signing.pem`, the file L5.17 imports into the keystore |
| CRA2.12 | RA2.R8 | `Cristian_Angeles_Review-rt`'s signer is `training-rt`'s (the same DER): the Code Signing lab's certificate |

Not decided by the jar: L5.14 (the keystore listing of the self-signed certificate: the jar holds only
the CA-issued one; its alias is in `code_signing.jceks`, which no row reads). The certificate's CN is
`(CodeSigning)`, with the parentheses, though the dname is `CN=${alias}` and the alias `CodeSigning`
(L5.10): no row asserts the CN.

Nested slots: the bog reader keeps only a composite slot's direct `value` and `status`, so a
dotted slot is read from `Comp.Nested`, which `bog.LoadOptions` fills only with
`bog.Options{NestedSlots: true}`. `course check` turns the option on only when a selected check
has a dotted slot (`--lab 4` reads the bog as before); a dotted check run on a bog read without
it is SKIP, never PASS on the type default. The path is from the component that owns the
composite slot: the Tcp Config of a driver network is its own component (`tcpConfig`, with a
handle), so its port is `address.port` on `trafficLightDriver:TrafficLightDriverTcpCommConfig`.

Checks per lab: 1: 12 (C1.9-C1.12 read the dev home), 2: 5, 3: 5, 4: 10 (C4.8 an `audit`, C4.9 and C4.10 `annotation`), 5: 14 (the station
holds nothing of code signing: the jars and their signer certificate, the install's `system.properties`
and the signing profile), 6: 8, 7: 8 (C7.6-C7.8 `source-text`), 9: 7, 10: 14 (three of them `file-text` on the
exported CSVs, three on the history files, C10.14 the job's lexicon), 11: 17 (8 of them nested: C11.9-C11.16, the driver and server
Tcp Config addresses and the ping health of the network and of every driver device; C11.17
`serverEnabled` is a plain slot), B: 5. C1.8, C2.5, C4.9, C4.10, C6.7, C6.8 and CB.5 are `annotation` rows. Lab 8 (ORDs) writes no code and changes nothing, so it
has none (its station steps, such as L8.15 and L8.17, happen in the Workbench UI). Appendix A
(`LA`) has none either.

History files. A history kind reads what the station recorded while it ran, which the bog does
not keep. A history file that cannot be decoded, or holds another record type than the kind reads,
is SKIP; no matching file is FAIL. The rows (TrainingStation, 2026-10-17):

| Row | Item | Reads |
|---|---|---|
| C4.8 | L4.8 (PARTIAL: the glyph) | an audited `set` on `SpaceTemp` whose old value shows Out with the unit: `24,0 °C {ok} (Out = 24,0 °C {ok} @ def)` |
| C10.11 | L10.19 (PARTIAL: the one Ramp) | nine `Added /Lab10 NumericWritable1..9` audit records, the paste |
| C10.12 | L10.18, L10.19 | every `NumericWritable*` trend history holds two or more values, all in the Ramp's 0..100 |
| C10.13 | L10.21 | ten `NumericWritable*` history files, named after their points (`files`) |

What the histories do not decide (2026-10-17): the LogHistory keeps its last 500 records only (on
the TrainingStation 2026-10-16 21:13:54 to 21:15:09, the minute before the Quit), so no log line of a
lab step is left; a ping is not audited and the batch-job step histories are empty. So L1.25 (the
console line; the audit has only `Invoked /Lab1/FirstComponent execute`), L8.17 (a view's
`?period=today` query), L11.26 and L11.32 (the ping), L11.41 (the subscription and `readOk`),
L11.42 (the order of Write and Read Value) and L11.43 (the audit shows `intersectionId` A -> Z -> A
and `lightId` N <-> S, but the WARNING it predicts was logged long before the log's window, and S is a
known light) stay as they were. The audit also shows why L10.19's one Ramp is not observed: the live
paste added `Ramp1`..`Ramp9` with the copies.

Source tree. labs.md derived its predictions from the 4.14 decompiled code (`labs.md:9-12`); the
`source-text` rows re-derive one from the 4.15.6.20 source:

| Row | Item | Reads (`baja/javax/baja/sys/BAbstractService.java`, 4.15.6.20) |
|---|---|---|
| C7.6 | L7.18 | `configOk()` sets `faultCause` to `""` and calls `updateStatus()` (:280-294) |
| C7.7 | L7.18 | `updateStatus()` sets the fault bit only on a fatal or a config fault (:229-236) |
| C7.8 | L7.18 | three properties, `status`, `faultCause`, `enabled` (:69, :95, :121): no "Last Ok" slot |

L7.18 stays PARTIAL: the `{ok}` status and the empty Fault Cause after the restored submit are
runtime state. C10.14 decides L10.22's texts from the built jar, not the source: the job logs
through lexicon keys (`historiesToCsvJob.beginningHistoriesExport`, ...), and `training-rt.lexicon`
holds `Beginning export of {0} histories`, `Exporting history {0}` and `Completed histories export`
(PARTIAL: the Success state and the log of a run). The 4.15 source does not answer L5.3 (the
signature check is `com.tridium.sys.module`, which the extraction does not hold: only `javax` and a
few `com.tridium` modules), L1.17 (the registry rebuild is `com.tridium.sys.registry`, absent too)
or L9.10 (the executors are lab code, the training module's, not framework code).

Lab sources. The `annotation` rows decide from the participant's own lab module what Slot-o-Matic
generates and what a slot view shows, offline (the shipped `LAB/` files hold no generated region and
another layout, so they are not the input):

| Row | Items | Fact (`Want`) |
|---|---|---|
| C1.8 | L1.9, L1.12 | `BFirstComponent` slots `property timeExecuted: BAbsTime, action execute()` |
| C2.5 | L2.2–L2.4, L2.11 | `BAdd` flags `out=rs in1=s in2=s in3=- in4=-` |
| C4.9 | L4.1, L4.2 | `BLoopAction` enum `DIRECT=0 REVERSE=1 DEFAULT=direct` |
| C4.10 | L4.3 | `BTstat` flags `out=rs cv=- sp=- diff=- action=-` |
| C6.7 | L6.2 | `BWorkOrderSubmissionState` enum `IDLE=0 SUBMITTING=1 DEFAULT=idle` |
| C6.8 | L6.29, L6.32 | `BWorkOrderService` slot `action submit(BWorkOrder = new BWorkOrder())` |
| CB.5 | LB.1, LB.3 | `BTrafficLightDriverDevice` facet `intersectionId=SfUtil.incl(SfUtil.MGR_EDIT)` |

Every link carries an `Unobserved` note (the view, the generated code, the class declaration or the
UI behaviour), so the items become PARTIAL, never CONFIRMED (on the live run, with the participant's
dev home `N4_Labs/niagara_dev_home`: L1.12, L2.11, L4.2 and LB.3 among the predicted items, L1.9,
L2.2–L2.4, L4.1, L4.3, L6.2, L6.29 and LB.1 among the identical ones, L6.32 among the written
ones; the counts are in [Live run](#live-run)). No row for L6.3: its answer
recommends `state` READONLY | TRANSIENT, which the participant's `state` (READONLY) does not
apply, and a recommendation is not a requirement. Not decided by annotations: L2.10 and L2.13 (UI
rules), L6.4 and L10.2 (compiler errors).

Verdicts: PASS; FAIL with what was found instead (`found 9 ..., want exactly 10`,
`0 of 1 pass, want 2.0: Lab4/My Tstat.diff = 0.0 (default)`); SKIP when the check could not run
(no `--install`, an unreadable jar), never PASS. A missing jar is FAIL.

Output: the table `ID  LAB  VERDICT  DESCRIPTION [keys]  EVIDENCE`, then
`checked N: PASS p, FAIL f, SKIP s; station <name> (<components>, <links>), install <dir>`.
With `--json`, stdout is a list of `{id, lab, keys, kind, description, verdict, evidence}` and
the summary goes to stderr.

Exit: 0 no FAIL (the SKIPs are in the summary); 1 at least one FAIL; 2 the station cannot be
read (no such file, not a BOG document) or every selected check was skipped.

### Review-assignment checks (RA1, RA2)

The rows `CRA1.<n>` and `CRA2.<n>` (`internal/course/check/spec_review.go`) read the review
station (`COURSE_REVIEW_STATION`) and the module `Cristian_Angeles_Review-rt` (the participant's
names: the module and type names follow from them). Keys are the requirement IDs of
review-assignments.md; a station step reaches its range item (`RA1.V10` -> `RA1.V9–V14`), and
the needs-a-run checks a station or a jar answers are keys too: X1 (`CRA1.7`, `CRA2.2`, the built
classes), X3 (`CRA1.9`, distinct ids), X6 (`CRA2.9`, `shared/Review.txt`), X10 (`CRA2.4`,
signed).

| Rows | Check |
|---|---|
| CRA1.1-CRA1.4 | module.xml: `moduleName`, `description` (`<First> <Last> Niagara Review Assignments`), `vendor="review"`, `runtimeProfile="rt"` |
| CRA1.5-CRA1.7 | palette entry, type registration, `BComponentWithId.class` |
| CRA1.8-CRA1.10 | two ComponentWithId, distinct ids, in the folder `Review` |
| CRA2.1-CRA2.5 | `control-rt` dependency, `BPointSummaryService.class`, palette entry, signed, built |
| CRA2.6-CRA2.10 | NumericWritables in `Review2`, one PointSummaryService under Services, `summaryFile = file:^Review.txt`, `shared/Review.txt` with `name: value {status} @ priority` lines |
| CRA2.11 | the `Review2` NumericWritables hold at least two distinct values (RA2.V11): the nested `fallback.value` (the value the set action writes; default 0.0), read with nested values |
| CRA2.12 | the module's signer certificate is `training-rt`'s (RA2.R8, the Code Signing lab's certificate: `jar-cert` `same`) |
| CRA2.13-CRA2.15 | the service's superclass, `summaryFile` default and `createSummaryFile` action, from the corpus solutions (below) |
| CRA2.16 | `Review.txt` names exactly the station's NumericWritables, each `@ def` with its fallback (RA2.V15, RA2.R5a, RA2.R5b: `listing`) |

The rows `CRA1.11`-`CRA1.15` and `CRA2.13`-`CRA2.15` read the participant's sources in the corpus
solutions (`--solutions`, `COURSE_SOLUTIONS`; annotation rows, offline) against the traceability of
review-assignments.md:

| Row | Item | Fact (`Want`) |
|---|---|---|
| CRA1.11 | RA1.R1 | `BComponentWithId` extends `BComponent` |
| CRA1.12 | RA1.R2 | defaults `id: String = ""` |
| CRA1.13 | RA1.R3a, RA1.R3b, RA1.R3c (PARTIAL: the Wire Sheet, the Property Sheet and a paste by hand) | flags `id=rsd` (SUMMARY, READONLY, DEFAULT_ON_CLONE) |
| CRA1.14 | RA1.R4 | slot `action regenerateId()` |
| CRA1.15 | RA1.R5 (PARTIAL: the dialog) | flags `regenerateId=c` (CONFIRM_REQUIRED) |
| CRA2.13 | RA2.R2 (PARTIAL: the key's `implements BIRestrictedComponent` and `getServiceTypes()`, which the participant's class does not declare) | `BPointSummaryService` extends `BAbstractService` |
| CRA2.14 | RA2.R3 | defaults `summaryFile: BOrd = BOrd.DEFAULT` |
| CRA2.15 | RA2.R4 (PARTIAL: the key's `flags = Flags.ASYNC`; the participant's action declares none, and RA2.R6 leaves the technique free) | slot `action createSummaryFile()` |

A requirement the participant meets in another way than the key's design is PARTIAL with the key's
part as the note, never CONFIRMED and never weakened to pass.

What no row observes is an `Unobserved` note: the confirmation dialog and other UI steps, the
superclass of a jar's class (the solutions rows read it from the sources), and what of a requirement
is runtime (an overwritten file, the BQL that produced the listing). CRA2.16 (`listing`) compares
`Review.txt` with the bog: it names exactly the station's NumericWritables, each at `@ def` with its
`fallback.value` as the value (RA2.V11's set writes the fallback), so the range item RA2.V10–V15 is
CONFIRMED live (every linked check reads the station); RA2.R5a and RA2.R5b stay PARTIAL.

On the live review station two rows FAIL, both real: CRA1.2 (the jar's description is
`Cristian Angeles Niagara Reviw Assignments`) and CRA1.10 (the components sit in
`Config/Cristian_Angeles_Review`, not in `Config/Review`); CRA2.11 passes with four distinct
fallbacks (111.0, 221.0, 33.0, 54.0). The counts are in [Live run](#live-run).

### Exam checks (EXAM)

The rows `CEXAM.<n>` (`internal/course/check/spec_exam.go`) read the exam station
(`COURSE_CERT_STATION`, `Cristian_Angeles_Cert`) and the jars `Cristian_Angeles_Cert-rt` and
`websiteDriver-rt`; keys are P1-P3.

| Problem | Rows |
|---|---|
| P1 Limiter | a `Limiter` in `Test`, `min = 25.0`, `max = 75.0`, the link `Ramp.out -> Limiter.in`, `out` stored, the jar signed, `BLimiter.class` |
| P2 HistoryExportService | one under Services, `historyId = /Cristian_Angeles_Cert/AuditHistory`, `shared/history.csv` with the header `Timestamp,Operation,Target,SlotName` and audit rows, `BHistoryExportService.class` |
| P3 websiteDriver | a `WebsiteDriverDevice` under Drivers with `website` stored and `health.lastOkTime` stored (nested), the jar signed, the network, device and folder classes by exact name (`BWebsiteDriver{Network,Device,DeviceFolder}.class`, the names in the live jar), `NETWORK_COMMUNICATION` |

On the live exam station every row passes. On the live TrainingStation one row FAILs, C3.4, because
the station really differs: its Lab2 folder has Point1..Point4 linked to
Add.in1..in4 and no `Output` point, so Lab 3 step 11 (`LI:576`, Add.Out -> Output.In10) was
not done.

## course verify

`course verify --status S [--lab GROUP] [--station <dir|config.bog>] [--install DIR] [--source DIR] [--labsrc DIR] [--solutions DIR] [--date YYYY-MM-DD] [--json]`
decides every item carrying status `S` (the vocabulary of `status`: `predicted`, `unverified`,
`corrected`, `identical`, `verified`, `written`) in the group `--lab` (the spellings of `status
--lab`: `3`, `L3`, `RA1`, `RA` for RA1, RA2 and RAX, `EXAM`; every group without it), with the
rules, output and exit of `verify-predictions` below; the summary starts with the status
(`written N: CONFIRMED ...`). Each linked check runs on the station of its role (the defaults
of `course check`), so `course verify --status written` with the three station variables set
decides the lab, review and exam items in one run. An unknown group is exit 1 with the group list.

On the live stations `verify --status written` contradicts three items, each from a real station
difference: L3.7 (C3.4, the missing Add.out -> Output link), RA1.M3 (CRA1.2, the description typo)
and RA1.V9–V14 (CRA1.10, no folder `Review`); its checks run on the TrainingStation and the review
station only (no written item links an exam check: the exam items are verified). L10.18 (the
NumericWritable chart has data) and L10.21 (ten histories named after their points) are CONFIRMED
from the history files (C10.12, C10.13); L10.24 is verified (C10.8-C10.10). The counts are in
[Live run](#live-run).

`course verify-predictions` is `course verify --status predicted` over every group: the same
stdout, stderr and exit (`TestVerifyPredictionsIsVerifyStatusPredicted`).

## course verify-predictions

`course verify-predictions [--station <dir|config.bog>] [--install DIR] [--source DIR] [--labsrc DIR] [--solutions DIR] [--date YYYY-MM-DD] [--json]`
reads the answer keys (it needs the root, like `show`) and the station, and decides every item
still marked `*(predicted)*` from the checks whose `Keys` resolve to it (`Corpus.Lookup`: the ID,
a range ID or a covered ID, so key `L4.15` reaches the range item `L4.15–L4.16`). A check is
linked once per item however many of its keys reach it, and its `Unobserved` note is read under
the key that matched, not the item's ID (`itemLinks`, which `TestCheckKeysReal` uses too). Only those linked checks run; the bog is read with nested values only when
one of them has a dotted slot.

| Verdict | When | Printed |
|---|---|---|
| `CONFIRMED` | every linked check PASS, and at least one observes the whole answer (no `Unobserved` note for the item) | each check's ID, verdict and evidence |
| `PARTIAL` | every linked check PASS, but each observes only the stored part | the evidence, then `not observed: ...` (the notes) |
| `CONTRADICTED` | at least one linked check FAIL | the FAIL checks and their evidence |
| `UNDECIDED` | no linked check (`no check covers it`), or a SKIP and no FAIL | the reason: the SKIP checks |

A check proves what the station and the jars store, never a console line, a job log, a Px view
or a runtime status. So a spec row may carry `Unobserved`, a map from a key to what of that
item's answer the check does not observe (C1.2 -> L1.25: `the Hello, World! line in the station
console`). `TestCheckKeysReal` fails on a link to an item the real key marks predicted that has
no note, unless the link is audited into `fullyObserved` (2026-10-17: C10.8-C10.10 ->
L10.24, whose answer is the content of the exported CSV the checks read; 2026-10-18: C1.12 ->
L1.14, whose answer is the `<type>` line of `module-include.xml` the check reads, written by the
compilation as `LI:266` says; every other predicted item a check links also predicts something no
bog, jar or file holds). L1.14 predicts `name` before `class`; the file has `class` first, which
is not significant in XML but must be fixed when the marker is applied.

L10.24 (open one CSV) is decided by the CSVs the live station exported
(`shared/TrainingStation_NumericWritable1.csv`): a UTF-8 BOM, `history:TrainingStation/NumericWritable1`,
a blank line, the header `Timestamp,Trend Flags,Status,Value`, then rows such as
`15 oct 2026 12:43:20 CST,{start},{ok},"82,9"` (the es-MX locale's decimal comma, so the value is
quoted). C10.8 (line 1), C10.9 (line 3) and C10.10 (the rows) check exactly that, so the item is
CONFIRMED; its answer text (`a header row, then timestamp / value / status rows`) predicts other
columns and no history line, and must be corrected with the verified marker.

The table `ID  VERDICT  EVIDENCE` lists the predicted items in answer-key order, then, when any
item is CONFIRMED, one ready-to-apply line per CONFIRMED item (never for PARTIAL):

    L4.13: *(verified live 2026-10-17)* evidence: C4.5 Lab4/My Tstat.diff = 2.0 (stored)
    L10.6–L10.7: *(verified offline 2026-10-17)* evidence: C10.7 modules/training-rt.jar!META-INF/module.xml has ...

The marker's evidence comes from the kinds of the item's linked checks (`check.Evidence`, the map
`evidenceOf` in `check/kinds.go`, which `TestEvidenceOfEveryKind` holds complete, and the row's root:
`checkEvidence` in `check/roots.go`):

| Evidence | Kinds | Reads |
|---|---|---|
| live | `type`, `slot`, `link` | the station's bog |
| live | `files`, `file-text` | files in the station directory (`shared/`), the default root |
| offline | `files`, `file-text` with `Root` `install` or `labsrc` | a file of the install or of the lab sources (`checkEvidence`) |
| live | `audit`, `log`, `trend` | the station's histories |
| live | `listing` | a file in the station directory compared with the bog |
| offline | `jar`, `jar-entry`, `jar-text`, `jar-signed`, `jar-cert` | a jar of the install |
| offline | `source-text` | the Niagara source tree |
| offline | `annotation` | the participant's lab sources, or the corpus solutions (`Root: solutions`) |

An item whose checks mix both is live only when every linked check is live; one offline check makes
the whole line offline, since part of the evidence was not read from the station.

`course` never edits the Markdown: a human or an agent applies the line. `--date` is the date
written in the lines (default: today, local time; the clock is injectable for tests); a value that is not `YYYY-MM-DD` is exit 2.

Summary: `predicted N: CONFIRMED c, PARTIAL p, CONTRADICTED x, UNDECIDED u; K checks run; station ...`.
With `--json`, stdout is a list of `{id, verdict, reason, checks: [{id, verdict, evidence}],
file, line, suggestion}` (`suggestion` is null unless CONFIRMED) and the summary goes to stderr.

Exit: 0 no item CONTRADICTED (PARTIAL and UNDECIDED items are counted, never confirmed); 1 at least one
CONTRADICTED, or no item carries the status in the selected groups (the honest zero
`course: no predicted item in RA1 (searched 7 items)` on stderr, as `status --status` prints it; no
station is read); 2 the answer keys or the station cannot be read, a bad `--date`, or every linked
check was skipped (nothing could be checked, as in `course check`).

On the live TrainingStation no predicted item is CONTRADICTED, and one is CONFIRMED: L1.14, offline
from the lab sources (C1.12) and the jar (C1.6) (the counts are in [Live run](#live-run)). L1.4 is
PARTIAL from the dev home (C1.9-C1.11). L4.8 is PARTIAL from C4.8 (the audited set); L7.18, L10.19 and L10.22 are
PARTIAL with evidence from the 4.15 source (C7.6-C7.8), the histories (C10.11-C10.12) and the
lexicon (C10.14). L10.24, CONFIRMED before, is marked verified live in labs.md.

Audit of the links to predicted items (2026-10-17): C3.5 (Add.out stored) no longer links L3.9
(`Output = Point1 + Point2 + Point3`): it says nothing of the Output point, which the live
station does not have (C3.4). C10.2 wants one Ramp, not ten, and links only L10.16: the lab
copies the point alone with "Keep all links" (`LI:1934-1936`), so one Ramp feeds all ten
points; the live station has one Ramp per point, which the PARTIAL note of L10.19 records as
not observed.

## course progress

`course progress [--lab GROUP] [--station <dir|config.bog>] [--install DIR] [--source DIR] [--labsrc DIR] [--solutions DIR] [--date YYYY-MM-DD] [--json]`
answers "how far am I, and what next?". It runs the `verify` loop over every item of the selected
groups (every group without `--lab`; the spellings of `status --lab`), whatever its status, with
the stations, flags and variables of `verify`, and prints one row per group and a total:

| Column | Counts |
|---|---|
| `items` | the items of the group |
| `checked` | the items at least one check links (the others are UNDECIDED: no check covers them) |
| `CONFIRMED`, `PARTIAL`, `CONTRADICTED`, `UNDECIDED` | the verdicts of `verify` |
| `live`, `offline` | the items already carrying a verified marker of that evidence (the columns of `status`) |

Then the next steps, in answer-key order (the `redo` from the live run, the `mark` from the
`predict-root` fixture with the `good` station):

    redo C3.4 (lab 3): Add.out -> NumericWritable.in10 (Output): found 0, want at least 1 (searched 18 links)
      L3.7 (labs.md:239, LI:576): Link `Add.Out → Output.In10` (`LI:576`).
    mark L4.13 (labs.md:25): *(verified live 2026-10-18)* evidence: C4.5 Lab4/My Tstat.diff = 2.0 (stored)

- `redo`: one per FAIL check, at the first item it links: the check's lab, description and
  evidence, then every selected item it links with its answer-key line, its course text (`LI:` for
  the lab instructions, else the text file's name) and its step (cut at 100 characters).
- `mark`: one per CONFIRMED item that carries no verified marker: the answer-key line to edit and
  the marker and evidence to add (the suggestion line of `verify`, with the live/offline rule
  above). PARTIAL items get no line: a check does not observe all of their answer.

No new evidence logic: the verdicts, evidence and markers are those of `verify`. The summary is
`progress N: checked c, CONFIRMED ..., UNDECIDED u; K checks run; station ...`. With `--json`,
stdout is `{groups: [{group, items, checked, confirmed, partial, contradicted, undecided,
verified_live, verified_offline}], total, actions: [{kind, id, group, text, items: [{id, source,
text, step}]}]}` and the summary goes to stderr. An action's `group` is the answer-key group of the
first item it points at (`L4`, `RA1`: the key of `--lab` and of the rows), for `redo` and `mark`
alike; only the text spells it `lab 4`.

Exit, as `verify`: 0 no item CONTRADICTED; 1 at least one CONTRADICTED item (so at least one
`redo` line), or an unknown group (with the group list); 2 the answer keys or a station cannot be
read, a missing station (the usage error of `verify`, naming the role's variable), a bad `--date`,
or every linked check skipped.

Why `progress`, not `next`: the command answers both "how far" (the table) and "what next" (the
lines); `next` would name only the second half, and `progress` reads naturally with `--lab 3`.

## course quiz

`course quiz [--lab GROUP] [--kind why|decision|trap] [-n N] [--seed S] [--answers]` draws study
questions from the answer keys; it reads no station.

| Kind | Items | Prompt | Answer |
|---|---|---|---|
| `why` | lab items whose step asks `*Why ...?*` (any case), or whose answer starts with one (the question then joins the prompt) | the step | the answer (without the leading question) |
| `decision` | the review-assignment decisions (`D<k>.<n>`, the `Why` tables) | `Why this decision? <decision>` | the `Why` cell |
| `trap` | the review-assignment traps (`T<k>.<n>`) | `What goes wrong, and what does this solution do? <trap>` | what goes wrong + this solution |

`--lab` and `--kind` narrow the pool; `-n` is the number of questions (default 5; fewer when the
pool holds fewer). The order is a Fisher-Yates shuffle driven by a PCG seeded with `--seed`, so the
same seed draws the same quiz on every platform; without `--seed` the clock gives one (below
1000000) and the header prints it. Each question prints `Q<n>. <ID> (<kind> · <section>)`, the
prompt and up to eight lines of the item's course text (the lines of `course text`, then `… more:
course text <ID>`). Without `--answers` the last line is the command that prints the same quiz
with its answers (`answers: course quiz --lab 3 --seed 1 --answers`); with it, an `answers` block
follows the questions: `A<n>. <ID> [<status>] <file>:<line>` and the answer. Non-interactive by
design (testable, no state files; spaced repetition is out of scope).

Exit: 0 at least one question drawn; 1 an unknown group, or no question of the kinds in the
groups (`course: no why, decision, trap question in RAX (searched 10 items)`); 2 the answer keys
or a text file cannot be read, a drawn item's range lies outside its text file (the message of
`course text`, before any question is printed), or `-n` below 1. The replay line shell-quotes a
`--lab` value the shell would split or expand.

## Live run

Every count of a live run lives in this one table (dated; the prose above names only the items
behind them). Inputs: the stations `TrainingStation`, `Cristian_Angeles_Review` and
`Cristian_Angeles_Cert` (Niagara 4.15), the install `Niagara-4.15.6.20`, the 4.15.6.20 source tree of
niagara-research, the participant's lab sources `N4_Labs/niagara_dev_home` (and `N4_Labs/certificates`
beside it), the corpus solutions `sources/n4-dev-course/solutions`, and the niagara-research answer
keys at `9e7c8a7eb`.

| Command (2026-10-18) | Result |
|---|---|
| `course check` (the labs) | 105 checked: PASS 104, FAIL 1 (C3.4), SKIP 0 |
| `course check --lab RA1 --lab RA2` | 31 checked: PASS 29, FAIL 2 (CRA1.2, CRA1.10) |
| `course check --lab EXAM` | 19 checked: PASS 19 |
| `course verify --status predicted` | 39: CONFIRMED 1 (L1.14, offline), PARTIAL 14, CONTRADICTED 0, UNDECIDED 24; 30 checks run |
| `course verify --status written` | 201: CONFIRMED 10 (RA2.V10–V15 live; L5.2, L5.8, L5.9, L5.17, RA1.R1, RA1.R2, RA1.R4, RA2.R3, RA2.R8 offline), PARTIAL 15, CONTRADICTED 3 (L3.7, RA1.M3, RA1.V9–V14), UNDECIDED 173; 38 checks run |
| `course verify --status identical` | 50: CONFIRMED 9 (offline), PARTIAL 7, CONTRADICTED 0, UNDECIDED 34; 16 checks run |
| `course verify --status corrected` | 10: CONFIRMED 2 (offline), UNDECIDED 8; 2 checks run |
| `course verify --status unverified` | 20: CONFIRMED 1 (live), PARTIAL 5, UNDECIDED 14; 7 checks run |
| `course verify --status verified` | 72: CONFIRMED 72 (30 live, 42 offline lines); 90 checks run |
| `course progress` | 370 items: checked 125, CONFIRMED 83, PARTIAL 39, CONTRADICTED 3, UNDECIDED 245; 155 checks run; 3 `redo` (C3.4, CRA1.2, CRA1.10), 11 `mark` (live: RA2.V10–V15; offline: L1.14, L5.2, L5.8, L5.9, L5.17, RA1.R1, RA1.R2, RA1.R4, RA2.R3, RA2.R8) |
| `course status` | verified 72: live 33, offline 39 |
| `course quiz -n 500` | 89 questions: why 34, decision 29, trap 26 |

To regenerate it, set the variables and run the commands of the table (zsh: run loops in `bash -c`):

```sh
export COURSE_TRAINING_STATION=/mnt/c/Users/<you>/Niagara4.15/tridium/stations/TrainingStation
export COURSE_REVIEW_STATION=/mnt/c/Users/<you>/Niagara4.15/tridium/stations/Cristian_Angeles_Review
export COURSE_CERT_STATION=/mnt/c/Users/<you>/Niagara4.15/tridium/stations/Cristian_Angeles_Cert
export COURSE_NIAGARA_INSTALL=/mnt/c/Niagara/Niagara-4.15.6.20
export COURSE_NIAGARA_SOURCE=~/niagara-research/niagara-help/versions/4.15.6.20/source
export COURSE_LAB_SOURCES=/mnt/c/N4-Developer-Training/Core-Class/N4_Labs/niagara_dev_home
export COURSE_SOLUTIONS=~/niagara-research/sources/n4-dev-course/solutions
export NIAGARA_RESEARCH_DIR=~/niagara-research
bash -c 'for st in predicted written identical corrected unverified verified; do
  course verify --status $st | tail -1; done'
course check | tail -1; course progress | tail -1; course status
```

Then replace the table and its date in one edit. The live tests (`TestCheckLiveStations`,
`TestVerifyPredictionsTrainingStation`, `TestRecallReal`) pin only what must not regress (the known
FAILs, no CONTRADICTED prediction, the item and status counts), not this table.

## Tests

- `internal/course/testdata/golden/`: hand-written expectations over the fixture roots
  (`root`, `dup-root`, `partial-root`, `norelated`); the help and usage cases were captured
  from the first green build and reviewed.
  `TestGolden` clears the `COURSE_*` variables first, so a developer's defaults never reach a
  case; `check-env-defaults`, `check-env-blank` and `check-flag-beats-env` set them in `cmd.json`.
  `check/stations_test.go`: the station roles and one read per station path.
- `find_test.go` and the `find-*` goldens: step and answer matches, case folding, the honest
  zero and a bad regexp.
- `root_test.go`: the discovery order (flag, variable, cwd walk, executable walk) and the
  not-found error, run from a temporary directory with the executable stubbed, so no `labs.md`
  above the real checkout can leak in.
- `check/history_test.go`: the `audit`, `log` and `trend` kinds over `.hdb` files built by
  `check/fixturehdb_test.go` (a one-page record store, a one-page fixed trend table, the layouts of
  `internal/hdbread`; numeric, boolean and enum trend tables): matches and misses, several files,
  a value range with NaN (the failure reason) and NaN left out of the distinct values, boolean and
  enum values, an unsupported value type (SKIP), no file, another record type and an undecodable
  file (SKIP), a bad regexp. The `good` and `bad` stations carry
  history files generated from the table `fixtureHistories` (`COURSE_UPDATE_FIXTURES=1 go test -run
  TestFixtureHistories`; the test fails when one drifts): `bad` audits three copies and a set without
  units, one constant trend and one out of 0..100.
- `check/sourcetext_test.go`: the `source-text` kind over the synthetic `testdata/source/` (a
  stand-in `BAbstractService.java` with only the members C7.6-C7.8 read), a variant with a fourth
  property (FAIL), no `--source` (SKIP), a missing tree or a file as the tree (SKIP), a missing
  file, a bad regexp and the read bound; `TestSourceTextReal` runs the rows on
  `COURSE_NIAGARA_SOURCE`. The `check-source-env` Go-only golden reads the tree from the variable;
  `check-source-missing` names a missing tree (SKIP), `check-source-flag-beats-env` and
  `check-source-flag-empty` pin that `--source` (even empty) wins over the variable; `check-bad`
  runs without it (SKIP).
- `check/annotation_test.go`: the `annotation` rows over the synthetic `testdata/labsrc/` (the
  annotations of the six lab classes in the module layout), a changed flag (FAIL with the declared
  and wanted facts), a missing slot or file (FAIL), an unparseable file, an undecidable flags
  expression and an enum fact on a class without one (SKIP), no `--labsrc` and a missing or file
  tree (SKIP); `TestAnnotationReal` runs the rows on `COURSE_LAB_SOURCES`. The
  `check-labsrc-flag-beats-env` and `check-labsrc-flag-empty` Go-only goldens pin that `--labsrc`
  (even empty: no tree, SKIP) wins over the variable;
  `check-good` reads the fixture tree, the other `check-*` cases run without it (SKIP).
  `internal/niagaraannot` has its own table tests and an oracle (`COURSE_NIAGARA_SOURCE`) that
  compares the parsed annotations with the generated code of a source tree (4.15.6.20: 1826
  files, 205 enums, 3699 slots).
- `check/roots_test.go`: the roots over temporary trees: an install and a dev home with
  `../certificates` beside it, `{a,b}` names, a folder name ending in `/` (and the same name without
  it, which matches no folder), `Whole` across lines (the same regexp line by line FAILs), a root not
  given or not a directory (SKIP), the offline evidence of both roots, and the Lab 5 rows over a
  profile before the lab's edits (an OU, no TSA, a local profile; a second setting in
  `system.properties`: FAIL). The fixtures `testdata/install/defaults/system.properties`,
  `testdata/certificates/` (no real password) and the Gradle files of `testdata/labsrc/` are
  synthetic; `testdata/` stands for `N4_Labs`.
- `check/jarcert_test.go`: the PKCS#7 reader over blocks built by `check/fixturecert_test.go` (a
  test-only generator: Ed25519 keys from fixed seeds, whose signatures are deterministic, so the
  certificates, the blocks and `testdata/certificates/code_signing.pem` are the same on every run and
  `TestFixtureCerts` and `TestFixtureJars` catch a drift; the SignerInfo's signature is a stand-in):
  the signer, the timestamp token, the chain (a same-named root that did not sign is not the issuer),
  a BER copy (`TestBERToDER`, and the fixture `training-rt` block is BER, as the live one), malformed
  blocks (not DER, truncated, trailing data, another content type, the signer's certificate absent, a
  token that is not a SignedData: errors), and every fact PASS and FAIL over an install built in the
  test (a self-signed jar, an unsigned one, a block that is not DER: SKIP). `FuzzParseSignedData`
  (`go test -run '^$' -fuzz=FuzzParseSignedData -fuzztime=30s -fuzzminimizetime=2s
  ./internal/course/check/`; without the last flag the display stalls while a new input is minimised).
- `check/solutions_test.go`: the solutions rows over the synthetic `testdata/solutions/` (the
  annotations and class declarations of the two review classes, no body), no tree (SKIP naming
  `--solutions`, the lab sources not a fallback), and the `extends` and `defaults` facts over
  variants (another superclass, a generic one, no extends clause: SKIP, another default, a property
  without one). `TestAnnotationReal` also runs the solutions rows when `COURSE_SOLUTIONS` is set,
  and `TestCheckLiveStations` reads it. The `check-review` golden passes `--solutions solutions`.
- `check/listing_test.go`: the `listing` kind over the mini station's one NumericWritable: CRLF and
  decimal-comma lines, a header line that does not match, a missing, an extra and a twice-listed
  name, another priority, another value, names only, no file, a nested slot read without nested
  values (SKIP) and a regexp without the name group (SKIP). `check-review` runs CRA2.16 on the review
  fixture.
- `check/rowcheck_test.go`: `validate` over every spec row and over contradictory rows (SKIP `bad
  row`), a station name with braces and glob characters (`TestStationNameBraces`), and C5.7's dname
  names (two orders pass; a duplicate C, an OU, a missing O fail). `TestListingAmbiguous` (two
  same-named NumericWritables in two folders: SKIP `ambiguous`, PASS once `Under` picks one; two
  files; an unreadable file), `TestExtendsIgnoresBraces`.
- `check/filetext_test.go`: the `file-text` kind over a BOM + CRLF trend CSV, an old-format CSV, a
  short file, a line count, no matching file, a bad regexp and a file over the read bound (SKIP).
- `check/check_test.go` and the `check-*` goldens: synthetic stations under
  `internal/course/testdata/stations/` (`good`: every check PASS; `bad`: no FirstComponent,
  default Tstat diff, no Add.out -> Output link, nine CSV files in the old header-first format, a driver Tcp Config on the
  default port, a network whose ping failed (`health.down` true, no `lastOkTime`) and a driver
  device never pinged (no `health`); `broken`:
  not a bog) and a synthetic install under `testdata/install/`, whose jars are generated from the table in
  `check/fixturejars_test.go` (`COURSE_UPDATE_FIXTURES=1 go test -run TestFixtureJars`; the test fails
  when a jar drifts). The zipped form of a bog is covered by `TestCheckZippedBog`.
  `TestCheckLiveStations` runs the rows of each role against its live station
  (`COURSE_TRAINING_STATION`, `COURSE_REVIEW_STATION`, `COURSE_CERT_STATION`, and the jars of
  `COURSE_NIAGARA_INSTALL`, the tree of `COURSE_NIAGARA_SOURCE`) and pins the known station
  differences; a role whose variable is unset skips, visibly, and a role with no row fails. The synthetic `stations/review` mirrors the live review station (components in
  `Cristian_Angeles_Review`, so CRA1.10 FAILs); the `check-review` golden runs it. `stations/cert`
  is a passing exam station (`check-exam`); `check-mixed-roles` reads three stations in one run and
  `check-missing-role` is the usage error of a role without a station. `stations/cert-bad` holds
  the exam failures (`TestExamFailPaths`: a wrong min and max, an empty historyId, history.csv rows
  in another format, a device never pinged under a pinged network, and, from an install built in
  the test, no network class and no `NETWORK_COMMUNICATION`). `TestCheckNestedNeedsNestedLoad`: a dotted check on a bog read without
  nested values is SKIP.
- `verify-status-ra`, `verify-unknown-group`, `verify-no-status`: `verify` over the `root`
  fixture's RA items with the review station.
- `check/verify_test.go` and the `verify-*` goldens: the verdict rules (`TestDecidePrediction`) and
  the command over `testdata/predict-root/` (a synthetic answer key whose predicted IDs are
  linked keys: one unlinked, one linked only to a jar check, the range item `L4.15–L4.16`
  reached through the noted key L4.15 (PARTIAL), the range `L10.6–L10.7` reached by two keys
  of C10.7 (linked once), L10.24 decided by the file-text checks, and L4.13 and L11.25 marked
  predicted so that a fully observing check yields CONFIRMED) with the `good` station (every
  linked check PASS: CONFIRMED and PARTIAL), the `bad` one (L1.25, L4.13, L11.25, L11.26 and
  L11.32 CONTRADICTED), no `--install`, the `root`
  fixture (its one linked check is a jar check: exit 2), a bad `--date` and a broken bog.
  `TestVerifyPredictionsTrainingStation` (with `COURSE_TRAINING_STATION` and
  `NIAGARA_RESEARCH_DIR`) pins only that no predicted item is CONTRADICTED, none is CONFIRMED from the station and the jars (L10.24, the one that was, is now verified; L1.14 needs the lab sources, which the test does not read)
  and some are PARTIAL on the live station. `TestCheckKeysReal` resolves every linked key in the real labs.md.
- `testdata/go/golden/` (D46, `TestGoldenGo` via `golden.RunGo`, regenerated with `GOLDEN_UPDATE=1`
  and reviewed in the diff): `verify-predictions-no-station` (the missing `--station` before a bad
  root and a bad `--date`), `verify-status-none` (the honest zero, exit 1), `verify-env-role-only`
  (only `COURSE_TRAINING_STATION` set passes the early gate: the output of `verify-good`),
  `verify-env-blank-and-role` (a blank training variable and a set review one pass the early gate,
  then the labs' role is the usage error naming `COURSE_TRAINING_STATION`), `verify-env-blank-only`
  (a blank variable alone is the plain usage error, before a bad root and `--date`) and
  `verify-predictions-none` (no `--lab`: the honest zero over every group of the synthetic
  `testdata/nopredict-root/`, whose answer key carries no predicted item).
- `progress_test.go` (with `--labsrc testdata/labsrc`: L1.14 gets a `mark` line, offline) and the
  Go-only goldens `progress-good`, `progress-bad-json` (the `bad`
  station: `redo` lines, exit 1), `progress-lab` (`--lab RA` on the review fixture) and
  `progress-no-station` (the usage error before a bad `--date`).
- `quiz_test.go` and the Go-only goldens `quiz-seed`, `quiz-answers`, `quiz-none` (the honest
  zero) and `quiz-bad-n`: the pool of the `root` fixture (L1.1 asks its why in the answer, L1.5 and
  L2.1 in the step, D1.1, T1.1), one seed one order, the clock's seed printed, the eight-line cap,
  a missing or short text file (exit 2, on a copy of the fixture) and the quoted `--lab` replay.
  `TestProgressCannotCheck` pins the two exit-2 paths of progress (a bad `--date` with a station;
  every linked check skipped), `TestProgressJSONGroup` the group key of the actions.
- `check/verify_test.go` `TestSuggestionEvidence` and `TestEvidenceOfEveryKind`: the live/offline
  rule; `key/labs_test.go` `TestVerifiedEvidence`: both markers, the counts and labels (the `root`
  fixture's LA.1 is verified offline, LA.2 live).
- `key/oracle_test.go`: the recall oracle (plain regexes over the raw files, its own range
  expansion) must equal the parsed ID set on the fixture, and on the real checkout when
  `NIAGARA_RESEARCH_DIR` is set, where the counts per group and per status are pinned; they are
  counted by `key.Counts.Add`, the counter `course status` prints.

Paths are printed relative to the root with `/`, on every OS.
