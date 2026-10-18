// Package glossary is the Spanish -> English query glossary of the
// niagara-research tools (tools/lib/glossary.py until Phase 5), with the views
// the module-navigator and niagara-help copies take of the same data
// (docs/text-variants.md).
//
// The data is glossary_es_en.json, embedded, and this file is the canonical
// copy (decision D44): the niagara-research one was deleted with the Python
// tools, so no drift test compares it any more. To change the glossary, edit
// glossary_es_en.json here; testdata/fixtures.json is the Python behaviour
// recorded on the same bytes at tag pre-go-retire (testdata/gen.py), so cases
// whose keys the edit touches change their expected values with it. Because
// the copy is compiled in, the Python fallback for a missing or unreadable
// file (search without English terms, one warning) has no Go counterpart:
// Parse reports the same problems as errors.
package glossary
