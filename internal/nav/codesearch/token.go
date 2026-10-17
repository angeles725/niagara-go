package codesearch

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
)

// The token index (indexes/token-index.db, tokens.py): postings(token,
// file_id, line_no) joined with files(id, path, module, class_name), and a
// meta(key, value) table. Every query is the SQL text of tokens.py, so SQLite
// orders the rows the same way (ties of module, class and line included).

// TokenHit is one occurrence of a token.
type TokenHit struct {
	Class  string // files.class_name
	Module string // files.module (a submodule name)
	Path   string // files.path, relative to the corpus root
	Line   int64  // postings.line_no
}

// Tokens returns the occurrences of word, in module only when module is not
// empty, ordered by module, class and line.
func Tokens(db *sql.DB, word, module string) ([]TokenHit, error) {
	var rows *sql.Rows
	var err error
	if module != "" {
		rows, err = db.Query(`
            SELECT p.token, f.class_name, f.module, f.path, p.line_no
            FROM postings p
            JOIN files f ON p.file_id = f.id
            WHERE p.token = ? AND f.module = ?
            ORDER BY f.module, f.class_name, p.line_no
        `, word, module)
	} else {
		rows, err = db.Query(`
            SELECT p.token, f.class_name, f.module, f.path, p.line_no
            FROM postings p
            JOIN files f ON p.file_id = f.id
            WHERE p.token = ?
            ORDER BY f.module, f.class_name, p.line_no
        `, word)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TokenHit
	for rows.Next() {
		var tok string
		var h TokenHit
		if err := rows.Scan(&tok, &h.Class, &h.Module, &h.Path, &h.Line); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// SimilarTokens returns up to 10 distinct tokens containing word, ignoring
// ASCII case (LIKE '%word%'; % and _ in word are wildcards), in the order
// SQLite yields them.
func SimilarTokens(db *sql.DB, word string) ([]string, error) {
	rows, err := db.Query(`
            SELECT DISTINCT token FROM postings
            WHERE token LIKE ? COLLATE NOCASE
            LIMIT 10
        `, "%"+word+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// CountToken returns the number of occurrences of word in every module.
func CountToken(db *sql.DB, word string) (int64, error) {
	var n int64
	err := db.QueryRow("SELECT COUNT(*) FROM postings WHERE token = ?", word).Scan(&n)
	return n, err
}

// TokenMeta returns the meta table as an object (dict(fetchall()): a repeated
// key keeps its last value), the _meta block corpus.source_root reads.
func TokenMeta(db *sql.DB) (*index.Object, error) {
	rows, err := db.Query("SELECT key, value FROM meta")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	meta := index.NewObject()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		meta.Set(k, v)
	}
	return meta, rows.Err()
}

// ModuleHits is the occurrences of one module: its hits in query order and
// the number of distinct class names among them.
type ModuleHits struct {
	Module  string
	Hits    []TokenHit
	Classes int
}

// ByModule groups hits by module, the modules sorted by name (each module's
// hits keep their order).
func ByModule(hits []TokenHit) []ModuleHits {
	idx := map[string]int{}
	var out []ModuleHits
	for _, h := range hits {
		i, ok := idx[h.Module]
		if !ok {
			i = len(out)
			idx[h.Module] = i
			out = append(out, ModuleHits{Module: h.Module})
		}
		out[i].Hits = append(out[i].Hits, h)
	}
	for i := range out {
		classes := map[string]bool{}
		for _, h := range out[i].Hits {
			classes[h.Class] = true
		}
		out[i].Classes = len(classes)
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Module < out[b].Module })
	return out
}

// TopModules is the "Top modules" order of _show_module_summary: modules by
// hits, most first, ties in name order; at most n.
func TopModules(mods []ModuleHits, n int) []ModuleHits {
	top := append([]ModuleHits(nil), mods...)
	sort.SliceStable(top, func(a, b int) bool { return len(top[a].Hits) > len(top[b].Hits) })
	return top[:min(n, len(top))]
}

// FormatTokenHeader is the "  TOKEN: 'word'[ in module] (N occurrences in C
// classes, M modules)" line; classes count distinct (class, module) pairs.
func FormatTokenHeader(word, module string, hits []TokenHit) string {
	pairs, mods := map[[2]string]bool{}, map[string]bool{}
	for _, h := range hits {
		pairs[[2]string{h.Class, h.Module}] = true
		mods[h.Module] = true
	}
	scope := ""
	if module != "" {
		scope = " in " + module
	}
	return fmt.Sprintf("  TOKEN: '%s'%s (%s occurrences in %s classes, %s modules)", word, scope,
		core.Commas(int64(len(hits))), core.Commas(int64(len(pairs))), core.Commas(int64(len(mods))))
}

// TokenTableHeader is the two header lines of the compact token table.
func TokenTableHeader() [2]string {
	return [2]string{fmt.Sprintf("  %-35s  %5s  %s", "CLASS", "LINE", "MODULE"), "  " + strings.Repeat("-", 65)}
}

// FormatTokenRow is the "  Class  line  module" row of the compact table,
// the class cut to 35 code points.
func FormatTokenRow(h TokenHit) string {
	return fmt.Sprintf("  %s  %5d  %s", core.LJust(Ellipsis(h.Class, 35), 35), h.Line, h.Module)
}

// FormatModuleHits is a "Top modules" row.
func FormatModuleHits(m ModuleHits) string {
	return fmt.Sprintf("    %s  %5d hits, %4d classes", core.LJust(m.Module, 35), len(m.Hits), m.Classes)
}
