# course: the N4 developer-course answer keys

`course` looks up the written answers of the N4 Developer Certification course by item ID,
counts them by verification status, checks a station against the labs (`course check`), and
decides the predicted answers from that station (`course verify-predictions`).
It reads three Markdown files of a niagara-research checkout and one or two course text
files. It is new (no Python original), so its golden cases run over the synthetic tree
`internal/course/testdata/root/`.

## Packages

| Package | Holds | Prints |
|---|---|---|
| `internal/course/key` | the answer-key parser and model: `Load(root)`, `Corpus`, `Item`, `Group`, the status vocabulary, `GroupKeys` | nothing (reusable, e.g. by nsearch) |
| `internal/course/check` | the check spec (`Spec`), the kinds, the station and jar reader (`Open`, `Station.Run`), the verdicts of `verify-predictions` | nothing |
| `internal/course` | the CLI: argument parsing, root discovery, the tables, JSON and summaries | everything |

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
| `verified` | `*(verified live <YYYY-MM-DD>)*` (an item confirmed on a live station); a file-level `**Status: verified ...**` line (certification-exam.md) |
| `written` | none of the above |

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
  overlap), then with `--status` the matching IDs and steps (truncated to 80 characters).
  No match is exit 1 with the scope and counts.
- `course text ID|GROUP`: the text-file lines of the item's (or the lab's) ranges, numbered,
  with no trailing whitespace (a blank line prints only its number).
  No range is exit 1; a missing text file or a range past its end is exit 2. Requirements
  print their `Review Assignment <k>.txt` lines.

## course check

`course check [--station <dir|config.bog>] [--install DIR] [--lab N ...] [--json]` verifies what a
correct solution of each lab leaves in a station and in the install, and prints one verdict
per check. It reads the station and the jars only; it needs no niagara-research root.

- `--station`: the station directory (its `config.bog`, zipped or plain XML, and `shared/`), or
  the bog file itself (its directory is the station directory). Every selected check runs on it.
- `--install`: a Niagara install; the jar checks read `<install>/modules/<part>.jar`. Without it
  (and without `COURSE_NIAGARA_INSTALL`) every jar check is SKIP.
- `--lab`: repeatable; `3`, `03`, `L3`, `B`, `LB`, `RA1`, `RA2` (any case). A lab without checks (8)
  is exit 1. Without `--lab` only the labs run (1-11, B: the TrainingStation); `RA1` and `RA2`
  run only when named, on their own station.

Defaults (explicit flags win; a blank variable counts as unset):

| Flag | Default |
|---|---|
| `--install` | `COURSE_NIAGARA_INSTALL` |
| `--station` | per check, the variable of its station role: `COURSE_TRAINING_STATION` for the labs (1-11, B), `COURSE_REVIEW_STATION` for RA1 and RA2, `COURSE_CERT_STATION` for EXAM |

Without `--station`, a run whose checks span several roles reads each role's station once, and
the summary names every station read. A role whose variable is unset is the usage error of a
missing `--station` (`the following arguments are required: --station`, exit 2). The same
defaults apply to `verify-predictions`.

The spec is the table `Spec` in `internal/course/check/spec.go`: one row per check with an
ID (`C<lab>.<n>`), the lab, the answer-key items it confirms (`L4.13`, ...), a kind and a
one-line description. Expected defaults are cited next to each lab's rows (lab `.java`
file:line, or `organized/` for a framework type). Component types are written
`module:Type` with the module name (`training:Tstat`, `control:NumericWritable`); the bog's
own prefixes (`t:`, `tld:`) are resolved through its `m=` declarations.

| Kind | Checks | PASS when |
|---|---|---|
| `type` | components of a type, optionally under an ancestor type (`Within`) or below a folder path (`Under`, `Review2`) | the count is at least / exactly N; with `Under`, a FAIL lists where the components are instead |
| `slot` | a slot on every component of a type; its effective value is the stored value (a composite's nested `value`), else the type default. A dotted slot (`address.port`, `health.lastOkTime`) is a value below a composite slot of the component | every value equals `Want`, and every `Stored` slot is stored (`health.down = false` is also the default of a device never pinged, so C11.12 and C11.14 need `health.lastOkTime` stored); or (`Set`) every slot is stored; or (`Distinct`) the distinct values reach N |
| `link` | links `From.FromSlot -> Type.Slot`, source by handle or `slot:` ord | the count is at least / exactly N |
| `files` | files matching a glob under the station dir, `{station}` = the `stationName` slot | the count is at least / exactly N |
| `jar` | `modules/<part>.jar` | it exists |
| `jar-entry` | entries matching a glob (`**` crosses `/`) | the count reaches N |
| `jar-text` | the first entry matching a glob | it holds every listed string |
| `jar-signed` | `META-INF/*.SF` and `META-INF/*.RSA`, `.DSA` or `.EC` | both exist |
| `file-text` | every file matching a glob under the station dir (`{station}` as for `files`), read as text: a UTF-8 BOM and CRLF line ends dropped; `Match` is a Go regexp (`{station}` = the station name, quoted) | `Line` > 0: that line matches; `Line` 0: the matching lines are at least / exactly N. No matching file is FAIL |

Nested slots: the bog reader keeps only a composite slot's direct `value` and `status`, so a
dotted slot is read from `Comp.Nested`, which `bog.LoadOptions` fills only with
`bog.Options{NestedSlots: true}`. `course check` turns the option on only when a selected check
has a dotted slot (`--lab 4` reads the bog as before); a dotted check run on a bog read without
it is SKIP, never PASS on the type default. The path is from the component that owns the
composite slot: the Tcp Config of a driver network is its own component (`tcpConfig`, with a
handle), so its port is `address.port` on `trafficLightDriver:TrafficLightDriverTcpCommConfig`.

Checks per lab: 1: 7, 2: 4, 3: 5, 4: 7, 5: 3 (jars only: the station holds nothing of code
signing), 6: 6, 7: 5, 9: 7, 10: 10 (three of them `file-text` on the exported CSVs), 11: 17 (8 of them nested: C11.9-C11.16, the driver and server
Tcp Config addresses and the ping health of the network and of every driver device; C11.17
`serverEnabled` is a plain slot), B: 4. Lab 8 (ORDs) writes no code and changes nothing, so it
has none (its station steps, such as L8.15 and L8.17, happen in the Workbench UI). Appendix A
(`LA`) has none either.

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

What no row observes is an `Unobserved` note: flags, the confirmation dialog and other UI steps,
superclasses, and that the `Review2` points hold different values: the bog reader drops a
writable's `fallback` (`internal/bog/parse.go`), so the values cannot be read.

The live review station (2026-10-17, with the 4.15.6.20 install): 20 checked, 18 PASS, 2 FAIL,
both real: CRA1.2 (the jar's description is `Cristian Angeles Niagara Reviw Assignments`) and
CRA1.10 (the components sit in `Config/Cristian_Angeles_Review`, not in `Config/Review`).

The live TrainingStation (2026-10-17, with the 4.15.6.20 install): 75 checked, 74 PASS, 1 FAIL.
C3.4 fails because the station really differs: its Lab2 folder has Point1..Point4 linked to
Add.in1..in4 and no `Output` point, so Lab 3 step 11 (`LI:576`, Add.Out -> Output.In10) was
not done.

## course verify-predictions

`course verify-predictions [--station <dir|config.bog>] [--install DIR] [--date YYYY-MM-DD] [--json]`
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
no note, unless the link is audited into `fullyObserved` (2026-10-17: only C10.8-C10.10 ->
L10.24, whose answer is the content of the exported CSV the checks read; every other predicted
item a check links also predicts something no bog or jar holds).

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

`course` never edits the Markdown: a human or an agent applies the line. `--date` is the date
written in the lines (default: today, local time; the clock is injectable for tests); a value that is not `YYYY-MM-DD` is exit 2.

Summary: `predicted N: CONFIRMED c, PARTIAL p, CONTRADICTED x, UNDECIDED u; K checks run; station ...`.
With `--json`, stdout is a list of `{id, verdict, reason, checks: [{id, verdict, evidence}],
file, line, suggestion}` (`suggestion` is null unless CONFIRMED) and the summary goes to stderr.

Exit: 0 no item CONTRADICTED (PARTIAL and UNDECIDED items are counted, never confirmed); 1 at least one
CONTRADICTED; 2 the answer keys or the station cannot be read, a bad `--date`, or every linked
check was skipped (nothing could be checked, as in `course check`).

The live TrainingStation and the real answer key (2026-10-17, with the 4.15.6.20 install):
40 predicted, 1 CONFIRMED (L10.24), 9 PARTIAL (L1.14, L1.25, L4.15, L7.18, L10.19, L10.22, L11.7,
L11.26, L11.32), 0 CONTRADICTED, 30 UNDECIDED (no check covers them); 18 checks run.

Audit of the links to predicted items (2026-10-17): C3.5 (Add.out stored) no longer links L3.9
(`Output = Point1 + Point2 + Point3`): it says nothing of the Output point, which the live
station does not have (C3.4). C10.2 wants one Ramp, not ten, and links only L10.16: the lab
copies the point alone with "Keep all links" (`LI:1934-1936`), so one Ramp feeds all ten
points; the live station has one Ramp per point, which the PARTIAL note of L10.19 records as
not observed.

## Tests

- `internal/course/testdata/golden/`: hand-written expectations over the fixture roots
  (`root`, `dup-root`, `partial-root`, `norelated`); the help and usage cases were captured
  from the first green build and reviewed.
  `TestGolden` clears the `COURSE_*` variables first, so a developer's defaults never reach a
  case; `check-env-defaults`, `check-env-blank` and `check-flag-beats-env` set them in `cmd.json`.
  `check/stations_test.go`: the station roles and one read per station path.
- `root_test.go`: the discovery order (flag, variable, cwd walk, executable walk) and the
  not-found error, run from a temporary directory with the executable stubbed, so no `labs.md`
  above the real checkout can leak in.
- `check/filetext_test.go`: the `file-text` kind over a BOM + CRLF trend CSV, an old-format CSV, a
  short file, a line count, no matching file and a bad regexp.
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
  `COURSE_NIAGARA_INSTALL`) and pins the known station differences; a role whose variable is unset
  skips, visibly. The synthetic `stations/review` mirrors the live review station (components in
  `Cristian_Angeles_Review`, so CRA1.10 FAILs); the `check-review` golden runs it. `TestCheckNestedNeedsNestedLoad`: a dotted check on a bog read without
  nested values is SKIP.
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
  `NIAGARA_RESEARCH_DIR`) pins only that no predicted item is CONTRADICTED, only L10.24 is CONFIRMED
  and some are PARTIAL on the live station. `TestCheckKeysReal` resolves every linked key in the real labs.md.
- `key/oracle_test.go`: the recall oracle (plain regexes over the raw files, its own range
  expansion) must equal the parsed ID set on the fixture, and on the real checkout when
  `NIAGARA_RESEARCH_DIR` is set, where the counts per group and per status are pinned.

Paths are printed relative to the root with `/`, on every OS.
