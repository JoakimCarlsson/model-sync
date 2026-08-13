// Package fireworks parses Fireworks AI's serverless pricing into the catalog
// model.
//
// Fireworks packs three rates into one cell. A cell reads
// "$3.00 / $0.30 / $15.00", which the page explains as input, cached input and
// output per million tokens, and there is one such cell per serving path:
// Standard and Priority. The same model also appears more than once under
// names that differ only by a serving suffix, as "Kimi K3", "Kimi K3 Fast" and
// "Kimi K3 US", all linking to the same model. Those are one model served
// three ways rather than three models, so the suffix becomes a dimension and
// the identifier comes from the link they share.
//
// The pricing page states nothing else about a model, but every row links to
// the model's page, and that page carries a record of it as embedded JSON: the
// context window, the display name, the description, where its weights are
// published, and flags for image input and tool use. Those pages are fetched
// too. The link is the join, so nothing has to be matched on names, and the
// three rows of a model served three ways all reach the same page.
//
// The record's quotes arrive escaped, and how many times depends on how deeply
// the page nested it, so the escaping is matched rather than undone.
//
// The record flags tool use and image input and no other capability. One more
// is stated, in the guide to it rather than against a model: Fireworks works
// an example of grammar-constrained output through one model by name and then
// says the feature is not particular to it, all its models support it. That
// sentence is matched and the capability recorded for every model that
// generates a response, which is why it needs no flag of its own. It stops
// there: the sentence says all of them and means all the models the guide is
// about, and an embedding model returns a vector, which no schema describes.
//
// Two of the page's tables are not model tables at all: they price by
// parameter count band, with rows like "4B - 16B parameters". They are rate
// cards rather than catalog entries and there is nothing to key them to, so
// they are not read.
//
// What Fireworks does not publish:
//
//   - A bound on output length, for any model. The record on a model's page
//     carries the context window and nothing about how much of it a reply may
//     take, and the pricing page states no bound at all.
//   - A context window for every model. The page of qwen3p7-plus records its
//     context length as zero and renders it as "N/A", which is Fireworks
//     stating that it has none to give rather than a page this parser fails to
//     read.
//   - Which models reason. Fireworks has a reasoning guide, and it is written
//     against a placeholder: every example calls a model named
//     "<reasoning-model>", and the three it names outright are illustrations
//     rather than a list. Reading a capability out of an example would claim
//     it for whichever model the example happened to use, so none is recorded.
//   - Anything at all about its embedding model beyond the rate. Its row on the
//     pricing page links to no model page, so it carries neither a context
//     window nor the width of the vector it returns.
package fireworks
