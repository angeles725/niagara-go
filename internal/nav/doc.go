// Package nav is module-nav (module-navigator tools/module_nav.py): the parser built
// from the command registry, base directory detection and dispatch. Each command group
// lives in its own package under nav/ and registers itself; importing them here is
// what makes them part of the CLI.
//
// Main locates the checkout per process (--base-dir, MODULE_NAVIGATOR_DIR or
// NAV_DIR, the working directory, the executable); RunAt runs the same command
// line pinned to a given checkout (core.NewAt), so another command (nsearch)
// gets mnav's exact output in-process, concurrently for different checkouts.
//
// # Libraries and commands
//
// The packages under internal/nav split into two kinds, by path:
//
//   - Library packages live under internal/nav/lib (classrank, callgraph,
//     codesearch, drivers, license, modlist, structure). Other tools (nsearch
//     in internal/searchdoor, the builders in internal/navbuild) import them.
//     A library package never imports internal/nav/registry, this package or
//     any command package: from internal/nav it imports only core and other
//     library packages. It never uses the runtime (core.Runtime, core.New),
//     never prints (no os.Stdout, os.Stderr, fmt.Print*, log), takes explicit
//     paths and decoded indexes, returns typed rows and errors, and exports the
//     formatters of the exact lines the command prints (decision D29).
//   - Every other package under internal/nav is command-side: the command
//     groups (audits, domain, flow, inventory, lookup, security, source,
//     structure) register their commands from init through registry; rankcmd
//     and cliargs are command helpers; core is the runtime and shared helpers
//     (the only command-side package a library may import, for its helpers).
//
// TestLibrariesAreImportClean enforces the library rule.
package nav
