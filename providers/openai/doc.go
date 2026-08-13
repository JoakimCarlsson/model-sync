// Package openai parses OpenAI's published documentation into the catalog
// model.
//
// Every string OpenAI uses lives here: its metric and unit names, its
// dimension keys, its column headers, its section and tier labels, and the
// shapes of its documents. OpenAI publishes markdown by appending .md to a doc
// URL, which makes it cheap to parse; that is a fact about OpenAI and not a
// pattern other providers follow, so the readers in this package are local to
// it and shared with nobody.
//
// Three documents shapes are read, because one provider already publishes in
// more than one:
//
//   - pricing.md, whose rates live in markdown pipe tables qualified by the
//     bare prose lines above them.
//   - the per-model pages under /api/docs/models/, which carry capabilities,
//     modalities, limits, endpoints, snapshots and rate limits.
//   - the guides, whose per-image dollar tables are raw JSX with rowSpan and
//     are invisible to a markdown table reader.
//
// Two models are priced only by reference. Daybreak Blue and Daybreak Red are
// aliases pointing at whichever frontier model the program has reached, and the
// rate table leaves their rows out, saying in a sentence beside it that an
// alias is priced as its target. Their rates are therefore taken from the
// snapshot their page names, with a note recording that they were.
//
// An embedding model records the width of its vector as the default it is and
// not as a list of the widths on offer. OpenAI exposes a dimensions parameter
// that shortens the vector to any smaller length, so there is no set of
// discrete widths to enumerate; a list would invent one. The default and the
// fact that it can be reduced are what it states, and both are recorded.
//
// What OpenAI does not publish: a rate for gpt-5.4-cyber, whose row in the rate
// table is a line of dashes, or for either open-weight gpt-oss model, which the
// tables leave out entirely. Those three are served and documented and carry no
// price for that reason.
//
// Nor does it publish a page for every model it prices. Six chat models are in
// the rate table and have no page: gpt-5-search-api, the two cyber models and
// three dated snapshots of the older GPT-4 and GPT-3.5 releases. A page is the
// only place OpenAI states a context window or an output ceiling, so those six
// carry rates and neither bound. That is OpenAI's silence rather than a document
// this parser fails to read.
//
// Nor does OpenAI publish a display name for a snapshot or a withdrawn
// model. Every model it currently sells is named on a page of its own, and the
// dated snapshots of those models and the models it has retired appear only in
// the deprecation table, which lists identifiers and dates and no names. That
// is why 72 of the 131 chat models carry none: 48 are shut down, 18 deprecated
// and the rest are snapshots.
package openai
