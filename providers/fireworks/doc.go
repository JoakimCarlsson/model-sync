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
// Two of the page's tables are not model tables at all: they price by
// parameter count band, with rows like "4B - 16B parameters". They are rate
// cards rather than catalog entries and there is nothing to key them to, so
// they are not read.
package fireworks
