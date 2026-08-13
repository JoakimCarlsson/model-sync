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
// Two of the page's tables are not model tables at all: they price by
// parameter count band, with rows like "4B - 16B parameters". They are rate
// cards rather than catalog entries and there is nothing to key them to, so
// they are not read.
package fireworks
