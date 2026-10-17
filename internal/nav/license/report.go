package license

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

// candidateTokens are the tokens whose files may hold a licensing call.
var candidateTokens = []string{"getLicenseManager", "checkFeature", "getFeature"}

// Report is the part of license_report the ported commands use.
type Report struct {
	Calls      []Call
	Files      int      // candidate files
	Modules    int      // distinct modules of the candidate files
	Unreadable []string // candidate files that could not be read
	IndexBuilt string   // the token index timestamp ("?" when absent)
}

// Scan is license_report: the licensing calls of the corpus, or nil after an
// exit-2 error (missing token index or corpus, or no candidate file readable).
// Unreadable candidate files are named on stderr.
func Scan(env *registry.Env) *Report {
	dbPath := filepath.Join(env.BaseDir, "indexes", "token-index.db")
	if st, err := os.Stat(dbPath); err != nil || !st.Mode().IsRegular() {
		env.Error(fmt.Sprintf("token index not found: %s", dbPath), "Run: ./reindex.sh")
		return nil
	}
	db := env.Connect("token-index.db", false)
	if db == nil {
		return nil
	}
	meta := index.NewObject()
	rows, err := db.Query("SELECT key, value FROM meta")
	if err != nil {
		env.Unreadable(err)
		return nil
	}
	for rows.Next() {
		var k, v string
		if err = rows.Scan(&k, &v); err != nil {
			break
		}
		meta.Set(k, v)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		env.Unreadable(err)
		return nil
	}
	root := env.SourceRoot(meta)
	if root == "" {
		return nil
	}
	type candidate struct{ path, module, class string }
	var cands []candidate
	rows, err = db.Query("SELECT DISTINCT f.path, f.module, f.class_name FROM postings p "+
		"JOIN files f ON p.file_id = f.id WHERE p.token IN (?,?,?) ORDER BY f.path",
		candidateTokens[0], candidateTokens[1], candidateTokens[2])
	if err != nil {
		env.Unreadable(err)
		return nil
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
		env.Unreadable(err)
		return nil
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
		rep.Calls = append(rep.Calls, scanFile(text, c.path, c.module, c.class, res)...)
	}
	if res.err != nil {
		env.Unreadable(res.err)
		return nil
	}
	if n := len(rep.Unreadable); n > 0 {
		msg := fmt.Sprintf("could not read %s of %s candidate files under %s (e.g. %s)", core.Commas(int64(n)),
			core.Commas(int64(len(cands))), root, strings.Join(rep.Unreadable[:min(3, n)], ", "))
		if n == len(cands) {
			env.Error(msg, "Check the corpus or rebuild the token index: ./reindex.sh")
			return nil
		}
		fmt.Fprintf(env.Err, "WARNING: %s; results exclude them (license-features --unresolved lists them)\n", msg)
	}
	return rep
}
