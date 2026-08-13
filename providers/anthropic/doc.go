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
// Three documents are read, in this order, because the later ones depend on
// identifiers the earlier ones establish:
//
//   - the deprecations page, whose status table is the only place every
//     model's real API identifier appears, along with whether it is retired
//     and when. Without it a retired model is filed under a guess and looks
//     current.
//   - the model overview, whose transposed table is authoritative for current
//     models and carries context windows, cutoffs and capabilities.
//   - the pricing page, whose standard, batch and fast-mode tables carry the
//     rates, and whose prose carries the server-side tool prices. It names
//     models only by display name, which is why it is read last.
//
// What a model takes and returns is not in the comparison table. The overview
// states it once, in a sentence above the table saying that every current model
// takes text and images and returns text, and that sentence is the only place
// Anthropic states it outside the Models API, which needs a key. It is read and
// then applied after every document, with its own scope: every chat model still
// served, including the one the overview describes in prose and only the
// pricing page names. Its mention of vision is the image modality under another
// word and is not recorded twice.
package anthropic
