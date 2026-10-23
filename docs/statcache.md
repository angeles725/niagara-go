# Stat-keyed caches and `internal/statcache`

niagara-go keeps several on-disk caches that are valid while the stat of some
files is unchanged. Each had grown its own copy of the same steps. The
`internal/statcache` package holds those steps once. Each cache still owns
its key type, file name, format version, invalidation rule and off switch.
None of them change.

## What is shared

Every function below is in `internal/statcache/statcache.go`, except `Sys`
(`sys_linux.go`, `sys_other.go`).

| API | What it does | Replaces |
| --- | --- | --- |
| `Stamp{Size, MtimeNs, CtimeNs, Ino}`, `Of(fi)`, `Stat(path)`, `Missing`, `Stamp.Exists` | size, mtime and ctime (Unix ns) and inode of a file (ctime and inode 0 where `Sys` reports nothing); `Stat` follows a link (`os.Stat`) and is `Missing` (`-1, -1, -1, 0`) when the path cannot be stat'ed | per-cache size+mtime structs built by hand |
| `Regular(path, minSize)` | the `Stamp` of a regular file of at least `minSize` bytes; `ok` false otherwise (a directory, a missing path, a smaller file) | "regular file of at least N bytes" checks before keying |
| `Same(path, stamp)` | `Stat(path) == stamp` and the stamp is not `Missing` | the stat-again check that keeps a result computed while the file changed out of the cache |
| `Files(paths, workers)` | the `Stat` of every path, in order; `workers > 1` stats contiguous chunks in that many goroutines, `workers < 1` is 1 | file-set stat loops (sequential, or parallel in contiguous chunks) |
| `ReadGob(path, v)` | reads the file and gob-decodes it into `v`; false when it is missing or does not decode. It checks no key or format: the caller compares its own | read + gob decode |
| `WriteGob(path, v)` | gob-encodes `v` in memory, creates the parent directory with mode 0700 when missing, then `atomicfile.WriteStream` (a temp file beside `path`, chmod 0600, rename); false, leaving nothing behind, when either step fails (`core.WriteGob` forwards here) | gob encode + atomic 0600 write |
| `Off(env)` | true when the variable is `off`, `0`, `no` or `false`, in any case, after `pycompat.Strip` | the switch parsing of modjar and the corpus-nav caches |
| `UserDir(app, name)` | `$XDG_CACHE_HOME/app/name`, else `~/.cache/app/name`; `""` when the home directory is unknown | the user cache directory of modjar and bognav |
| `TestBinary()`, `IsTestName(exe)` | the executable's base name, without `.exe`, ends in `.test` (`TestBinary` is also true when the executable cannot be found) | the "never write a user cache from a test binary" check |
| `Sys(fi)` | ctime (Unix ns), inode and device on Linux; zeros and `ok` false elsewhere | the Linux-only ctime/inode/device read (build-tagged files) |

Gob-serialized key and entry types stay in their packages: gob writes the
type and field names, so a type moved (and renamed) or a field renamed would
change the cache bytes. Each migrated caller has a `statcache_equiv_test.go`
(`statscache_equiv_test.go` in `nav/inventory`) that keeps its pre-statcache
code verbatim and checks that the current code makes the same hit, miss and
store decisions and writes the same bytes. It compares against that copy, not
a hex golden: gob numbers types per process, so fixed bytes would depend on
which tests ran first.

## The mechanisms

| Cache | Keys on | Format / switch | Shares |
| --- | --- | --- | --- |
| `nav/core` class-index.gob | resolved path and `Stamp` of the class index; only from `classCacheMinBytes` (1 MiB) | `classCacheFormat` | `Regular`, `ReadGob`, `WriteGob` (through `core.WriteGob`) |
| `nav/lib/callgraph` callgraph.gob | resolved path and `Stamp` of the xref index; only from `cacheMinBytes` (1 MiB) | `cacheFormat` | `Regular`, `ReadGob`, `WriteGob` |
| `nav/lib/classrank` class-rank.gob | ranker parameters + `Stamp` of class-index.json and xref-index.json (`Present` false when absent) | `cacheFormat`; cached only when class-index.json is `cacheMinBytes` (1 MiB) or more | `Stat`, `ReadGob`, `WriteGob` |
| `nav/inventory` stats.gob | per index file of `statsCacheMinBytes` (1 MiB) or more: resolved path and `Stamp` | `statsCacheFormat` | `Of`, `Same`, `Stat` (dropping removed files), `ReadGob`, `WriteGob` |
| `modjar` `<member hash>.gob` | absolute path and `Stamp` per jar | `cacheFormat`, `NIAGARA_GO_MODJAR_CACHE` (`Off`) | `Of`, `Same`, `ReadGob`, `WriteGob`, `Off`, `UserDir`, `TestBinary` |
| `bognav` type catalog | dir + every .java path and its whole `Stamp` (`Missing` when it cannot be stat'ed) | `catalogCacheFormat` (2), `NIAGARA_CACHE_DIR=off` only | `Files(…, 1)`, `ReadGob`, `WriteGob`, `UserDir` |
| `help/fts` fingerprint.gob | per source: label, dir, link target, every walked folder's mtime (a folder modified less than `fpRecent`, 2 s, before the walk is not trusted) and every file's size/mtime, stat'ed in parallel on purpose | `fpFormat` + `EngineVersion` | `Files(…, fpStatWorkers)` (4), `ReadGob`, `WriteGob` |
| `blockfiles` subblocks walk cache | schema, uid, the executable's stamp and the root; per walked dir its mtime/ctime/ino/dev/mode; custom binary with a CRC | `subSchema`, `CORPUS_NAV_FIND_INDEX` (`Off`) | `Sys`, `Off`, `IsTestName` |
| `corpusnav/blocksearch` find-index and term-count (tfidf-blocks) caches | sha256 of the schema, the executable's stamp and `size:mtime:ctime:ino` per file (`size:mtime` where `Sys` reports nothing, `-` when missing); custom binary | `indexSchema`, `termSchema`, `CORPUS_NAV_FIND_INDEX` (`Off`) | `Sys`, `Off` |

## Differences kept on purpose

- **The bog-nav off switch** accepts only `NIAGARA_CACHE_DIR=off`, lower-cased
  by `textfold` and not trimmed. `Off` also accepts `0`/`no`/`false` and
  trims spaces. Using `Off` would change what turns the cache off, so bognav
  keeps its own check.
- **The nh fulltext `CachePath`** ignores a `UserHomeDir` error and writes
  under `niagara-help`, not `niagara-go`. It is left as it is.
- **The find-index fingerprint string** and the walk-cache binary stamp keep
  their own encodings, because the files hash or store them. Only the
  ctime/inode read is shared.
- **The help memo's parallel stat** is the `workers` parameter of `Files`.
  Other callers pass 1.
