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
package deepseek
