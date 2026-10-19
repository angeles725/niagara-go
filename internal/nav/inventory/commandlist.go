package inventory

import "github.com/angeles725/niagara-go/internal/nav/registry"

// statsCommands is the COMMANDS list stats prints: every registered command
// the binary runs, sorted. Python printed a hand-kept STATS_COMMAND_LIST
// (inventory/command_list.py) that lagged the registry; reading the registry
// keeps the list from naming a retired command again (intended difference,
// golden-real/stats and golden/nav-stats*).
func statsCommands() []string {
	var out []string
	for _, n := range registry.Names() {
		if c, _ := registry.Lookup(n); c.Run != nil {
			out = append(out, n)
		}
	}
	return out
}
