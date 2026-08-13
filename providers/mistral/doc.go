// Package mistral parses Mistral's model documentation into the catalog
// model.
//
// Only lifecycle is recorded, because only lifecycle is published in a form
// that can be read. Mistral's documentation is a client-rendered application:
// its pricing page carries no rates in the document it serves, its per-model
// pages carry no tables at all, and the rates appear only once a browser has
// run the page. The one thing served as markup is the deprecation table, which
// states every withdrawn model's identifier, version, the dates it was
// deprecated and retired on, and what to move to instead.
//
// Recording that and no prices is the accurate account of what Mistral
// publishes, rather than a gap in this parser.
//
// The page states the two dates in one cell with nothing between them, so they
// are separated by position: the first is the deprecation, the second the
// retirement.
package mistral
