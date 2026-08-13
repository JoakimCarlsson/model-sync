// Package deepseek parses DeepSeek's pricing page into the catalog model.
//
// DeepSeek publishes two models and describes them in a transposed table:
// models are columns and each row is one fact about both. The table also uses
// spanning cells, so a row can carry a section label ahead of its own label,
// and a fact shared by both models appears once rather than twice. Rows are
// therefore read from the right: the last cells are the per-model values, and
// what precedes them names the row. A row with fewer values than there are
// models states one value that applies to all of them.
//
// It separates a cache hit from a cache miss rather than charging for cache
// writes, so its cheapest input rate is fifty times below its standard one.
//
// What DeepSeek does not publish: a display name distinct from the identifier,
// or any modality. Its table heads each column with the identifier itself and
// states a model version beside it, which is the build the identifier
// currently resolves to rather than a name for the model.
package deepseek
