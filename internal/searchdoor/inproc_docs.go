package searchdoor

import (
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/help/scope"
)

// docsFind is `niagara_help [--niagara-version V] find <query> -n N [--and|--or]`
// (docsJob) as nh answers it, on the data root niagara_help.py would resolve:
// $NIAGARA_HELP_DIR, else the backend's checkout (toolDir), then the requested (or
// default) version. A version that cannot be resolved prints nh's ERROR lines
// (exit 2); a missing or stale full-text index is rebuilt first, atomically and
// with nh's notices on stderr (help/scope.OpenFulltext, decision D38); a search
// that cannot run is nh's "ERROR: ..." line (exit 2); a miss is nh's honest-zero
// block (scope.Miss, exit 1).
func docsFind(getenv envFunc, argv []string) answer {
	args, version := argv[1:], ""
	if len(args) >= 2 && args[0] == "--niagara-version" {
		version, args = args[1], args[2:]
	}
	v, flag, ok := shape(args, []string{"find", "*", "-n", "#"}, "--and", "--or")
	if !ok {
		return noAnswer
	}
	repo := getenv("NIAGARA_HELP_DIR")
	if repo == "" {
		repo = toolDir(argv[0], false)
	}
	if repo == "" { // nh Main without a niagara-help directory (exit 1)
		return exact(1, "ERROR: Cannot find niagara-help directory.\nSet NIAGARA_HELP_DIR or run from tools/ directory.\n", "")
	}
	id, root, err := scope.Resolve(repo, version, getenv)
	if err != nil { // nh prints these on stdout
		return exact(2, strings.Join(scope.VersionLines(err), "\n")+"\n", "")
	}
	mode := "and"
	if flag == "--or" {
		mode = "or"
	}
	var stderr strings.Builder
	cannotSearch := func(err error) answer { // nh's run: the ERROR line on stderr, exit 2
		return exact(2, "", stderr.String()+scope.ErrorLine(err)+"\n")
	}
	query := v[0]
	r, err := fts.SearchUsing(scope.OpenFulltext(&stderr), root, id, query, nil, mode)
	if err != nil {
		return cannotSearch(err)
	}
	if len(r.Hits) == 0 {
		lines, missErr := scope.Miss(root, "find", query, "fulltext", nil)
		if missErr != nil {
			return cannotSearch(missErr)
		}
		return exact(1, strings.Join(lines, "\n")+"\n", stderr.String())
	}
	perSource, _ := strconv.Atoi(v[1])
	return exact(0, scope.FindReport(root, query, r, perSource), stderr.String())
}
