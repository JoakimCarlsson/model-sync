// Package anthropic parses Anthropic's published documentation into the
// catalog model.
//
// Nothing here is shared with any other provider, and the differences from
// OpenAI are the reason. Anthropic quotes rates as "$10 / MTok" with the
// denominator inside the cell, treats cache lifetime as a pricing dimension
// (5m writes cost more than 1h reads), names models by display name and states
// the API identifier somewhere else entirely, writes MDX rather than plain
// markdown, glues footnote digits onto values ("May 20262"), and lists two
// models in one cell when they share a rate. Its capability table is also
// transposed: models are columns and features are rows.
//
// Two documents are read:
//
//   - the model overview, whose transposed table carries the display name to
//     API identifier mapping, context windows, cutoffs and capabilities. It is
//     parsed first because the pricing tables identify models only by display
//     name.
//   - the pricing page, whose standard, batch and fast-mode tables carry the
//     rates, and whose prose carries the server-side tool prices.
package anthropic
