package help

import (
	"errors"

	"github.com/angeles725/niagara-go/internal/help/buildapi"
)

// Builders is the index-building side the maintenance commands run
// (buildapi.Builders). The parsers of those commands stay in this package,
// so --help reads the same with or without builders.
type Builders = buildapi.Builders

// The builders' parameters and the gate statuses help reads.
type (
	RebuildParams    = buildapi.RebuildParams
	AddVersionParams = buildapi.AddVersionParams
	ExtractParams    = buildapi.ExtractParams
)

const (
	SweepPass        = buildapi.SweepPass
	SweepUnavailable = buildapi.SweepUnavailable
)

// errNoBuilders is a maintenance command in a binary whose main did not
// plug the builders in (a wiring defect, never a user error).
var errNoBuilders = errors.New("the index builders are not linked into this binary")

var builders Builders

// UseBuilders plugs the index builders into the maintenance commands; the
// composition root calls it once before Main.
func UseBuilders(b Builders) { builders = b }

// theBuilders returns the plugged-in builders, or errNoBuilders.
func theBuilders() (Builders, error) {
	if builders == nil {
		return nil, errNoBuilders
	}
	return builders, nil
}
