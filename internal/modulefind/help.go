package modulefind

// description is module-find.py __doc__ (its Exit line updated for the D-I2-6 exit fixes), the
// parser description (RawDescriptionHelpFormatter).
const description = "module-find: read-only finder for a Niagara N4 module's Java SOURCE tree.\n" +
	"\n" +
	"WHY: bog-nav navigates a station's saved graph; this navigates the MODULE that defines the\n" +
	"types in it. Niagara slots/actions are declared with multi-line @NiagaraProperty /\n" +
	"@NiagaraAction annotations whose args wrap across lines, and a slot's writers are the\n" +
	"`.set(\"slot\", ...)` / setSlot(...) calls scattered across a servlet or logic class. A naive\n" +
	"grep splits on the line break, misses the paren-balanced tail (the flags= / type= that decide\n" +
	"OPERATOR-vs-hidden), and cannot follow `extends` to say a servlet's name is inherited from\n" +
	"BWebServlet. This walks the tree, joins each annotation by PAREN BALANCE (bog-audit D9 lesson),\n" +
	"and answers those questions structurally.\n" +
	"\n" +
	"Slot/action/extends/.set() extraction REUSED from build-n4-module-kit/toolbelt/bog-audit.sh\n" +
	"(kit main 3f666a0), source-dir scanning section: the paren-balanced @NiagaraProperty /\n" +
	"@NiagaraAction join, the `class X extends Y` capture, and the `.set(\"SLOT\", ...)` writer scan.\n" +
	"\n" +
	"Subcommands:\n" +
	"  slots [--type T] [--flags F] [--name RE]   @NiagaraProperty declarations (type/flags/min)\n" +
	"  actions [--name RE]                        @NiagaraAction declarations\n" +
	"  writers <slotName>                         who writes a slot: setSlot(/ .set(\"slot\",\n" +
	"  extends [--of CLASS]                       class -> superclass map (follow a chain with --of)\n" +
	"  ords [--name RE]                           ORD-shaped string literals in the source\n" +
	"  grep <regex>                               regex over class names / slot names / action names\n" +
	"  grep --source <regex>                      regex over every source line (file:line text)\n" +
	"  callers <method>                           call sites of a method\n" +
	"\n" +
	"Read-only. Prints rows; --json/--csv for machine output, --limit N (-n) caps the rows.\n" +
	"Exit: 0 found · 1 slots/actions/ords/grep/callers/extends --of matched nothing (the miss line\n" +
	"names the root and its .java count) · 2 usage · 3 unreadable root or srcB / no .java. Prunes\n" +
	"dot-dirs. stdlib only.\n"
