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
// The page's URL needs its trailing slash. Without it the site answers with its
// home page at a 200 and with no redirect, so the fetch succeeds and yields a
// document holding one table of base URLs and no model at all.
//
// A second table follows the first, and it is a schedule rather than a rate. A
// footnote dates it: from 16:00 UTC on 16 August 2026 the rates become peak and
// off-peak, with off-peak at half of peak, peak being 01:00-04:00 and
// 06:00-10:00 UTC. Those amounts are not recorded, because recording a rate that
// is not yet charged as though it were would misprice every call made before
// that date, and the six figures involved are between one and a half and four
// times the current ones. What is charged today is the first table, and after
// that date the first table is what DeepSeek will restate.
//
// What DeepSeek does not publish: a display name distinct from the identifier,
// or any modality. Its table heads each column with the identifier itself and
// states a model version beside it, which is the build the identifier
// currently resolves to rather than a name for the model.
package deepseek
