package bognav

// description is the module docstring of tools/bognav/cli.py, the --help description.
const description = `bog-nav: read-only navigator for a Niagara N4 station config.bog (or a bare file.xml).

WHY: the four session research tools (corpus-nav, module-navigator, niagara-help, hdbread)
navigate the CORPUS; this one navigates a live STATION's saved graph. A config.bog is a ZIP
whose file.xml is a BOG-XML tree of <p> component nodes (h='handle', t='pfx:Type') plus
<p t='b:Link'> link records that live on the TARGET component and point back at a source via
sourceOrd='h:xxxx'. Grep cannot resolve h:xxxx -> a component path, cannot walk the tree, and
cannot answer "which link feeds Cuarto1.setpoint?". This does.

Grammar + handle-graph parser REUSED from build-n4-module-kit/toolbelt/bog-audit.sh
(kit main 3f666a0): TAG_RE, ga(), the Comp class, prefix_map/handle_map/handle_count,
the link_list (container_h/src_h/src_slot/tgt_slot), stack, nearest_comp(). This tool adds
parent/child tracking for tree walks and the query subcommands; it never writes.

Subcommands:
  tree [--type PFX:Type] [--depth N]      component tree (optionally filtered by type)
  types [--slots] [--grep S]              histogram of component types (characterise an unknown bog)
  slot <path|h:handle> [slotName]         a component's value slots + child components
  find --type PFX:Type [--slots]          flat list of a type; --slots also finds handle-less value wrappers
  pointcache [--device N] [--stale]       Alerton Compass DisplayService PointCache: decode ORD + freshness
  links [--to P] [--from P] [--slot NAME] links, sourceOrd h:xxxx resolved to a path
  handle <h:xxxx>                         the component at a handle + its links
  writable [--module PFX]                 classify direct value slots by external writability
  grep <regex>                            regex over component paths, types, and slot names
  diff <bogB>                             component/slot/link delta vs another bog

Alerton Compass note: Compass is a DERIVED-but-distinct Niagara. Its point universe is NOT the
Tridium control:ControlPoint model -- it lives in the DisplayService PointCache (disp:PointCacheObj,
keyed by a bacref ORD) with the value as a handle-LESS ` + "`" + `bac:*` + "`" + ` wrapper. ` + "`" + `pointcache` + "`" + ` decodes it;
` + "`" + `find --slots` + "`" + ` / ` + "`" + `types --slots` + "`" + ` stop those wrappers reading as a silent zero.

Read-only. Prints rows; pass --json for machine output. stdlib only.
Exit codes: 0 found, 1 searched and no match (find/links/grep/writable print the searched bog
+ counts) or a named component/handle not found, 2 usage error, 3 unreadable input (missing, empty, non-BOG, corrupt/unsupported ZIP,
nesting or size over the bogio limits) or python2.
`
