// Package niagaraannot reads the Slot-o-Matic annotations of a Niagara Java source file
// (owner Integrante3): @NiagaraType, @NiagaraProperty, @NiagaraAction, @NiagaraTopic,
// @NiagaraEnum and their containers (@NiagaraProperties, @NiagaraActions, @NiagaraTopics,
// @NiagaraSlots). It reads what the author declared, not the code Slot-o-Matic generated from
// it; internal/slotomatic parses that generated code (newProperty(...)) and is a different tool.
//
// The annotation grammar is the one of javax.baja.nre.annotations (4.3.58.18 source,
// niagara-help/versions/4.3.58.18/source/nre/javax/baja/nre/annotations): every element of
// NiagaraProperty (name, type, facets, flags, defaultValue, override), NiagaraAction (name,
// parameterType, returnType, facets, flags, defaultValue, override), NiagaraTopic (name,
// eventType, facets, flags, override), NiagaraEnum (range, defaultValue) and Range (value,
// ordinal, default -1).
//
// Parse lexes the file as Java (comments, string, char and text-block literals are tokens, so
// an "@Niagara" inside them is never an annotation) and reads only the annotations written
// before the top-level type declaration: annotations of members and nested types, and the
// @Generated markers of a generated region, are not the type's.
//
// FlagLetters turns a flags expression (Flags.SUMMARY | Flags.READONLY, a number, or 0) into
// the letters Flags.encodeToString writes and the AX Slot Sheet shows ("rs"); an expression it
// cannot evaluate is an error, never a guess. Enum.Constants applies the Slot-o-Matic naming and
// numbering of a range (toActive → TO_ACTIVE; an omitted ordinal is the previous one plus 1).
package niagaraannot
