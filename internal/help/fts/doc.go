// Package fts is the niagara-help full-text search (fulltext.py,
// fulltext_terms.py, fulltext_index.py): the tokenizer and query parser, the
// text sources, a read-only reader of the FTS5 index the Python tool builds,
// the ranking and the snippets. Go never builds the index (decision D21):
// Open accepts it only when its stored fingerprint equals the one Python
// would compute for the current text sources.
package fts
