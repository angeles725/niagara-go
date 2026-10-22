# Stat-keyed caches and `internal/statcache`

niagara-go keeps several on-disk caches that are valid while the stat of some
files is unchanged. Each had grown its own copy of the same steps. The
`internal/statcache` package holds those steps once. Each cache still owns
its key type, file name, format version, invalidation rule and off switch.
None of them change.

## What is shared

| API | Replaces |
| --- | --- |
| `Stamp{Size, MtimeNs}`, `Stat`, `Of`, `Missing` | per-cache size+mtime structs built by hand |
| `Regular(path, minSize)` | "regular file of at least N bytes" checks before keying |
| `Same(path, stamp)` | the stat-again check that keeps a result computed while the file changed out of the cache |
| `Files(paths, workers)` | file-set stat loops (sequential, or parallel in contiguous chunks) |
| `ReadGob` / `WriteGob` | read + gob decode; gob encode + atomic 0600 write (`core.WriteGob` now forwards here) |
| `Off(env)` | the `off`/`0`/`no`/`false` switch parsing |
| `UserDir(app, name)` | `$XDG_CACHE_HOME` (else `~/.cache`)`/app/name` |
| `TestBinary`, `IsTestName` | the "never write a user cache from a test binary" check |
| `Sys(fi)` | the Linux-only ctime/inode/device read (build-tagged files) |

Gob-serialized key and entry types stay in their packages: gob writes the
type names, so moving a type would change the cache bytes.

## The mechanisms

| Cache | Keys on | Format / switch | Shares |
| --- | --- | --- | --- |
| `nav/core` class-index.gob | resolved path, size, mtime; ≥1 MiB | `classCacheFormat` | `Regular`, `ReadGob` |
| `nav/lib/callgraph` callgraph.gob | resolved path, size, mtime; ≥ min bytes | `cacheFormat` | `Regular`, `ReadGob` |
| `modjar` `<member hash>.gob` | absolute path, size, mtime per jar | `cacheFormat`, `NIAGARA_GO_MODJAR_CACHE=off` | `Same`, `ReadGob`, `Off`, `UserDir`, `TestBinary` |
| `bognav` type catalog | dir + every .java path, size, mtime (-1 missing) | `catalogCacheFormat`, `NIAGARA_CACHE_DIR=off` | `Files`, `ReadGob`, `UserDir` |
| `help/fts` fingerprint.gob | per-source folder mtimes + every file's size/mtime, stat'ed in parallel on purpose | `fpFormat` + engine | `Files(…, 4)`, `ReadGob`, `WriteGob` |
| `blockfiles` subblocks walk cache | dir mtime/ctime/ino/dev/mode, custom binary | `subSchema`, `CORPUS_NAV_FIND_INDEX=off` | `Sys`, `Off`, `IsTestName` |
| `corpusnav/blocksearch` find-index and term-count (tfidf-blocks) caches | sha256 of `size:mtime:ctime:ino` per file, custom binary | `indexSchema`, `CORPUS_NAV_FIND_INDEX=off` | `Sys`, `Off` |
| `nav/lib/classrank`, `nav/inventory` stats.gob | size, mtime | `cacheFormat` | not migrated: outside this change's edit surface; they use `core.WriteGob`, which forwards to `statcache.WriteGob` |

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
