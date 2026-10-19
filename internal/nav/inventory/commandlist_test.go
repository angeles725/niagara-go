package inventory

import (
	"reflect"
	"testing"

	"github.com/angeles725/niagara-go/internal/nav/registry"
)

// TestStatsCommandsComeFromTheRegistry: the COMMANDS list stats prints is the
// set of commands the binary runs, so it cannot name a retired command.
func TestStatsCommandsComeFromTheRegistry(t *testing.T) {
	var want []string
	for _, n := range registry.Names() {
		if c, _ := registry.Lookup(n); c.Run != nil {
			want = append(want, n)
		}
	}
	if got := statsCommands(); !reflect.DeepEqual(got, want) {
		t.Errorf("statsCommands() = %v\nwant the registry's runnable commands %v", got, want)
	}
	for _, retired := range []string{"snippet", "repl", "xref", "method"} {
		for _, n := range statsCommands() {
			if n == retired {
				t.Errorf("retired command %q is listed", retired)
			}
		}
	}
}
