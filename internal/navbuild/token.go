package navbuild

import (
	"fmt"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The token index (build_token_index.py): every identifier-like token of every
// top-level source, with its file and line, in the SQLite database
// indexes/token-index.db (tables files, postings and meta). Its parity with
// Python is logical (schema text, rows in rowid order, meta), not byte-wise.

// TokenFile is the index the token builder writes.
const TokenFile = "token-index.db"

// DefaultMinTokenLen is build_token_index.py's --min-len default.
const DefaultMinTokenLen = 2

// tokenStopWords is STOP_WORDS: Java keywords, literals, common package
// segments and annotations.
var tokenStopWords = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(`abstract assert boolean break byte case catch
		char class const continue default do double
		else enum extends final finally float for
		goto if implements import instanceof int
		interface long native new package private
		protected public return short static strictfp
		super switch synchronized this throw throws
		transient try void volatile while
		true false null
		com org javax java
		Override Deprecated SuppressWarnings`) {
		m[w] = true
	}
	return m
}()

// isTokenByte is the complement of RE_TOKEN_SPLIT's [^a-zA-Z0-9_] (an ASCII
// class: every byte of a multi-byte character splits).
func isTokenByte(c byte) bool {
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// isDecompilerVar is RE_DECOMPILER_VAR ^var\d+$ for an ASCII token.
func isDecompilerVar(t string) bool {
	if len(t) < 4 || t[:3] != "var" {
		return false
	}
	for i := 3; i < len(t); i++ {
		if t[i] < '0' || t[i] > '9' {
			return false
		}
	}
	return true
}

// tokenizeLine is tokenize_line: the runs of [a-zA-Z0-9_] of at least two
// characters that are neither stop words nor decompiler variables, in order.
func tokenizeLine(line string) []string {
	var out []string
	for i := 0; i < len(line); {
		if !isTokenByte(line[i]) {
			i++
			continue
		}
		j := i
		for j < len(line) && isTokenByte(line[j]) {
			j++
		}
		if t := line[i:j]; len(t) >= 2 && !tokenStopWords[t] && !isDecompilerVar(t) {
			out = append(out, t)
		}
		i = j
	}
	return out
}

// posting is one postings row of a file.
type posting struct {
	token string
	line  int
}

// tokenizedFile is what build_index takes from one source: its line count,
// whether any line had tokens (before the --min-len filter, as in Python),
// and its postings (one per distinct token of a line).
type tokenizedFile struct {
	readable  bool
	lines     int
	hasTokens bool
	postings  []posting
}

// tokenizeFile reads path like open(errors="replace").readlines() and
// tokenizes every line.
func tokenizeFile(path string, minLen int) tokenizedFile {
	content, err := pycompat.ReadTextReplace(path)
	if err != nil {
		return tokenizedFile{}
	}
	tf := tokenizedFile{readable: true}
	seen := map[string]bool{}
	for lineNo := 1; content != ""; lineNo++ {
		line := content
		if i := strings.IndexByte(content, '\n'); i >= 0 {
			line, content = content[:i], content[i+1:]
		} else {
			content = ""
		}
		tf.lines++
		tokens := tokenizeLine(line)
		if len(tokens) == 0 {
			continue
		}
		tf.hasTokens = true
		clear(seen)
		for _, t := range tokens {
			if len(t) < minLen || seen[t] {
				continue
			}
			seen[t] = true
			tf.postings = append(tf.postings, posting{t, lineNo})
		}
	}
	return tf
}

// TokenIndex describes a finished token build (the counters stored in meta).
type TokenIndex struct {
	FilesScanned, FilesWithTokens, TotalLines, TotalPostings, UniqueTokens int
	MinTokenLen                                                            int
	InsertTime, IndexTime, Elapsed                                         time.Duration
	Source                                                                 string
	// Path is the database written.
	Path string
}

// tokenSource is one file to scan: the files row it becomes (id = its position
// in the scan list + 1, so a skipped file leaves a gap).
type tokenSource struct {
	full, rel, className, module string
}

// tokenSources is build_index's files_to_scan: the top-level files that exist.
func tokenSources(entries []classIndexEntry, organized string) []tokenSource {
	var out []tokenSource
	for _, e := range topLevelFiles(entries) {
		full := sourcePath(organized, e.Path)
		if isFile(full) {
			out = append(out, tokenSource{full, e.Path, e.Name, e.Module})
		}
	}
	return out
}

// BuildTokenIndex reads <baseDir>'s class index and the sources under
// organized and writes <baseDir>/indexes/token-index.db (build_index without
// the printed summary). minLen is --min-len.
func BuildTokenIndex(baseDir, organized string, minLen int) (*TokenIndex, error) {
	entries, err := readClassIndex(baseDir)
	if err != nil {
		return nil, err
	}
	for _, e := range topLevelFiles(entries) {
		if e.ModuleNull && isFile(sourcePath(organized, e.Path)) {
			// files.module is NOT NULL: Python dies with an IntegrityError.
			return nil, fmt.Errorf("%s: classes[%q] has a null module (files.module is NOT NULL)", ClassIndexFile, e.Name)
		}
	}
	ti := &TokenIndex{MinTokenLen: minLen, Source: organized, Path: IndexPath(baseDir, TokenFile)}
	if err := writeTokenDB(ti, tokenSources(entries, organized)); err != nil {
		return nil, fmt.Errorf("writing %s: %w", TokenFile, err)
	}
	return ti, nil
}
