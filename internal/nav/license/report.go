package license

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
)

// This package is a library (decision D29): it prints nothing, returns typed
// rows and exports the formatters that produce the CLI lines, so module-nav's
// license commands and nsearch show the same text.

// candidateTokens are the tokens whose files may hold a licensing call.
var candidateTokens = []string{"getLicenseManager", "checkFeature", "getFeature"}

// Report is license_report: every licensing call and getLicenseManager() site
// of the candidate files of the token index.
type Report struct {
	Calls      []Call
	Sites      []Site
	Files      int      // candidate files
	Modules    int      // distinct modules of the candidate files
	Unreadable []string // candidate files that could not be read
	IndexBuilt string   // the token index timestamp ("?" when absent)
	Warning    string   // set when some (not all) candidate files could not be read
}

// Failure is an exit-2 outcome of Scan: the search could not run. The CLI
// prints "ERROR: <Message>" and, when set, "  <Hint>" on stderr.
type Failure struct{ Message, Hint string }

func (f *Failure) Error() string { return f.Message }

// unreadableIndex is the CLI's message for a database error met mid-scan.
func unreadableIndex(err error) *Failure {
	return &Failure{fmt.Sprintf("unreadable index: %v", err), "Run: ./reindex.sh"}
}

// Scan is license_report over baseDir/indexes/token-index.db and the
// organized/ corpus: the report, or the Failure that stopped it (missing
// token index or corpus, a database error, or no candidate file readable).
// When only some candidate files cannot be read the report carries Warning.
func Scan(baseDir string) (*Report, *Failure) {
	dbPath := filepath.Join(baseDir, "indexes", "token-index.db")
	if st, err := os.Stat(dbPath); err != nil || !st.Mode().IsRegular() {
		return nil, &Failure{fmt.Sprintf("token index not found: %s", dbPath), "Run: ./reindex.sh"}
	}
	db, err := index.OpenSQLite(dbPath)
	if err != nil {
		return nil, unreadableIndex(err)
	}
	defer db.Close()
	meta, err := metaTable(db)
	if err != nil {
		return nil, unreadableIndex(err)
	}
	root, message, hint := core.FindSourceRoot(meta)
	if root == "" {
		return nil, &Failure{message, hint}
	}
	type candidate struct{ path, module, class string }
	var cands []candidate
	rows, err := db.Query("SELECT DISTINCT f.path, f.module, f.class_name FROM postings p "+
		"JOIN files f ON p.file_id = f.id WHERE p.token IN (?,?,?) ORDER BY f.path",
		candidateTokens[0], candidateTokens[1], candidateTokens[2])
	if err != nil {
		return nil, unreadableIndex(err)
	}
	modules := map[string]bool{}
	for rows.Next() {
		var c candidate
		if err = rows.Scan(&c.path, &c.module, &c.class); err != nil {
			break
		}
		cands = append(cands, c)
		modules[c.module] = true
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return nil, unreadableIndex(err)
	}
	built := "?"
	if v, ok := meta.String("timestamp"); ok {
		built = v
	}
	rep := &Report{Files: len(cands), Modules: len(modules), IndexBuilt: built}
	res := newResolver(root, db)
	for _, c := range cands {
		text, ok := res.text(c.path)
		if !ok {
			rep.Unreadable = append(rep.Unreadable, c.path)
			continue
		}
		calls, sites := scanFile(text, c.path, c.module, c.class, res)
		rep.Calls = append(rep.Calls, calls...)
		rep.Sites = append(rep.Sites, sites...)
	}
	if res.err != nil {
		return nil, unreadableIndex(res.err)
	}
	if n := len(rep.Unreadable); n > 0 {
		msg := fmt.Sprintf("could not read %s of %s candidate files under %s (e.g. %s)", core.Commas(int64(n)),
			core.Commas(int64(len(cands))), root, strings.Join(rep.Unreadable[:min(3, n)], ", "))
		if n == len(cands) {
			return nil, &Failure{msg, "Check the corpus or rebuild the token index: ./reindex.sh"}
		}
		rep.Warning = msg + "; results exclude them (license-features --unresolved lists them)"
	}
	return rep, nil
}

// metaTable is dict(SELECT key, value FROM meta) in row order.
func metaTable(db *sql.DB) (*index.Object, error) {
	meta := index.NewObject()
	rows, err := db.Query("SELECT key, value FROM meta")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		meta.Set(k, v)
	}
	return meta, rows.Err()
}
