package typecatalog

// description is the module docstring of tools/n4-type-catalog.py, the --help description; its
// Usage lines name the installed n4-type-catalog command.
const description = `n4-type-catalog: static catalog of Niagara 4 component slot declarations (B1185 route T2, gap AM30).

WHY: a station's .bog stores only NON-DEFAULT values, and the live station serves its own type contract only
over BOX (` + "`" + `reg.loadContract` + "`" + `, B1173). This tool recovers the same information OFFLINE and per version from
Slotomatic-generated declarations that survive decompilation:

    public static final Property in8 = newProperty(Flags.READONLY, new BStatusNumeric(...), null);
    public static final Action   set = newAction(Flags.OPERATOR, BDouble.DEFAULT, null);
    public static final Topic    ev  = newTopic(Flags.SUMMARY, null);

It reads ORIGINAL javadoc source (docSource), CFR/Vineflower output (numeric flags such as ` + "`" + `(int)1026` + "`" + `) and
Procyon output (` + "`" + `public static final Property x;` + "`" + ` assigned later in a ` + "`" + `static {}` + "`" + ` block),
decodes the flag expression to an int + letter list, and emits one record per type:
{package, class, extends, properties[], actions[], topics[]}. Value expressions stay STRINGS (never evaluated).

Read-only, stdlib only. Usage:
    n4-type-catalog build <dir>... [--out catalog.json]
    n4-type-catalog show <catalog.json|dir> <BClass|pkg.BClass|module:Type|Type>
    n4-type-catalog selftest
Provenance rule: a catalog entry is ` + "`" + `[CERT]` + "`" + `-grade only for the file/line it was parsed from; inherited slots
are NOT merged (use ` + "`" + `extends` + "`" + ` to walk), and a type with no declarations is omitted.
`
